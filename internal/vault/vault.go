// SPDX-License-Identifier: AGPL-3.0-or-later

// Package vault keeps the user Secrets (VLT-IS-003 TASK 03-00-00-004).
//
// WARNING: An error in this package can expose all user Secrets.
//
// Design:
//   - The DEK has 32 bytes from crypto/rand.
//   - Each Secret is encrypted with AES-256-GCM, a new 96-bit random nonce
//     for each encryption and the additional data
//     "vlt-secret-v1|" + Secret ID + "|" + version.
//   - The host binding (KeyProvider) gives the DEK: systemd credentials on
//     Linux, DPAPI on Windows.
//   - A canary Secret with a known value is decrypted at each start. If this
//     fails, the Vault state is "locked: host binding failed".
//   - The settings MAC key comes from the DEK through HKDF-SHA256 with the
//     info text "vlt-settings-mac-v1".
package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// DEKSize is the DEK length in bytes.
const DEKSize = 32

// NonceSize is the AES-GCM nonce length in bytes (96 bits).
const NonceSize = 12

// ResetConfirmation is the confirmation text for Reset.
const ResetConfirmation = "RESET VAULT"

// CanaryID is the reserved ID of the canary Secret.
const CanaryID = "vlt.canary"

// canaryValue is the known canary value. It is not a Secret. It is never
// written to a log.
var canaryValue = []byte("VoltioVolt Vault canary v1")

// settingsMACInfo is the HKDF info text of the settings MAC key.
const settingsMACInfo = "vlt-settings-mac-v1"

// State is the Vault state.
type State string

// Vault states (TASK 03-00-00-004 step 6).
const (
	StateAvailable     State = "available"
	StateReduced       State = "reduced protection"
	StateLocked        State = "locked"
	StateNotConfigured State = "not configured"
)

// Errors.
var (
	ErrNotConfigured = errors.New("vault: not configured")
	ErrUnavailable   = errors.New("vault: not available")
	ErrNotFound      = errors.New("vault: secret not set")
	ErrBadID         = errors.New("vault: secret ID is not permitted")
	ErrConfirmation  = errors.New("vault: confirmation text is not correct")
	ErrDecrypt       = errors.New("vault: decryption failed")
)

// Key modes (Linux /etc/vltd/key-mode, Windows "dpapi").
const (
	KeyModeHost          = "host"
	KeyModeTPM2          = "tpm2"
	KeyModeHostAndTPM2   = "host+tpm2"
	KeyModeDPAPI         = "dpapi"
	KeyModeSecretService = "secret-service"
	KeyModePortableFile  = "file"
	KeyModeUnknown       = "unknown"
)

// KeyProvider is the host binding. It gives the DEK.
type KeyProvider interface {
	// Load returns the DEK and the key mode. It returns ErrNotConfigured
	// when there is no DEK. Each other error means: host binding failed.
	Load(ctx context.Context) (dek []byte, keyMode string, err error)
	// Create makes and stores a new DEK, if the platform permits it
	// (Windows). Otherwise it returns an error.
	Create(ctx context.Context) (dek []byte, keyMode string, err error)
	// CanCreate reports whether vltd can make the DEK (Windows: yes,
	// Linux: no, the installer makes it).
	CanCreate() bool
}

// Auditor writes audit entries. vltd gives the audit log.
type Auditor interface {
	Record(ctx context.Context, actor, action, target, result string, details map[string]any) error
}

// Status is the Vault status. It contains no Secret.
type Status struct {
	State   State  `json:"state"`
	Reason  string `json:"reason,omitempty"`
	KeyMode string `json:"key_mode,omitempty"`
}

