// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"
)

// TestFallbackConfig is VLT-IS-005 TASK 03-00-00-001 step 1.1: the fixed
// values of the Fallback Policy.
func TestFallbackConfig(t *testing.T) {
	want := Config{Action: ActionShutdown, RuntimeBelowMinutes: 2, LowBatteryFlag: true, CommLostOnBatterySecs: 300, CountdownSeconds: 30, CancelWhenMainsReturns: true, ForceCloseApplications: true}
	if FallbackConfig() != want {
		t.Fatalf("%+v", FallbackConfig())
	}
}

// TestFallbackSimulation is the CI Simulation of the Fallback Policy
// (step 1.4): low_battery gives a request, power_fail_short gives a cancel,
// comm_lost_on_battery gives a request at a time equal to or less than comm
// lost time + trigger + countdown + 10 s.
func TestFallbackSimulation(t *testing.T) {
	r := SimulateFallback(t0)
	if !r.Passed || !r.Fallback || r.Version != Version(FallbackConfig()) {
		t.Fatalf("passed=%v fallback=%v", r.Passed, r.Fallback)
	}
	got := map[string]ScenarioResult{}
	for _, s := range r.Scenarios {
		got[s.Name] = s
		t.Logf("%s: outcome=%s at=%.0fs expected=%q pass=%v", s.Name, s.Outcome, s.At, s.Expected, s.Pass)
	}
	if s := got[ScenarioLowBattery]; s.Outcome != OutcomeShutdown || !s.Pass || len(s.Calls) != 1 {
		t.Errorf("low_battery %+v", s)
	}
	if s := got[ScenarioPowerFailShort]; s.Outcome != OutcomeCancel || !s.Pass || len(s.Calls) != 0 {
		t.Errorf("power_fail_short %+v", s)
	}
	c := got[ScenarioCommLostOnBattery]
	deadline := (20*time.Second + 300*time.Second + 30*time.Second + 10*time.Second).Seconds()
	if c.Outcome != OutcomeShutdown || !c.Pass || c.Deadline != deadline || c.At > deadline || c.At <= 20 {
		t.Errorf("comm_lost_on_battery %+v", c)
	}
	// power_fail_long has no pass rule for the Fallback Policy; the
	// discharge of the model ends it with a request.
	if s := got[ScenarioPowerFailLong]; s.Expected != "" || s.Outcome != OutcomeShutdown {
		t.Errorf("power_fail_long %+v", s)
	}
}

// TestCommLostPassRule is Q-IA-4 for a stored policy: a request after the
// deadline fails.
func TestCommLostPassRule(t *testing.T) {
	r := Simulate(DefaultConfig(), t0)
	c := r.Scenarios[3]
	if c.Name != ScenarioCommLostOnBattery || c.Expected != OutcomeShutdown || c.Deadline != 270 || c.At != 260 || !c.Pass {
		t.Fatalf("%+v", c)
	}
	// A late request fails (a deadline less than the request time).
	s := runScenario(DefaultConfig(), ScenarioCommLostOnBattery)
	if s.At > CommLostDeadline(DefaultConfig()).Seconds() {
		t.Fatal("late")
	}
	cfg := DefaultConfig()
	cfg.CommLostOnBatterySecs = 0 // trigger off: no pass rule
	if want, ok := expectedFor(cfg, ScenarioCommLostOnBattery, false); ok {
		t.Errorf("rule for trigger off: %s", want)
	}
}

// TestFallbackService is step 1.2 and 1.3: the Fallback Policy only in the
// Vault state "locked", the audit entry, policy.get "fallback": true and
// "vault_locked" for set, activate and deactivate.
func TestFallbackService(t *testing.T) {
	ctx := context.Background()
	s, _, au := openService(t, &DryRun{})
	state := "available"
	s.SetVaultState(func() string { return state })
	in, err := s.Info(ctx)
	if err != nil || in.Fallback || in.Active {
		t.Fatalf("available: %+v %v", in, err)
	}
	state = "reduced protection"
	if in, _ := s.Info(ctx); in.Fallback {
		t.Fatal("fallback in reduced protection")
	}
	state = "locked"
	in, err = s.Info(ctx)
	if err != nil || !in.Fallback || !in.Active || in.Config != FallbackConfig() || in.Version != Version(FallbackConfig()) {
		t.Fatalf("locked: %+v %v", in, err)
	}
	b, _ := json.Marshal(in)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m["fallback"] != true {
		t.Fatalf("json %s", b)
	}
	// One audit entry when it starts, not one at each refresh.
	s.Tick(ctx, Input{})
	s.Info(ctx)
	n := 0
	for _, e := range au.entries {
		if e == "system policy_fallback_start success" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("audit %v", au.entries)
	}
	if _, err := s.Set(ctx, map[string]json.RawMessage{"countdown_seconds": json.RawMessage("90")}, "gui"); !errors.Is(err, ErrVaultLocked) {
		t.Errorf("set %v", err)
	}
	if err := s.Activate(ctx, "gui"); !errors.Is(err, ErrVaultLocked) {
		t.Errorf("activate %v", err)
	}
	if err := s.Deactivate(ctx, "gui"); !errors.Is(err, ErrVaultLocked) {
		t.Errorf("deactivate %v", err)
	}
	// The Fallback Policy acts: LOW_BATTERY on battery starts its countdown.
	steps := s.Tick(ctx, Input{OnBattery: true, LowBattery: true})
	if !has(steps, StepCountdownStart, TriggerLowBattery) {
		t.Errorf("steps %v", kinds(steps))
	}
	if cd := s.Countdown(); !cd.Active || cd.Remaining > 30 {
		t.Errorf("countdown %+v", cd)
	}
	// Unlocked: the stored policy (inactive) is used again.
	state = "available"
	in, _ = s.Info(ctx)
	if in.Fallback || in.Active || in.Config != DefaultConfig() {
		t.Fatalf("unlocked %+v", in)
	}
	if !slices.Contains(au.entries, "system policy_fallback_end success") {
		t.Errorf("audit %v", au.entries)
	}
}

// TestNotConfiguredInactive: "not configured" means no active policy, also
// after a passed Simulation and an activation.
func TestNotConfiguredInactive(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openService(t, &DryRun{})
	if _, err := s.Simulate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	if err := s.Activate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	s.SetVaultState(func() string { return "not configured" })
	if in, _ := s.Info(ctx); in.Active || in.Fallback {
		t.Fatalf("%+v", in)
	}
}
