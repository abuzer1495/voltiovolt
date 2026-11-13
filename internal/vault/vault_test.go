// SPDX-License-Identifier: AGPL-3.0-or-later

package vault

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// testOnlyDEKProvider is a fake host binding for tests (SEC-091: only in
// _test.go files).
type testOnlyDEKProvider struct {
	dek       []byte
	mode      string
	canCreate bool
	loadErr   error
}

func (p *testOnlyDEKProvider) Load(context.Context) ([]byte, string, error) {
	if p.loadErr != nil {
		return nil, "", p.loadErr
	}
	if p.dek == nil {
		return nil, "", ErrNotConfigured
	}
	return append([]byte(nil), p.dek...), p.mode, nil
}

func (p *testOnlyDEKProvider) Create(context.Context) ([]byte, string, error) {
	if !p.canCreate {
		return nil, "", errors.New("cannot create")
	}
	p.dek = randomDEK()
	return append([]byte(nil), p.dek...), p.mode, nil
}

func (p *testOnlyDEKProvider) CanCreate() bool { return p.canCreate }

type testOnlyAuditor struct {
	mu      sync.Mutex
	actions []string
}

func (a *testOnlyAuditor) Record(_ context.Context, _, action, _, _ string, details map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, action)
	return nil
}

func (a *testOnlyAuditor) has(action string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, x := range a.actions {
		if x == action {
			return true
		}
	}
	return false
}

func randomDEK() []byte {
	b := make([]byte, DEKSize)
	rand.Read(b)
	return b
}

func newStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(dir, store.DBFileName), store.Options{Logger: quietLog(nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func quietLog(w io.Writer) *slog.Logger {
	if w == nil {
		w = io.Discard
	}
	return slog.New(slog.NewTextHandler(w, nil))
}

func fakeSecret() []byte { return []byte("fake-" + strings.Repeat("Vq7", 8)) }

func TestEncryptDecrypt(t *testing.T) {
	ctx := context.Background()
	st := newStore(t, t.TempDir())
	var logBuf bytes.Buffer
	v := Open(ctx, st, &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}, nil, quietLog(&logBuf))
	if s := v.Status(); s.State != StateAvailable {
		t.Fatalf("state = %+v", s)
	}
	sec := fakeSecret()
	if err := v.Set(ctx, "telegram.token", sec); err != nil {
		t.Fatal(err)
	}
	var got []byte
	var seen []byte
	if err := v.Use(ctx, "telegram.token", func(b []byte) error {
		got = append([]byte(nil), b...)
		seen = b
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, sec) {
		t.Fatal("decrypted value is not the same")
	}
	if !bytes.Equal(seen, make([]byte, len(seen))) {
		t.Fatal("Use did not clear the buffer")
	}
	r, _ := st.GetSecret(ctx, "telegram.token")
	if bytes.Contains(r.Ciphertext, sec) || r.Version != 1 {
		t.Fatalf("row: version %d", r.Version)
	}
	// Set again increases the version.
	v.Set(ctx, "telegram.token", sec)
	if r, _ := st.GetSecret(ctx, "telegram.token"); r.Version != 2 {
		t.Fatalf("version = %d, want 2", r.Version)
	}
	ss, _ := v.SecretStatus(ctx, "telegram.token")
	if !ss.Set || ss.Hint != string(sec[len(sec)-4:]) || ss.UpdatedAt == "" {
		t.Fatalf("status %+v", ss)
	}
	if err := v.Delete(ctx, "telegram.token"); err != nil {
		t.Fatal(err)
	}
	if ss, _ := v.SecretStatus(ctx, "telegram.token"); ss.Set {
		t.Fatal("deleted Secret is set")
	}
	if err := v.Use(ctx, "telegram.token", func([]byte) error { return nil }); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// Reserved and bad IDs.
	for _, id := range []string{"", CanaryID, "vlt.x", "a b", "a/b", strings.Repeat("a", 65)} {
		if err := v.Set(ctx, id, sec); !errors.Is(err, ErrBadID) {
			t.Errorf("Set(%q) = %v", id, err)
		}
	}
	if strings.Contains(logBuf.String(), string(sec)) || strings.Contains(logBuf.String(), string(canaryValue)) {
		t.Fatal("value in log")
	}
}

func TestShortSecretNoHint(t *testing.T) {
	ctx := context.Background()
	v := Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}, nil, quietLog(nil))
	v.Set(ctx, "smtp.password", []byte("short-pw"))
	if ss, _ := v.SecretStatus(ctx, "smtp.password"); !ss.Set || ss.Hint != "" {
		t.Fatalf("status %+v", ss)
	}
}

