// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// Lockout values (step 6, SEC-024).
const (
	MaxFailures   = 5
	FirstBlock    = 5 * time.Minute
	MaxBlock      = 24 * time.Hour
	keyPINHash    = "pin_hash"
	keyFailures   = "failed_count:"
	keyBlockUntil = "block_until:"

	// SECURITY_ALERT (QI-2): more than AlertFailures failures of all
	// sources in AlertWindow.
	AlertFailures    = 100
	AlertWindow      = time.Hour
	ActionAlert      = "SECURITY_ALERT"
	keyWindowStart   = "fail_window_start"
	keyWindowCount   = "fail_window_count"
	keyWindowAlerted = "fail_window_alerted"
)

// Source is the source of a PIN entry (VLT-IS-004 QI-2, SEC-024): the actor
// and the peer identity (Linux: "uid:<n>", Windows: "sid:<SID>", later
// Telegram: "chat:<id>"). The counter and the block are per source.
type Source struct {
	Actor string
	Peer  string
}

// Key returns the auth_state key part of the source.
func (s Source) Key() string { return s.Actor + "|" + s.Peer }

// Errors.
var (
	ErrBlocked   = errors.New("auth: PIN entry is blocked")
	ErrNoPIN     = errors.New("auth: no PIN is set")
	ErrIncorrect = errors.New("auth: PIN is not correct")
	ErrPINIsSet  = errors.New("auth: a PIN is already set")
)

// Auditor writes audit entries.
type Auditor interface {
	Record(ctx context.Context, actor, action, target, result string, details map[string]any) error
}

// Manager checks and sets the PIN. The failure counter and the block end
// time are in the Store (auth_state), so a restart does not reset them.
type Manager struct {
	st     *store.Store
	audit  Auditor
	log    *slog.Logger
	now    func() time.Time
	params Params
	mu     sync.Mutex
}

// New returns a Manager with the default Argon2id parameters.
func New(st *store.Store, audit Auditor, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{st: st, audit: audit, log: log, now: time.Now, params: DefaultParams}
}

func (m *Manager) record(ctx context.Context, actor, action, result string, details map[string]any) {
	if m.audit == nil {
		return
	}
	if err := m.audit.Record(ctx, actor, action, "pin", result, details); err != nil {
		m.log.Error("audit write failed", "action", action, "err", err)
	}
}

// Status is the PIN state. It contains no Secret.
type Status struct {
	PINSet     bool      `json:"pin_set"`
	Failures   int       `json:"failures"`
	BlockUntil time.Time `json:"block_until,omitzero"`
}

func (m *Manager) pinHash(ctx context.Context) (string, error) {
	h, err := m.st.GetAuthState(ctx, keyPINHash)
	if errors.Is(err, store.ErrNotFound) {
		return "", nil
	}
	return h, err
}

