// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

type rec struct {
	mu      sync.Mutex
	entries []string
}

func (r *rec) Record(_ context.Context, actor, action, target, result string, d map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, actor+" "+action+" "+result)
	return nil
}

func f(v float64) *float64 { return &v }

type tick struct {
	e   *Engine
	dry *DryRun
	au  *rec
	now time.Time
}

func newTick(c Config) *tick {
	dry, au := &DryRun{}, &rec{}
	e := NewEngine(c, dry, au, nil)
	e.SetActive(true)
	return &tick{e: e, dry: dry, au: au, now: t0}
}

// run ticks every second for d and returns all steps.
func (k *tick) run(d time.Duration, in Input) []Step {
	var out []Step
	for end := k.now.Add(d); k.now.Before(end); k.now = k.now.Add(time.Second) {
		out = append(out, k.e.Tick(context.Background(), k.now, in)...)
	}
	return out
}

func kinds(st []Step) []string {
	var out []string
	for _, s := range st {
		out = append(out, s.Kind+":"+s.Detail)
	}
	return out
}

func has(st []Step, kind, detail string) bool {
	for _, s := range st {
		if s.Kind == kind && (detail == "" || s.Detail == detail) {
			return true
		}
	}
	return false
}

var mains = Input{}
var battery = Input{OnBattery: true, Charge: f(100), Runtime: f(58)}

// TestEachTrigger is step 7: each trigger starts the countdown, and the
// end of the countdown calls the executor.
func TestEachTrigger(t *testing.T) {
	only := func(mod func(*Config)) Config {
		c := DefaultConfig()
		c.OnBatterySeconds, c.RuntimeBelowMinutes, c.ChargeBelowPercent, c.LowBatteryFlag, c.CommLostOnBatterySecs = 0, 0, 0, false, 0
		mod(&c)
		return c
	}
	cases := []struct {
		name    string
		cfg     Config
		in      Input
		trigger string
		after   time.Duration // the trigger is true after this time
	}{
		{"on battery", only(func(c *Config) { c.OnBatterySeconds = 300 }), battery, TriggerOnBattery, 300 * time.Second},
		{"runtime", only(func(c *Config) { c.RuntimeBelowMinutes = 5 }), Input{OnBattery: true, Runtime: f(4)}, TriggerRuntime, 0},
		{"charge", only(func(c *Config) { c.ChargeBelowPercent = 30 }), Input{OnBattery: true, Charge: f(29)}, TriggerCharge, 0},
		{"low battery flag", only(func(c *Config) { c.LowBatteryFlag = true }), Input{OnBattery: true, LowBattery: true}, TriggerLowBattery, 0},
		{"comm lost on battery", only(func(c *Config) { c.CommLostOnBatterySecs = 180 }), Input{OnBattery: true, CommLost: true}, TriggerCommLost, 180 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := newTick(c.cfg)
			k.run(5*time.Second, mains)
			if c.after > 0 {
				if st := k.run(c.after, c.in); has(st, StepCountdownStart, "") {
					t.Fatalf("countdown before %v: %v", c.after, kinds(st))
				}
			}
			st := k.run(time.Second, c.in)
			if !has(st, StepCountdownStart, c.trigger) {
				t.Fatalf("no countdown: %v", kinds(st))
			}
			cd := k.e.Countdown(k.now)
			if !cd.Active || cd.Remaining != 59 || cd.Trigger != c.trigger { // 1 s after the start
				t.Fatalf("countdown %+v", cd)
			}
			st = k.run(59*time.Second, c.in)
			if has(st, StepShutdownRequest, "") || len(k.dry.Calls()) != 0 {
				t.Fatal("request before the end of the countdown")
			}
			st = k.run(2*time.Second, c.in)
			if !has(st, StepShutdownRequest, ActionShutdown) {
				t.Fatalf("no request: %v", kinds(st))
			}
			if calls := k.dry.Calls(); len(calls) != 1 || calls[0].Action != ActionShutdown {
				t.Fatalf("calls %+v", calls)
			}
			// No second call.
			k.run(5*time.Minute, c.in)
			if len(k.dry.Calls()) != 1 {
				t.Fatalf("calls %d", len(k.dry.Calls()))
			}
			want := []string{"system policy_countdown_start success", "system policy_shutdown_request success"}
			if len(k.au.entries) != 2 || k.au.entries[0] != want[0] || k.au.entries[1] != want[1] {
				t.Fatalf("audit %v", k.au.entries)
			}
		})
	}
}

