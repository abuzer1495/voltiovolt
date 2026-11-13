// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

// Link sends a CONTROL command to the UPS. The Poller implements it: it
// sends the command in place of the next fast-group poll and keeps the
// 500 ms minimum time between requests (TASK 03-00-00-005 step 6).
type Link interface {
	// SendControl sends c and returns the time of the request.
	SendControl(ctx context.Context, c Command) (time.Time, error)
}

// Auditor writes the audit entries.
type Auditor interface {
	Record(ctx context.Context, actor, action, target, result string, details map[string]any) error
}

// Status is the status of one fast-group poll (Q1 or QS).
type Status struct {
	At    time.Time
	Flags dialect.Q1Flags
}

// StatusOf returns the status of a parsed fast-group response.
func StatusOf(v any, at time.Time) (Status, bool) {
	switch s := v.(type) {
	case dialect.Status:
		if s.Bits.Raw == "" {
			return Status{}, false
		}
		return Status{At: at, Flags: s.Flags}, true
	case *dialect.Status:
		if s == nil || s.Bits.Raw == "" {
			return Status{}, false
		}
		return Status{At: at, Flags: s.Flags}, true
	}
	return Status{}, false
}

// Test results (TASK 03-00-00-005 step 7).
const (
	TestPassed = "TEST_PASSED"
	TestFailed = "TEST_FAILED"
)

// TestResult is the result of one battery test.
type TestResult struct {
	Result string    `json:"result"` // TEST_PASSED or TEST_FAILED
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	// UPSFailed and BatteryLow are true if b4 or b6 was 1 during the test.
	UPSFailed  bool `json:"ups_failed"`
	BatteryLow bool `json:"battery_low"`
	// Cancelled is true if VLT sent "cancel test" during the test. The
	// result is still TEST_PASSED or TEST_FAILED (step 7).
	Cancelled bool `json:"cancelled,omitempty"`
}

// Result is the result of one CONTROL request.
type Result struct {
	Func     Func   `json:"func"`
	Bytes    string `json:"bytes"` // without CR; empty if nothing was sent
	Sends    int    `json:"sends"`
	Verified bool   `json:"verified"`
	// NoSend is true when the beeper was already in the wanted state.
	NoSend bool `json:"no_send,omitempty"`
	// DryRun is true in demo mode: no CONTROL bytes were sent (OD-43).
	DryRun bool `json:"dry_run,omitempty"`
}

// Verification table of TASK 03-00-00-005 step 4.
type rule struct {
	maxSends int
	// between is the number of polls checked after a send when another
	// send follows (UPSilon 2000 pattern, VLT-REF-001 4.5 and 4.6: one poll
	// between two sends, 2 seconds apart). After the last send, Polls
	// polls are checked.
	between int
	ok      func(prev, s Status) bool
}

// Polls is the number of fast-group polls for a verification.
const Polls = 2

var rules = map[Func]rule{
	FuncTest10s:         {3, 1, func(_, s Status) bool { return s.Flags.TestInProgress }},
	FuncTestMinutes:     {3, 1, func(_, s Status) bool { return s.Flags.TestInProgress }},
	FuncTestUntilLow:    {3, 1, func(_, s Status) bool { return s.Flags.TestInProgress }},
	FuncCancelTest:      {1, 0, func(_, s Status) bool { return !s.Flags.TestInProgress }},
	FuncShutdownRestore: {3, 1, func(_, s Status) bool { return s.Flags.ShutdownActive }},
	// Beeper: 1 send, then 1 retry. Each send is checked with 2 polls,
	// because a second toggle too early would change the beeper back.
	FuncBeeper:         {2, Polls, func(prev, s Status) bool { return s.Flags.BeeperOn != prev.Flags.BeeperOn }},
	FuncCancelShutdown: {1, 0, func(_, s Status) bool { return !s.Flags.ShutdownActive }},
}

// Errors of the Controller.
var (
	ErrNoLink     = errors.New("control: no UPS connected")
	ErrNoStatus   = errors.New("control: no UPS status yet")
	ErrNotSupport = errors.New("control: the Dialect does not support this function")
	ErrOnBattery  = errors.New("control: the UPS is on battery")
	ErrNotVerify  = errors.New("control: the UPS status did not confirm the command")
	ErrBusy       = errors.New("control: another CONTROL request runs")
)

