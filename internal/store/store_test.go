// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func openTest(t *testing.T, dir string, opts Options) *Store {
	t.Helper()
	if opts.Logger == nil {
		opts.Logger = quiet()
	}
	s, err := Open(context.Background(), filepath.Join(dir, DBFileName), opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrationFromEmpty(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir, Options{})
	ms, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	latest := len(ms)
	v, err := s.SchemaVersion(ctx)
	if err != nil || v != latest {
		t.Fatalf("schema version = %d, %v; want %d", v, err, latest)
	}
	for _, tbl := range []string{"meta", "settings", "secrets", "measurements", "events", "audit", "auth_state", "schema_version"} {
		var n int
		if err := s.DB().QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s: %d %v", tbl, n, err)
		}
	}
	id, err := s.InstallID(ctx)
	if err != nil || len(id) != 16 {
		t.Fatalf("install ID: %x %v", id, err)
	}
	s.Close()
	// A second open does not change the install ID or the version.
	s2 := openTest(t, dir, Options{})
	id2, _ := s2.InstallID(ctx)
	if !bytes.Equal(id, id2) {
		t.Fatal("install ID changed")
	}
	if v, _ := s2.SchemaVersion(ctx); v != latest {
		t.Fatalf("schema version after reopen = %d", v)
	}
	if err := s2.SetMeta(ctx, "install_id", []byte("x")); err == nil {
		t.Fatal("install ID must not change")
	}
}

func TestPragmas(t *testing.T) {
	s := openTest(t, t.TempDir(), Options{})
	ctx := context.Background()
	var mode string
	var fk, bt int
	s.DB().QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode)
	s.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk)
	s.DB().QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&bt)
	if mode != "wal" || fk != 1 || bt != 5000 {
		t.Fatalf("journal_mode=%s foreign_keys=%d busy_timeout=%d", mode, fk, bt)
	}
}

func TestInvalidDatabase(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, DBFileName)
	if err := os.WriteFile(p, bytes.Repeat([]byte("not a database "), 300), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), p, Options{Logger: quiet()}); err == nil {
		t.Fatal("Open of a file that is not a database must fail")
	}
}

func TestNewerSchemaRefused(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir, Options{})
	if _, err := s.DB().Exec("UPDATE schema_version SET version = 99"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	_, err := Open(context.Background(), filepath.Join(dir, DBFileName), Options{Logger: quiet()})
	if err == nil || !strings.Contains(err.Error(), "forward only") {
		t.Fatalf("err = %v", err)
	}
}

