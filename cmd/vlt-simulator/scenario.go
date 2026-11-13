// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
)

// The waits of scenarios 2 and 7. Tests may shorten them.
var (
	scenario2Wait = 2 * time.Minute
	scenario7Wait = 30 * time.Second
)

// NewRunner returns an empty Runner.
func NewRunner() *Runner { return &Runner{} }

// Runner holds a single timed scenario action (mains restore or end of
// communication loss).
type Runner struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	left   time.Duration
	label  string
	end    time.Time
}

// Cancel stops a pending timed action.
func (r *Runner) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.left = 0
	r.label = ""
	r.end = time.Time{}
}

// After runs fn once after d, unless Cancel is called first.
func (r *Runner) After(d time.Duration, label string, fn func()) {
	r.Cancel()
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancel = cancel
	r.label = label
	r.end = time.Now().Add(d)
	r.mu.Unlock()
	go func() {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.mu.Lock()
			r.cancel = nil
			r.left = 0
			r.label = ""
			r.end = time.Time{}
			r.mu.Unlock()
			fn()
		}
	}()
}

// Left is the remaining time and a short label of a timed scenario, if any.
func (r *Runner) Left() (time.Duration, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.end.IsZero() {
		return 0, ""
	}
	d := time.Until(r.end)
	if d < 0 {
		d = 0
	}
	return d, r.label
}

func applyScenario(m *sim.Model, s *server.Server, r *Runner, n int) {
	switch n {
	case 1:
		r.Cancel()
		s.SetFault(server.FaultNone)
		m.SetMains(false)
	case 2:
		s.SetFault(server.FaultNone)
		m.SetMains(false)
		r.After(scenario2Wait, "mains", func() { m.SetMains(true) })
	case 3:
		r.Cancel()
		s.SetFault(server.FaultNone)
		m.SetMains(true)
		m.SetInputVoltage(180)
	case 4:
		r.Cancel()
		s.SetFault(server.FaultNone)
		m.SetMains(true)
		m.SetInputVoltage(260)
	case 5:
		r.Cancel()
		m.SetLoadPercent(110)
	case 6:
		r.Cancel()
		s.SetFault(server.FaultNone)
		m.SetChargeConsole(15)
		m.SetMains(false)
	case 7:
		s.SetFault(server.FaultNoResponse)
		r.After(scenario7Wait, "comm", func() { s.SetFault(server.FaultNone) })
	case 8:
		r.Cancel()
		m.SetTemperature(55)
	case 9:
		r.Cancel()
		m.ApplyConsoleDefaults()
		s.SetFault(server.FaultNone)
		s.SetOther(server.OtherEcho)
		s.SetDelay(50 * time.Millisecond)
	}
}