// SecretStatus is the status of one Secret. It never contains the value.
type SecretStatus struct {
	ID        string `json:"id"`
	Set       bool   `json:"set"`
	Hint      string `json:"hint,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Vault keeps the Secrets.
type Vault struct {
	st    *store.Store
	kp    KeyProvider
	audit Auditor
	log   *slog.Logger

	mu     sync.Mutex
	aead   cipher.AEAD
	status Status
}

// Open opens the Vault. It never returns an error for a host binding
// failure: then the Vault state is "locked" or "not configured" and vltd
// continues (SEC-090). The settings MAC key is given to the Store when the
// Vault is available.
func Open(ctx context.Context, st *store.Store, kp KeyProvider, audit Auditor, log *slog.Logger) *Vault {
	if log == nil {
		log = slog.Default()
	}
	v := &Vault{st: st, kp: kp, audit: audit, log: log}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.open(ctx)
	return v
}

// ReasonFallback is the Vault reason in the fallback unit (VLT-IS-004 QI-1).
const ReasonFallback = "primary unit failed"

// ErrFallback is returned by the key provider of the fallback mode.
var ErrFallback = errors.New("vault: fallback mode: no DEK (" + ReasonFallback + ")")

// fallbackProvider never gives a DEK.
type fallbackProvider struct{}

func (fallbackProvider) Load(context.Context) ([]byte, string, error)   { return nil, "", ErrFallback }
func (fallbackProvider) Create(context.Context) ([]byte, string, error) { return nil, "", ErrFallback }
func (fallbackProvider) CanCreate() bool                                { return false }

// OpenFallback opens the Vault in the fallback mode (VLT_FALLBACK_MODE=1,
// vltd-fallback.service). The state is "locked" with the reason "primary
// unit failed". It does not try to read a DEK. vault.reset is refused.
func OpenFallback(ctx context.Context, st *store.Store, audit Auditor, log *slog.Logger) *Vault {
	if log == nil {
		log = slog.Default()
	}
	v := &Vault{st: st, kp: fallbackProvider{}, audit: audit, log: log}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.setLocked(ctx, ReasonFallback, ErrFallback)
	return v
}

func (v *Vault) record(ctx context.Context, action, result string, details map[string]any) {
	if v.audit == nil {
		return
	}
	if err := v.audit.Record(ctx, "system", action, "vault", result, details); err != nil {
		v.log.Error("audit write failed", "action", action, "err", err)
	}
}

func (v *Vault) setLocked(ctx context.Context, reason string, err error) {
	v.aead = nil
	v.status = Status{State: StateLocked, Reason: reason}
	v.st.SetMACKey(nil)
	v.log.Error("vault locked", "reason", reason, "err", err)
	v.record(ctx, "vault_locked", "failure", map[string]any{"reason": reason})
}

// open runs with v.mu held.
func (v *Vault) open(ctx context.Context) {
	_, cerr := v.st.GetSecret(ctx, CanaryID)
	haveCanary := cerr == nil
	if cerr != nil && !errors.Is(cerr, store.ErrNotFound) {
		v.setLocked(ctx, "store error", cerr)
		return
	}
	dek, mode, err := v.kp.Load(ctx)
	switch {
	case errors.Is(err, ErrNotConfigured) && haveCanary:
		v.setLocked(ctx, "host binding failed", err)
		return
	case errors.Is(err, ErrNotConfigured) && v.kp.CanCreate():
		dek, mode, err = v.kp.Create(ctx)
		if err != nil {
			v.setLocked(ctx, "host binding failed", err)
			return
		}
		v.log.Info("vault DEK made")
	case errors.Is(err, ErrNotConfigured):
		v.status = Status{State: StateNotConfigured, Reason: "no DEK"}
		v.st.SetMACKey(nil)
		v.log.Warn("vault not configured: no DEK; functions that need a Secret are off")
		return
	case err != nil:
		v.setLocked(ctx, "host binding failed", err)
		return
	}
	defer wipe(dek)
	if err := v.useDEK(dek); err != nil {
		v.setLocked(ctx, "host binding failed", err)
		return
	}
	if !haveCanary {
		if err := v.putLocked(ctx, CanaryID, canaryValue); err != nil {
			v.setLocked(ctx, "store error", err)
			return
		}
		v.log.Info("vault made: canary stored")
		v.record(ctx, "vault_created", "success", nil)
	} else if err := v.checkCanary(ctx); err != nil {
		v.setLocked(ctx, "host binding failed", err)
		return
	}
	v.setAvailable(mode)
	v.log.Info("vault canary check passed", "state", string(v.status.State), "key_mode", mode)
}

func (v *Vault) setAvailable(mode string) {
	v.status = Status{State: StateAvailable, KeyMode: mode}
	switch mode {
	case KeyModeTPM2, KeyModeHostAndTPM2, KeyModeDPAPI, KeyModeSecretService:
	case KeyModeHost:
		v.status.State = StateReduced
		v.status.Reason = "key mode host: no TPM2 (SEC-005)"
	case KeyModePortableFile:
		v.status.State = StateReduced
		v.status.Reason = "key mode file: Secret Service is not available"
	default:
		v.status.State = StateReduced
		v.status.Reason = "key mode unknown"
	}
}

func (v *Vault) checkCanary(ctx context.Context) error {
	got, err := v.decryptLocked(ctx, CanaryID)
	if err != nil {
		return err
	}
	defer wipe(got)
	if subtle.ConstantTimeCompare(got, canaryValue) != 1 {
		return errors.New("vault: canary value is not correct")
	}
	return nil
}

// useDEK makes the AEAD and the settings MAC key from the DEK.
func (v *Vault) useDEK(dek []byte) error {
	if len(dek) != DEKSize {
		return fmt.Errorf("vault: DEK has %d bytes, want %d", len(dek), DEKSize)
	}
	aead, err := newAEAD(dek)
	if err != nil {
		return err
	}
	mk, err := SettingsMACKey(dek)
	if err != nil {
		return err
	}
	v.aead = aead
	v.st.SetMACKey(mk)
	wipe(mk)
	return nil
}

// newAEAD returns AES-256-GCM with a 96-bit nonce.
func newAEAD(dek []byte) (cipher.AEAD, error) {
	if len(dek) != DEKSize {
		return nil, fmt.Errorf("vault: DEK has %d bytes, want %d", len(dek), DEKSize)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithNonceSize(block, NonceSize)
}

// SettingsMACKey derives the settings MAC key from the DEK:
// HKDF-SHA256, no salt, info "vlt-settings-mac-v1", 32 bytes.
func SettingsMACKey(dek []byte) ([]byte, error) {
	return hkdf.Key(sha256.New, dek, nil, settingsMACInfo, 32)
}

// additionalData is "vlt-secret-v1|" + id + "|" + version.
func additionalData(id string, version int64) []byte {
	return []byte("vlt-secret-v1|" + id + "|" + strconv.FormatInt(version, 10))
}

func seal(aead cipher.AEAD, id string, version int64, plain []byte) (nonce, ct []byte, err error) {
	nonce = make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, aead.Seal(nil, nonce, plain, additionalData(id, version)), nil
}

func open(aead cipher.AEAD, r store.SecretRow) ([]byte, error) {
	if len(r.Nonce) != NonceSize {
		return nil, ErrDecrypt
	}
	p, err := aead.Open(nil, r.Nonce, r.Ciphertext, additionalData(r.ID, r.Version))
	if err != nil {
		return nil, ErrDecrypt
	}
	return p, nil
}

func (v *Vault) usable() bool {
	return v.aead != nil && (v.status.State == StateAvailable || v.status.State == StateReduced)
}

// putLocked runs with v.mu held.
func (v *Vault) putLocked(ctx context.Context, id string, value []byte) error {
	var ver int64 = 1
	old, err := v.st.GetSecret(ctx, id)
	switch {
	case err == nil:
		ver = old.Version + 1
	case !errors.Is(err, store.ErrNotFound):
		return err
	}
	nonce, ct, err := seal(v.aead, id, ver, value)
	if err != nil {
		return err
	}
	hint := ""
	if id != CanaryID {
		hint = store.Hint(value)
	}
	return v.st.PutSecret(ctx, store.SecretRow{ID: id, Version: ver, Nonce: nonce, Ciphertext: ct, Hint: hint, UpdatedAt: store.Now()})
}

func (v *Vault) decryptLocked(ctx context.Context, id string) ([]byte, error) {
	r, err := v.st.GetSecret(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return open(v.aead, r)
}

// checkID refuses empty IDs, reserved IDs ("vlt." prefix) and IDs with
// characters other than letters, digits, ".", "_" and "-".
func checkID(id string) error {
	if id == "" || len(id) > 64 || strings.HasPrefix(id, "vlt.") {
		return ErrBadID
	}
	for _, c := range id {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-'
		if !ok {
			return ErrBadID
		}
	}
	return nil
}

// Status returns the Vault status.
func (v *Vault) Status() Status {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.status
}

// Set encrypts and stores the Secret. It increases the version.
func (v *Vault) Set(ctx context.Context, id string, value []byte) error {
	if err := checkID(id); err != nil {
		return err
	}
	if len(value) == 0 {
		return errors.New("vault: empty secret")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.usable() {
		return ErrUnavailable
	}
	return v.putLocked(ctx, id, value)
}

// Delete removes the Secret.
func (v *Vault) Delete(ctx context.Context, id string) error {
	if err := checkID(id); err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.usable() {
		return ErrUnavailable
	}
	err := v.st.DeleteSecret(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// SecretStatus returns set or not set, the hint and updated_at. Never the
// value. It works in all Vault states.
func (v *Vault) SecretStatus(ctx context.Context, id string) (SecretStatus, error) {
	if err := checkID(id); err != nil {
		return SecretStatus{}, err
	}
	r, err := v.st.GetSecret(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return SecretStatus{ID: id}, nil
	}
	if err != nil {
		return SecretStatus{}, err
	}
	return SecretStatus{ID: id, Set: true, Hint: r.Hint, UpdatedAt: r.UpdatedAt}, nil
}

// Use gives the Secret bytes to fn. The buffer is cleared after fn returns.
// fn must not keep the bytes.
//
// Only code in internal/vault can call Use in this instruction (go/ast test).
func (v *Vault) Use(ctx context.Context, id string, fn func([]byte) error) error {
	if err := checkID(id); err != nil {
		return err
	}
	v.mu.Lock()
	if !v.usable() {
		v.mu.Unlock()
		return ErrUnavailable
	}
	buf, err := v.decryptLocked(ctx, id)
	v.mu.Unlock()
	if err != nil {
		return err
	}
	defer wipe(buf)
	return fn(buf)
}

// Reset deletes all Secrets. On a platform where vltd makes the DEK
// (Windows), it makes a new DEK. Otherwise (Linux) it keeps the DEK and
// makes a new canary. The confirmation text must be "RESET VAULT".
func (v *Vault) Reset(ctx context.Context, confirmation string) error {
	if subtle.ConstantTimeCompare([]byte(confirmation), []byte(ResetConfirmation)) != 1 {
		return ErrConfirmation
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	var (
		dek  []byte
		mode string
		err  error
	)
	if v.kp.CanCreate() {
		dek, mode, err = v.kp.Create(ctx)
	} else {
		dek, mode, err = v.kp.Load(ctx)
	}
	if err != nil {
		v.log.Error("vault reset failed: no DEK", "err", err)
		v.record(ctx, "vault_reset", "failure", map[string]any{"reason": "no DEK"})
		return fmt.Errorf("vault: reset: %w", err)
	}
	defer wipe(dek)
	mk, err := SettingsMACKey(dek)
	if err != nil {
		return err
	}
	defer wipe(mk)
	if err := v.st.DeleteAllSecrets(ctx); err != nil {
		return err
	}
	// Keep valid settings: a new MAC with the new key.
	if err := v.st.RekeySettings(ctx, mk); err != nil {
		return err
	}
	if err := v.useDEK(dek); err != nil {
		v.setLocked(ctx, "host binding failed", err)
		return err
	}
	if err := v.putLocked(ctx, CanaryID, canaryValue); err != nil {
		v.setLocked(ctx, "store error", err)
		return err
	}
	v.setAvailable(mode)
	v.log.Warn("vault reset: all secrets deleted", "new_dek", v.kp.CanCreate())
	v.record(ctx, "vault_reset", "success", map[string]any{"new_dek": v.kp.CanCreate()})
	return nil
}

func wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