// Controller sends CONTROL commands and verifies them with the next
// fast-group polls. Observe gets each fast-group status.
type Controller struct {
	Audit Auditor
	Log   *slog.Logger
	// PollWait is the maximum wait for one fast-group poll. Zero: 5 s.
	PollWait time.Duration
	// OnTestEnd is called at the end of a test (optional).
	OnTestEnd func(TestResult)

	mu     sync.Mutex
	link   Link
	d      dialect.Dialect
	last   *Status
	hist   []Status      // the last fast-group statuses, in order
	notify chan struct{} // closed at each Observe
	busy   sync.Mutex

	inTest   bool
	cancel   bool // "cancel test" sent during the running test
	test     TestResult
	lastTest *TestResult
	dry      bool // demo mode: do not send CONTROL bytes
}

// SetDryRun enables or disables CONTROL dry runs (VLT-IS-009 OD-43).
func (c *Controller) SetDryRun(on bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dry = on
}

// SetLink sets the connected UPS. A nil link removes it.
func (c *Controller) SetLink(l Link, d dialect.Dialect) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.link, c.d = l, d
	if l == nil {
		c.last = nil
	}
}

// Capabilities returns the functions for the connected UPS.
func (c *Controller) Capabilities() []Func {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.link == nil {
		return nil
	}
	return Capabilities(c.d)
}

// LastStatus returns the last fast-group status.
func (c *Controller) LastStatus() (Status, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last == nil {
		return Status{}, false
	}
	return *c.last, true
}

// LastTest returns the result of the last test.
func (c *Controller) LastTest() (TestResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastTest == nil {
		return TestResult{}, false
	}
	return *c.lastTest, true
}

// Observe gets the status of one fast-group poll. It also finds the end
// of a test: b2 changes from 1 to 0 (step 7).
func (c *Controller) Observe(s Status) {
	var ended *TestResult
	c.mu.Lock()
	switch {
	case s.Flags.TestInProgress && !c.inTest:
		c.inTest = true
		c.cancel = false
		c.test = TestResult{Start: s.At}
		fallthrough
	case s.Flags.TestInProgress:
		c.test.UPSFailed = c.test.UPSFailed || s.Flags.UPSFailed
		c.test.BatteryLow = c.test.BatteryLow || s.Flags.BatteryLow
	case c.inTest:
		c.inTest = false
		r := c.test
		r.End = s.At
		r.Cancelled = c.cancel
		r.Result = TestPassed
		if r.UPSFailed || r.BatteryLow {
			r.Result = TestFailed
		}
		c.lastTest = &r
		ended = &r
	}
	c.last = &s
	c.hist = append(c.hist, s)
	if len(c.hist) > histLen {
		c.hist = c.hist[len(c.hist)-histLen:]
	}
	if c.notify != nil {
		close(c.notify)
		c.notify = nil
	}
	c.mu.Unlock()
	if ended != nil {
		c.record(context.Background(), "system", "ups_test_end", "test", ended.Result, map[string]any{
			"start": ended.Start.UTC().Format(time.RFC3339), "end": ended.End.UTC().Format(time.RFC3339),
			"ups_failed": ended.UPSFailed, "battery_low": ended.BatteryLow, "cancelled": ended.Cancelled,
		})
		if c.OnTestEnd != nil {
			c.OnTestEnd(*ended)
		}
	}
}

const histLen = 256

// polls returns the fast-group statuses of the polls after the time at, in
// order, up to n. It waits for a maximum of PollWait for each poll.
func (c *Controller) polls(ctx context.Context, at time.Time, n int, f func(Status) bool) (bool, int) {
	wait := c.PollWait
	if wait <= 0 {
		wait = 5 * time.Second
	}
	seen := 0
	var lastAt time.Time
	for seen < n {
		c.mu.Lock()
		var nextS *Status
		for i := range c.hist {
			x := c.hist[i]
			if x.At.After(at) && x.At.After(lastAt) {
				nextS = &x
				break
			}
		}
		if nextS == nil {
			if c.notify == nil {
				c.notify = make(chan struct{})
			}
			ch := c.notify
			c.mu.Unlock()
			t := time.NewTimer(wait)
			select {
			case <-ch:
				t.Stop()
				continue
			case <-t.C:
				seen++ // a poll without a status
				continue
			case <-ctx.Done():
				t.Stop()
				return false, seen
			}
		}
		c.mu.Unlock()
		seen++
		lastAt = nextS.At
		if f(*nextS) {
			return true, seen
		}
	}
	return false, seen
}

