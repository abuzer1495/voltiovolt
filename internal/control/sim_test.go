// SPDX-License-Identifier: AGPL-3.0-or-later

package control_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/poller"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// vclock is a virtual clock. Each Sleep also waits 1 ms of real time, so
// the Controller can follow the Poller.
type vclock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *vclock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

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

// timedPort records the virtual time of each write.
type timedPort struct {
	*sim.Sim
	clk    *vclock
	mu     sync.Mutex
	writes []time.Time
}

func (p *timedPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.writes = append(p.writes, p.clk.Now())
	p.mu.Unlock()
	return p.Sim.Write(b)
}

type sink struct{ c *control.Controller }

func (s sink) OnCycle(c poller.Cycle) {
	if c.Group != poller.GroupFast {
		return
	}
	for _, r := range c.Results {
		if st, ok := control.StatusOf(r.Value, r.Time); ok && r.Err == nil {
			s.c.Observe(st)
		}
	}
}
func (sink) OnState(string) {}

type audit struct {
	mu   sync.Mutex
	rows []string
}

func (a *audit) Record(_ context.Context, _, action, _, result string, _ map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, action+" "+result)
	return nil
}

type rig struct {
	s    *sim.Sim
	port *timedPort
	clk  *vclock
	ctl  *control.Controller
	au   *audit
	stop func()
	done chan error
}

