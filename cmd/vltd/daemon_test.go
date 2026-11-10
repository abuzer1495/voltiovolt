// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"crypto/rand"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/platform"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// testOnlyDEKProvider is the test DEK provider (SEC-091: only in _test.go
// files; the check in docs/security.md makes sure it is not in vltd).
type testOnlyDEKProvider struct{ dek []byte }

func (p *testOnlyDEKProvider) Load(context.Context) ([]byte, string, error) {
	if p.dek == nil {
		return nil, "", vault.ErrNotConfigured
	}
	return append([]byte(nil), p.dek...), vault.KeyModeHostAndTPM2, nil
}

func (p *testOnlyDEKProvider) Create(context.Context) ([]byte, string, error) {
	return nil, "", vault.ErrNotConfigured
}

func (p *testOnlyDEKProvider) CanCreate() bool { return false }

func testOnlyDEK() []byte {
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

// startTestDaemon starts vltd in the test process (step 7.1).
func startTestDaemon(t *testing.T, dir string, p vault.KeyProvider) (*daemon, string) {
	t.Helper()
	listen, addr := testListen(t)
	d := newDaemon(config{
		DataDir:    dir,
		CheckPerms: runtime.GOOS != "windows", // Windows: the ACL check needs the installer (dev-install.ps1)
		Provider:   func(string, []byte) vault.KeyProvider { return p },
		Listen:     listen,
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:    "test",
	})
	if err := d.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopTestDaemon(t, d) })
	return d, addr
}

func stopTestDaemon(t *testing.T, d *daemon) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), platform.StopTimeout)
	defer cancel()
	start := time.Now()
	if err := d.Stop(ctx); err != nil {
		t.Errorf("stop: %v", err)
	}
	if el := time.Since(start); el > platform.StopTimeout {
		t.Errorf("stop took %v", el)
	}
}

func dataDir(t *testing.T) string {
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func dialTest(t *testing.T, addr string) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

const (
	pin1 = "Vq7k2m9x"
	pin2 = "p4Lz8nW3"
)

// TestIntegration is step 7.2: first PIN, login, settings write and read,
// Secret write and status, audit list, Vault reset, lockout.
func TestIntegration(t *testing.T) {
	ctx := context.Background()
	dir := dataDir(t)
	prov := &testOnlyDEKProvider{dek: testOnlyDEK()}
	d, addr := startTestDaemon(t, dir, prov)
	if s := d.vt.Status(); s.State != vault.StateAvailable {
		t.Fatalf("vault: %+v", s)
	}
	c := dialTest(t, addr)

	h, err := c.Hello(ctx)
	if err != nil || h.Version != "test" || h.Protocol != wire.Version {
		t.Fatalf("hello: %+v %v", h, err)
	}
	st, err := c.Status(ctx)
	if err != nil || st.Vault.State != string(vault.StateAvailable) || st.Store.State != "ok" || st.Store.SchemaVersion < 1 {
		t.Fatalf("status: %+v %v", st, err)
	}
	// First PIN.
	if as, _ := c.AuthStatus(ctx); as.PINSet {
		t.Fatal("PIN set at first start")
	}
	if err := c.SetSecret(ctx, "telegram.bot_token", "x"); client.Code(err) != wire.CodePINNotSet {
		t.Fatalf("secret.set before PIN: %v", err)
	}
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	// Login.
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	// Settings write and read.
	if v, err := c.SetSetting(ctx, store.SettingLanguage, "tr"); err != nil || v != 1 {
		t.Fatalf("settings.set: %d %v", v, err)
	}
	list, err := c.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range list {
		if s.Key == store.SettingLanguage && string(s.Value) == `"tr"` && !s.Default && s.Version == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("settings.list: %+v", list)
	}
	// Secret write and status.
	secret := "integration-test-secret-value-WXYZ"
	if err := c.SetSecret(ctx, "telegram.bot_token", secret); err != nil {
		t.Fatal(err)
	}
	ss, err := c.SecretStatus(ctx, "telegram.bot_token")
	if err != nil || !ss.Set || ss.Hint != "WXYZ" || ss.UpdatedAt == "" {
		t.Fatalf("secret.status: %+v %v", ss, err)
	}
	// Audit list with paging.
	page1, err := c.Audit(ctx, 0, 2)
	if err != nil || len(page1.Rows) != 2 {
		t.Fatalf("audit.list: %+v %v", page1, err)
	}
	page2, err := c.Audit(ctx, page1.Next, 500)
	if err != nil || len(page2.Rows) == 0 || page2.Rows[0].Seq != page1.Next+1 {
		t.Fatalf("audit.list page 2: %+v %v", page2, err)
	}
	// Vault reset.
	if err := c.ResetVault(ctx, "RESET"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("reset with wrong text: %v", err)
	}
	if err := c.ResetVault(ctx, "RESET VAULT"); err != nil {
		t.Fatal(err)
	}
	if ss, err := c.SecretStatus(ctx, "telegram.bot_token"); err != nil || ss.Set {
		t.Fatalf("secret after reset: %+v %v", ss, err)
	}
	// Lockout: 5 incorrect entries give a block; the correct PIN is then
	// refused.
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		if err := c.Login(ctx, pin2); client.Code(err) != wire.CodePINIncorrect {
			t.Fatalf("incorrect entry %d: %v", i, err)
		}
	}
	if err := c.Login(ctx, pin1); client.Code(err) != wire.CodeBlocked {
		t.Fatalf("correct PIN during block: %v", err)
	}
	if as, err := c.AuthStatus(ctx); err != nil || as.BlockUntil == "" || as.Elevated {
		t.Fatalf("auth.status during block: %+v %v", as, err)
	}

	// A restart does not end the block.
	stopTestDaemon(t, d)
	_, addr2 := startTestDaemon(t, dir, prov)
	c2 := dialTest(t, addr2)
	if err := c2.Login(ctx, pin1); client.Code(err) != wire.CodeBlocked {
		t.Fatalf("after restart: %v", err)
	}
}

