// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/poller"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// vclock is a virtual clock for the Poller of the UPS rig. Each Sleep
// also waits 1 ms of real time.
type vclock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *vclock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }

func (c *vclock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if d > 0 {
		c.now = c.now.Add(d)
	}
	c.mu.Unlock()
	time.Sleep(time.Millisecond)
	return ctx.Err()
}

type ctlSink struct{ c *control.Controller }

func (s ctlSink) OnCycle(c poller.Cycle) {
	if c.Group != poller.GroupFast {
		return
	}
	for _, r := range c.Results {
		if st, ok := control.StatusOf(r.Value, r.Time); ok && r.Err == nil {
			s.c.Observe(st)
		}
	}
}
func (ctlSink) OnState(string) {}

// upsRig is a Controller with a Poller over a simulator (SEC-099: no real
// port).
type upsRig struct {
	ctl *control.Controller
	s   *sim.Sim
}

func newUPSRig(t *testing.T) *upsRig {
	t.Helper()
	clk := &vclock{now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	m := sim.NewModel(sim.ProfileQ1)
	m.Now = clk.Now
	s := sim.New(m)
	// The Timeout is real time; 200 ms keeps the detection stable under load.
	tr := s.StreamTransport(transport.Options{Timeout: 200 * time.Millisecond, MinInterval: device.MinGap, FlushTime: time.Millisecond, Now: clk.Now, Sleep: clk.Sleep})
	ctx, cancel := context.WithCancel(context.Background())
	p, _, err := device.Detect(ctx, device.Config{Transport: "serial", SerialPort: "sim0"}, func(context.Context, device.Config) ([]device.Candidate, error) {
		return []device.Candidate{{Transport: "sim", Port: "sim0", New: func() transport.Transport { return tr }}}, nil
	}, clk, quiet())
	if err != nil {
		t.Fatal(err)
	}
	ctl := &control.Controller{PollWait: 2 * time.Second}
	pl := poller.New(tr, p, ctlSink{c: ctl}, poller.Options{Clock: clk, Log: quiet()})
	ctl.SetLink(pl, p.Dialect)
	done := make(chan struct{})
	go func() { _ = pl.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := ctl.LastStatus(); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no status")
		}
		time.Sleep(time.Millisecond)
	}
	return &upsRig{ctl: ctl, s: s}
}

// withUPS gives the env a UPS rig.
func withUPS(t *testing.T, e *env) *upsRig {
	t.Helper()
	r := newUPSRig(t)
	r.ctl.Audit = e.au
	e.srv.b.UPS = r.ctl
	return r
}

