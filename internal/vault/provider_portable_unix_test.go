// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package vault

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"testing"
)

// memorySS is a fake Secret Service for tests. It never calls D-Bus.
type memorySS struct {
	available bool
	putErr    error
	getErr    error
	items     map[string][]byte
}

func (m *memorySS) key(id []byte) string { return hex.EncodeToString(id) }

func (m *memorySS) Available(context.Context) bool { return m.available }

func (m *memorySS) Get(_ context.Context, installID []byte) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.items == nil {
		return nil, ErrNotConfigured
	}
	dek, ok := m.items[m.key(installID)]
	if !ok {
		return nil, ErrNotConfigured
	}
	return append([]byte(nil), dek...), nil
}

func (m *memorySS) Put(_ context.Context, installID, dek []byte) error {
	if m.putErr != nil {
		return m.putErr
	}
	if m.items == nil {
		m.items = map[string][]byte{}
	}
	m.items[m.key(installID)] = append([]byte(nil), dek...)
	return nil
}

func testID() []byte {
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	return id
}

func TestPortableProviderFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := &PortableProvider{DataDir: dir, InstallID: testID(), ss: &memorySS{}}
	if !p.CanCreate() {
		t.Fatal("portable provider must make the DEK")
	}
	if _, _, err := p.Load(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("empty: %v", err)
	}
	dek, mode, err := p.Create(ctx)
	if err != nil || mode != KeyModePortableFile || len(dek) != DEKSize {
		t.Fatalf("create: mode=%q err=%v", mode, err)
	}
	st, err := os.Lstat(p.filePath())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o, want 0600", st.Mode().Perm())
	}
	got, mode, err := p.Load(ctx)
	if err != nil || mode != KeyModePortableFile || !bytes.Equal(got, dek) {
		t.Fatalf("load: mode=%q err=%v", mode, err)
	}
	q := &PortableProvider{DataDir: dir, InstallID: testID(), ss: &memorySS{}}
	got, mode, err = q.Load(ctx)
	if err != nil || mode != KeyModePortableFile || !bytes.Equal(got, dek) {
		t.Fatalf("second load: mode=%q err=%v", mode, err)
	}
	if err := os.WriteFile(p.filePath(), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Load(ctx); err == nil || errors.Is(err, ErrNotConfigured) {
		t.Fatalf("short file: %v", err)
	}
}

func TestPortableProviderSecretService(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ss := &memorySS{available: true}
	p := &PortableProvider{DataDir: dir, InstallID: testID(), ss: ss}
	dek, mode, err := p.Create(ctx)
	if err != nil || mode != KeyModeSecretService {
		t.Fatalf("create: mode=%q err=%v", mode, err)
	}
	if _, err := os.Lstat(p.filePath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Create must not write a file when the Secret Service works")
	}
	got, mode, err := p.Load(ctx)
	if err != nil || mode != KeyModeSecretService || !bytes.Equal(got, dek) {
		t.Fatalf("load: mode=%q err=%v", mode, err)
	}
	other := &PortableProvider{DataDir: dir, InstallID: make([]byte, 16), ss: ss}
	if _, _, err := other.Load(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("other install ID: %v", err)
	}
}

func TestPortableProviderFileTakesPrecedence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := &PortableProvider{DataDir: dir, InstallID: testID(), ss: &memorySS{}}
	dek, _, err := p.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ss := &memorySS{available: true}
	if err := ss.Put(ctx, testID(), randomDEK()); err != nil {
		t.Fatal(err)
	}
	q := &PortableProvider{DataDir: dir, InstallID: testID(), ss: ss}
	got, mode, err := q.Load(ctx)
	if err != nil || mode != KeyModePortableFile || !bytes.Equal(got, dek) {
		t.Fatalf("file must win: mode=%q err=%v", mode, err)
	}
}

func TestPortableProviderSecretServicePutFallback(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := &PortableProvider{DataDir: dir, InstallID: testID(), ss: &memorySS{available: true, putErr: errors.New("denied")}}
	dek, mode, err := p.Create(ctx)
	if err != nil || mode != KeyModePortableFile || len(dek) != DEKSize {
		t.Fatalf("fallback: mode=%q err=%v", mode, err)
	}
	got, mode, err := p.Load(ctx)
	if err != nil || mode != KeyModePortableFile || !bytes.Equal(got, dek) {
		t.Fatalf("load after fallback: mode=%q err=%v", mode, err)
	}
}

func TestPortableVaultOpenFileReduced(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st := newStore(t, dir)
	p := &PortableProvider{DataDir: dir, InstallID: testID(), ss: &memorySS{}}
	v := Open(ctx, st, p, nil, quietLog(nil))
	if v.Status().State != StateReduced || v.Status().KeyMode != KeyModePortableFile {
		t.Fatalf("open: %+v", v.Status())
	}
	if err := v.Set(ctx, "a", fakeSecret()); err != nil {
		t.Fatal(err)
	}
	v2 := Open(ctx, st, p, nil, quietLog(nil))
	if v2.Status().State != StateReduced || v2.Status().KeyMode != KeyModePortableFile {
		t.Fatalf("reopen: %+v", v2.Status())
	}
}

func TestPortableVaultOpenSecretServiceAvailable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := &PortableProvider{DataDir: dir, InstallID: testID(), ss: &memorySS{available: true}}
	st := newStore(t, dir)
	v := Open(ctx, st, p, nil, quietLog(nil))
	if v.Status().State != StateAvailable || v.Status().KeyMode != KeyModeSecretService {
		t.Fatalf("open: %+v", v.Status())
	}
	v2 := Open(ctx, st, p, nil, quietLog(nil))
	if v2.Status().State != StateAvailable {
		t.Fatalf("reopen: %+v", v2.Status())
	}
}

func TestNewHostProviderUnchanged(t *testing.T) {
	p, ok := NewHostProvider("", nil).(*CredentialsProvider)
	if !ok {
		t.Fatalf("NewHostProvider type %T", NewHostProvider("", nil))
	}
	if p.CanCreate() {
		t.Fatal("service provider must not make the DEK")
	}
	if p.ModeFile != KeyModeFile {
		t.Fatalf("ModeFile %q", p.ModeFile)
	}
}

func TestSecretServiceLive(t *testing.T) {
	ctx := context.Background()
	ss := liveSecretService{}
	if !ss.Available(ctx) {
		t.Skip("Secret Service is not available")
	}
	id := testID()
	id[0] = 0xff
	p := NewPortableProvider(t.TempDir(), id).(*PortableProvider)
	dek, mode, err := p.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if mode != KeyModeSecretService {
		t.Fatalf("live create mode %q (file fallback: Secret Service Put failed)", mode)
	}
	got, mode, err := p.Load(ctx)
	if err != nil || mode != KeyModeSecretService || !bytes.Equal(got, dek) {
		t.Fatalf("live load: mode=%q err=%v", mode, err)
	}
}
