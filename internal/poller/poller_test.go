// SPDX-License-Identifier: AGPL-3.0-or-later

package poller

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// fakeClock is a virtual clock. Sleep moves the time forward at once and
// calls the hook (the test changes the simulator at virtual times).
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
	hook   func(now time.Time)
}

var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	if d > 0 {
		c.now = c.now.Add(d)
	}
	now, hook := c.now, c.hook
	c.mu.Unlock()
	if hook != nil {
		hook(now)
	}
	return ctx.Err()
}

var testOpts = transport.Options{Timeout: 20 * time.Millisecond, MinInterval: time.Nanosecond, FlushTime: time.Millisecond}

type recSink struct {
	mu      sync.Mutex
	cycles  []Cycle
	states  []string
	stop    func(Cycle) bool
	cancel  context.CancelFunc
	offset  int // commands of the detection
	onState func(string)
}

func (s *recSink) OnCycle(c Cycle) {
	s.mu.Lock()
	s.cycles = append(s.cycles, c)
	stop := s.stop != nil && s.stop(c)
	s.mu.Unlock()
	if stop {
		s.cancel()
	}
}

func (s *recSink) OnState(st string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states = append(s.states, st)
	if s.onState != nil {
		s.onState(st)
	}
}

// setup detects the simulator and returns a Poller and its sink. The run
// stops when the virtual time passes until.
func setup(t *testing.T, s *sim.Sim, until time.Duration) (*Poller, *recSink, *fakeClock, context.Context) {
	t.Helper()
	clk := &fakeClock{now: t0}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tr := s.StreamTransport(testOpts)
	p, err := detect(ctx, s, tr, clk)
	if err != nil {
		t.Fatal(err)
	}
	clk.sleeps = nil
	clk.now = t0
	sink := &recSink{cancel: cancel, offset: len(s.Received())}
	sink.stop = func(c Cycle) bool { return c.End.Sub(t0) >= until }
	return New(tr, p, sink, Options{Clock: clk}), sink, clk, ctx
}

func detect(ctx context.Context, s *sim.Sim, tr transport.Transport, clk device.Clock) (device.Profile, error) {
	p, _, err := device.Detect(ctx, device.DefaultConfig(), func(context.Context, device.Config) ([]device.Candidate, error) {
		return []device.Candidate{{Transport: "sim", Port: "sim0", New: func() transport.Transport { return tr }}}, nil
	}, clk, nil)
	return p, err
}

func checkReadOnly(t *testing.T, s *sim.Sim) {
	t.Helper()
	for _, c := range s.Received() {
		e, ok := dialect.LookupRead(c)
		if !ok || e.Class != dialect.ClassRead {
			t.Errorf("command %q is not a READ command of the registry", c)
		}
	}
}

func TestPlanFor(t *testing.T) {
	all := device.ProbeList
	for _, c := range []struct {
		d    dialect.Dialect
		sup  []string
		want Plan
	}{
		{dialect.DialectVoltronic, all, Plan{Fast: []string{"QGS", "QMOD"}, Slow: []string{"QS", "QBV", "QWS", "QFLAG"}, Once: []string{"QRI", "I", "QVFW", "QMD"}}},
		{dialect.DialectQS, all, Plan{Fast: []string{"QS", "QMOD"}, Slow: []string{"QBV", "QWS"}, Once: []string{"F", "I"}}},
		{dialect.DialectQS, []string{"Q1", "QS", "F", "I"}, Plan{Fast: []string{"QS"}, Once: []string{"F", "I"}}},
		{dialect.DialectQ1, all, Plan{Fast: []string{"Q1"}, Once: []string{"F", "I"}}},
	} {
		got := PlanFor(device.Profile{Dialect: c.d, Supported: c.sup})
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v", c.d, got)
		}
	}
}

