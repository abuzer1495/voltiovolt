// SPDX-License-Identifier: AGPL-3.0-or-later

// Package audit is the vltd audit log (VLT-IS-003 TASK 03-00-00-006).
//
// The audit table is append-only: this package has no function to change or
// delete a row, and SQLite triggers refuse UPDATE and DELETE. Each row has a
// hash: hash = SHA-256(prev_hash || canonical JSON of the row without the
// hash), as lowercase hex. prev_hash of the first row is 64 "0" characters.
package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// Actors (step 2).
const (
	ActorGUI      = "gui"
	ActorTelegram = "telegram"
	ActorSystem   = "system"
)

// GenesisHash is prev_hash of the first row.
var GenesisHash = strings.Repeat("0", 64)

// ActionChainBroken is the action of the entry for a broken chain.
const ActionChainBroken = "audit_chain_broken"

// Row is one audit row.
type Row struct {
	Seq      int64           `json:"seq"`
	TS       string          `json:"ts"`
	Actor    string          `json:"actor"`
	Action   string          `json:"action"`
	Target   string          `json:"target"`
	Result   string          `json:"result"`
	Details  json.RawMessage `json:"details"`
	PrevHash string          `json:"prev_hash"`
	Hash     string          `json:"hash"`
}

// canonicalRow is the row without the hash, in a fixed field order.
type canonicalRow struct {
	Seq      int64           `json:"seq"`
	TS       string          `json:"ts"`
	Actor    string          `json:"actor"`
	Action   string          `json:"action"`
	Target   string          `json:"target"`
	Result   string          `json:"result"`
	Details  json.RawMessage `json:"details"`
	PrevHash string          `json:"prev_hash"`
}

