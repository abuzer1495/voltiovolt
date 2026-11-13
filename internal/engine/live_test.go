// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/event"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// fakeExecutor is the fake shutdown executor of the integration test
// (SEC-091: only in _test.go). It never calls the OS.
type fakeExecutor struct {
	mu    sync.Mutex
	calls []string
	ch    chan string
}

func (f *fakeExecutor) Shutdown(_ context.Context, r string) error  { return f.add("shutdown") }
func (f *fakeExecutor) Hibernate(_ context.Context, r string) error { return f.add("hibernate") }

func (f *fakeExecutor) add(a string) error {
	f.mu.Lock()
	f.calls = append(f.calls, a)
	f.mu.Unlock()
	select {
	case f.ch <- a:
	default:
	}
	return nil
}

func (f *fakeExecutor) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

var simOpts = transport.Options{Timeout: 30 * time.Millisecond, MinInterval: time.Nanosecond, FlushTime: time.Millisecond}

// withSim gives vltd the simulator Transport and the fake executor.
func withSim(s *sim.Sim, ex *fakeExecutor) func(*Config) {
	return func(c *Config) {
		c.Lister = func(context.Context, device.Config) ([]device.Candidate, error) {
			return []device.Candidate{{Transport: device.TransportSerial, Port: "sim0",
				New: func() transport.Transport { return s.StreamTransport(simOpts) }}}, nil
		}
		c.MakeExecutor = func(func() bool, *audit.Log) policy.Executor { return ex }
	}
}

func waitFor(t *testing.T, d time.Duration, what string, f func() bool) {
	t.Helper()
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if f() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", what)
}

// TestPowerFailureIntegration is VLT-IS-004 TASK 03-00-00-008 step 5:
// vltd with the simulator Transport. The client subscribes, the simulator
// has a power failure and a low battery, the client gets the Events and
// the countdown, and the fake executor gets one call. No real shutdown.
func TestPowerFailureIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("about 25 seconds")
	}
	ctx := context.Background()
	m := sim.NewModel(sim.ProfileQ1)
	s := sim.New(m)
	ex := &fakeExecutor{ch: make(chan string, 4)}
	_, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{dek: testOnlyDEK()}, withSim(s, ex))
	c := dialTest(t, addr)
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	// VLT-IS-006 TASK 03-00-00-004 step 3.1: the default is "not
	// configured"; the simulator gets no byte until the user selects it.
	waitFor(t, 10*time.Second, "not configured", func() bool {
		ds, err := c.DeviceStatus(ctx)
		return err == nil && ds.State == device.StateNotConfigured
	})
	if n := len(s.Received()); n != 0 {
		t.Fatalf("%d commands before the port selection", n)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{"device.transport": "serial", "device.serial_port": "/dev/ttySIM0"}); err != nil {
		t.Fatal(err)
	}
	// Policy: countdown 10 s (the minimum). Simulation, then activation.
	if err := c.ActivatePolicy(ctx); client.Code(err) != wire.CodeNoSimulation {
		t.Fatalf("activate before the Simulation: %v", err)
	}
	if _, err := c.SetPolicy(ctx, map[string]any{"countdown_seconds": 10}); err != nil {
		t.Fatal(err)
	}
	if r, err := c.Simulate(ctx); err != nil || !r.Passed {
		t.Fatalf("simulate: %+v %v", r, err)
	}
	if err := c.ActivatePolicy(ctx); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 30*time.Second, "device connected", func() bool {
		ds, err := c.DeviceStatus(ctx)
		return err == nil && ds.State == device.StateConnected && ds.Profile != nil && ds.Profile.Dialect == dialect.DialectMegatec
	})
	if err := c.Subscribe(ctx); err != nil {
		t.Fatal(err)
	}

	// Power failure with a low battery (Charge < 20 sets status bit 6).
	m.SetCharge(15)
	m.SetMains(false)

	seen := map[string]bool{}
	var countdowns int
	var lastCD policy.Countdown
	deadline := time.After(45 * time.Second)
	gotCall := false
	for !gotCall || !seen["ON_BATTERY"] || !seen["LOW_BATTERY"] || countdowns == 0 || !seen["status"] {
		select {
		case p, ok := <-c.Pushes():
			if !ok {
				t.Fatal("push channel closed")
			}
			switch p.Push {
			case wire.PushEvent:
				var e event.Event
				if err := json.Unmarshal(p.Data, &e); err != nil {
					t.Fatal(err)
				}
				seen[e.Type] = true
			case wire.PushCountdown:
				if err := json.Unmarshal(p.Data, &lastCD); err != nil {
					t.Fatal(err)
				}
				if !lastCD.Active || lastCD.Trigger == "" {
					t.Fatalf("countdown push %s", p.Data)
				}
				countdowns++
				if countdowns == 1 {
					// The countdown cannot be cancelled while LOW_BATTERY
					// is active.
					if err := c.CancelShutdown(ctx); client.Code(err) != wire.CodeLowBattery {
						t.Fatalf("cancel with LOW_BATTERY: %v", err)
					}
				}
			case wire.PushStatus:
				seen["status"] = true
			}
		case a := <-ex.ch:
			if a != "shutdown" {
				t.Fatalf("executor call %q", a)
			}
			gotCall = true
		case <-deadline:
			t.Fatalf("timeout: events %v, countdown pushes %d, executor %v", seen, countdowns, ex.Calls())
		}
	}
	if countdowns < 5 {
		t.Errorf("only %d countdown pushes for a 10-second countdown", countdowns)
	}
	if cd, err := c.ShutdownStatus(ctx); err != nil || !cd.Requested {
		t.Fatalf("shutdown.status: %+v %v", cd, err)
	}
	time.Sleep(1500 * time.Millisecond)
	if n := len(ex.Calls()); n != 1 {
		t.Fatalf("%d executor calls, want 1", n)
	}
	// The events are stored, the simulator got only READ commands.
	list, _, err := c.Events(ctx, 0, 100)
	if err != nil || len(list) < 2 {
		t.Fatalf("events.list: %d %v", len(list), err)
	}
	for _, cmd := range s.Received() {
		if e, ok := dialect.LookupRead(cmd); !ok || e.Class != dialect.ClassRead {
			t.Errorf("command %q is not a READ command of the registry", cmd)
		}
	}
	if _, rows, err := c.Measurements(ctx, "input.voltage", time.Now().Add(-time.Hour), time.Now().Add(time.Minute), "raw"); err != nil || len(rows) == 0 {
		t.Fatalf("measurements.query: %d %v", len(rows), err)
	}
	// VLT-IS-006 TASK 03-00-00-003 step 8: the megatec Dialect gives
	// battery.charge_estimate from Q1.
	if _, rows, err := c.Measurements(ctx, "battery.charge_estimate", time.Now().Add(-time.Hour), time.Now().Add(time.Minute), "raw"); err != nil || len(rows) == 0 {
		t.Fatalf("battery.charge_estimate: %d %v", len(rows), err)
	}
}

