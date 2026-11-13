// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// ErrNoSimulation is returned by Activate when no passed Simulation of the
// current version exists (SEC-055).
var ErrNoSimulation = errors.New("policy: no passed Simulation of the current policy version")

// ErrNotPolicyKey is returned by Set for a key that is not a policy key.
var ErrNotPolicyKey = errors.New("policy: not a policy Setting")

// ErrVaultLocked is returned by Set, Activate and Deactivate while the
// Fallback Policy is in use (Vault state "locked").
var ErrVaultLocked = errors.New("policy: the Vault is locked; the Fallback Policy is in use")

// Vault states that change the policy (internal/vault).
const (
	vaultLocked        = "locked"
	vaultNotConfigured = "not configured"
)

const stateActiveVersion = "active_version"

// SimRecord is one stored Simulation.
type SimRecord struct {
	ID      int64     `json:"id"`
	Time    time.Time `json:"time"`
	Version string    `json:"version"`
	Passed  bool      `json:"passed"`
	Result  Result    `json:"result"`
}

// Info is the result of policy.get.
type Info struct {
	Config         Config          `json:"settings"`
	Version        string          `json:"version"`
	Active         bool            `json:"active"`
	Fallback       bool            `json:"fallback"` // the Fallback Policy is in use
	LastSimulation *SimRecord      `json:"last_simulation,omitempty"`
	OSShutdown     *OSShutdownStep `json:"os_shutdown,omitempty"`
}

// Clock gives the time.
type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Service is the policy for vltd: the Settings, the version, the active
// state, the Simulation results and the Engine.
type Service struct {
	st    *store.Store
	db    *sql.DB
	audit Auditor
	log   *slog.Logger
	clock Clock
	eng   *Engine

	mu         sync.Mutex
	cfg        Config
	version    string
	vaultState func() string
	fallback   bool
	checker    ShutdownChecker
}

// SetVaultState gives the Service the Vault state. In the state "locked"
// the Service uses the Fallback Policy; in the state "not configured" no
// policy is active (VLT-IS-005 Q-IA-1). Without it, the stored policy is
// used.
func (s *Service) SetVaultState(f func() string) {
	s.mu.Lock()
	s.vaultState = f
	s.mu.Unlock()
}

func (s *Service) vault() string {
	s.mu.Lock()
	f := s.vaultState
	s.mu.Unlock()
	if f == nil {
		return ""
	}
	return f()
}

// Refresh reads the Vault state and the Settings now.
func (s *Service) Refresh(ctx context.Context) error { return s.refresh(ctx) }

// Fallback reports whether the Fallback Policy is in use.
func (s *Service) Fallback() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fallback
}

// NewService loads the policy. The Engine gets exec; only cmd/vltd gives
// the OS executor (SEC-092).
func NewService(ctx context.Context, st *store.Store, audit Auditor, exec Executor, clock Clock, log *slog.Logger) (*Service, error) {
	if log == nil {
		log = slog.Default()
	}
	if clock == nil {
		clock = realClock{}
	}
	s := &Service{st: st, db: st.DB(), audit: audit, log: log, clock: clock}
	s.eng = NewEngine(DefaultConfig(), exec, audit, log)
	if c, ok := exec.(ShutdownChecker); ok {
		s.checker = c
	}
	if err := s.refresh(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// refresh reads the Settings. If the version changed, the policy is
// inactive (SEC-093). The Engine is active only if the stored active
// version is the current version and a passed Simulation of it exists.
func (s *Service) refresh(ctx context.Context) error {
	vs := s.vault()
	if vs == vaultLocked {
		c := FallbackConfig()
		s.mu.Lock()
		was := s.fallback
		s.cfg, s.version, s.fallback = c, Version(c), true
		s.mu.Unlock()
		s.eng.SetConfig(c)
		s.eng.SetActive(true)
		if !was {
			s.log.Warn("policy: the Vault is locked; vltd uses the Fallback Policy", "version", Version(c))
			s.record(ctx, "system", "policy_fallback_start", "success", map[string]any{"version": Version(c), "vault": vs})
		}
		return nil
	}
	s.mu.Lock()
	was := s.fallback
	s.fallback = false
	s.mu.Unlock()
	if was {
		s.log.Info("policy: the Vault is not locked now; the Fallback Policy is not in use")
		s.record(ctx, "system", "policy_fallback_end", "success", map[string]any{"vault": vs})
	}
	c, err := Load(ctx, s.st)
	if err != nil {
		return err
	}
	v := Version(c)
	s.mu.Lock()
	changed := s.version != "" && v != s.version && !was
	s.cfg, s.version = c, v
	s.mu.Unlock()
	active, err := s.isActive(ctx, v)
	if err != nil {
		return err
	}
	if vs == vaultNotConfigured {
		// No active policy until the first-run wizard sets the policy.
		active = false
	}
	s.eng.SetConfig(c)
	if changed && s.eng.Active() && !active {
		s.log.Warn("policy: the Settings changed; the policy is inactive", "version", v)
	}
	s.eng.SetActive(active)
	return nil
}

func (s *Service) isActive(ctx context.Context, version string) (bool, error) {
	var av string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM policy_state WHERE key = ?", stateActiveVersion).Scan(&av)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if av != version {
		return false, nil
	}
	return s.passed(ctx, version)
}

func (s *Service) passed(ctx context.Context, version string) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM policy_simulations WHERE version = ? AND passed = 1", version).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Service) record(ctx context.Context, actor, action, result string, d map[string]any) {
	if s.audit == nil {
		return
	}
	if err := s.audit.Record(ctx, actor, action, "policy", result, d); err != nil {
		s.log.Error("policy: audit write failed", "action", action, "err", err)
	}
}