// Hash returns the row hash: SHA-256 of prev_hash (text) and the canonical
// JSON of the row without the hash.
func (r Row) ComputeHash() (string, error) {
	det, err := canonicalJSON(r.Details)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(canonicalRow{r.Seq, r.TS, r.Actor, r.Action, r.Target, r.Result, det, r.PrevHash})
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(r.PrevHash))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// canonicalJSON returns the JSON value with sorted object keys, no spaces
// and numbers as written.
func canonicalJSON(in json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(in)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	d := json.NewDecoder(bytes.NewReader(in))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if d.More() {
		return nil, errors.New("audit: extra data after details")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

// Log is the audit log.
type Log struct {
	db  *sql.DB
	log *slog.Logger
	now func() time.Time
	mu  sync.Mutex
}

// New returns the audit log on the Store database.
func New(st *store.Store, log *slog.Logger) *Log {
	if log == nil {
		log = slog.Default()
	}
	return &Log{db: st.DB(), log: log, now: time.Now}
}

// cleanDetails removes Secret patterns from text values. A redact.Secret
// value gives "[REDACTED]" (MarshalJSON).
func cleanDetails(details map[string]any) (json.RawMessage, error) {
	if len(details) == 0 {
		return json.RawMessage(`{}`), nil
	}
	c := make(map[string]any, len(details))
	for k, v := range details {
		switch x := v.(type) {
		case string:
			c[k] = redact.Scrub(x)
		case redact.Secret, *redact.Secret:
			c[k] = redact.Redacted
		case error:
			c[k] = redact.Scrub(x.Error())
		default:
			c[k] = v
		}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	b = []byte(redact.Scrub(string(b)))
	return canonicalJSON(b)
}

// Record appends one row. It implements the Auditor interfaces of the
// vault and auth packages.
func (l *Log) Record(ctx context.Context, actor, action, target, result string, details map[string]any) error {
	switch actor {
	case ActorGUI, ActorTelegram, ActorSystem:
	default:
		return fmt.Errorf("audit: unknown actor %q", actor)
	}
	if action == "" {
		return errors.New("audit: empty action")
	}
	det, err := cleanDetails(details)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r := Row{Seq: 1, PrevHash: GenesisHash, TS: l.now().UTC().Format(time.RFC3339Nano),
		Actor: actor, Action: action, Target: target, Result: result, Details: det}
	var lastSeq int64
	var lastHash string
	err = tx.QueryRowContext(ctx, "SELECT seq, hash FROM audit ORDER BY seq DESC LIMIT 1").Scan(&lastSeq, &lastHash)
	switch {
	case err == nil:
		r.Seq, r.PrevHash = lastSeq+1, lastHash
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	if r.Hash, err = r.ComputeHash(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit (seq, ts, actor, action, target, result, details, prev_hash, hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, r.Seq, r.TS, r.Actor, r.Action, r.Target, r.Result, string(r.Details), r.PrevHash, r.Hash); err != nil {
		return err
	}
	return tx.Commit()
}

// MaxPage is the maximum number of rows that List returns.
const MaxPage = 500

// List returns rows with seq > after, in seq order, at most limit rows
// (1 to MaxPage).
func (l *Log) List(ctx context.Context, after int64, limit int) ([]Row, error) {
	if limit <= 0 || limit > MaxPage {
		limit = MaxPage
	}
	rows, err := l.db.QueryContext(ctx, `SELECT seq, ts, actor, action, target, result, details, prev_hash, hash
		FROM audit WHERE seq > ? ORDER BY seq LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		var r Row
		var det string
		if err := rows.Scan(&r.Seq, &r.TS, &r.Actor, &r.Action, &r.Target, &r.Result, &det, &r.PrevHash, &r.Hash); err != nil {
			return nil, err
		}
		r.Details = json.RawMessage(det)
		out = append(out, r)
	}
	return out, rows.Err()
}

// VerifyResult is the result of a chain verification.
type VerifyResult struct {
	OK       bool   `json:"ok"`
	Rows     int64  `json:"rows"`
	BrokenAt int64  `json:"broken_at,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Verify checks the full chain: continuous seq from 1, prev_hash equal to
// the hash of the previous row, and each hash.
func (l *Log) Verify(ctx context.Context) (VerifyResult, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT seq, ts, actor, action, target, result, details, prev_hash, hash FROM audit ORDER BY seq`)
	if err != nil {
		return VerifyResult{}, err
	}
	defer rows.Close()
	res := VerifyResult{OK: true}
	prev, want := GenesisHash, int64(1)
	for rows.Next() {
		var r Row
		var det string
		if err := rows.Scan(&r.Seq, &r.TS, &r.Actor, &r.Action, &r.Target, &r.Result, &det, &r.PrevHash, &r.Hash); err != nil {
			return VerifyResult{}, err
		}
		r.Details = json.RawMessage(det)
		res.Rows++
		if !res.OK {
			continue
		}
		fail := func(reason string) { res.OK, res.BrokenAt, res.Reason = false, r.Seq, reason }
		switch {
		case r.Seq != want:
			fail(fmt.Sprintf("seq %d, want %d (row missing)", r.Seq, want))
		case r.PrevHash != prev:
			fail("prev_hash does not match the previous row")
		default:
			h, err := r.ComputeHash()
			if err != nil || h != r.Hash {
				fail("hash is not correct (row changed)")
			}
		}
		prev, want = r.Hash, r.Seq+1
	}
	return res, rows.Err()
}

// VerifyAtStart verifies the full chain (step 4). If the chain is broken,
// it writes a log entry and an audit entry with the action
// "audit_chain_broken". vltd continues.
func (l *Log) VerifyAtStart(ctx context.Context) VerifyResult {
	res, err := l.Verify(ctx)
	if err != nil {
		l.log.Error("audit chain verification failed", "err", err)
		res = VerifyResult{OK: false, Reason: "verification error"}
	}
	if res.OK {
		l.log.Info("audit chain verified", "rows", res.Rows)
		return res
	}
	l.log.Error("audit chain broken", "seq", res.BrokenAt, "reason", res.Reason)
	if err := l.Record(ctx, ActorSystem, ActionChainBroken, "audit", "failure", map[string]any{"seq": res.BrokenAt, "reason": res.Reason}); err != nil {
		l.log.Error("audit write failed", "err", err)
	}
	return res
}
