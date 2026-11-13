// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// testParams are small Argon2id parameters for fast tests only.
var testParams = Params{Time: 1, MemoryKiB: 64, Threads: 1, SaltLen: 16, KeyLen: 32}

type testOnlyAuditor struct {
	mu   sync.Mutex
	rows []string
}

func (a *testOnlyAuditor) Record(_ context.Context, actor, action, target, result string, d map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, action+":"+result)
	return nil
}

func (a *testOnlyAuditor) count(s string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, r := range a.rows {
		if r == s {
			n++
		}
	}
	return n
}

func newStore(t *testing.T) *store.Store {
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), store.DBFileName), store.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestValidatePIN(t *testing.T) {
	good := []string{"7a3k9z", "Xk29pq", "839201", "correcthorse42", strings.Repeat("a1", 32)}
	for _, p := range good {
		if err := ValidatePIN(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	bad := map[string]error{
		"12345":                  ErrPINLength,
		strings.Repeat("a1", 33): ErrPINLength,
		"12 3456":                ErrPINChars,
		"çok1234":                ErrPINChars,
		"abc-123":                ErrPINChars,
		"000000":                 ErrPINWeak,
		"AAAAAAA":                ErrPINWeak,
		"aaAAaa":                 ErrPINWeak,
		"123456":                 ErrPINWeak,
		"654321":                 ErrPINWeak,
		"1234567890":             ErrPINWeak,
		"9876543210":             ErrPINWeak,
		"abcdef":                 ErrPINWeak,
		"FEDCBA":                 ErrPINWeak,
		"Password":               ErrPINWeak,
		"123123":                 ErrPINWeak,
		"QWERTY":                 ErrPINWeak,
	}
	for p, want := range bad {
		if err := ValidatePIN(p); !errors.Is(err, want) {
			t.Errorf("%q: err = %v, want %v", p, err, want)
		}
	}
	for _, c := range commonPINs {
		if err := ValidatePIN(c); !errors.Is(err, ErrPINWeak) {
			t.Errorf("common PIN %q accepted", c)
		}
	}
}

func TestHashDefaultParams(t *testing.T) {
	if DefaultParams != (Params{Time: 3, MemoryKiB: 65536, Threads: 4, SaltLen: 16, KeyLen: 32}) {
		t.Fatal("parameters differ from VLT-IS-003 TASK 03-00-00-005 step 3")
	}
	h, err := Hash("7a3k9z", DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("PHC string %q", h)
	}
	parts := strings.Split(h, "$")
	if s, _ := b64.DecodeString(parts[4]); len(s) != 16 {
		t.Fatal("salt is not 16 bytes")
	}
	if k, _ := b64.DecodeString(parts[5]); len(k) != 32 {
		t.Fatal("key is not 32 bytes")
	}
	if ok, err := Verify("7a3k9z", h); !ok || err != nil {
		t.Fatalf("verify: %v %v", ok, err)
	}
	if ok, _ := Verify("7a3k9y", h); ok {
		t.Fatal("wrong PIN accepted")
	}
	h2, _ := Hash("7a3k9z", DefaultParams)
	if h2 == h {
		t.Fatal("same salt for two hashes")
	}
	for _, bad := range []string{"", "$argon2i$v=19$m=65536,t=3,p=4$x$y", "$argon2id$v=19$m=99999999,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", strings.Replace(h, "v=19", "v=16", 1)} {
		if _, err := Verify("7a3k9z", bad); !errors.Is(err, ErrBadHash) {
			t.Errorf("Verify(%q): err = %v", bad, err)
		}
	}
}

func newManager(t *testing.T, st *store.Store, a Auditor, now *time.Time) *Manager {
	m := New(st, a, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.params = testParams
	m.now = func() time.Time { return *now }
	return m
}

func TestLockout(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	au := &testOnlyAuditor{}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	m := newManager(t, st, au, &now)

	if _, err := m.Login(ctx, srcA, "7a3k9z"); !errors.Is(err, ErrNoPIN) {
		t.Fatalf("no PIN: %v", err)
	}
	if err := m.SetPIN(ctx, "gui", "123456"); !errors.Is(err, ErrPINWeak) {
		t.Fatalf("weak PIN: %v", err)
	}
	if err := m.SetInitialPIN(ctx, "gui", "7a3k9z"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetInitialPIN(ctx, "gui", "9q8w7e"); !errors.Is(err, ErrPINIsSet) {
		t.Fatalf("second initial PIN: %v", err)
	}
	if _, err := m.Login(ctx, srcA, "7a3k9z"); err != nil {
		t.Fatalf("correct PIN: %v", err)
	}
	// 4 failures: no block.
	for i := 0; i < 4; i++ {
		if u, err := m.Login(ctx, srcA, "wrong0"); !errors.Is(err, ErrIncorrect) || !u.IsZero() {
			t.Fatalf("failure %d: %v %v", i+1, u, err)
		}
	}
	// 5th failure: block 5 minutes.
	u, err := m.Login(ctx, srcA, "wrong0")
	if !errors.Is(err, ErrIncorrect) || !u.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("5th failure: %v %v", u, err)
	}
	// During the block, the correct PIN is refused.
	if _, err := m.Login(ctx, srcA, "7a3k9z"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("during block: %v", err)
	}
	// A restart (new Manager on the same Store) does not reset the counter.
	m = newManager(t, st, au, &now)
	if s, _ := m.Status(ctx, srcA); s.Failures != 5 || !s.BlockUntil.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("after restart: %+v", s)
	}
	if _, err := m.Login(ctx, srcA, "7a3k9z"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("after restart: %v", err)
	}
	// Each new failure doubles the block time: 10, 20, 40 ... maximum 24 h.
	want := []time.Duration{10, 20, 40, 80, 160, 320, 640, 1280}
	for _, w := range want {
		now = now.Add(25 * time.Hour)
		u, err := m.Login(ctx, srcA, "wrong0")
		if !errors.Is(err, ErrIncorrect) || u.Sub(now) != w*time.Minute {
			t.Fatalf("block %v: got %v (%v)", w, u.Sub(now), err)
		}
	}
	now = now.Add(25 * time.Hour)
	if u, _ := m.Login(ctx, srcA, "wrong0"); u.Sub(now) != 24*time.Hour {
		t.Fatalf("maximum block: %v", u.Sub(now))
	}
	// After the block, the correct PIN resets the counter.
	now = now.Add(25 * time.Hour)
	if _, err := m.Login(ctx, srcA, "7a3k9z"); err != nil {
		t.Fatal(err)
	}
	if s, _ := m.Status(ctx, srcA); s.Failures != 0 || !s.BlockUntil.IsZero() {
		t.Fatalf("after correct PIN: %+v", s)
	}
	// Audit entries: PIN set, correct entries, incorrect entries, blocks.
	if au.count("pin_set:success") != 1 || au.count("pin_entry:success") != 2 || au.count("pin_entry:failure") != 14 ||
		au.count("pin_block:success") != 10 || au.count("pin_entry:blocked") != 2 || au.count("pin_set:rejected") != 2 {
		t.Fatalf("audit rows: %v", au.rows)
	}
}

var (
	srcA = Source{Actor: "gui", Peer: "uid:1000"}
	srcB = Source{Actor: "gui", Peer: "uid:1001"}
)

// TestLockoutPerSource is VLT-IS-004 TASK 03-00-00-001 step 1.
func TestLockoutPerSource(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	m := newManager(t, st, &testOnlyAuditor{}, &now)
	if err := m.SetInitialPIN(ctx, "gui", "7a3k9z"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxFailures; i++ {
		m.Login(ctx, srcA, "wrong0")
	}
	// 1. Source A is blocked.
	if _, err := m.Login(ctx, srcA, "7a3k9z"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("source A: %v", err)
	}
	// 2. Source B can still log in.
	if _, err := m.Login(ctx, srcB, "7a3k9z"); err != nil {
		t.Fatalf("source B: %v", err)
	}
	if s, _ := m.Status(ctx, srcB); s.Failures != 0 || !s.BlockUntil.IsZero() {
		t.Fatalf("source B status: %+v", s)
	}
	// A different actor with the same peer is a different source.
	if _, err := m.Login(ctx, Source{Actor: "telegram", Peer: "uid:1000"}, "7a3k9z"); err != nil {
		t.Fatalf("other actor: %v", err)
	}
	// 3. A restart does not reset the block of source A.
	m = newManager(t, st, &testOnlyAuditor{}, &now)
	if _, err := m.Login(ctx, srcA, "7a3k9z"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("source A after restart: %v", err)
	}
	if _, err := m.Login(ctx, srcB, "7a3k9z"); err != nil {
		t.Fatalf("source B after restart: %v", err)
	}
}

// TestSecurityAlert: more than 100 failures of all sources in one hour
// give one SECURITY_ALERT audit entry (QI-2).
func TestSecurityAlert(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	au := &testOnlyAuditor{}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	m := newManager(t, st, au, &now)
	if err := m.SetInitialPIN(ctx, "gui", "7a3k9z"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < AlertFailures; i++ {
		m.Login(ctx, Source{Actor: "gui", Peer: fmt.Sprintf("uid:%d", 2000+i)}, "wrong0")
		now = now.Add(30 * time.Second)
	}
	if n := au.count(ActionAlert + ":failure"); n != 0 {
		t.Fatalf("alert after 100 failures: %d", n)
	}
	now = time.Date(2026, 11, 10, 12, 59, 0, 0, time.UTC)
	m.Login(ctx, Source{Actor: "gui", Peer: "uid:9999"}, "wrong0")
	m.Login(ctx, Source{Actor: "gui", Peer: "uid:9998"}, "wrong0")
	if n := au.count(ActionAlert + ":failure"); n != 1 {
		t.Fatalf("alerts after 102 failures in one hour: %d", n)
	}
	// A new window starts after one hour.
	now = now.Add(2 * time.Hour)
	for i := 0; i < 50; i++ {
		m.Login(ctx, Source{Actor: "gui", Peer: fmt.Sprintf("uid:%d", 5000+i)}, "wrong0")
	}
	if n := au.count(ActionAlert + ":failure"); n != 1 {
		t.Fatalf("alerts in a new window: %d", n)
	}
}

func TestBlockFor(t *testing.T) {
	cases := map[int]time.Duration{0: 0, 4: 0, 5: 5 * time.Minute, 6: 10 * time.Minute, 13: 1280 * time.Minute, 14: 24 * time.Hour, 100: 24 * time.Hour}
	for n, w := range cases {
		if got := blockFor(n); got != w {
			t.Errorf("blockFor(%d) = %v, want %v", n, got, w)
		}
	}
}

func TestSession(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	idle := 10 * time.Minute
	var s Session
	if s.Elevated(t0, idle) {
		t.Fatal("new session is elevated")
	}
	s.Elevate(t0)
	if !s.Elevated(t0.Add(9*time.Minute), idle) {
		t.Fatal("not elevated after 9 minutes")
	}
	if s.Elevated(t0.Add(10*time.Minute), idle) {
		t.Fatal("elevated after 10 minutes without a request")
	}
	// Requests keep the state, but not for more than 8 hours.
	s.Elevate(t0)
	for m := 5; m < 8*60; m += 5 {
		now := t0.Add(time.Duration(m) * time.Minute)
		if !s.Elevated(now, idle) {
			t.Fatalf("not elevated at %d minutes", m)
		}
		s.Touch(now)
	}
	if s.Elevated(t0.Add(8*time.Hour), idle) {
		t.Fatal("elevated after 8 hours")
	}
	s.Elevate(t0)
	s.Drop()
	if s.Elevated(t0, idle) {
		t.Fatal("elevated after Drop")
	}
}

func TestRemovePIN(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	au := &testOnlyAuditor{}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	m := newManager(t, st, au, &now)
	if _, err := m.RemovePIN(ctx, srcA, "7a3k9z"); !errors.Is(err, ErrNoPIN) {
		t.Fatalf("no PIN: %v", err)
	}
	if err := m.SetInitialPIN(ctx, "gui", "7a3k9z"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RemovePIN(ctx, srcA, "wrong0"); !errors.Is(err, ErrIncorrect) {
		t.Fatalf("wrong PIN: %v", err)
	}
	if set, err := m.PINSet(ctx); err != nil || !set {
		t.Fatalf("still set after wrong PIN: %v %v", set, err)
	}
	if _, err := m.RemovePIN(ctx, srcA, "7a3k9z"); err != nil {
		t.Fatal(err)
	}
	if set, err := m.PINSet(ctx); err != nil || set {
		t.Fatalf("PIN still set: %v %v", set, err)
	}
	if au.count("pin_remove:success") != 1 {
		t.Fatalf("audit count: %d", au.count("pin_remove:success"))
	}
	// After remove, SetInitialPIN works again.
	if err := m.SetInitialPIN(ctx, "gui", "9q8w7e"); err != nil {
		t.Fatal(err)
	}
}

func TestClearPIN(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	au := &testOnlyAuditor{}
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	m := newManager(t, st, au, &now)
	if err := m.ClearPIN(ctx, "gui"); !errors.Is(err, ErrNoPIN) {
		t.Fatalf("no PIN: %v", err)
	}
	if err := m.SetInitialPIN(ctx, "gui", "7a3k9z"); err != nil {
		t.Fatal(err)
	}
	if err := m.ClearPIN(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	if set, err := m.PINSet(ctx); err != nil || set {
		t.Fatalf("PIN still set: %v %v", set, err)
	}
	if au.count("pin_forgot:success") != 1 {
		t.Fatalf("audit count: %d", au.count("pin_forgot:success"))
	}
	if err := m.SetInitialPIN(ctx, "gui", "9q8w7e"); err != nil {
		t.Fatal(err)
	}
}