func TestTriggersNeedBattery(t *testing.T) {
	k := newTick(DefaultConfig())
	// Low charge and runtime on mains (after a power failure): no trigger.
	if st := k.run(10*time.Minute, Input{Charge: f(10), Runtime: f(1)}); len(st) != 0 {
		t.Fatalf("%v", kinds(st))
	}
	// Comm lost while on mains: no trigger.
	if st := k.run(10*time.Minute, Input{CommLost: true}); len(st) != 0 {
		t.Fatalf("%v", kinds(st))
	}
	// Zero turns a trigger off.
	c := DefaultConfig()
	c.OnBatterySeconds, c.CommLostOnBatterySecs = 0, 0
	k = newTick(c)
	if st := k.run(time.Hour, Input{OnBattery: true, CommLost: true}); len(st) != 0 {
		t.Fatalf("%v", kinds(st))
	}
}

func TestHibernateAction(t *testing.T) {
	c := DefaultConfig()
	c.Action = ActionHibernate
	k := newTick(c)
	k.run(time.Second, Input{OnBattery: true, LowBattery: true})
	k.run(61*time.Second, Input{OnBattery: true, LowBattery: true})
	if calls := k.dry.Calls(); len(calls) != 1 || calls[0].Action != ActionHibernate {
		t.Fatalf("%+v", calls)
	}
}

// TestCancelPaths is step 7: mains returns, operator cancel, no cancel
// with LOW_BATTERY, cancel_when_mains_returns off, inactive policy.
func TestCancelPaths(t *testing.T) {
	ctx := context.Background()
	start := func(c Config) *tick {
		k := newTick(c)
		k.run(301*time.Second, battery)
		if cd := k.e.Countdown(k.now); !cd.Active {
			t.Fatal("no countdown")
		}
		return k
	}
	// Mains returns.
	k := start(DefaultConfig())
	st := k.run(time.Second, mains)
	if !has(st, StepCancel, ReasonMains) || k.e.Countdown(k.now).Active {
		t.Fatalf("%v", kinds(st))
	}
	k.run(5*time.Minute, mains)
	if len(k.dry.Calls()) != 0 {
		t.Fatal("call after cancel")
	}
	if k.au.entries[1] != "system policy_countdown_cancel success" {
		t.Fatalf("%v", k.au.entries)
	}

	// cancel_when_mains_returns off: the countdown continues.
	c := DefaultConfig()
	c.CancelWhenMainsReturns = false
	k = start(c)
	k.run(61*time.Second, mains)
	if len(k.dry.Calls()) != 1 {
		t.Fatal("no call with cancel_when_mains_returns off")
	}

	// Operator cancel; the same trigger does not start again on battery.
	k = start(DefaultConfig())
	s, err := k.e.Cancel(ctx, k.now, "gui")
	if err != nil || s.Kind != StepCancel || s.Detail != ReasonOperator {
		t.Fatalf("%+v %v", s, err)
	}
	if st := k.run(10*time.Minute, battery); has(st, StepCountdownStart, "") {
		t.Fatalf("restart after operator cancel: %v", kinds(st))
	}
	// A new trigger starts a new countdown.
	st = k.run(time.Second, Input{OnBattery: true, Charge: f(20), Runtime: f(58)})
	if !has(st, StepCountdownStart, TriggerCharge) {
		t.Fatalf("%v", kinds(st))
	}
	if _, err := k.e.Cancel(ctx, k.now, "gui"); err != nil {
		t.Fatal(err)
	}
	// After mains returns, a new power failure starts a new countdown.
	k.run(5*time.Second, mains)
	if st := k.run(301*time.Second, battery); !has(st, StepCountdownStart, TriggerOnBattery) {
		t.Fatalf("%v", kinds(st))
	}

	// LOW_BATTERY: the operator cannot cancel.
	k = newTick(DefaultConfig())
	low := Input{OnBattery: true, LowBattery: true}
	k.run(time.Second, low)
	if cd := k.e.Countdown(k.now); !cd.Active || cd.Cancelable {
		t.Fatalf("%+v", cd)
	}
	if _, err := k.e.Cancel(ctx, k.now, "gui"); !errors.Is(err, ErrLowBattery) {
		t.Fatalf("err %v", err)
	}
	k.run(61*time.Second, low)
	if len(k.dry.Calls()) != 1 {
		t.Fatal("no call with LOW_BATTERY")
	}
	if _, err := k.e.Cancel(ctx, k.now, "gui"); !errors.Is(err, ErrNoCountdown) {
		t.Fatalf("err %v", err)
	}

	// Inactive policy: no countdown; deactivation cancels a countdown.
	k = start(DefaultConfig())
	k.e.SetActive(false)
	if st := k.run(time.Second, battery); !has(st, StepCancel, ReasonInactive) {
		t.Fatalf("%v", kinds(st))
	}
	if st := k.run(time.Hour, low); len(st) != 0 || len(k.dry.Calls()) != 0 {
		t.Fatalf("inactive policy: %v", kinds(st))
	}
}

