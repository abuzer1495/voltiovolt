// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
)

// order records the calls of the fake UPS and the executor in order.
type order struct {
	mu    sync.Mutex
	calls []string
}

func (o *order) add(s string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls = append(o.calls, s)
}

func (o *order) list() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.calls...)
}

type fakeUPS struct {
	o       *order
	err     error
	src     control.Source
	delay   time.Duration
	restore int
	block   chan struct{}
}

func (u *fakeUPS) ShutdownRestore(_ context.Context, g control.Grant, delay time.Duration, restore int) (control.Result, error) {
	if u.block != nil {
		<-u.block
	}
	u.src, u.delay, u.restore = g.Source(), delay, restore
	u.o.add("ups")
	r, err := control.DryShutdownRestore(g, delay, restore)
	if err != nil {
		return r, err
	}
	if u.err != nil {
		r.Sends = 3
		return r, u.err
	}
	r.Sends, r.Verified = 1, true
	return r, nil
}

type orderExec struct{ o *order }

func (x orderExec) Shutdown(context.Context, string) error  { x.o.add("shutdown"); return nil }
func (x orderExec) Hibernate(context.Context, string) error { x.o.add("hibernate"); return nil }

func upsOffConfig() Config {
	c := DefaultConfig()
	c.UPSOffEnabled = true
	c.UPSOffDelayMinutes = 2
	c.UPSOffRestoreMinutes = 1
	return c
}

func runToEnd(t *testing.T, e *Engine) []Step {
	t.Helper()
	now := t0
	var out []Step
	in := Input{OnBattery: true, LowBattery: true, Charge: f(10), Runtime: f(1)}
	for i := 0; i < 70; i++ {
		out = append(out, e.Tick(context.Background(), now, in)...)
		now = now.Add(time.Second)
	}
	return out
}

// TestUPSOffSequence is step 2 and 6: at the end of the countdown the
// Engine sends the shutdown and restore command with the Policy Grant,
// then calls the OS executor.
func TestUPSOffSequence(t *testing.T) {
	o := &order{}
	u := &fakeUPS{o: o}
	au := &rec{}
	e := NewEngine(upsOffConfig(), orderExec{o}, au, nil)
	e.SetActive(true)
	e.SetUPSOff(u, false)
	steps := runToEnd(t, e)
	if got := strings.Join(o.list(), ","); got != "ups,shutdown" {
		t.Fatalf("order %s", got)
	}
	if u.src != control.SourcePolicy || u.delay != 2*time.Minute || u.restore != 1 {
		t.Fatalf("ups call %+v", u)
	}
	if !has(steps, StepUPSOff, "S02R0001") || !has(steps, StepShutdownRequest, "") {
		t.Fatalf("steps %v", kinds(steps))
	}
	ks := kinds(steps)
	if ks[len(ks)-2] != StepUPSOff+":S02R0001" || !strings.HasPrefix(ks[len(ks)-1], StepShutdownRequest) {
		t.Fatalf("steps %v", ks)
	}
	if !contains(au.entries, "system policy_ups_off success") {
		t.Fatalf("audit %v", au.entries)
	}
}

// TestUPSOffVerificationFailure is step 2.3: the executor is called also
// when the verification fails, with an audit entry for the failure.
func TestUPSOffVerificationFailure(t *testing.T) {
	o := &order{}
	u := &fakeUPS{o: o, err: control.ErrNotVerify}
	au := &rec{}
	e := NewEngine(upsOffConfig(), orderExec{o}, au, nil)
	e.SetActive(true)
	e.SetUPSOff(u, false)
	steps := runToEnd(t, e)
	if got := strings.Join(o.list(), ","); got != "ups,shutdown" {
		t.Fatalf("order %s", got)
	}
	if !has(steps, StepUPSOffFailed, "") || !has(steps, StepShutdownRequest, "") {
		t.Fatalf("steps %v", kinds(steps))
	}
	if !contains(au.entries, "system policy_ups_off failure") || !contains(au.entries, "system policy_shutdown_request success") {
		t.Fatalf("audit %v", au.entries)
	}
	// No UPS connection: the executor is called.
	o2 := &order{}
	e2 := NewEngine(upsOffConfig(), orderExec{o2}, &rec{}, nil)
	e2.SetActive(true)
	steps = runToEnd(t, e2)
	if got := strings.Join(o2.list(), ","); got != "shutdown" || !has(steps, StepUPSOffFailed, "no UPS connection") {
		t.Fatalf("order %s steps %v", got, kinds(steps))
	}
}

// TestUPSOffDisabled: with ups_off.enabled off (the default), nothing is
// sent to the UPS.
func TestUPSOffDisabled(t *testing.T) {
	o := &order{}
	e := NewEngine(DefaultConfig(), orderExec{o}, &rec{}, nil)
	e.SetActive(true)
	e.SetUPSOff(&fakeUPS{o: o}, false)
	steps := runToEnd(t, e)
	if got := strings.Join(o.list(), ","); got != "shutdown" || has(steps, StepUPSOff, "") {
		t.Fatalf("order %s steps %v", got, kinds(steps))
	}
}