func start(t *testing.T, prof sim.Profile) *rig {
	t.Helper()
	clk := &vclock{now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	m := sim.NewModel(prof)
	m.Now = clk.Now
	s := sim.New(m)
	port := &timedPort{Sim: s, clk: clk}
	// The Timeout is real time (the context of Exchange). 200 ms keeps the
	// detection stable on a loaded CI computer.
	opts := transport.Options{Timeout: 200 * time.Millisecond, MinInterval: device.MinGap, FlushTime: time.Millisecond, Now: clk.Now, Sleep: clk.Sleep}
	tr := transport.NewStream(func(context.Context) (transport.Port, error) { return port, nil }, opts)
	ctx, cancel := context.WithCancel(context.Background())
	p, _, err := device.Detect(ctx, device.Config{Transport: "serial", SerialPort: "sim0"}, func(context.Context, device.Config) ([]device.Candidate, error) {
		return []device.Candidate{{Transport: "sim", Port: "sim0", New: func() transport.Transport { return tr }}}, nil
	}, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	au := &audit{}
	ctl := &control.Controller{Audit: au, PollWait: 2 * time.Second}
	pl := poller.New(tr, p, sink{c: ctl}, poller.Options{Clock: clk})
	ctl.SetLink(pl, p.Dialect)
	r := &rig{s: s, port: port, clk: clk, ctl: ctl, au: au, stop: cancel, done: make(chan error, 1)}
	go func() { r.done <- pl.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-r.done })
	// Wait for the first status.
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
	return r
}

func waitFor(t *testing.T, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// checkRecorder: the simulator received only READ commands and the
// CONTROL commands of internal/control (step 10), with 500 ms or more
// between two requests (step 6).
func checkRecorder(t *testing.T, r *rig, controls ...string) {
	t.Helper()
	ok := map[string]bool{}
	for _, c := range controls {
		ok[c] = true
	}
	for _, c := range r.s.Received() {
		e, read := dialect.LookupRead(c)
		if (!read || e.Class != dialect.ClassRead) && !ok[c] {
			t.Errorf("command %q is not a READ command or an expected CONTROL command", c)
		}
	}
	r.port.mu.Lock()
	defer r.port.mu.Unlock()
	for i := 1; i < len(r.port.writes); i++ {
		if g := r.port.writes[i].Sub(r.port.writes[i-1]); g < device.MinGap {
			t.Errorf("gap %s between request %d and %d", g, i-1, i)
		}
	}
}

func grant(t *testing.T, f control.Func, conf string) control.Grant {
	t.Helper()
	g, err := control.IPCGrant(true, f, conf)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestSimTestAndBeeper: test start, verification, test end (TEST_PASSED),
// beeper off and on, with the simulator and the Poller.
func TestSimTestAndBeeper(t *testing.T) {
	r := start(t, sim.ProfileQ1)
	ctx := context.Background()
	res, err := r.ctl.StartTest(ctx, grant(t, control.FuncTest10s, control.ConfirmStartTest), control.FuncTest10s, 0)
	if err != nil || !res.Verified || res.Sends != 1 {
		t.Fatalf("test: %+v %v", res, err)
	}
	waitFor(t, "test end", func() bool { _, ok := r.ctl.LastTest(); return ok })
	if lt, _ := r.ctl.LastTest(); lt.Result != control.TestPassed || lt.End.Sub(lt.Start) < 8*time.Second || lt.End.Sub(lt.Start) > 14*time.Second {
		t.Fatalf("last test %+v", lt)
	}
	res, err = r.ctl.SetBeeper(ctx, grant(t, control.FuncBeeper, ""), false)
	if err != nil || !res.Verified || res.Sends != 1 || r.s.Model.BeeperOn() {
		t.Fatalf("beeper off: %+v %v", res, err)
	}
	res, err = r.ctl.SetBeeper(ctx, grant(t, control.FuncBeeper, ""), false)
	if err != nil || !res.NoSend {
		t.Fatalf("beeper off again: %+v %v", res, err)
	}
	res, err = r.ctl.SetBeeper(ctx, grant(t, control.FuncBeeper, ""), true)
	if err != nil || !res.Verified || !r.s.Model.BeeperOn() {
		t.Fatalf("beeper on: %+v %v", res, err)
	}
	if got := r.s.Model.Controls(); len(got) != 3 || got[0] != "T" || got[1] != "Q" || got[2] != "Q" {
		t.Fatalf("controls %v", got)
	}
	checkRecorder(t, r, "T", "Q")
}

// TestSimTestFailed: the battery becomes low during a test until low:
// TEST_FAILED.
func TestSimTestFailed(t *testing.T) {
	r := start(t, sim.ProfileQ1)
	res, err := r.ctl.StartTest(context.Background(), grant(t, control.FuncTestUntilLow, control.ConfirmTestUntilLow), control.FuncTestUntilLow, 0)
	if err != nil || !res.Verified {
		t.Fatalf("%+v %v", res, err)
	}
	r.s.Model.SetCharge(10)
	waitFor(t, "test end", func() bool { _, ok := r.ctl.LastTest(); return ok })
	if lt, _ := r.ctl.LastTest(); lt.Result != control.TestFailed || !lt.BatteryLow {
		t.Fatalf("%+v", lt)
	}
	checkRecorder(t, r, "TL")
}

// TestSimCancelTest: CT ends a test of 5 minutes.
func TestSimCancelTest(t *testing.T) {
	r := start(t, sim.ProfileQS)
	ctx := context.Background()
	if _, err := r.ctl.StartTest(ctx, grant(t, control.FuncTestMinutes, control.ConfirmStartTest), control.FuncTestMinutes, 5); err != nil {
		t.Fatal(err)
	}
	res, err := r.ctl.CancelTest(ctx, grant(t, control.FuncCancelTest, ""))
	if err != nil || !res.Verified || res.Sends != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	checkRecorder(t, r, "T05", "CT")
}

// TestSimShutdownRestore: b1 = 1 after the send, the output goes off after
// the delay and on after the restore time with mains power.
func TestSimShutdownRestore(t *testing.T) {
	r := start(t, sim.ProfileVoltronic)
	res, err := r.ctl.ShutdownRestore(context.Background(), control.PolicyGrant(), 12*time.Second, 1)
	if err != nil || !res.Verified || res.Bytes != "S.2R0001" {
		t.Fatalf("%+v %v", res, err)
	}
	t1 := r.clk.Now()
	waitFor(t, "output off", func() bool { return !r.s.Model.OutputOn() })
	if d := r.clk.Now().Sub(t1); d > 14*time.Second {
		t.Fatalf("output off after %s", d)
	}
	waitFor(t, "output on", func() bool { return r.s.Model.OutputOn() })
	if d := r.clk.Now().Sub(t1); d < 60*time.Second {
		t.Fatalf("output on after %s", d)
	}
	checkRecorder(t, r, "S.2R0001")
}

// TestSimNoRestoreWithoutMains: without mains power the output stays off.
func TestSimNoRestoreWithoutMains(t *testing.T) {
	r := start(t, sim.ProfileQ1)
	r.s.Model.SetMains(false)
	if _, err := r.ctl.ShutdownRestore(context.Background(), control.PolicyGrant(), time.Minute, 1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "output off", func() bool { return !r.s.Model.OutputOn() })
	t1 := r.clk.Now()
	waitFor(t, "3 minutes", func() bool { return r.clk.Now().Sub(t1) > 3*time.Minute })
	if r.s.Model.OutputOn() {
		t.Fatal("output on without mains")
	}
	r.s.Model.SetMains(true)
	waitFor(t, "output on", func() bool { return r.s.Model.OutputOn() })
}