// Info returns the policy (policy.get).
func (s *Service) Info(ctx context.Context) (Info, error) {
	if err := s.refresh(ctx); err != nil {
		return Info{}, err
	}
	s.mu.Lock()
	in := Info{Config: s.cfg, Version: s.version, Fallback: s.fallback}
	s.mu.Unlock()
	in.Active = s.eng.Active()
	last, err := s.lastSimulation(ctx)
	if err != nil {
		return Info{}, err
	}
	in.LastSimulation = last
	in.OSShutdown = s.checkOSShutdown(ctx)
	return in, nil
}

func (s *Service) lastSimulation(ctx context.Context) (*SimRecord, error) {
	var r SimRecord
	var ts int64
	var passed int
	var res string
	err := s.db.QueryRowContext(ctx, "SELECT id, ts, version, passed, result FROM policy_simulations ORDER BY id DESC LIMIT 1").Scan(&r.ID, &ts, &r.Version, &passed, &res)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Time, r.Passed = time.UnixMilli(ts).UTC(), passed == 1
	if err := json.Unmarshal([]byte(res), &r.Result); err != nil {
		return nil, err
	}
	return &r, nil
}

// Set changes policy Settings (policy.set). The new version is inactive.
// keys are full Setting keys ("policy.countdown_seconds") or short keys
// ("countdown_seconds").
func (s *Service) Set(ctx context.Context, values map[string]json.RawMessage, actor string) (Info, error) {
	if s.vault() == vaultLocked {
		s.record(ctx, actor, "policy_set", "rejected", map[string]any{"reason": "vault locked"})
		return Info{}, ErrVaultLocked
	}
	known := map[string]bool{}
	for _, k := range Keys {
		known[k] = true
	}
	full := map[string]json.RawMessage{}
	for k, v := range values {
		if !strings.HasPrefix(k, "policy.") {
			k = "policy." + k
		}
		if !known[k] {
			return Info{}, fmt.Errorf("%w: %q", ErrNotPolicyKey, k)
		}
		full[k] = v
	}
	if len(full) == 0 {
		return Info{}, errors.New("policy: no Setting to change")
	}
	// Check all values first, so that a bad value changes nothing.
	for _, d := range store.DefaultSettings() {
		if v, ok := full[d.Key]; ok && d.Check != nil {
			if err := d.Check(v); err != nil {
				return Info{}, fmt.Errorf("policy: %s: %w", d.Key, err)
			}
		}
	}
	for _, k := range Keys {
		if v, ok := full[k]; ok {
			if _, err := s.st.SetSetting(ctx, k, v, actor); err != nil {
				return Info{}, err
			}
		}
	}
	if err := s.deactivateState(ctx); err != nil {
		return Info{}, err
	}
	if err := s.refresh(ctx); err != nil {
		return Info{}, err
	}
	in, err := s.Info(ctx)
	if err != nil {
		return Info{}, err
	}
	s.record(ctx, actor, "policy_set", "success", map[string]any{"version": in.Version, "keys": len(full)})
	return in, nil
}

func (s *Service) deactivateState(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM policy_state WHERE key = ?", stateActiveVersion)
	return err
}

