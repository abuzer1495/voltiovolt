// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/auth"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// testOnlyDEKProvider is a fake host binding (SEC-091: only in _test.go).
type testOnlyDEKProvider struct{ dek []byte }

func (p *testOnlyDEKProvider) Load(context.Context) ([]byte, string, error) {
	return append([]byte(nil), p.dek...), vault.KeyModeHostAndTPM2, nil
}

func (p *testOnlyDEKProvider) Create(context.Context) ([]byte, string, error) {
	return nil, "", errors.New("cannot create")
}

func (p *testOnlyDEKProvider) CanCreate() bool { return false }

// pipeListener is an in-memory net.Listener (net.Pipe) for tests.
type pipeListener struct {
	ch     chan net.Conn
	once   sync.Once
	closed chan struct{}
}

func newPipeListener() *pipeListener {
	return &pipeListener{ch: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error { l.once.Do(func() { close(l.closed) }); return nil }

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

func (l *pipeListener) dial(t *testing.T) net.Conn {
	a, b := net.Pipe()
	select {
	case l.ch <- b:
	case <-time.After(5 * time.Second):
		t.Fatal("dial: no accept")
	}
	t.Cleanup(func() { a.Close() })
	return a
}

type env struct {
	srv *Server
	ln  *pipeListener
	st  *store.Store
	au  *audit.Log
	dev *fakeDevice
	hub *Hub
	pol *policy.Service
	dry *policy.DryRun
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), store.DBFileName), store.Options{Logger: quiet()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	dek := make([]byte, 32)
	rand.Read(dek)
	au := audit.New(st, quiet())
	v := vault.Open(ctx, st, &testOnlyDEKProvider{dek: dek}, au, quiet())
	if s := v.Status(); s.State != vault.StateAvailable {
		t.Fatalf("vault: %+v", s)
	}
	dry := &policy.DryRun{}
	pol, err := policy.NewService(ctx, st, au, dry, nil, quiet())
	if err != nil {
		t.Fatal(err)
	}
	dev := &fakeDevice{status: device.Status{State: device.StateConnected, Profile: &device.Profile{
		Transport: device.TransportSerial, Port: "sim0", Dialect: dialect.DialectVoltronic, SerialNumber: redact.Redacted}}}
	hub := NewHub()
	srv := NewServer(Backend{Store: st, Vault: v, Auth: auth.New(st, au, quiet()), Audit: au, Version: "test", Log: quiet(),
		Device: dev, Policy: pol, Hub: hub})
	ln := newPipeListener()
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln, nil) }()
	t.Cleanup(func() {
		srv.Close()
		if err := <-done; !errors.Is(err, ErrClosed) {
			t.Errorf("Serve: %v", err)
		}
	})
	return &env{srv: srv, ln: ln, st: st, au: au, dev: dev, hub: hub, pol: pol, dry: dry}
}

// fakeDevice is a fake Device Manager (SEC-091: only in _test.go).
type fakeDevice struct {
	mu       sync.Mutex
	status   device.Status
	restarts int
}

func (d *fakeDevice) Status() device.Status { d.mu.Lock(); defer d.mu.Unlock(); return d.status }
func (d *fakeDevice) Restart()              { d.mu.Lock(); d.restarts++; d.mu.Unlock() }

// recorder keeps all response bytes for the Secret check.
type recorder struct {
	net.Conn
	mu  sync.Mutex
	buf bytes.Buffer
}

func (r *recorder) Read(p []byte) (int, error) {
	n, err := r.Conn.Read(p)
	r.mu.Lock()
	r.buf.Write(p[:n])
	r.mu.Unlock()
	return n, err
}

const testPIN = "7a3k9zQ2"

