// SPDX-License-Identifier: AGPL-3.0-or-later

// Package demo is the in-process UPS simulator session of the Demo
// transport (VLT-IS-009 TASK 03-00-00-005, OD-43).
package demo

import (
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// PowerFailDuration is the time on battery after "Simulate power failure".
const PowerFailDuration = 60 * time.Second

// Session is one in-process simulator for transport "demo".
type Session struct {
	mu        sync.Mutex
	model     *sim.Model
	sim       *sim.Sim
	active    bool
	failTimer *time.Timer
	message   string // "Demo: the computer would shut down now."
}

// New returns a Session with live, slowly changing values.
func New() *Session {
	m := sim.NewModel(sim.ProfileQ1)
	m.SetLive(true)
	return &Session{model: m, sim: sim.New(m)}
}

// Model is the simulator model (for tests).
func (s *Session) Model() *sim.Model { return s.model }

// Sim is the simulator (for tests).
func (s *Session) Sim() *sim.Sim { return s.sim }

// SetActive records whether the Device Manager is polling this session.
func (s *Session) SetActive(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = on
	if !on {
		s.stopFailLocked()
	}
}

// Active reports whether demo mode is running.
func (s *Session) Active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Candidates returns the only Detect candidate of demo mode: the
// in-process simulator. No real port is listed (SEC-099).
func (s *Session) Candidates() []device.Candidate {
	simu := s.sim
	return []device.Candidate{{
		Transport: device.TransportDemo,
		Port:      "demo",
		New: func() transport.Transport {
			return simu.StreamTransport(transport.Options{
				Timeout:     device.ResponseTimeout,
				MinInterval: time.Millisecond,
				FlushTime:   time.Millisecond,
			})
		},
	}}
}

// PowerFail switches the simulator to battery for d (default
// PowerFailDuration) and then back to mains.
func (s *Session) PowerFail(d time.Duration) {
	if d <= 0 {
		d = PowerFailDuration
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopFailLocked()
	s.model.SetMains(false)
	s.failTimer = time.AfterFunc(d, func() { s.model.SetMains(true) })
}

func (s *Session) stopFailLocked() {
	if s.failTimer != nil {
		s.failTimer.Stop()
		s.failTimer = nil
	}
}

// SetShutdownMessage stores the dry-run shutdown text.
func (s *Session) SetShutdownMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

// ShutdownMessage returns the last dry-run shutdown text.
func (s *Session) ShutdownMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.message
}

// ShutdownMessageEN is the exact English text of a demo countdown end.
const ShutdownMessageEN = "Demo: the computer would shut down now."