func TestNonces(t *testing.T) {
	block := randomDEK()
	v := &Vault{}
	if err := v.useDEKForTest(block); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		n, _, err := seal(v.aead, "x", 1, []byte("p"))
		if err != nil {
			t.Fatal(err)
		}
		if len(n) != NonceSize {
			t.Fatalf("nonce length %d", len(n))
		}
		seen[string(n)] = true
	}
	if len(seen) != 10000 {
		t.Fatalf("%d different nonces, want 10000", len(seen))
	}
}

func TestMovedCiphertext(t *testing.T) {
	ctx := context.Background()
	st := newStore(t, t.TempDir())
	v := Open(ctx, st, &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}, nil, quietLog(nil))
	v.Set(ctx, "a", fakeSecret())
	v.Set(ctx, "b", []byte("other-value-xx"))
	ra, _ := st.GetSecret(ctx, "a")
	rb, _ := st.GetSecret(ctx, "b")
	ra.ID = "b"
	ra.Version = rb.Version
	st.PutSecret(ctx, ra)
	if err := v.Use(ctx, "b", func([]byte) error { return nil }); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("moved ciphertext: err = %v", err)
	}
	// A changed version also fails.
	r, _ := st.GetSecret(ctx, "a")
	r.Version++
	st.PutSecret(ctx, r)
	if err := v.Use(ctx, "a", func([]byte) error { return nil }); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("changed version: err = %v", err)
	}
}

// TestDifferentDEK is a simulation of SEC-006: the data on a second
// computer (different DEK) does not decrypt. The canary check locks the
// Vault.
func TestDifferentDEK(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st := newStore(t, dir)
	v := Open(ctx, st, &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}, nil, quietLog(nil))
	v.Set(ctx, "a", fakeSecret())
	r, _ := st.GetSecret(ctx, "a")

	other := &Vault{}
	other.useDEKForTest(randomDEK())
	if _, err := open(other.aead, r); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("different DEK: err = %v", err)
	}
}

func TestCanaryFailureLocks(t *testing.T) {
	ctx := context.Background()
	st := newStore(t, t.TempDir())
	au := &testOnlyAuditor{}
	Open(ctx, st, &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}, au, quietLog(nil))
	if !au.has("vault_created") {
		t.Fatal("no vault_created audit entry")
	}
	var logBuf bytes.Buffer
	v := Open(ctx, st, &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}, au, quietLog(&logBuf))
	s := v.Status()
	if s.State != StateLocked || s.Reason != "host binding failed" {
		t.Fatalf("state = %+v", s)
	}
	if !au.has("vault_locked") {
		t.Fatal("no vault_locked audit entry")
	}
	if strings.Contains(logBuf.String(), string(canaryValue)) {
		t.Fatal("canary value in log")
	}
	if err := v.Set(ctx, "a", fakeSecret()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Set in locked state: %v", err)
	}
	// Settings writes are not possible.
	if _, err := st.SetSetting(ctx, store.SettingLanguage, []byte(`"tr"`), "gui"); !errors.Is(err, store.ErrVaultUnavailable) {
		t.Fatalf("settings write in locked state: %v", err)
	}
}

