// SPDX-License-Identifier: AGPL-3.0-or-later

// Package poller polls the UPS in three groups (VLT-IS-004 TASK
// 03-00-00-003 steps 3 to 6): the fast group every 2 seconds, the slow
// group every 30 seconds, and the once group at start and every 6 hours.
// The minimum time between two requests is 500 ms. The fast group has
// priority. After 3 failed fast-group polls in sequence, the state is "comm
// lost" and the Poller reconnects. It sends only READ commands.
package poller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// Group is a poll group.
type Group string

// Poll groups.
const (
	GroupFast Group = "fast"
	GroupSlow Group = "slow"
	GroupOnce Group = "once"
)

// Periods of the groups.
const (
	FastPeriod = 2 * time.Second
	SlowPeriod = 30 * time.Second
	OncePeriod = 6 * time.Hour
	// LostAfter is the number of failed fast-group polls in sequence that
	// gives the state "comm lost".
	LostAfter = 3
)

// Plan is the command list of each group.
type Plan struct {
	Fast, Slow, Once []string
}

// PlanFor returns the poll groups of the table in step 3. A command that
// did not answer at detection is not in the plan ("if supported").
func PlanFor(p device.Profile) Plan {
	var base Plan
	switch p.Dialect {
	case dialect.DialectVoltronic:
		base = Plan{Fast: []string{"QS", "QGS"}, Slow: []string{"QMOD", "QBV", "QWS", "QFLAG"}, Once: []string{"QRI", "I", "QVFW", "QMD"}}
	case dialect.DialectVoltronicQS:
		base = Plan{Fast: []string{"QS", "QMOD"}, Slow: []string{"QBV", "QWS"}, Once: []string{"F", "I"}}
	case dialect.DialectMegatec:
		base = Plan{Fast: []string{"Q1"}, Once: []string{"F", "I"}}
	}
	keep := func(l []string) []string {
		var out []string
		for _, c := range l {
			if p.Supports(c) {
				out = append(out, c)
			}
		}
		return out
	}
	return Plan{Fast: keep(base.Fast), Slow: keep(base.Slow), Once: keep(base.Once)}
}

// Result is one command result.
type Result struct {
	Command string
	Time    time.Time
	Value   any // parsed value; nil if Err is set
	Err     error
}

// Cycle is one poll of a group.
type Cycle struct {
	Group   Group
	Start   time.Time
	End     time.Time
	Results []Result
}

// OK reports whether at least one command gave a valid response.
func (c Cycle) OK() bool {
	for _, r := range c.Results {
		if r.Err == nil {
			return true
		}
	}
	return false
}

// Sink gets the poll results and the state changes. Calls come from the
// Poller goroutine, one at a time.
type Sink interface {
	OnCycle(Cycle)
	OnState(state string)
}

// Options are the Poller options. Zero values give the defaults.
type Options struct {
	Clock device.Clock
	Log   *slog.Logger
}

// ErrDialectChanged is returned when the Dialect after a reconnect is not
// the Dialect of the profile. Then the Device Manager starts a new
// detection.
var ErrDialectChanged = errors.New("poller: the Dialect changed after a reconnect")

type groupState struct {
	g       Group
	period  time.Duration
	cmds    []string
	next    time.Time // due time of the next cycle
	pending []string  // commands of the running cycle
	cur     Cycle
}

// Poller polls one device.
type Poller struct {
	tr     transport.Transport
	prof   device.Profile
	plan   Plan
	sink   Sink
	clock  device.Clock
	log    *slog.Logger
	sender *device.Sender
	groups []*groupState // in priority order: fast, slow, once
	failed int
}