// TestAllMethodsNoSecretInResponses calls each method and checks that no
// response contains the test Secret value (step 7).
func TestAllMethodsNoSecretInResponses(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	// This test sends more than 20 requests in one second. The rate limit
	// has its own test (TestRateLimit).
	e.srv.rate = 10000
	rec := &recorder{Conn: e.ln.dial(t)}
	c := client.New(rec)
	secret := "S3cr3tValue-" + strings.Repeat("q", 20) + "-ZZZZ"

	// First start: only hello, status, auth.status and set_pin.
	if _, err := c.Hello(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Settings(ctx); client.Code(err) != wire.CodePINNotSet {
		t.Fatalf("settings before PIN: %v", err)
	}
	if err := c.Login(ctx, testPIN); client.Code(err) != wire.CodePINNotSet {
		t.Fatalf("login before PIN: %v", err)
	}
	if err := c.SetPIN(ctx, "123456"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("weak PIN: %v", err)
	}
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	// After the first PIN, set_pin needs elevation.
	if err := c.SetPIN(ctx, "9q8w7e6r"); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("second set_pin: %v", err)
	}
	if err := c.SetSecret(ctx, "telegram.bot_token", secret); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("secret.set without PIN: %v", err)
	}
	if err := c.Login(ctx, "0000xx11"); client.Code(err) != wire.CodePINIncorrect {
		t.Fatalf("incorrect PIN: %v", err)
	}
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	as, err := c.AuthStatus(ctx)
	if err != nil || !as.PINSet || !as.Elevated {
		t.Fatalf("auth.status: %+v %v", as, err)
	}
	if _, err := c.Status(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSecret(ctx, "telegram.bot_token", secret); err != nil {
		t.Fatal(err)
	}
	ss, err := c.SecretStatus(ctx, "telegram.bot_token")
	if err != nil || !ss.Set || ss.Hint != "ZZZZ" {
		t.Fatalf("secret.status: %+v %v", ss, err)
	}
	if err := c.SetSecret(ctx, "vlt.canary", "x"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("reserved ID: %v", err)
	}
	if v, err := c.SetSetting(ctx, store.SettingSessionIdleMinutes, 15); err != nil || v != 1 {
		t.Fatalf("settings.set: %d %v", v, err)
	}
	if _, err := c.SetSetting(ctx, store.SettingSessionIdleMinutes, 500); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("settings.set out of range: %v", err)
	}
	list, err := c.Settings(ctx)
	if err != nil || len(list) == 0 {
		t.Fatalf("settings.list: %v %v", list, err)
	}
	al, err := c.Audit(ctx, 0, 500)
	if err != nil || len(al.Rows) == 0 {
		t.Fatalf("audit.list: %v", err)
	}
	if err := c.SetPIN(ctx, "9q8w7e6r"); err != nil {
		t.Fatalf("set_pin when elevated: %v", err)
	}
	// A PIN change ends the elevated state.
	if err := c.DeleteSecret(ctx, "telegram.bot_token"); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("after PIN change: %v", err)
	}
	if err := c.Login(ctx, "9q8w7e6r"); err != nil {
		t.Fatal(err)
	}
	liveMethods(t, ctx, e, c, secret)
	if err := c.ResetVault(ctx, "reset vault"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("reset with wrong text: %v", err)
	}
	if err := c.SetSecret(ctx, "telegram.bot_token", secret); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSecret(ctx, "telegram.bot_token"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSecret(ctx, "telegram.bot_token"); client.Code(err) != wire.CodeNotFound {
		t.Fatalf("delete again: %v", err)
	}
	if err := c.SetSecret(ctx, "telegram.bot_token", secret); err != nil {
		t.Fatal(err)
	}
	if err := c.ResetVault(ctx, "RESET VAULT"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Audit(ctx, 0, 500); err != nil {
		t.Fatal(err)
	}
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Audit(ctx, 0, 10); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("after logout: %v", err)
	}
	if err := c.Call(ctx, "system.unknown", nil, nil); client.Code(err) != wire.CodeUnknownMethod {
		t.Fatalf("unknown method: %v", err)
	}

	rec.mu.Lock()
	all := rec.buf.String()
	rec.mu.Unlock()
	if strings.Contains(all, secret) || strings.Contains(all, "S3cr3tValue") {
		t.Fatal("a response contains the Secret value")
	}
	// The audit rows do not contain the Secret value either.
	rows, err := e.au.List(ctx, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(rows)
	if bytes.Contains(b, []byte("S3cr3tValue")) {
		t.Fatal("an audit row contains the Secret value")
	}
	for _, want := range []string{"pin_set", "pin_entry", "secret_set", "secret_delete", "settings_set", "vault_reset"} {
		if !bytes.Contains(b, []byte(`"action":"`+want+`"`)) {
			t.Errorf("no audit row %s", want)
		}
	}
	// Each method was used.
	for _, m := range wire.Methods {
		if _, ok := methodAccess[m]; !ok {
			t.Errorf("method %s has no access rule", m)
		}
	}
}

func rawFrame(b []byte) []byte {
	out := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(out, uint32(len(b)))
	copy(out[4:], b)
	return out
}

func readResp(t *testing.T, c net.Conn) wire.Response {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	b, err := wire.ReadFrame(c)
	if err != nil {
		t.Fatal(err)
	}
	var r wire.Response
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestBadRequests(t *testing.T) {
	e := newEnv(t)
	c := e.ln.dial(t)
	for _, in := range []string{
		`{"v":1,"id":3,"method":"system.hello","params":{"x":1}}`,
		`{"v":1,"id":3,"method":"system.hello","other":1}`,
		`{"v":9,"id":3,"method":"system.hello"}`,
		`not json`,
	} {
		go c.Write(rawFrame([]byte(in)))
		if r := readResp(t, c); r.Error == nil || r.Error.Code != wire.CodeBadRequest {
			t.Errorf("%s: %+v", in, r)
		}
	}
	// The connection still works.
	go c.Write(rawFrame([]byte(`{"v":1,"id":4,"method":"system.hello"}`)))
	if r := readResp(t, c); r.Error != nil || r.ID != 4 {
		t.Fatalf("hello: %+v", r.Error)
	}
	// A frame larger than 1 MiB closes the connection.
	go func() {
		h := make([]byte, 4)
		binary.BigEndian.PutUint32(h, wire.MaxFrame+1)
		c.Write(h)
	}()
	if r := readResp(t, c); r.Error == nil || r.Error.Code != wire.CodeFrameTooLarge {
		t.Fatalf("large frame: %+v", r)
	}
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection not closed after a large frame")
	}
}

func TestConnectionLimit(t *testing.T) {
	e := newEnv(t)
	var cs []*client.Client
	for i := 0; i < MaxConns; i++ {
		c := client.New(e.ln.dial(t))
		if _, err := c.Hello(context.Background()); err != nil {
			t.Fatal(err)
		}
		cs = append(cs, c)
	}
	extra := e.ln.dial(t)
	extra.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := extra.Read(make([]byte, 1)); err == nil {
		t.Fatal("ninth connection not closed")
	}
	if n := e.srv.Conns(); n != MaxConns {
		t.Fatalf("conns = %d", n)
	}
	cs[0].Close()
	deadline := time.Now().Add(5 * time.Second)
	for e.srv.Conns() != MaxConns-1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	c := client.New(e.ln.dial(t))
	if _, err := c.Hello(context.Background()); err != nil {
		t.Fatalf("connection after one closed: %v", err)
	}
}

func TestRateLimit(t *testing.T) {
	e := newEnv(t)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	e.srv.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	c := client.New(e.ln.dial(t))
	ctx := context.Background()
	for i := 0; i < RequestsPerSec; i++ {
		if _, err := c.Hello(ctx); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if _, err := c.Hello(ctx); client.Code(err) != wire.CodeRateLimited {
		t.Fatalf("request 21: %v", err)
	}
	mu.Lock()
	now = now.Add(time.Second)
	mu.Unlock()
	if _, err := c.Hello(ctx); err != nil {
		t.Fatalf("after one second: %v", err)
	}
}

func TestPartialFrameTimeout(t *testing.T) {
	e := newEnv(t)
	e.srv.partial = 200 * time.Millisecond
	c := e.ln.dial(t)
	// No byte: the connection stays open.
	time.Sleep(400 * time.Millisecond)
	if e.srv.Conns() != 1 {
		t.Fatal("idle connection closed")
	}
	go c.Write([]byte{0, 0})
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	start := time.Now()
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection with a partial frame not closed")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("closed after %v", d)
	}
}

func TestSessionIdle(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	e.srv.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	step := func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
	c := client.New(e.ln.dial(t))
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	step(9 * time.Minute)
	if _, err := c.Audit(ctx, 0, 1); err != nil {
		t.Fatalf("after 9 min: %v", err)
	}
	step(10 * time.Minute)
	if _, err := c.Audit(ctx, 0, 1); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("after 10 min idle: %v", err)
	}
	// The elevated state is for one connection only.
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	other := client.New(e.ln.dial(t))
	if _, err := other.Audit(ctx, 0, 1); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("other connection: %v", err)
	}
}