func TestStates(t *testing.T) {
	ctx := context.Background()
	// No DEK on Linux: not configured.
	v := Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{}, nil, quietLog(nil))
	if v.Status().State != StateNotConfigured {
		t.Fatalf("no DEK: %+v", v.Status())
	}
	// Key mode host: reduced protection.
	v = Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeHost}, nil, quietLog(nil))
	if v.Status().State != StateReduced {
		t.Fatalf("host: %+v", v.Status())
	}
	if err := v.Set(ctx, "a", fakeSecret()); err != nil {
		t.Fatalf("Set in reduced protection: %v", err)
	}
	// Unknown key mode: reduced protection.
	v = Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{dek: randomDEK(), mode: ""}, nil, quietLog(nil))
	if v.Status().State != StateReduced {
		t.Fatalf("unknown: %+v", v.Status())
	}
	// Secret Service: available.
	v = Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeSecretService}, nil, quietLog(nil))
	if v.Status().State != StateAvailable || v.Status().KeyMode != KeyModeSecretService {
		t.Fatalf("secret-service: %+v", v.Status())
	}
	// File DEK: reduced protection.
	v = Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModePortableFile}, nil, quietLog(nil))
	if v.Status().State != StateReduced || v.Status().KeyMode != KeyModePortableFile {
		t.Fatalf("file: %+v", v.Status())
	}
	// Load error: locked.
	v = Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{loadErr: errors.New("x")}, nil, quietLog(nil))
	if v.Status().State != StateLocked {
		t.Fatalf("load error: %+v", v.Status())
	}
	// Wrong DEK length: locked.
	v = Open(ctx, newStore(t, t.TempDir()), &testOnlyDEKProvider{dek: []byte("short"), mode: KeyModeTPM2}, nil, quietLog(nil))
	if v.Status().State != StateLocked {
		t.Fatalf("short DEK: %+v", v.Status())
	}
	// Windows-like: first start makes the DEK. A missing wrapped DEK
	// after that locks the Vault.
	dir := t.TempDir()
	st := newStore(t, dir)
	p := &testOnlyDEKProvider{canCreate: true, mode: KeyModeDPAPI}
	if v := Open(ctx, st, p, nil, quietLog(nil)); v.Status().State != StateAvailable {
		t.Fatalf("first start: %+v", v.Status())
	}
	p.dek = nil
	if v := Open(ctx, st, p, nil, quietLog(nil)); v.Status().State != StateLocked {
		t.Fatalf("missing wrapped DEK: %+v", v.Status())
	}
}

func TestReset(t *testing.T) {
	ctx := context.Background()
	for _, canCreate := range []bool{false, true} {
		st := newStore(t, t.TempDir())
		p := &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2, canCreate: canCreate}
		au := &testOnlyAuditor{}
		v := Open(ctx, st, p, au, quietLog(nil))
		v.Set(ctx, "a", fakeSecret())
		st.SetSetting(ctx, store.SettingLanguage, []byte(`"tr"`), "gui")
		old := append([]byte(nil), p.dek...)
		if err := v.Reset(ctx, "reset vault"); !errors.Is(err, ErrConfirmation) {
			t.Fatalf("wrong confirmation: %v", err)
		}
		if err := v.Reset(ctx, ResetConfirmation); err != nil {
			t.Fatal(err)
		}
		if ss, _ := v.SecretStatus(ctx, "a"); ss.Set {
			t.Fatal("Secret not deleted")
		}
		if bytes.Equal(old, p.dek) == canCreate {
			t.Fatalf("canCreate=%v: new DEK = %v", canCreate, !bytes.Equal(old, p.dek))
		}
		if v.Status().State != StateAvailable {
			t.Fatalf("after reset: %+v", v.Status())
		}
		if s, _ := st.GetSetting(ctx, store.SettingLanguage); s.Default {
			t.Fatal("valid setting lost at reset")
		}
		// A new start passes the canary check.
		if v2 := Open(ctx, st, p, au, quietLog(nil)); v2.Status().State != StateAvailable {
			t.Fatalf("reopen after reset: %+v", v2.Status())
		}
		if !au.has("vault_reset") {
			t.Fatal("no vault_reset audit entry")
		}
	}
	// Reset of a locked Vault on Linux (canary made with another DEK).
	st := newStore(t, t.TempDir())
	Open(ctx, st, &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}, nil, quietLog(nil))
	p := &testOnlyDEKProvider{dek: randomDEK(), mode: KeyModeTPM2}
	v := Open(ctx, st, p, nil, quietLog(nil))
	if v.Status().State != StateLocked {
		t.Fatal("not locked")
	}
	if err := v.Reset(ctx, ResetConfirmation); err != nil || v.Status().State != StateAvailable {
		t.Fatalf("reset of locked vault: %v %+v", err, v.Status())
	}
}

func TestSettingsMACKey(t *testing.T) {
	a, _ := SettingsMACKey(bytes.Repeat([]byte{1}, 32))
	b, _ := SettingsMACKey(bytes.Repeat([]byte{2}, 32))
	if len(a) != 32 || bytes.Equal(a, b) {
		t.Fatal("MAC key derivation")
	}
}

func TestAdditionalData(t *testing.T) {
	if got := string(additionalData("telegram.token", 3)); got != "vlt-secret-v1|telegram.token|3" {
		t.Fatalf("additional data = %q", got)
	}
}

// useDEKForTest makes only the AEAD (no Store).
func (v *Vault) useDEKForTest(dek []byte) error {
	aead, err := newAEAD(dek)
	v.aead = aead
	return err
}
