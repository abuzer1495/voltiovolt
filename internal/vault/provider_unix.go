// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package vault

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Linux host binding (TASK 03-00-00-004 step 3).
const (
	// CredentialName is the systemd credential name of the DEK.
	CredentialName = "vlt-dek"
	// KeyModeFile is written by the installer (root-owned, mode 0644, no
	// Secret).
	KeyModeFile = "/etc/vltd/key-mode"
)

// CredentialsProvider reads the DEK from $CREDENTIALS_DIRECTORY/vlt-dek.
// systemd decrypts the credential (LoadCredentialEncrypted=) before vltd
// starts. vltd never starts systemd-creds.
type CredentialsProvider struct {
	// Dir is the credentials directory ($CREDENTIALS_DIRECTORY).
	Dir string
	// ModeFile is the key mode file.
	ModeFile string
	// modeOwner is the required owner uid of ModeFile (root).
	modeOwner int
}

// NewHostProvider returns the Linux host binding. dataDir and installID are
// not used on Linux.
func NewHostProvider(string, []byte) KeyProvider {
	return &CredentialsProvider{Dir: os.Getenv("CREDENTIALS_DIRECTORY"), ModeFile: KeyModeFile}
}

// CanCreate implements KeyProvider. The installer makes the DEK on Linux.
func (*CredentialsProvider) CanCreate() bool { return false }

// Create implements KeyProvider. It always fails on Linux.
func (*CredentialsProvider) Create(context.Context) ([]byte, string, error) {
	return nil, "", errors.New("vault: vltd does not make the DEK on Linux; run the installer")
}

// Load implements KeyProvider.
func (p *CredentialsProvider) Load(context.Context) ([]byte, string, error) {
	if p.Dir == "" {
		return nil, "", fmt.Errorf("%w: CREDENTIALS_DIRECTORY is not set", ErrNotConfigured)
	}
	path := filepath.Join(p.Dir, CredentialName)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("%w: no credential %s", ErrNotConfigured, CredentialName)
	}
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	dek := make([]byte, DEKSize+1)
	n, err := io.ReadFull(f, dek)
	switch {
	case err == nil:
		wipe(dek)
		return nil, "", fmt.Errorf("vault: credential %s is longer than %d bytes", CredentialName, DEKSize)
	case err != io.ErrUnexpectedEOF && err != io.EOF:
		wipe(dek)
		return nil, "", fmt.Errorf("vault: credential %s: %v", CredentialName, err)
	}
	if n != DEKSize {
		wipe(dek)
		return nil, "", fmt.Errorf("vault: credential %s has %d bytes, want %d", CredentialName, n, DEKSize)
	}
	return dek[:DEKSize], p.keyMode(), nil
}

// keyMode reads the key mode file. A file that is missing, not root-owned,
// writable by group or others, or with an unknown value gives "unknown"
// (state "reduced protection").
func (p *CredentialsProvider) keyMode() string {
	st, err := os.Lstat(p.ModeFile)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0o022 != 0 {
		return KeyModeUnknown
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); !ok || int(sys.Uid) != p.modeOwner {
		return KeyModeUnknown
	}
	b, err := os.ReadFile(p.ModeFile)
	if err != nil {
		return KeyModeUnknown
	}
	switch m := strings.TrimSpace(string(b)); m {
	case KeyModeHost, KeyModeTPM2, KeyModeHostAndTPM2:
		return m
	}
	return KeyModeUnknown
}