// TestTiming is step 7.2: fast every 2 s, slow every 30 s, once at start,
// 500 ms between requests, fast group first.
func TestTiming(t *testing.T) {
	s := sim.New(sim.NewModel(sim.ProfileVoltronic))
	p, sink, _, ctx := setup(t, s, 95*time.Second)
	if err := p.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var fast, slow, once []time.Time
	var reqs []time.Time
	for _, c := range sink.cycles {
		if !c.OK() || len(c.Results) == 0 {
			t.Fatalf("cycle %+v", c)
		}
		for _, r := range c.Results {
			if r.Err != nil {
				t.Fatalf("%s: %v", r.Command, r.Err)
			}
			reqs = append(reqs, r.Time)
		}
		switch c.Group {
		case GroupFast:
			fast = append(fast, c.Start)
		case GroupSlow:
			slow = append(slow, c.Start)
		case GroupOnce:
			once = append(once, c.Start)
		}
	}
	// Request times in send order (results of interleaved cycles).
	sortTimes(reqs)
	for i := 1; i < len(reqs); i++ {
		if d := reqs[i].Sub(reqs[i-1]); d < device.MinGap {
			t.Fatalf("gap %v between request %d and %d", d, i-1, i)
		}
	}
	if len(fast) < 45 {
		t.Fatalf("%d fast cycles", len(fast))
	}
	for i := 1; i < len(fast); i++ {
		if d := fast[i].Sub(fast[i-1]); d != FastPeriod {
			t.Fatalf("fast cycle %d after %v", i, d)
		}
	}
	if len(slow) != 4 || slow[1].Sub(slow[0]) != SlowPeriod || slow[3].Sub(slow[2]) != SlowPeriod {
		t.Fatalf("slow cycles %v", slow)
	}
	if len(once) != 1 {
		t.Fatalf("once cycles %v", once)
	}
	// The fast group is never late: each fast cycle starts on time, so the
	// slow and once commands fill the gaps only.
	checkReadOnly(t, s)
	plan := map[string]bool{}
	for _, c := range append(append(p.Plan().Fast, p.Plan().Slow...), p.Plan().Once...) {
		plan[c] = true
	}
	for _, c := range s.Received()[sink.offset:] {
		if !plan[c] {
			t.Fatalf("command %q is not in the plan", c)
		}
	}
}

func TestOnceEverySixHours(t *testing.T) {
	s := sim.New(sim.NewModel(sim.ProfileQ1))
	p, sink, _, ctx := setup(t, s, OncePeriod+5*time.Second)
	if err := p.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var once []time.Time
	for _, c := range sink.cycles {
		if c.Group == GroupOnce {
			once = append(once, c.Start)
		}
	}
	if len(once) != 2 || once[1].Sub(once[0]) != OncePeriod {
		t.Fatalf("once %v", once)
	}
}

func sortTimes(ts []time.Time) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Before(ts[j-1]); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}

// TestCommLostAndReconnect is step 7.3.
func TestCommLostAndReconnect(t *testing.T) {
	m := sim.NewModel(sim.ProfileQS)
	s := sim.New(m)
	p, sink, clk, ctx := setup(t, s, 10*time.Minute)
	var lostAt, backAt time.Time
	clk.hook = func(now time.Time) {
		switch {
		case lostAt.IsZero() && now.Sub(t0) >= 10*time.Second:
			lostAt = now
			m.SetFault(sim.FaultNoResponse, true)
		case !lostAt.IsZero() && backAt.IsZero() && now.Sub(lostAt) >= 40*time.Second:
			backAt = now
			m.SetFault(sim.FaultNoResponse, false)
		}
	}
	sink.stop = func(c Cycle) bool {
		return !backAt.IsZero() && c.Group == GroupFast && c.OK() && c.End.Sub(backAt) > 10*time.Second
	}
	from, to := -1, -1
	sink.onState = func(st string) {
		clk.mu.Lock()
		defer clk.mu.Unlock()
		switch {
		case st == device.StateCommLost:
			from = len(clk.sleeps)
		case st == device.StateConnected && from >= 0:
			to = len(clk.sleeps)
		}
	}
	if err := p.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if want := []string{device.StateConnected, device.StateCommLost, device.StateConnected}; !reflect.DeepEqual(sink.states, want) {
		t.Fatalf("states %v", sink.states)
	}
	// Exactly 3 failed fast polls before "comm lost".
	failed := 0
	for _, c := range sink.cycles {
		if c.Group == GroupFast && !c.OK() {
			failed++
		}
	}
	if failed != LostAfter {
		t.Fatalf("%d failed fast polls", failed)
	}
	// The reconnect waits: 1, 2, 5, 10, 30, ... seconds.
	// Between "comm lost" and "connected": the waits, and the 500 ms gaps
	// of the Dialect check.
	var waits []time.Duration
	for _, d := range clk.sleeps[from:to] {
		if d != device.MinGap {
			waits = append(waits, d)
		}
	}
	want := []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}
	if !reflect.DeepEqual(waits, want) {
		t.Fatalf("waits %v", waits)
	}
	// After the reconnect, the Dialect check commands, then the fast group.
	checkReadOnly(t, s)
}