func TestSettingsMAC(t *testing.T) {
	ctx := context.Background()
	var failed []string
	s := openTest(t, t.TempDir(), Options{OnMACFailure: func(k string) { failed = append(failed, k) }})

	// No MAC key: reads give the default, writes are refused.
	st, err := s.GetSetting(ctx, SettingSessionIdleMinutes)
	if err != nil || !st.Default || string(st.Value) != "10" {
		t.Fatalf("default: %+v %v", st, err)
	}
	if _, err := s.SetSetting(ctx, SettingSessionIdleMinutes, json.RawMessage(`15`), "gui"); !errors.Is(err, ErrVaultUnavailable) {
		t.Fatalf("write without key: %v", err)
	}

	key := bytes.Repeat([]byte{7}, 32)
	s.SetMACKey(key)
	v, err := s.SetSetting(ctx, SettingSessionIdleMinutes, json.RawMessage(` 15 `), "gui")
	if err != nil || v != 1 {
		t.Fatalf("set: %d %v", v, err)
	}
	if v, _ := s.SetSetting(ctx, SettingSessionIdleMinutes, json.RawMessage(`20`), "gui"); v != 2 {
		t.Fatalf("version = %d, want 2", v)
	}
	st, _ = s.GetSetting(ctx, SettingSessionIdleMinutes)
	if st.Default || string(st.Value) != "20" || st.Version != 2 || st.UpdatedBy != "gui" {
		t.Fatalf("read: %+v", st)
	}
	// Bad values and unknown keys are refused.
	for _, bad := range []string{`0`, `500`, `"x"`, `{}`} {
		if _, err := s.SetSetting(ctx, SettingSessionIdleMinutes, json.RawMessage(bad), "gui"); err == nil {
			t.Errorf("value %s accepted", bad)
		}
	}
	if _, err := s.SetSetting(ctx, "no.such", json.RawMessage(`1`), "gui"); err == nil {
		t.Error("unknown key accepted")
	}

	// Change the row in the database: the MAC is not correct.
	if _, err := s.DB().Exec("UPDATE settings SET value = '120' WHERE key = ?", SettingSessionIdleMinutes); err != nil {
		t.Fatal(err)
	}
	st, _ = s.GetSetting(ctx, SettingSessionIdleMinutes)
	if !st.Default || string(st.Value) != "10" {
		t.Fatalf("changed row was used: %+v", st)
	}
	if len(failed) != 1 || failed[0] != SettingSessionIdleMinutes {
		t.Fatalf("OnMACFailure calls: %v", failed)
	}
	// A changed version also fails.
	s.SetSetting(ctx, SettingLanguage, json.RawMessage(`"tr"`), "gui")
	s.DB().Exec("UPDATE settings SET version = 9 WHERE key = ?", SettingLanguage)
	if st, _ := s.GetSetting(ctx, SettingLanguage); !st.Default {
		t.Fatal("changed version was used")
	}
	// A different key fails.
	s.SetSetting(ctx, SettingLanguage, json.RawMessage(`"tr"`), "gui")
	s.SetMACKey(bytes.Repeat([]byte{8}, 32))
	if st, _ := s.GetSetting(ctx, SettingLanguage); !st.Default {
		t.Fatal("row with a different MAC key was used")
	}
}

func TestRekeySettings(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir(), Options{})
	s.SetMACKey(bytes.Repeat([]byte{1}, 32))
	s.SetSetting(ctx, SettingLanguage, json.RawMessage(`"tr"`), "gui")
	if err := s.RekeySettings(ctx, bytes.Repeat([]byte{2}, 32)); err != nil {
		t.Fatal(err)
	}
	st, _ := s.GetSetting(ctx, SettingLanguage)
	if st.Default || string(st.Value) != `"tr"` {
		t.Fatalf("after rekey: %+v", st)
	}
}

func TestHint(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"short":                 "",
		"123456789012345":       "",
		"1234567890123456":      "3456",
		"abcdefghijklmnopqrstu": "rstu",
		"ççççççççççççğüşı":      "ğüşı",
	}
	for in, want := range cases {
		if got := Hint([]byte(in)); got != want {
			t.Errorf("Hint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSecretsRows(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir(), Options{})
	r := SecretRow{ID: "a", Version: 1, Nonce: []byte{1}, Ciphertext: []byte{2}, Hint: "", UpdatedAt: Now()}
	if err := s.PutSecret(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSecret(ctx, "a")
	if err != nil || got.Version != 1 {
		t.Fatal(got, err)
	}
	if err := s.DeleteSecret(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSecret(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.GetSecret(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestAuditAppendOnly(t *testing.T) {
	s := openTest(t, t.TempDir(), Options{})
	db := s.DB()
	if _, err := db.Exec("INSERT INTO audit (ts, actor, action, target, result, details, prev_hash, hash) VALUES ('t','system','a','b','ok','{}','p','h')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE audit SET result = 'x'"); err == nil {
		t.Fatal("UPDATE on audit must fail")
	}
	if _, err := db.Exec("DELETE FROM audit"); err == nil {
		t.Fatal("DELETE on audit must fail")
	}
	if _, err := db.Exec("INSERT INTO audit (ts, actor, action, target, result, details, prev_hash, hash) VALUES ('t','root','a','b','ok','{}','p','h')"); err == nil {
		t.Fatal("unknown actor accepted")
	}
}