func (m *Manager) getInt(ctx context.Context, key string) (int, error) {
	v, err := m.st.GetAuthState(ctx, key)
	if errors.Is(err, store.ErrNotFound) || v == "" {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(v)
	return n, nil
}

func (m *Manager) getTime(ctx context.Context, key string) (time.Time, error) {
	v, err := m.st.GetAuthState(ctx, key)
	if errors.Is(err, store.ErrNotFound) || v == "" {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	t, _ := time.Parse(time.RFC3339Nano, v)
	return t, nil
}

func (m *Manager) state(ctx context.Context, src Source) (hash string, failures int, blockUntil time.Time, err error) {
	if hash, err = m.pinHash(ctx); err != nil {
		return
	}
	if failures, err = m.getInt(ctx, keyFailures+src.Key()); err != nil {
		return
	}
	blockUntil, err = m.getTime(ctx, keyBlockUntil+src.Key())
	return
}

// Status returns the PIN state for one source.
func (m *Manager) Status(ctx context.Context, src Source) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, f, b, err := m.state(ctx, src)
	if err != nil {
		return Status{}, err
	}
	s := Status{PINSet: h != "", Failures: f}
	if b.After(m.now()) {
		s.BlockUntil = b.UTC()
	}
	return s, nil
}

// PINSet reports whether a PIN is set.
func (m *Manager) PINSet(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, err := m.pinHash(ctx)
	return h != "", err
}

// SetPIN sets a new PIN. The caller (IPC) checks the permission: first
// start, or elevated state.
func (m *Manager) SetPIN(ctx context.Context, actor, pin string) error {
	return m.setPIN(ctx, actor, pin, false)
}

// SetInitialPIN sets the first PIN. It returns ErrPINIsSet if a PIN is
// already set. The check and the write are one step, so two requests at
// first start cannot both set a PIN.
func (m *Manager) SetInitialPIN(ctx context.Context, actor, pin string) error {
	return m.setPIN(ctx, actor, pin, true)
}

func (m *Manager) setPIN(ctx context.Context, actor, pin string, initial bool) error {
	if err := ValidatePIN(pin); err != nil {
		m.record(ctx, actor, "pin_set", "rejected", map[string]any{"reason": err.Error()})
		return err
	}
	h, err := Hash(pin, m.params)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if initial {
		cur, err := m.pinHash(ctx)
		if err != nil {
			return err
		}
		if cur != "" {
			m.record(ctx, actor, "pin_set", "rejected", map[string]any{"reason": "a PIN is already set"})
			return ErrPINIsSet
		}
	}
	if err := m.st.SetAuthState(ctx, map[string]string{keyPINHash: h}); err != nil {
		return err
	}
	m.log.Info("PIN set")
	m.record(ctx, actor, "pin_set", "success", nil)
	return nil
}

// RemovePIN removes the PIN after a correct current PIN (VLT-IS-008
// TASK 03-00-00-003). The lockout of Login applies. After a success, no
// PIN is set.
func (m *Manager) RemovePIN(ctx context.Context, src Source, pin string) (time.Time, error) {
	until, err := m.Login(ctx, src, pin)
	if err != nil {
		return until, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.st.SetAuthState(ctx, map[string]string{keyPINHash: ""}); err != nil {
		return time.Time{}, err
	}
	m.log.Info("PIN removed")
	m.record(ctx, src.Actor, "pin_remove", "success", nil)
	return time.Time{}, nil
}

// ClearPIN removes the PIN without the current PIN. The Forgot PIN
// method (VLT-IS-009 OD-40, portable mode) calls it after the
// confirmation "RESET".
func (m *Manager) ClearPIN(ctx context.Context, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, err := m.pinHash(ctx)
	if err != nil {
		return err
	}
	if h == "" {
		return ErrNoPIN
	}
	if err := m.st.SetAuthState(ctx, map[string]string{keyPINHash: ""}); err != nil {
		return err
	}
	m.log.Info("PIN removed (forgot PIN)")
	m.record(ctx, actor, "pin_forgot", "success", nil)
	return nil
}

// blockFor returns the block time after n failures: 5 minutes after 5
// failures, doubled after each new failure, 24 hours maximum.
func blockFor(n int) time.Duration {
	if n < MaxFailures {
		return 0
	}
	d := FirstBlock
	for i := MaxFailures; i < n; i++ {
		d *= 2
		if d >= MaxBlock {
			return MaxBlock
		}
	}
	return d
}

// Login checks the PIN. It returns nil for a correct PIN, ErrIncorrect for
// an incorrect PIN, ErrBlocked during a block (the PIN is not checked) and
// ErrNoPIN when no PIN is set.
//
// The counter and the block are for src only (SEC-024 Revision 03).
func (m *Manager) Login(ctx context.Context, src Source, pin string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	actor := src.Actor
	h, failures, blockUntil, err := m.state(ctx, src)
	if err != nil {
		return time.Time{}, err
	}
	if h == "" {
		return time.Time{}, ErrNoPIN
	}
	now := m.now()
	if blockUntil.After(now) {
		m.record(ctx, actor, "pin_entry", "blocked", map[string]any{"peer": src.Peer, "block_until": blockUntil.UTC().Format(time.RFC3339)})
		return blockUntil, ErrBlocked
	}
	// Length check before the hash: a long input does not cost time.
	ok := false
	if len(pin) <= MaxPINLength {
		ok, err = Verify(pin, h)
		if err != nil {
			m.log.Error("stored PIN hash is not valid")
			return time.Time{}, err
		}
	}
	if ok {
		if err := m.st.SetAuthState(ctx, map[string]string{keyFailures + src.Key(): "0", keyBlockUntil + src.Key(): ""}); err != nil {
			return time.Time{}, err
		}
		m.record(ctx, actor, "pin_entry", "success", map[string]any{"peer": src.Peer})
		return time.Time{}, nil
	}
	failures++
	kv := map[string]string{keyFailures + src.Key(): strconv.Itoa(failures)}
	var until time.Time
	if d := blockFor(failures); d > 0 {
		until = now.Add(d)
		kv[keyBlockUntil+src.Key()] = until.UTC().Format(time.RFC3339Nano)
	}
	alert, total, err := m.countWindow(ctx, now, kv)
	if err != nil {
		return time.Time{}, err
	}
	if err := m.st.SetAuthState(ctx, kv); err != nil {
		return time.Time{}, err
	}
	m.record(ctx, actor, "pin_entry", "failure", map[string]any{"peer": src.Peer, "failures": failures})
	if alert {
		m.log.Error("SECURITY_ALERT: many incorrect PIN entries", "failures_in_window", total)
		m.record(ctx, "system", ActionAlert, "failure", map[string]any{"failures_in_window": total, "window_seconds": int(AlertWindow / time.Second)})
	}
	if !until.IsZero() {
		m.log.Warn("PIN entry blocked", "failures", failures, "until", until.UTC())
		m.record(ctx, actor, "pin_block", "success", map[string]any{
			"peer": src.Peer, "failures": failures, "block_seconds": int(blockFor(failures) / time.Second), "block_until": until.UTC().Format(time.RFC3339),
		})
	}
	return until, ErrIncorrect
}

// countWindow counts one failure of any source in a fixed window of one
// hour. It adds the new window values to kv. It returns true one time per
// window, when the count becomes more than AlertFailures.
func (m *Manager) countWindow(ctx context.Context, now time.Time, kv map[string]string) (bool, int, error) {
	start, err := m.getTime(ctx, keyWindowStart)
	if err != nil {
		return false, 0, err
	}
	count, err := m.getInt(ctx, keyWindowCount)
	if err != nil {
		return false, 0, err
	}
	alerted, err := m.getInt(ctx, keyWindowAlerted)
	if err != nil {
		return false, 0, err
	}
	if start.IsZero() || now.Sub(start) >= AlertWindow || now.Before(start) {
		start, count, alerted = now, 0, 0
		kv[keyWindowStart] = now.UTC().Format(time.RFC3339Nano)
	}
	count++
	alert := count > AlertFailures && alerted == 0
	if alert {
		alerted = 1
	}
	kv[keyWindowCount] = strconv.Itoa(count)
	kv[keyWindowAlerted] = strconv.Itoa(alerted)
	return alert, count, nil
}

// Session limits (step 7.2). The idle time is a Setting.
const MaxSession = 8 * time.Hour

// Session is the elevated state of one IPC connection. There is no token.
// It is not safe for concurrent use; one IPC connection uses it.
type Session struct {
	elevated bool
	start    time.Time
	last     time.Time
}

// Elevate sets the elevated state after a correct PIN.
func (s *Session) Elevate(now time.Time) {
	s.elevated, s.start, s.last = true, now, now
}

// Drop ends the elevated state (logout, connection close).
func (s *Session) Drop() { *s = Session{} }

// Elevated reports the elevated state at time now. The state ends after
// idle without a request, or after 8 hours. Call Touch for each request.
func (s *Session) Elevated(now time.Time, idle time.Duration) bool {
	if !s.elevated {
		return false
	}
	if now.Sub(s.last) >= idle || now.Sub(s.start) >= MaxSession {
		s.Drop()
		return false
	}
	return true
}

// Touch records a request.
func (s *Session) Touch(now time.Time) {
	if s.elevated {
		s.last = now
	}
}

func (s Session) String() string { return fmt.Sprintf("elevated=%v", s.elevated) }