// switchTransport opens a second simulator after a reconnect.
type switchTransport struct {
	mu   sync.Mutex
	cur  transport.Transport
	next transport.Transport
}

func (s *switchTransport) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.next != nil {
		s.cur, s.next = s.next, nil
	}
	return s.cur.Open(ctx)
}
func (s *switchTransport) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.cur.Close() }
func (s *switchTransport) Exchange(ctx context.Context, r []byte) (transport.Response, error) {
	s.mu.Lock()
	c := s.cur
	s.mu.Unlock()
	return c.Exchange(ctx, r)
}

func TestDialectChangedAfterReconnect(t *testing.T) {
	m := sim.NewModel(sim.ProfileVoltronic)
	s1 := sim.New(m)
	s2 := sim.New(sim.NewModel(sim.ProfileQ1))
	clk := &fakeClock{now: t0}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sw := &switchTransport{cur: s1.StreamTransport(testOpts)}
	prof, err := detect(ctx, s1, sw, clk)
	if err != nil {
		t.Fatal(err)
	}
	sink := &recSink{cancel: cancel}
	p := New(sw, prof, sink, Options{Clock: clk})
	clk.hook = func(now time.Time) {
		if now.Sub(t0) >= 5*time.Second && !m.HasFault(sim.FaultNoResponse) {
			m.SetFault(sim.FaultNoResponse, true)
			sw.mu.Lock()
			sw.next = s2.StreamTransport(testOpts)
			sw.mu.Unlock()
		}
	}
	if err := p.Run(ctx); !errors.Is(err, ErrDialectChanged) {
		t.Fatalf("err %v", err)
	}
	if got := s2.Received(); !reflect.DeepEqual(got, device.IdentifyList) {
		t.Fatalf("second device got %v", got)
	}
	checkReadOnly(t, s1)
	checkReadOnly(t, s2)
}

// TestFaultInjection is step 7.4 (faults of VLT-IS-002 TASK 03-00-00-007
// step 4).
func TestFaultInjection(t *testing.T) {
	cases := []struct {
		f      sim.Fault
		hid    bool
		wantOK bool
	}{
		{sim.FaultStale, false, true},
		{sim.FaultSplit, false, true},
		{sim.FaultBrokenRpt, true, false},
		{sim.FaultNAK, false, false},
		{sim.FaultEcho, false, false},
		{sim.FaultNoResponse, false, false},
		{sim.FaultWrongLength, false, false},
		{sim.FaultUPSFailed, false, true},
	}
	for _, c := range cases {
		t.Run(string(c.f), func(t *testing.T) {
			m := sim.NewModel(sim.ProfileVoltronic)
			s := sim.New(m)
			clk := &fakeClock{now: t0}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var tr transport.Transport = s.StreamTransport(testOpts)
			if c.hid {
				h, err := s.HIDTransport(testOpts)
				if err != nil {
					t.Fatal(err)
				}
				tr = h
			}
			prof, err := detect(ctx, s, tr, clk)
			if err != nil {
				t.Fatal(err)
			}
			m.SetFault(c.f, true)
			sink := &recSink{cancel: cancel}
			fast := 0
			sink.stop = func(cy Cycle) bool {
				if cy.Group == GroupFast {
					fast++
				}
				return fast >= 2
			}
			p := New(tr, prof, sink, Options{Clock: clk})
			if err := p.Run(ctx); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			var fc Cycle
			for _, cy := range sink.cycles {
				if cy.Group == GroupFast {
					fc = cy
				}
			}
			if fc.OK() != c.wantOK {
				t.Fatalf("fast cycle OK=%v: %+v", fc.OK(), fc.Results)
			}
			if c.f == sim.FaultUPSFailed {
				st, ok := fc.Results[0].Value.(dialect.StatusQGS)
				if !ok || st.Bits.Raw == "" {
					t.Fatalf("value %+v", fc.Results[0].Value)
				}
			}
			checkReadOnly(t, s)
		})
	}
}
