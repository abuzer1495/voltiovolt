// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package vault

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestDPAPIRoundTrip is VLT-IS-003 TASK 03-00-00-004 step 9.6: a DPAPI round
// trip in the CI user account.
func TestDPAPIRoundTrip(t *testing.T) {
	ctx := context.Background()
	id := make([]byte, 16)
	rand.Read(id)
	dir := t.TempDir()
	p := NewHostProvider(dir, id).(*DPAPIProvider)
	if _, _, err := p.Load(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no file: %v", err)
	}
	dek, mode, err := p.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dek) != DEKSize || mode != KeyModeDPAPI {
		t.Fatalf("create: %d bytes, mode %q", len(dek), mode)
	}
	w, _ := os.ReadFile(filepath.Join(dir, WrappedDEKFile))
	if len(w) == 0 || bytes.Contains(w, dek) {
		t.Fatal("wrapped DEK file contains the DEK")
	}
	got, _, err := p.Load(ctx)
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("round trip: %v", err)
	}
	// A different install ID (entropy) does not unwrap.
	other := make([]byte, 16)
	rand.Read(other)
	q := NewHostProvider(dir, other).(*DPAPIProvider)
	if _, _, err := q.Load(ctx); err == nil || errors.Is(err, ErrNotConfigured) {
		t.Fatalf("other entropy: err = %v", err)
	}
	// A changed blob does not unwrap.
	w[len(w)/2] ^= 0xff
	os.WriteFile(filepath.Join(dir, WrappedDEKFile), w, 0o600)
	if _, _, err := p.Load(ctx); err == nil {
		t.Fatal("changed blob was unwrapped")
	}
}

func TestVaultWithDPAPI(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st := newStore(t, dir)
	id, _ := st.InstallID(ctx)
	v := Open(ctx, st, NewHostProvider(dir, id), nil, quietLog(nil))
	if s := v.Status(); s.State != StateAvailable || s.KeyMode != KeyModeDPAPI {
		t.Fatalf("first start: %+v", s)
	}
	if err := v.Set(ctx, "a", fakeSecret()); err != nil {
		t.Fatal(err)
	}
	if v2 := Open(ctx, st, NewHostProvider(dir, id), nil, quietLog(nil)); v2.Status().State != StateAvailable {
		t.Fatalf("second start: %+v", v2.Status())
	}
	os.Remove(filepath.Join(dir, WrappedDEKFile))
	if v3 := Open(ctx, st, NewHostProvider(dir, id), nil, quietLog(nil)); v3.Status().State != StateLocked {
		t.Fatalf("missing vault.dek: %+v", v3.Status())
	}
}