// actorOf returns the audit actor of a Grant source: "gui" for an IPC
// request, "system" for the Policy Engine and the test end.
func actorOf(s Source) string {
	if s == SourceIPC {
		return "gui"
	}
	return "system"
}

func (c *Controller) record(ctx context.Context, actor, action, target, result string, d map[string]any) {
	if c.Audit == nil {
		return
	}
	if err := c.Audit.Record(ctx, actor, action, target, result, d); err != nil && c.Log != nil {
		c.Log.Error("control: audit", "action", action, "err", err)
	}
}

func (c *Controller) linkFor(f Func) (Link, Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.link == nil {
		return nil, Status{}, ErrNoLink
	}
	if f != FuncShutdownRestore && f != FuncCancelShutdown {
		ok := false
		for _, x := range Capabilities(c.d) {
			ok = ok || x == f
		}
		if !ok {
			return nil, Status{}, ErrNotSupport
		}
	} else if Capabilities(c.d) == nil {
		return nil, Status{}, ErrNotSupport
	}
	if c.last == nil {
		return c.link, Status{}, ErrNoStatus
	}
	return c.link, *c.last, nil
}

// run sends cmd and verifies it (steps 4 and 8). Each send, each
// verification result and each failure has an audit entry.
func (c *Controller) run(ctx context.Context, cmd Command, needStatus bool) (Result, error) {
	f := cmd.Func()
	res := Result{Func: f, Bytes: cmd.Text()}
	base := map[string]any{"func": string(f), "bytes": cmd.Text(), "source": string(cmd.Source())}
	actor := actorOf(cmd.Source())
	fail := func(err error) (Result, error) {
		d := map[string]any{"error": err.Error()}
		for k, v := range base {
			d[k] = v
		}
		c.record(ctx, actor, "ups_control", string(f), "failure", d)
		return res, err
	}
	if !cmd.Valid() {
		return fail(ErrBytes)
	}
	c.mu.Lock()
	dry := c.dry
	c.mu.Unlock()
	if dry {
		res.DryRun = true
		res.Verified = true
		c.record(ctx, actor, "ups_control", string(f), "dry_run", base)
		return res, nil
	}
	// The shutdown and restore command has the first priority (VLT-REF-001
	// 4.4): it does not wait for a running test or beeper request.
	if f != FuncShutdownRestore {
		if !c.busy.TryLock() {
			return fail(ErrBusy)
		}
		defer c.busy.Unlock()
	}
	link, prev, err := c.linkFor(f)
	if err != nil && (needStatus || !errors.Is(err, ErrNoStatus)) {
		return fail(err)
	}
	r := rules[f]
	for res.Sends < r.maxSends {
		at, err := link.SendControl(ctx, cmd)
		if err != nil {
			return fail(fmt.Errorf("send: %w", err))
		}
		res.Sends++
		c.record(ctx, actor, "ups_control", string(f), "sent", map[string]any{"func": string(f), "bytes": cmd.Text(), "source": string(cmd.Source()), "send": res.Sends})
		polls := r.between
		if res.Sends == r.maxSends || polls == 0 {
			polls = Polls
		}
		if ok, n := c.polls(ctx, at, polls, func(s Status) bool { return r.ok(prev, s) }); ok {
			res.Verified = true
			c.record(ctx, actor, "ups_control_verify", string(f), "verified", map[string]any{"func": string(f), "sends": res.Sends, "polls": n})
			return res, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	c.record(ctx, actor, "ups_control_verify", string(f), "not verified", map[string]any{"func": string(f), "sends": res.Sends})
	return fail(ErrNotVerify)
}

// StartTest starts a battery test: kind FuncTest10s, FuncTestMinutes
// (minutes 1 to 99) or FuncTestUntilLow. It rejects a request when the
// UPS is on battery (b7 = 1).
func (c *Controller) StartTest(ctx context.Context, g Grant, kind Func, minutes int) (Result, error) {
	var cmd Command
	var err error
	switch kind {
	case FuncTest10s:
		cmd, err = EncodeTest10s(g)
	case FuncTestMinutes:
		cmd, err = EncodeTestMinutes(g, minutes)
	case FuncTestUntilLow:
		cmd, err = EncodeTestUntilLow(g)
	default:
		err = fmt.Errorf("%w: %s", ErrNotPermitted, kind)
	}
	if err != nil {
		c.record(ctx, actorOf(g.Source()), "ups_control", string(kind), "failure", map[string]any{"func": string(kind), "error": err.Error()})
		return Result{Func: kind}, err
	}
	if s, ok := c.LastStatus(); ok && s.Flags.UtilityFail {
		c.record(ctx, actorOf(g.Source()), "ups_control", string(kind), "failure", map[string]any{"func": string(kind), "error": ErrOnBattery.Error()})
		return Result{Func: kind}, ErrOnBattery
	}
	return c.run(ctx, cmd, true)
}

// CancelTest sends CT and verifies b2 = 0.
func (c *Controller) CancelTest(ctx context.Context, g Grant) (Result, error) {
	cmd, err := EncodeCancelTest(g)
	if err != nil {
		c.record(ctx, actorOf(g.Source()), "ups_control", string(FuncCancelTest), "failure", map[string]any{"func": string(FuncCancelTest), "error": err.Error()})
		return Result{Func: FuncCancelTest}, err
	}
	c.mu.Lock()
	c.cancel = c.inTest
	c.mu.Unlock()
	return c.run(ctx, cmd, true)
}

// SetBeeper sets the wanted beeper state (step 5). It reads b0 first and
// sends Q only if b0 is not the wanted state.
func (c *Controller) SetBeeper(ctx context.Context, g Grant, on bool) (Result, error) {
	cmd, err := EncodeBeeperToggle(g)
	if err != nil {
		c.record(ctx, actorOf(g.Source()), "ups_control", string(FuncBeeper), "failure", map[string]any{"func": string(FuncBeeper), "error": err.Error()})
		return Result{Func: FuncBeeper}, err
	}
	s, ok := c.LastStatus()
	if !ok {
		c.record(ctx, actorOf(g.Source()), "ups_control", string(FuncBeeper), "failure", map[string]any{"func": string(FuncBeeper), "error": ErrNoStatus.Error()})
		return Result{Func: FuncBeeper}, ErrNoStatus
	}
	if s.Flags.BeeperOn == on {
		c.record(ctx, actorOf(g.Source()), "ups_control", string(FuncBeeper), "no send", map[string]any{"func": string(FuncBeeper), "wanted_on": on})
		return Result{Func: FuncBeeper, NoSend: true, Verified: true}, nil
	}
	return c.run(ctx, cmd, true)
}

// ShutdownRestore sends the shutdown and restore command and verifies
// b1 = 1. Only the Policy Engine calls it, with the Policy Grant.
func (c *Controller) ShutdownRestore(ctx context.Context, g Grant, delay time.Duration, restoreMinutes int) (Result, error) {
	cmd, err := EncodeShutdownRestore(g, delay, restoreMinutes)
	if err != nil {
		c.record(ctx, actorOf(g.Source()), "ups_control", string(FuncShutdownRestore), "failure", map[string]any{"func": string(FuncShutdownRestore), "error": err.Error()})
		return Result{Func: FuncShutdownRestore}, err
	}
	return c.run(ctx, cmd, false)
}

// DryShutdownRestore checks the inputs and returns the bytes of the
// shutdown and restore command without a send (the dry-run Simulation of
// the Policy Engine, VLT-IS-006 TASK 03-00-00-006 step 5).
func DryShutdownRestore(g Grant, delay time.Duration, restoreMinutes int) (Result, error) {
	cmd, err := EncodeShutdownRestore(g, delay, restoreMinutes)
	if err != nil {
		return Result{Func: FuncShutdownRestore}, err
	}
	return Result{Func: FuncShutdownRestore, Bytes: cmd.Text()}, nil
}
