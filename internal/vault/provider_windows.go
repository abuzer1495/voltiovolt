// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package vault

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows host binding (TASK 03-00-00-004 step 4).
const (
	// WrappedDEKFile is the file name of the wrapped DEK in the data
	// directory.
	WrappedDEKFile = "vault.dek"
	// entropyPrefix is the first part of the DPAPI entropy.
	entropyPrefix = "VoltioVolt-DEK-v1"
)

// DPAPIProvider wraps the DEK with CryptProtectData (user scope of the
// service account, CRYPTPROTECT_UI_FORBIDDEN, no prompt structure). The
// entropy is "VoltioVolt-DEK-v1" and the install ID.
type DPAPIProvider struct {
	Path      string
	InstallID []byte
}

// NewHostProvider returns the Windows host binding.
func NewHostProvider(dataDir string, installID []byte) KeyProvider {
	return &DPAPIProvider{Path: filepath.Join(dataDir, WrappedDEKFile), InstallID: append([]byte(nil), installID...)}
}

// CanCreate implements KeyProvider.
func (*DPAPIProvider) CanCreate() bool { return true }

func (p *DPAPIProvider) entropy() []byte {
	return append([]byte(entropyPrefix), p.InstallID...)
}

func blob(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func takeBlob(out *windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	src := unsafe.Slice(out.Data, out.Size)
	b := make([]byte, len(src))
	copy(b, src)
	for i := range src {
		src[i] = 0
	}
	return b
}

// Protect wraps plain with DPAPI (user scope).
func (p *DPAPIProvider) Protect(plain []byte) ([]byte, error) {
	if len(p.InstallID) != 16 {
		return nil, errors.New("vault: install ID must have 16 bytes")
	}
	var out windows.DataBlob
	// Flags: CRYPTPROTECT_UI_FORBIDDEN only. Not CRYPTPROTECT_LOCAL_MACHINE.
	if err := windows.CryptProtectData(blob(plain), nil, blob(p.entropy()), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("vault: CryptProtectData: %w", err)
	}
	return takeBlob(&out), nil
}

// Unprotect unwraps a DPAPI blob.
func (p *DPAPIProvider) Unprotect(wrapped []byte) ([]byte, error) {
	if len(wrapped) == 0 {
		return nil, errors.New("vault: empty wrapped DEK")
	}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(blob(wrapped), nil, blob(p.entropy()), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("vault: CryptUnprotectData: %w", err)
	}
	return takeBlob(&out), nil
}

// Load implements KeyProvider.
func (p *DPAPIProvider) Load(context.Context) ([]byte, string, error) {
	w, err := os.ReadFile(p.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("%w: no %s", ErrNotConfigured, WrappedDEKFile)
	}
	if err != nil {
		return nil, "", err
	}
	dek, err := p.Unprotect(w)
	if err != nil {
		return nil, "", err
	}
	if len(dek) != DEKSize {
		wipe(dek)
		return nil, "", fmt.Errorf("vault: DEK has %d bytes", len(dek))
	}
	return dek, KeyModeDPAPI, nil
}

// Create implements KeyProvider: a new DEK from crypto/rand, wrapped and
// written to vault.dek (temporary file, then rename).
func (p *DPAPIProvider) Create(context.Context) ([]byte, string, error) {
	dek := make([]byte, DEKSize)
	if _, err := rand.Read(dek); err != nil {
		return nil, "", err
	}
	w, err := p.Protect(dek)
	if err != nil {
		wipe(dek)
		return nil, "", err
	}
	tmp := p.Path + ".new"
	if err := os.WriteFile(tmp, w, 0o600); err != nil {
		wipe(dek)
		return nil, "", err
	}
	if err := os.Rename(tmp, p.Path); err != nil {
		os.Remove(tmp)
		wipe(dek)
		return nil, "", err
	}
	return dek, KeyModeDPAPI, nil
}