// TestStartWithoutDEK is step 7.3.
func TestStartWithoutDEK(t *testing.T) {
	ctx := context.Background()
	_, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{})
	c := dialTest(t, addr)
	st, err := c.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Vault.State != string(vault.StateNotConfigured) && st.Vault.State != string(vault.StateLocked) {
		t.Fatalf("vault state %q", st.Vault.State)
	}
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSecret(ctx, "telegram.bot_token", "abc"); client.Code(err) != wire.CodeVaultUnavailable {
		t.Fatalf("secret.set without DEK: %v", err)
	}
	if _, err := c.SetSetting(ctx, store.SettingLanguage, "tr"); client.Code(err) != wire.CodeVaultUnavailable {
		t.Fatalf("settings.set without DEK: %v", err)
	}
}

// TestStartWithWrongDEK: a different DEK (SEC-006 copy to a second
// computer) gives the state "locked".
func TestStartWithWrongDEK(t *testing.T) {
	dir := dataDir(t)
	d, _ := startTestDaemon(t, dir, &testOnlyDEKProvider{dek: testOnlyDEK()})
	stopTestDaemon(t, d)
	d2, addr := startTestDaemon(t, dir, &testOnlyDEKProvider{dek: testOnlyDEK()})
	if s := d2.vt.Status(); s.State != vault.StateLocked {
		t.Fatalf("vault: %+v", s)
	}
	c := dialTest(t, addr)
	if st, err := c.Status(context.Background()); err != nil || st.Vault.State != string(vault.StateLocked) {
		t.Fatalf("status: %+v %v", st, err)
	}
}

// testOnlyNoCallProvider fails the test if vltd tries to read a DEK.
type testOnlyNoCallProvider struct{ t *testing.T }

func (p testOnlyNoCallProvider) Load(context.Context) ([]byte, string, error) {
	p.t.Error("fallback mode: vltd tried to read a DEK")
	return nil, "", vault.ErrNotConfigured
}

func (p testOnlyNoCallProvider) Create(context.Context) ([]byte, string, error) {
	p.t.Error("fallback mode: vltd tried to make a DEK")
	return nil, "", vault.ErrNotConfigured
}

func (p testOnlyNoCallProvider) CanCreate() bool { return false }

// TestFallbackMode is VLT-IS-004 TASK 03-00-00-001 step 3.4.
func TestFallbackMode(t *testing.T) {
	ctx := context.Background()
	dir := dataDir(t)
	// First a normal start, so that the database has a canary.
	d, _ := startTestDaemon(t, dir, &testOnlyDEKProvider{dek: testOnlyDEK()})
	stopTestDaemon(t, d)

	t.Setenv(EnvFallbackMode, "1")
	d2, addr := startTestDaemon(t, dir, testOnlyNoCallProvider{t})
	if s := d2.vt.Status(); s.State != vault.StateLocked || s.Reason != "primary unit failed" {
		t.Fatalf("vault: %+v", s)
	}
	c := dialTest(t, addr)
	st, err := c.Status(ctx)
	if err != nil || st.Vault.State != string(vault.StateLocked) || st.Vault.Reason != "primary unit failed" {
		t.Fatalf("status: %+v %v", st, err)
	}
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSecret(ctx, "telegram.bot_token", "abc"); client.Code(err) != wire.CodeVaultUnavailable {
		t.Fatalf("secret.set in fallback mode: %v", err)
	}
	if err := c.ResetVault(ctx, "RESET VAULT"); err == nil {
		t.Fatal("vault.reset in fallback mode was accepted")
	}
}

func TestPermissionCheckStopsStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux mode check")
	}
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	listen, _ := testListen(t)
	d := newDaemon(config{DataDir: dir, CheckPerms: true, Listen: listen,
		Provider: func(string, []byte) vault.KeyProvider { return &testOnlyDEKProvider{} },
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err := d.Start(context.Background()); err == nil {
		d.Stop(context.Background())
		t.Fatal("start with mode 0755 did not fail")
	}
}