// TestUPSOffIntegration is VLT-IS-006 TASK 03-00-00-006 with the
// simulator: at the end of the countdown vltd sends the shutdown and
// restore command (verified with b1 = 1), then calls the executor. The
// simulator gets only READ commands and this one CONTROL command.
func TestUPSOffIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("about 25 seconds")
	}
	ctx := context.Background()
	m := sim.NewModel(sim.ProfileQ1)
	s := sim.New(m)
	ex := &fakeExecutor{ch: make(chan string, 4)}
	_, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{dek: testOnlyDEK()}, withSim(s, ex))
	c := dialTest(t, addr)
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{"device.transport": "serial", "device.serial_port": "/dev/ttySIM0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetPolicy(ctx, map[string]any{"countdown_seconds": 10, "ups_off.enabled": true, "ups_off.delay_minutes": 1, "ups_off.restore_minutes": 1}); err != nil {
		t.Fatal(err)
	}
	if r, err := c.Simulate(ctx); err != nil || !r.Passed {
		t.Fatalf("simulate: %+v %v", r, err)
	}
	if err := c.ActivatePolicy(ctx); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 30*time.Second, "device connected", func() bool {
		ds, err := c.DeviceStatus(ctx)
		return err == nil && ds.State == device.StateConnected
	})
	m.SetCharge(15)
	m.SetMains(false)
	select {
	case a := <-ex.ch:
		if a != "shutdown" {
			t.Fatalf("executor call %q", a)
		}
	case <-time.After(60 * time.Second):
		t.Fatalf("no executor call; controls %v", m.Controls())
	}
	if got := m.Controls(); len(got) != 1 || got[0] != "S01R0001" {
		t.Fatalf("controls %v", got)
	}
	for _, cmd := range s.Received() {
		if e, ok := dialect.LookupRead(cmd); (!ok || e.Class != dialect.ClassRead) && cmd != "S01R0001" {
			t.Errorf("command %q is not a READ command or the expected CONTROL command", cmd)
		}
	}
	// The Policy Engine writes the audit entry of the shutdown request
	// after the executor call returns, so wait for it.
	wants := []string{"ups_control sent", "ups_control_verify verified", "policy_ups_off success", "policy_shutdown_request success"}
	found := map[string]bool{}
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := c.Audit(ctx, 0, 500)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range res.Rows {
			found[r.Action+" "+r.Result] = true
		}
		all := true
		for _, w := range wants {
			all = all && found[w]
		}
		if all || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, want := range wants {
		if !found[want] {
			t.Errorf("no audit entry %q", want)
		}
	}
}

// TestUPSControlVltd: ups.control.capabilities and ups.beeper.set through
// vltd with the simulator (VLT-IS-006 TASK 03-00-00-007).
func TestUPSControlVltd(t *testing.T) {
	if testing.Short() {
		t.Skip("about 10 seconds")
	}
	ctx := context.Background()
	m := sim.NewModel(sim.ProfileQ1)
	s := sim.New(m)
	ex := &fakeExecutor{ch: make(chan string, 4)}
	_, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{dek: testOnlyDEK()}, withSim(s, ex))
	c := dialTest(t, addr)
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if caps, err := c.UPSControlCapabilities(ctx); err != nil || caps.Connected {
		t.Fatalf("capabilities before the port selection: %+v %v", caps, err)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{"device.transport": "serial", "device.serial_port": "/dev/ttySIM0"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 30*time.Second, "capabilities", func() bool {
		caps, err := c.UPSControlCapabilities(ctx)
		return err == nil && caps.Connected && len(caps.Functions) == 5
	})
	// Wait for a status of the Poller.
	time.Sleep(3 * time.Second)
	r, err := c.UPSBeeperSet(ctx, false)
	if err != nil || !r.Verified || m.BeeperOn() {
		t.Fatalf("beeper off: %+v %v", r, err)
	}
	if got := m.Controls(); len(got) != 1 || got[0] != "Q" {
		t.Fatalf("controls %v", got)
	}
}