// Simulate runs the Simulation of the current version and stores the
// result (policy.simulate).
func (s *Service) Simulate(ctx context.Context, actor string) (Result, error) {
	if err := s.refresh(ctx); err != nil {
		return Result{}, err
	}
	s.mu.Lock()
	c := s.cfg
	s.mu.Unlock()
	r := Simulate(c, s.clock.Now())
	s.applyOSShutdown(ctx, &r)
	b, err := json.Marshal(r)
	if err != nil {
		return Result{}, err
	}
	passed := 0
	if r.Passed {
		passed = 1
	}
	if _, err := s.db.ExecContext(ctx, "INSERT INTO policy_simulations (ts, version, passed, result) VALUES (?, ?, ?, ?)",
		r.Time.UnixMilli(), r.Version, passed, string(b)); err != nil {
		return Result{}, err
	}
	res := "failure"
	if r.Passed {
		res = "success"
	}
	s.record(ctx, actor, "policy_simulate", res, map[string]any{"version": r.Version})
	return r, nil
}

// Activate makes the current version active (policy.activate).
func (s *Service) Activate(ctx context.Context, actor string) error {
	if s.vault() == vaultLocked {
		s.record(ctx, actor, "policy_activate", "rejected", map[string]any{"reason": "vault locked"})
		return ErrVaultLocked
	}
	if err := s.refresh(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	v := s.version
	s.mu.Unlock()
	ok, err := s.passed(ctx, v)
	if err != nil {
		return err
	}
	if !ok {
		s.record(ctx, actor, "policy_activate", "rejected", map[string]any{"version": v, "reason": "no passed Simulation"})
		return ErrNoSimulation
	}
	if _, err := s.db.ExecContext(ctx, "INSERT OR REPLACE INTO policy_state (key, value) VALUES (?, ?)", stateActiveVersion, v); err != nil {
		return err
	}
	s.eng.SetActive(true)
	s.record(ctx, actor, "policy_activate", "success", map[string]any{"version": v})
	return nil
}

// Deactivate makes the policy inactive (policy.deactivate).
func (s *Service) Deactivate(ctx context.Context, actor string) error {
	if s.vault() == vaultLocked {
		s.record(ctx, actor, "policy_deactivate", "rejected", map[string]any{"reason": "vault locked"})
		return ErrVaultLocked
	}
	if err := s.deactivateState(ctx); err != nil {
		return err
	}
	s.eng.SetActive(false)
	s.record(ctx, actor, "policy_deactivate", "success", nil)
	return nil
}

// Tick reads the Settings (a change makes the policy inactive) and runs
// the Engine. vltd calls it after each poll cycle and each second.
func (s *Service) Tick(ctx context.Context, in Input) []Step {
	if err := s.refresh(ctx); err != nil {
		s.log.Error("policy: cannot read the Settings; the policy is inactive", "err", err)
		s.eng.SetActive(false)
	}
	return s.eng.Tick(ctx, s.clock.Now(), in)
}

// Cancel cancels the countdown (shutdown.cancel).
func (s *Service) Cancel(ctx context.Context, actor string) error {
	_, err := s.eng.Cancel(ctx, s.clock.Now(), actor)
	return err
}

// SetUPSOff sets the sender of the UPS output off step (VLT-IS-006 TASK
// 03-00-00-006). vltd gives the control.Controller; the step runs in a
// new goroutine (async), because Tick runs also in the Poller goroutine.
func (s *Service) SetUPSOff(u UPSOff) { s.eng.SetUPSOff(u, true) }

// Countdown returns the countdown state (shutdown.status).
func (s *Service) Countdown() Countdown { return s.eng.Countdown(s.clock.Now()) }

// checkOSShutdown runs the right check when the executor implements
// ShutdownChecker. A nil result means there is no check.
func (s *Service) checkOSShutdown(ctx context.Context) *OSShutdownStep {
	if s.checker == nil {
		return nil
	}
	step := &OSShutdownStep{Name: OSShutdownName, Pass: true}
	if err := s.checker.CanShutdown(ctx); err != nil {
		step.Pass = false
		step.Error = err.Error()
	}
	return step
}

// applyOSShutdown adds the "OS shutdown" step to a Simulation result. If
// the check fails, Passed is false.
func (s *Service) applyOSShutdown(ctx context.Context, r *Result) {
	step := s.checkOSShutdown(ctx)
	if step == nil {
		return
	}
	r.OSShutdown = step
	if !step.Pass {
		r.Passed = false
	}
}
