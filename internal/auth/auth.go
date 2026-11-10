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
	keyFailures   = "failed_count"
	keyBlockUntil = "block_until"
)

// Errors.
var (
	ErrBlocked   = errors.New("auth: PIN entry is blocked")
	ErrNoPIN     = errors.New("auth: no PIN is set")
	ErrIncorrect = errors.New("auth: PIN is not correct")
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

func (m *Manager) state(ctx context.Context) (hash string, failures int, blockUntil time.Time, err error) {
	hash, err = m.st.GetAuthState(ctx, keyPINHash)
	if errors.Is(err, store.ErrNotFound) {
		hash, err = "", nil
	}
	if err != nil {
		return
	}
	if v, e := m.st.GetAuthState(ctx, keyFailures); e == nil {
		failures, _ = strconv.Atoi(v)
	} else if !errors.Is(e, store.ErrNotFound) {
		err = e
		return
	}
	if v, e := m.st.GetAuthState(ctx, keyBlockUntil); e == nil && v != "" {
		blockUntil, _ = time.Parse(time.RFC3339Nano, v)
	} else if e != nil && !errors.Is(e, store.ErrNotFound) {
		err = e
	}
	return
}

// Status returns the PIN state.
func (m *Manager) Status(ctx context.Context) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, f, b, err := m.state(ctx)
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
	s, err := m.Status(ctx)
	return s.PINSet, err
}

// SetPIN sets a new PIN. The caller (IPC) checks the permission: first
// start, or elevated state.
func (m *Manager) SetPIN(ctx context.Context, actor, pin string) error {
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
	if err := m.st.SetAuthState(ctx, map[string]string{keyPINHash: h, keyFailures: "0", keyBlockUntil: ""}); err != nil {
		return err
	}
	m.log.Info("PIN set")
	m.record(ctx, actor, "pin_set", "success", nil)
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
func (m *Manager) Login(ctx context.Context, actor, pin string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, failures, blockUntil, err := m.state(ctx)
	if err != nil {
		return time.Time{}, err
	}
	if h == "" {
		return time.Time{}, ErrNoPIN
	}
	now := m.now()
	if blockUntil.After(now) {
		m.record(ctx, actor, "pin_entry", "blocked", map[string]any{"block_until": blockUntil.UTC().Format(time.RFC3339)})
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
		if err := m.st.SetAuthState(ctx, map[string]string{keyFailures: "0", keyBlockUntil: ""}); err != nil {
			return time.Time{}, err
		}
		m.record(ctx, actor, "pin_entry", "success", nil)
		return time.Time{}, nil
	}
	failures++
	kv := map[string]string{keyFailures: strconv.Itoa(failures)}
	var until time.Time
	if d := blockFor(failures); d > 0 {
		until = now.Add(d)
		kv[keyBlockUntil] = until.UTC().Format(time.RFC3339Nano)
	}
	if err := m.st.SetAuthState(ctx, kv); err != nil {
		return time.Time{}, err
	}
	m.record(ctx, actor, "pin_entry", "failure", map[string]any{"failures": failures})
	if !until.IsZero() {
		m.log.Warn("PIN entry blocked", "failures", failures, "until", until.UTC())
		m.record(ctx, actor, "pin_block", "success", map[string]any{
			"failures": failures, "block_seconds": int(blockFor(failures) / time.Second), "block_until": until.UTC().Format(time.RFC3339),
		})
	}
	return until, ErrIncorrect
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