// New returns a Poller for an open Transport.
func New(tr transport.Transport, p device.Profile, sink Sink, o Options) *Poller {
	if o.Clock == nil {
		o.Clock = device.RealClock{}
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	pl := &Poller{tr: tr, prof: p, plan: PlanFor(p), sink: sink, clock: o.Clock, log: o.Log}
	pl.sender = &device.Sender{T: tr, Clock: o.Clock}
	return pl
}

// Plan returns the poll groups.
func (p *Poller) Plan() Plan { return p.plan }

func (p *Poller) reset() {
	now := p.clock.Now()
	p.groups = nil
	for _, g := range []struct {
		g    Group
		per  time.Duration
		cmds []string
	}{{GroupFast, FastPeriod, p.plan.Fast}, {GroupSlow, SlowPeriod, p.plan.Slow}, {GroupOnce, OncePeriod, p.plan.Once}} {
		if len(g.cmds) == 0 {
			continue
		}
		p.groups = append(p.groups, &groupState{g: g.g, period: g.per, cmds: g.cmds, next: now})
	}
	p.failed = 0
}

// Run polls until ctx is done. It returns ctx.Err().
func (p *Poller) Run(ctx context.Context) error {
	if len(p.plan.Fast) == 0 {
		return fmt.Errorf("poller: no fast-group command for the Dialect %q", p.prof.Dialect)
	}
	p.reset()
	p.sink.OnState(device.StateConnected)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		g := p.pick()
		if g == nil {
			if err := p.clock.Sleep(ctx, p.nextDue().Sub(p.clock.Now())); err != nil {
				return err
			}
			continue
		}
		cmd := g.pending[0]
		g.pending = g.pending[1:]
		v, at, err := p.sender.Send(ctx, cmd)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		g.cur.Results = append(g.cur.Results, Result{Command: cmd, Time: at, Value: v, Err: err})
		if len(g.pending) > 0 {
			continue
		}
		// End of the cycle. QA-2: the next cycle starts one period after
		// the start of this cycle, or at the end of this cycle (the later).
		g.cur.End = p.clock.Now()
		g.next = g.cur.Start.Add(g.period)
		if g.cur.End.After(g.next) {
			g.next = g.cur.End
		}
		c := g.cur
		g.cur = Cycle{}
		p.sink.OnCycle(c)
		if g.g != GroupFast {
			continue
		}
		if c.OK() {
			p.failed = 0
			continue
		}
		p.failed++
		if p.failed < LostAfter {
			continue
		}
		p.log.Warn("poller: communication lost", "failed_fast_polls", p.failed)
		p.sink.OnState(device.StateCommLost)
		if err := p.reconnect(ctx); err != nil {
			return err
		}
		p.reset()
		p.sink.OnState(device.StateConnected)
	}
}

// pick returns the group of the next command: a running cycle of the
// fast group first, then a due fast cycle, then the slow group, then the
// once group.
func (p *Poller) pick() *groupState {
	now := p.clock.Now()
	for _, g := range p.groups {
		if len(g.pending) > 0 {
			if g.g == GroupFast {
				return g
			}
			// A due fast cycle has priority over a running slow cycle.
			if f := p.groups[0]; f.g == GroupFast && len(f.pending) == 0 && !now.Before(f.next) {
				p.start(f, now)
				return f
			}
			return g
		}
		if !now.Before(g.next) {
			p.start(g, now)
			return g
		}
	}
	return nil
}

func (p *Poller) start(g *groupState, now time.Time) {
	g.pending = append([]string(nil), g.cmds...)
	g.cur = Cycle{Group: g.g, Start: now}
}

func (p *Poller) nextDue() time.Time {
	var t time.Time
	for _, g := range p.groups {
		if t.IsZero() || g.next.Before(t) {
			t = g.next
		}
	}
	return t
}

// reconnect closes the Transport and opens it again with the waits of
// device.Backoff. It returns nil when the Dialect is the same, and
// ErrDialectChanged when it is not.
func (p *Poller) reconnect(ctx context.Context) error {
	for i := 0; ; i++ {
		p.tr.Close()
		if err := p.clock.Sleep(ctx, device.BackoffAt(i)); err != nil {
			return err
		}
		if err := p.tr.Open(ctx); err != nil {
			p.log.Info("poller: reconnect: cannot open", "attempt", i+1, "err", err)
			continue
		}
		p.sender = &device.Sender{T: p.tr, Clock: p.clock}
		d, err := device.Identify(ctx, p.sender)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			p.log.Info("poller: reconnect: no answer", "attempt", i+1)
			continue
		}
		if d != p.prof.Dialect {
			p.log.Error("poller: reconnect", "err", ErrDialectChanged, "want", string(p.prof.Dialect), "got", string(d))
			return ErrDialectChanged
		}
		p.log.Info("poller: communication restored", "attempt", i+1)
		return nil
	}
}
