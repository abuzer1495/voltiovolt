// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialsProvider(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	modeFile := filepath.Join(t.TempDir(), "key-mode")
	p := &CredentialsProvider{Dir: dir, ModeFile: modeFile, modeOwner: os.Getuid()}

	if _, _, err := p.Load(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no file: %v", err)
	}
	if _, _, err := (&CredentialsProvider{}).Load(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no CREDENTIALS_DIRECTORY: %v", err)
	}
	dek := randomDEK()
	os.WriteFile(filepath.Join(dir, CredentialName), dek, 0o400)
	got, mode, err := p.Load(ctx)
	if err != nil || len(got) != DEKSize || string(got) != string(dek) {
		t.Fatalf("load: %v", err)
	}
	if mode != KeyModeUnknown {
		t.Fatalf("no mode file: mode = %q", mode)
	}
	for _, m := range []string{KeyModeHost, KeyModeTPM2, KeyModeHostAndTPM2} {
		os.WriteFile(modeFile, []byte(m+"\n"), 0o644)
		if _, got, _ := p.Load(ctx); got != m {
			t.Errorf("mode %q: got %q", m, got)
		}
	}
	os.WriteFile(modeFile, []byte("null"), 0o644)
	if _, got, _ := p.Load(ctx); got != KeyModeUnknown {
		t.Errorf("bad mode value: %q", got)
	}
	os.WriteFile(modeFile, []byte("tpm2"), 0o644)
	os.Chmod(modeFile, 0o666)
	if _, got, _ := p.Load(ctx); got != KeyModeUnknown {
		t.Errorf("writable mode file: %q", got)
	}
	os.Chmod(modeFile, 0o644)
	p.modeOwner = os.Getuid() + 1
	if _, got, _ := p.Load(ctx); got != KeyModeUnknown {
		t.Errorf("mode file with other owner: %q", got)
	}

	for _, n := range []int{0, 16, 31, 33, 64} {
		os.Chmod(filepath.Join(dir, CredentialName), 0o600)
		os.WriteFile(filepath.Join(dir, CredentialName), make([]byte, n), 0o600)
		if _, _, err := p.Load(ctx); err == nil || errors.Is(err, ErrNotConfigured) {
			t.Errorf("DEK of %d bytes: err = %v", n, err)
		}
	}
	if p.CanCreate() {
		t.Fatal("Linux provider must not make the DEK")
	}
	if _, _, err := p.Create(ctx); err == nil {
		t.Fatal("Create must fail on Linux")
	}
}
