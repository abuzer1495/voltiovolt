// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// Manager states.
const (
	StateDetecting = "detecting"
	StateNoDevice  = "no device"
	StateConnected = "connected"
	StateCommLost  = "comm lost"
	StateStopped   = "stopped"
	// StateNotConfigured: the user did not select a Transport. vltd sends
	// no byte (VLT-IS-006 TASK 03-00-00-004 step 3.1).
	StateNotConfigured = "not configured"
	// StatePortBusy: another program uses the serial port (step 2.1).
	StatePortBusy = "port busy"
	// StatePortPermission: the user cannot open /dev/ttyUSB* or /dev/ttyACM*
	// (VLT-IS-008 TASK 03-00-00-005).
	StatePortPermission = "port permission denied"
	// StateHIDPermission: the user cannot open a hidraw device.
	StateHIDPermission = "hid permission denied"
	// StatePermissionDenied: both a Linux USB serial port and a hidraw
	// device fail with permission denied.
	StatePermissionDenied = "permission denied"
	// StateSimulatorNotFound: the test connection is selected and
	// vlt-simulator is not listening (VLT-IS-010 TASK 03-00-00-004).
	StateSimulatorNotFound = "simulator not found"
)

// SimulatorRetry is the fixed wait between test-connection attempts
// when the simulator is not found (not the 1/2/5/10/30 Backoff).
const SimulatorRetry = 5 * time.Second

// RetryWait is the wait after a failed detection of the given state.
func RetryWait(state string, attempt int) time.Duration {
	if state == StateSimulatorNotFound {
		return SimulatorRetry
	}
	return BackoffAt(attempt)
}

// Backoff is the wait list for a new detection and for a reconnect
// (TASK 03-00-00-003 step 5): 1, 2, 5, 10 and 30 seconds, then 30 seconds
// each time.
var Backoff = []time.Duration{1 * time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second}

// BackoffAt returns the wait before attempt i (0 is the first).
func BackoffAt(i int) time.Duration {
	if i < len(Backoff) {
		return Backoff[i]
	}
	return Backoff[len(Backoff)-1]
}

// Status is the Device Manager state for device.status.
type Status struct {
	State   string    `json:"state"`
	Profile *Profile  `json:"profile,omitempty"`
	Since   time.Time `json:"since"`
	Config  Config    `json:"config"`
}

// PollFunc polls one detected device until ctx is done or the
// communication cannot come back. It gets the open Transport.
type PollFunc func(ctx context.Context, tr transport.Transport, p Profile, setState func(string)) error

// Manager detects the UPS and runs the Poller. Restart starts a new
// detection with the current Device Settings.
type Manager struct {
	Config func(ctx context.Context) (Config, error)
	List   Lister
	Poll   PollFunc
	Clock  Clock
	Log    *slog.Logger

	mu      sync.Mutex
	status  Status
	restart chan struct{}
}

func (m *Manager) setStatus(state string, p *Profile, c *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State != state {
		m.status.Since = m.Clock.Now()
	}
	m.status.State = state
	if p != nil || state == StateDetecting || state == StateNoDevice || state == StateNotConfigured || state == StatePortBusy || state == StatePortPermission || state == StateHIDPermission || state == StatePermissionDenied || state == StateSimulatorNotFound {
		m.status.Profile = p
	}
	if c != nil {
		m.status.Config = *c
	}
}

// Status returns a copy of the state.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	if s.Profile != nil {
		p := *s.Profile
		p.Supported = append([]string(nil), p.Supported...)
		s.Profile = &p
	}
	return s
}

// Restart stops the Poller and starts a new detection (device.settings.set).
func (m *Manager) Restart() {
	m.mu.Lock()
	ch := m.restart
	m.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Run detects and polls until ctx is done.
func (m *Manager) Run(ctx context.Context) error {
	if m.Clock == nil {
		m.Clock = RealClock{}
	}
	if m.Log == nil {
		m.Log = slog.Default()
	}
	m.mu.Lock()
	m.restart = make(chan struct{}, 1)
	restart := m.restart
	m.mu.Unlock()
	defer m.setStatus(StateStopped, nil, nil)

	attempt := 0
	for ctx.Err() == nil {
		cfg, err := m.Config(ctx)
		if err != nil {
			m.Log.Error("device: cannot read the Device Settings; use the defaults", "err", err)
			cfg = DefaultConfig()
		}
		if cfg.Transport == TransportNotConfigured || cfg.Transport == "" {
			// No detection and no byte until the user selects a Transport.
			m.setStatus(StateNotConfigured, nil, &cfg)
			select {
			case <-restart:
				attempt = 0
				continue
			case <-ctx.Done():
				return nil
			}
		}
		m.setStatus(StateDetecting, nil, &cfg)
		p, tr, err := Detect(ctx, cfg, m.List, m.Clock, m.Log)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			state := StateNoDevice
			switch {
			case errors.Is(err, ErrSimulatorNotFound):
				state = StateSimulatorNotFound
			case errors.Is(err, ErrPermissionDenied):
				state = StatePermissionDenied
			case errors.Is(err, ErrPortPermission):
				state = StatePortPermission
			case errors.Is(err, ErrHIDPermission):
				state = StateHIDPermission
			case errors.Is(err, ErrPortBusy):
				state = StatePortBusy
			case !errors.Is(err, ErrNoDevice):
				m.Log.Error("device: detection failed", "err", err)
			}
			m.setStatus(state, nil, nil)
			wctx, cancel := context.WithCancel(ctx)
			go func() {
				select {
				case <-restart:
					cancel()
				case <-wctx.Done():
				}
			}()
			_ = m.Clock.Sleep(wctx, RetryWait(state, attempt))
			cancel()
			attempt++
			continue
		}
		attempt = 0
		m.setStatus(StateConnected, &p, nil)
		pctx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			select {
			case <-restart:
				m.Log.Info("device: restart detection")
				cancel()
			case <-done:
			}
		}()
		err = m.Poll(pctx, tr, p, func(s string) { m.setStatus(s, nil, nil) })
		close(done)
		cancel()
		tr.Close()
		if err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) {
			m.Log.Error("device: poller stopped", "err", err)
		}
	}
	return nil
}
