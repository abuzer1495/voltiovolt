// SPDX-License-Identifier: AGPL-3.0-or-later

// Package store is the vltd Store: one SQLite database (modernc.org/sqlite,
// no cgo) with embedded, numbered, forward-only migrations
// (VLT-IS-003 TASK 03-00-00-003).
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // database/sql driver "sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// DBFileName is the name of the database file in the data directory.
const DBFileName = "vltd.db"

// ErrVaultUnavailable is returned by settings writes when there is no MAC
// key (the Vault is not available).
var ErrVaultUnavailable = errors.New("store: vault not available, settings writes are not possible")

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("store: not found")

// Options are the Store options.
type Options struct {
	// Logger receives Store log entries. If nil, slog.Default is used.
	Logger *slog.Logger
	// OnMACFailure is called when a settings row has a MAC that is not
	// correct. vltd writes an audit entry here.
	OnMACFailure func(key string)
	// Settings are the known Settings. If nil, DefaultSettings is used.
	Settings []SettingDef
}

// Store is the vltd database.
type Store struct {
	db   *sql.DB
	log  *slog.Logger
	opts Options
	defs map[string]SettingDef

	mu         sync.Mutex
	macKey     []byte
	noKeyNoted bool
}

// Open opens (or makes) the database at path and applies the migrations.
// It sets WAL mode, foreign keys on and a busy timeout of 5 seconds.
func Open(ctx context.Context, path string, opts Options) (*Store, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Settings == nil {
		opts.Settings = DefaultSettings()
	}
	// Make the file with mode 0600 before SQLite opens it.
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, fmt.Errorf("store: make database file: %w", err)
		}
		f.Close()
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_txlock", "immediate")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// One connection: SQLite has one writer. This also gives a fixed order
	// for audit appends.
	db.SetMaxOpenConns(1)
	s := &Store{db: db, log: opts.Logger, opts: opts, defs: map[string]SettingDef{}}
	for _, d := range opts.Settings {
		s.defs[d.Key] = d
	}
	if err := s.check(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.ensureInstallID(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// check confirms that the file is a SQLite database and that the pragmas
// are set.
func (s *Store) check(ctx context.Context) error {
	var mode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return fmt.Errorf("store: database is not valid: %w", err)
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("store: journal mode is %q, not wal", mode)
	}
	var fk, bt int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		return fmt.Errorf("store: foreign keys not on (%v)", err)
	}
	if err := s.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&bt); err != nil || bt != 5000 {
		return fmt.Errorf("store: busy timeout not 5000 ms (%v)", err)
	}
	var ic string
	if err := s.db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&ic); err != nil || ic != "ok" {
		return fmt.Errorf("store: database is not valid: %q %v", ic, err)
	}
	return nil
}

type migration struct {
	n    int
	name string
	sql  string
}

func migrations() ([]migration, error) {
	ents, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range ents {
		name := e.Name()
		num, _, ok := strings.Cut(name, "_")
		n, err := strconv.Atoi(num)
		if !ok || err != nil || n < 1 || !strings.HasSuffix(name, ".sql") {
			return nil, fmt.Errorf("store: bad migration name %q", name)
		}
		b, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}
		out = append(out, migration{n: n, name: name, sql: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].n < out[j].n })
	for i, m := range out {
		if m.n != i+1 {
			return nil, fmt.Errorf("store: migration numbers are not continuous at %q", m.name)
		}
	}
	return out, nil
}

// SchemaVersion returns the schema version recorded in the database.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, "SELECT version FROM schema_version").Scan(&v)
	return v, err
}

func (s *Store) migrate(ctx context.Context) error {
	ms, err := migrations()
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL) STRICT"); err != nil {
		return fmt.Errorf("store: database is not valid: %w", err)
	}
	var cur int
	err = s.db.QueryRowContext(ctx, "SELECT version FROM schema_version").Scan(&cur)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := s.db.ExecContext(ctx, "INSERT INTO schema_version (version) VALUES (0)"); err != nil {
			return err
		}
	case err != nil:
		return fmt.Errorf("store: read schema version: %w", err)
	}
	if cur > len(ms) {
		return fmt.Errorf("store: schema version %d is newer than this vltd (%d); migrations are forward only", cur, len(ms))
	}
	for _, m := range ms[cur:] {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE schema_version SET version = ?", m.n); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		s.log.Info("store migration applied", "migration", m.name)
	}
	return nil
}

func (s *Store) ensureInstallID(ctx context.Context) error {
	_, err := s.GetMeta(ctx, "install_id")
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO meta (key, value) VALUES ('install_id', ?)", id)
	return err
}

// InstallID returns the 16-byte install ID from the meta table.
func (s *Store) InstallID(ctx context.Context) ([]byte, error) {
	return s.GetMeta(ctx, "install_id")
}

// GetMeta returns a meta value.
func (s *Store) GetMeta(ctx context.Context, key string) ([]byte, error) {
	var v []byte
	err := s.db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

// SetMeta sets a meta value. The install ID cannot be changed.
func (s *Store) SetMeta(ctx context.Context, key string, value []byte) error {
	if key == "install_id" {
		return errors.New("store: install_id cannot be changed")
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

// DB returns the database handle for the packages that own a table
// (audit, vault, auth).
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Now returns the time text used in the Store (UTC, RFC 3339 with nanoseconds).
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