type failExec struct{ n int }

func (f *failExec) Shutdown(context.Context, string) error  { f.n++; return errors.New("denied") }
func (f *failExec) Hibernate(context.Context, string) error { f.n++; return errors.New("denied") }

func TestExecutorFailureRetry(t *testing.T) {
	fe := &failExec{}
	e := NewEngine(DefaultConfig(), fe, nil, nil)
	e.SetActive(true)
	now := t0
	low := Input{OnBattery: true, LowBattery: true}
	var st []Step
	for i := 0; i < 121; i++ {
		st = append(st, e.Tick(context.Background(), now, low)...)
		now = now.Add(time.Second)
	}
	// Call at 60 s, then again after 30 s and 60 s.
	if fe.n != 3 || !has(st, StepShutdownFailed, "denied") {
		t.Fatalf("calls %d %v", fe.n, kinds(st))
	}
}

func TestVersion(t *testing.T) {
	a := Version(DefaultConfig())
	if len(a) != 64 || a != Version(DefaultConfig()) {
		t.Fatal(a)
	}
	c := DefaultConfig()
	c.CountdownSeconds = 61
	if Version(c) == a {
		t.Fatal("same version after a change")
	}
}

// TestScenarios is step 6 and 7: all scenarios, the pass rule, virtual
// time, dry-run executor only.
func TestScenarios(t *testing.T) {
	start := time.Now()
	r := Simulate(DefaultConfig(), t0)
	if real := time.Since(start); real > 5*time.Second {
		t.Fatalf("Simulation took %v", real)
	}
	if !r.Passed || len(r.Scenarios) != 4 || r.Version != Version(DefaultConfig()) {
		b, _ := json.MarshalIndent(r, "", " ")
		t.Fatalf("%s", b)
	}
	want := map[string]struct {
		outcome string
		calls   int
		steps   []string
	}{
		ScenarioPowerFailLong:     {OutcomeShutdown, 1, []string{"event:ON_BATTERY", "trigger:on_battery_seconds", "countdown_start:on_battery_seconds", "shutdown_request:shutdown"}},
		ScenarioPowerFailShort:    {OutcomeCancel, 0, []string{"event:ON_BATTERY", "trigger:on_battery_seconds", "countdown_start:on_battery_seconds", "event:ONLINE", "cancel:mains returned"}},
		ScenarioLowBattery:        {OutcomeShutdown, 1, []string{"event:ON_BATTERY", "event:LOW_BATTERY", "trigger:charge_below_percent", "countdown_start:charge_below_percent", "cancel_refused:LOW_BATTERY active", "shutdown_request:shutdown"}},
		ScenarioCommLostOnBattery: {OutcomeShutdown, 1, []string{"event:ON_BATTERY", "event:COMM_LOST", "trigger:comm_lost_on_battery_seconds", "countdown_start:comm_lost_on_battery_seconds", "shutdown_request:shutdown"}},
	}
	for _, s := range r.Scenarios {
		w := want[s.Name]
		got := kinds(s.Steps)
		if s.Outcome != w.outcome || len(s.Calls) != w.calls || !equal(got, w.steps) || s.RealMs > 5000 {
			t.Errorf("%s: %s %d %v", s.Name, s.Outcome, len(s.Calls), got)
		}
	}
	// Virtual times of power_fail_long: failure at 10 s, trigger at
	// 310 s, request at 370 s.
	long := r.Scenarios[0]
	if long.Steps[0].T != 10 || long.Steps[1].T != 310 || long.Steps[3].T != 370 {
		t.Errorf("times %+v", long.Steps)
	}
	// A policy without a trigger fails the Simulation.
	c := DefaultConfig()
	c.OnBatterySeconds, c.RuntimeBelowMinutes, c.ChargeBelowPercent, c.LowBatteryFlag, c.CommLostOnBatterySecs = 0, 0, 0, false, 0
	if Simulate(c, t0).Passed {
		t.Error("a policy without triggers passed")
	}
	// cancel_when_mains_returns off: power_fail_short gives a request.
	c = DefaultConfig()
	c.CancelWhenMainsReturns = false
	if Simulate(c, t0).Passed {
		t.Error("power_fail_short without a cancel passed")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func openService(t *testing.T, exec Executor) (*Service, *store.Store, *rec) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "vlt.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.SetMACKey(bytes.Repeat([]byte{7}, 32))
	au := &rec{}
	s, err := NewService(ctx, st, au, exec, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, st, au
}

// TestActivationRules is step 5 (SEC-055, SEC-093).
func TestActivationRules(t *testing.T) {
	ctx := context.Background()
	dry := &DryRun{}
	s, st, _ := openService(t, dry)
	in, err := s.Info(ctx)
	if err != nil || in.Active || in.Version != Version(DefaultConfig()) || in.LastSimulation != nil {
		t.Fatalf("%+v %v", in, err)
	}
	// No Simulation: no activation.
	if err := s.Activate(ctx, "gui"); !errors.Is(err, ErrNoSimulation) {
		t.Fatalf("err %v", err)
	}
	// A passed Simulation of the default policy: activation.
	r, err := s.Simulate(ctx, "gui")
	if err != nil || !r.Passed {
		t.Fatalf("%v %v", r.Passed, err)
	}
	if err := s.Activate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	if in, _ := s.Info(ctx); !in.Active || in.LastSimulation == nil || !in.LastSimulation.Passed || in.LastSimulation.Version != in.Version {
		t.Fatalf("%+v", in)
	}
	// policy.set: a new version, inactive.
	in, err = s.Set(ctx, map[string]json.RawMessage{"countdown_seconds": json.RawMessage(`90`)}, "gui")
	if err != nil || in.Active || in.Version == Version(DefaultConfig()) || in.Config.CountdownSeconds != 90 {
		t.Fatalf("%+v %v", in, err)
	}
	if err := s.Activate(ctx, "gui"); !errors.Is(err, ErrNoSimulation) {
		t.Fatalf("err %v", err)
	}
	// Back to the old values: the old Simulation does not make it active
	// again without an activation.
	if _, err := s.Set(ctx, map[string]json.RawMessage{"countdown_seconds": json.RawMessage(`60`)}, "gui"); err != nil {
		t.Fatal(err)
	}
	if in, _ := s.Info(ctx); in.Active {
		t.Fatal("active after set")
	}
	if err := s.Activate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	// A change through the generic settings.set also makes it inactive.
	if _, err := st.SetSetting(ctx, store.SettingPolicyOnBatterySeconds, json.RawMessage(`120`), "gui"); err != nil {
		t.Fatal(err)
	}
	s.Tick(ctx, Input{})
	if in, _ := s.Info(ctx); in.Active {
		t.Fatal("active after a Setting change")
	}
	// A failed Simulation does not permit activation.
	if _, err := s.Set(ctx, map[string]json.RawMessage{"cancel_when_mains_returns": json.RawMessage(`false`)}, "gui"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Simulate(ctx, "gui"); r.Passed {
		t.Fatal("passed")
	}
	if err := s.Activate(ctx, "gui"); !errors.Is(err, ErrNoSimulation) {
		t.Fatalf("err %v", err)
	}
	// Bad values and keys change nothing.
	v0, _ := s.Info(ctx)
	for _, bad := range []map[string]json.RawMessage{
		{"countdown_seconds": json.RawMessage(`5`)},
		{"countdown_seconds": json.RawMessage(`700`)},
		{"action": json.RawMessage(`"reboot"`)},
		{"ui.language": json.RawMessage(`"tr"`)},
		{"countdown_seconds": json.RawMessage(`30`), "trigger.charge_below_percent": json.RawMessage(`101`)},
		{},
	} {
		if _, err := s.Set(ctx, bad, "gui"); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	if v1, _ := s.Info(ctx); v1.Version != v0.Version {
		t.Fatal("a refused set changed the version")
	}
	// Deactivate.
	if _, err := s.Set(ctx, map[string]json.RawMessage{"cancel_when_mains_returns": json.RawMessage(`true`), "trigger.on_battery_seconds": json.RawMessage(`300`)}, "gui"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Simulate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	if err := s.Activate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	if err := s.Deactivate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	if in, _ := s.Info(ctx); in.Active {
		t.Fatal("active after deactivate")
	}
	if len(dry.Calls()) != 0 {
		t.Fatal("the Simulation used the service executor")
	}
}

// TestServiceTick: an active policy calls its executor; the Simulation
// never calls it.
func TestServiceTick(t *testing.T) {
	ctx := context.Background()
	dry := &DryRun{}
	clk := &fakeClock{now: t0}
	s, _, au := openService(t, dry)
	s.clock = clk
	if _, err := s.Simulate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	if len(dry.Calls()) != 0 {
		t.Fatal("Simulation called the executor")
	}
	if err := s.Activate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	low := Input{OnBattery: true, LowBattery: true}
	for i := 0; i <= 61; i++ {
		s.Tick(ctx, low)
		if i == 1 {
			if cd := s.Countdown(); !cd.Active || cd.Remaining != 59 {
				t.Fatalf("%+v", cd)
			}
			if err := s.Cancel(ctx, "gui"); !errors.Is(err, ErrLowBattery) {
				t.Fatalf("err %v", err)
			}
		}
		clk.now = clk.now.Add(time.Second)
	}
	if len(dry.Calls()) != 1 {
		t.Fatalf("calls %d", len(dry.Calls()))
	}
	want := []string{"gui policy_simulate success", "gui policy_activate success", "system policy_countdown_start success", "gui policy_countdown_cancel rejected", "system policy_shutdown_request success"}
	if !equal(au.entries, want) {
		t.Fatalf("audit %v", au.entries)
	}
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// checkExec is a DryRun executor with a CanShutdown result.
type checkExec struct {
	DryRun
	err error
}

func (c *checkExec) CanShutdown(context.Context) error { return c.err }

// TestOSShutdownCheck is VLT-IS-008 TASK 03-00-00-004: if the user cannot
// shut down the computer, the Simulation step "OS shutdown" shows the
// exact error and the policy cannot be activated.
func TestOSShutdownCheck(t *testing.T) {
	ctx := context.Background()
	const msg = "This user cannot shut down the computer."

	fe := &checkExec{err: errors.New(msg)}
	s, _, _ := openService(t, fe)
	in, err := s.Info(ctx)
	if err != nil || in.OSShutdown == nil || in.OSShutdown.Pass || in.OSShutdown.Error != msg || in.OSShutdown.Name != OSShutdownName {
		t.Fatalf("info %+v %v", in.OSShutdown, err)
	}
	r, err := s.Simulate(ctx, "gui")
	if err != nil || r.Passed || r.OSShutdown == nil || r.OSShutdown.Pass || r.OSShutdown.Error != msg || r.OSShutdown.Name != "OS shutdown" {
		t.Fatalf("sim passed=%v step=%+v err=%v", r.Passed, r.OSShutdown, err)
	}
	if err := s.Activate(ctx, "gui"); !errors.Is(err, ErrNoSimulation) {
		t.Fatalf("activate %v, want ErrNoSimulation", err)
	}

	pe := &checkExec{}
	s2, _, _ := openService(t, pe)
	in2, err := s2.Info(ctx)
	if err != nil || in2.OSShutdown == nil || !in2.OSShutdown.Pass || in2.OSShutdown.Error != "" {
		t.Fatalf("info pass %+v %v", in2.OSShutdown, err)
	}
	r2, err := s2.Simulate(ctx, "gui")
	if err != nil || !r2.Passed || r2.OSShutdown == nil || !r2.OSShutdown.Pass || r2.OSShutdown.Error != "" {
		t.Fatalf("pass sim passed=%v step=%+v err=%v", r2.Passed, r2.OSShutdown, err)
	}
	if err := s2.Activate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
}