func elevatedClient(t *testing.T, e *env) *typed.Client {
	t.Helper()
	ctx := context.Background()
	c := typed.New(e.ln.dial(t))
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestUPSControlIntegration is VLT-IS-006 TASK 03-00-00-007 step 4: test
// start, verification, test end and beeper set through IPC with the
// simulator.
func TestUPSControlIntegration(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	r := withUPS(t, e)
	c := elevatedClient(t, e)

	caps, err := c.UPSControlCapabilities(ctx)
	if err != nil || !caps.Connected || len(caps.Functions) != 5 || caps.LastTest != nil {
		t.Fatalf("capabilities: %+v %v", caps, err)
	}
	for _, f := range caps.Functions {
		if f == string(control.FuncShutdownRestore) || f == string(control.FuncCancelShutdown) {
			t.Fatalf("capability %s", f)
		}
	}
	// Wrong confirmation texts send nothing.
	if _, err := c.UPSTestStart(ctx, wire.TestType10s, 0, "start test"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("lowercase confirmation: %v", err)
	}
	if _, err := c.UPSTestStart(ctx, wire.TestTypeUntilLow, 0, wire.TestConfirmation); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("until low with START TEST: %v", err)
	}
	if _, err := c.UPSTestStart(ctx, wire.TestTypeMinutes, 100, wire.TestConfirmation); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("100 minutes: %v", err)
	}
	if _, err := c.UPSTestStart(ctx, "shutdown", 0, wire.TestConfirmation); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("bad type: %v", err)
	}
	if n := len(r.s.Model.Controls()); n != 0 {
		t.Fatalf("controls before a valid request: %v", r.s.Model.Controls())
	}
	res, err := c.UPSTestStart(ctx, wire.TestType10s, 0, wire.TestConfirmation)
	if err != nil || !res.Verified || res.Bytes != "T" || res.Sends != 1 {
		t.Fatalf("test start: %+v %v", res, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		caps, err = c.UPSControlCapabilities(ctx)
		if err == nil && caps.LastTest != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no test end")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if caps.LastTest.Result != control.TestPassed {
		t.Fatalf("last test %+v", caps.LastTest)
	}
	res, err = c.UPSBeeperSet(ctx, false)
	if err != nil || !res.Verified || res.Bytes != "Q" || r.s.Model.BeeperOn() {
		t.Fatalf("beeper off: %+v %v", res, err)
	}
	res, err = c.UPSBeeperSet(ctx, false)
	if err != nil || !res.NoSend {
		t.Fatalf("beeper off again: %+v %v", res, err)
	}
	if _, err := c.UPSTestStart(ctx, wire.TestTypeMinutes, 2, wire.TestConfirmation); err != nil {
		t.Fatal(err)
	}
	res, err = c.UPSTestCancel(ctx)
	if err != nil || !res.Verified || res.Bytes != "CT" {
		t.Fatalf("cancel: %+v %v", res, err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		if lt, ok := r.ctl.LastTest(); ok && lt.Cancelled {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the cancelled test has no result")
		}
		time.Sleep(time.Millisecond)
	}
	if got := r.s.Model.Controls(); len(got) != 4 || got[0] != "T" || got[1] != "Q" || got[2] != "T02" || got[3] != "CT" {
		t.Fatalf("controls %v", got)
	}
	for _, cmd := range r.s.Received() {
		if e, ok := dialect.LookupRead(cmd); (!ok || e.Class != dialect.ClassRead) && cmd != "T" && cmd != "Q" && cmd != "T02" && cmd != "CT" {
			t.Errorf("command %q", cmd)
		}
	}
	// On battery: ups.test.start is refused.
	r.s.Model.SetMains(false)
	deadline = time.Now().Add(5 * time.Second)
	for {
		if st, ok := r.ctl.LastStatus(); ok && st.Flags.UtilityFail {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no on-battery status")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := c.UPSTestStart(ctx, wire.TestType10s, 0, wire.TestConfirmation); client.Code(err) != wire.CodeOnBattery {
		t.Fatalf("on battery: %v", err)
	}
	// Audit entries of the sends and the IPC requests.
	al, err := c.Audit(ctx, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	n := map[string]int{}
	for _, row := range al.Rows {
		n[row.Action+" "+row.Result]++
	}
	if n["ups_control sent"] != 4 || n["ups_control_verify verified"] != 4 || n["ups_test_end TEST_PASSED"] != 2 ||
		n["ups_test_start success"] != 2 || n["ups_beeper_set success"] != 2 || n["ups_test_cancel success"] != 1 || n["ups_test_start rejected"] < 2 {
		t.Fatalf("audit %v", n)
	}
}

// TestUPSControlAccess: the 3 sending methods need the elevated state; a
// request without it sends nothing. ups.control.capabilities is read only.
func TestUPSControlAccess(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	r := withUPS(t, e)
	c := typed.New(e.ln.dial(t))
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	// OD-50 (VLT-IS-011): the connection keeps the elevated state after
	// set_pin. Lock to test the access without elevation.
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UPSTestStart(ctx, wire.TestType10s, 0, wire.TestConfirmation); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("test start: %v", err)
	}
	if _, err := c.UPSTestCancel(ctx); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("test cancel: %v", err)
	}
	if _, err := c.UPSBeeperSet(ctx, false); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("beeper: %v", err)
	}
	if _, err := c.UPSControlCapabilities(ctx); err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if got := r.s.Model.Controls(); len(got) != 0 {
		t.Fatalf("controls %v", got)
	}
}

// TestUPSTestRejectedDuringCountdown is step 1: no test while a countdown
// runs.
func TestUPSTestRejectedDuringCountdown(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	r := withUPS(t, e)
	e.srv.b.Policy = countdownPolicy{e.srv.b.Policy}
	c := elevatedClient(t, e)
	if _, err := c.UPSTestStart(ctx, wire.TestType10s, 0, wire.TestConfirmation); client.Code(err) != wire.CodeCountdown {
		t.Fatalf("countdown: %v", err)
	}
	if got := r.s.Model.Controls(); len(got) != 0 {
		t.Fatalf("controls %v", got)
	}
}

// countdownPolicy is a PolicyControl with a running countdown.
type countdownPolicy struct{ PolicyControl }

func (countdownPolicy) Countdown() policy.Countdown { return policy.Countdown{Active: true} }