// TestUPSOffAsync: in vltd the step runs in a new goroutine, so Tick does
// not wait for the verification. The executor call follows the step.
func TestUPSOffAsync(t *testing.T) {
	o := &order{}
	u := &fakeUPS{o: o, block: make(chan struct{})}
	e := NewEngine(upsOffConfig(), orderExec{o}, &rec{}, nil)
	e.SetActive(true)
	e.SetUPSOff(u, true)
	done := make(chan []Step)
	go func() { done <- runToEnd(t, e) }()
	var steps []Step
	select {
	case steps = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Tick waited for the UPS")
	}
	if len(o.list()) != 0 || !has(steps, StepUPSOff, "started") {
		t.Fatalf("calls %v steps %v", o.list(), kinds(steps))
	}
	close(u.block)
	e.Wait()
	if got := strings.Join(o.list(), ","); got != "ups,shutdown" {
		t.Fatalf("order %s", got)
	}
}

// TestSimulationUPSOffStep is step 5: the Simulation result has the UPS
// output off step with its virtual time and the command bytes. The dry run
// sends nothing (dryUPS has no Link).
func TestSimulationUPSOffStep(t *testing.T) {
	c := upsOffConfig()
	c.UPSOffDelayMinutes, c.UPSOffRestoreMinutes = 3, 10
	r := Simulate(c, time.Now())
	if !r.Passed {
		t.Fatalf("not passed: %+v", r)
	}
	n := 0
	for _, sc := range r.Scenarios {
		var up, req *Step
		for i := range sc.Steps {
			switch sc.Steps[i].Kind {
			case StepUPSOff:
				up = &sc.Steps[i]
			case StepShutdownRequest:
				req = &sc.Steps[i]
			}
		}
		if sc.Outcome != OutcomeShutdown {
			if up != nil {
				t.Fatalf("%s: UPS off step without a shutdown", sc.Name)
			}
			continue
		}
		n++
		if up == nil || req == nil || up.Detail != "S03R0010" || up.T <= 0 || up.T != req.T || up.Data["output_off_after_seconds"] != 180.0 {
			t.Fatalf("%s: %+v %+v", sc.Name, up, req)
		}
	}
	if n == 0 {
		t.Fatal("no scenario with a shutdown")
	}
}

// TestVersionUPSOff: the default UPS output off Settings do not change the
// VLT-IS-005 version; a change of one of them gives a new version.
func TestVersionUPSOff(t *testing.T) {
	c := DefaultConfig()
	m := map[string]any{}
	for _, k := range Keys {
		if !upsOffKeys[k] {
			m[k] = c.field(k)
		}
	}
	b, _ := json.Marshal(m)
	sum := sha256.Sum256(b)
	if Version(c) != hex.EncodeToString(sum[:]) {
		t.Fatal("the default version changed")
	}
	seen := map[string]bool{Version(c): true}
	for _, ch := range []func(*Config){
		func(x *Config) { x.UPSOffEnabled = true },
		func(x *Config) { x.UPSOffDelayMinutes = 3 },
		func(x *Config) { x.UPSOffRestoreMinutes = 2 },
	} {
		x := DefaultConfig()
		ch(&x)
		v := Version(x)
		if seen[v] {
			t.Fatalf("same version for %+v", x)
		}
		seen[v] = true
	}
}

// TestUPSOffSettingsInactive is step 4 (SEC-093): a change of the UPS
// output off Settings makes the policy inactive. Values outside the
// ranges are refused.
func TestUPSOffSettingsInactive(t *testing.T) {
	ctx := context.Background()
	s, _, _ := openService(t, &DryRun{})
	if r, err := s.Simulate(ctx, "gui"); err != nil || !r.Passed {
		t.Fatal(err)
	}
	if err := s.Activate(ctx, "gui"); err != nil {
		t.Fatal(err)
	}
	in, err := s.Set(ctx, map[string]json.RawMessage{"ups_off.enabled": json.RawMessage(`true`)}, "gui")
	if err != nil || in.Active || !in.Config.UPSOffEnabled {
		t.Fatalf("%+v %v", in, err)
	}
	for _, kv := range [][2]string{{"ups_off.delay_minutes", "0"}, {"ups_off.delay_minutes", "11"}, {"ups_off.restore_minutes", "0"}, {"ups_off.restore_minutes", "10000"}, {"ups_off.enabled", `"yes"`}} {
		if _, err := s.Set(ctx, map[string]json.RawMessage{kv[0]: json.RawMessage(kv[1])}, "gui"); err == nil {
			t.Errorf("%s=%s accepted", kv[0], kv[1])
		}
	}
	if _, err := s.Set(ctx, map[string]json.RawMessage{"ups_off.delay_minutes": json.RawMessage(`10`), "ups_off.restore_minutes": json.RawMessage(`9999`)}, "gui"); err != nil {
		t.Fatal(err)
	}
	if in, _ := s.Info(ctx); in.Active || in.Config.UPSOffDelayMinutes != 10 || in.Config.UPSOffRestoreMinutes != 9999 {
		t.Fatalf("%+v", in)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
