// SPDX-License-Identifier: AGPL-3.0-or-later

// Package device is the Device Manager of vltd (VLT-IS-004 TASK
// 03-00-00-003): it reads the Device Settings, finds the UPS on a
// Transport, selects the Dialect and records the device profile.
//
// The package sends only READ commands of the registry (SEC-037): each
// request comes from dialect.EncodeRead, and transport.Stream refuses all
// other requests.
package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// Transport names of the Device Settings.
const (
	// TransportNotConfigured is the default (VLT-IS-006 TASK 03-00-00-004
	// step 3.1, OD-27): vltd sends no byte until the user selects a
	// Transport.
	TransportNotConfigured = "not_configured"
	// TransportAuto: HID VID 0665 PID 5161 devices and only the serial
	// ports in device.serial.allowed_ports.
	TransportAuto   = "auto"
	TransportHID    = "hid"
	TransportSerial = "serial"
)

// MinGap is the minimum time between two requests (VLT-IS-002 TASK
// 03-00-00-006 step 3).
const MinGap = 500 * time.Millisecond

// ResponseTimeout is the time vltd waits for the CR of a response
// (VLT-IS-006 TASK 03-00-00-003 step 7). Comm lost stays at 3 failed
// fast-group polls in sequence (poller.LostAfter).
const ResponseTimeout = 1500 * time.Millisecond

// ProbeList is the Probe List of VLT-IS-006 TASK 03-00-00-003 step 4, in order.
var ProbeList = []string{"Q1", "F", "I", "QS", "QGS", "QMOD", "QBV", "QWS", "QFLAG", "QRI", "QID", "QVFW", "QMD"}

// ErrNoDevice is returned when no Transport gives a known Dialect.
var ErrNoDevice = errors.New("device: no UPS found")

// ErrPortBusy is returned when no Transport gives a known Dialect and a
// serial port could not open because another program uses it.
var ErrPortBusy = fmt.Errorf("device: %w", transport.ErrPortBusy)

// ErrPortPermission is returned when the open of /dev/ttyUSB* or
// /dev/ttyACM* fails with permission denied (VLT-IS-008 TASK 03-00-00-005).
var ErrPortPermission = errors.New("device: port permission denied")

// ErrHIDPermission is returned when the open of a hidraw device fails
// with permission denied.
var ErrHIDPermission = errors.New("device: HID permission denied")

// ErrPermissionDenied is returned when both a Linux USB serial port and
// a hidraw device fail with permission denied.
var ErrPermissionDenied = errors.New("device: permission denied")

// Config is the Device Settings.
type Config struct {
	Transport    string         `json:"transport"`
	SerialPort   string         `json:"serial_port"`
	DTR          transport.Line `json:"serial_dtr"`
	RTS          transport.Line `json:"serial_rts"`
	AllowedPorts []string       `json:"allowed_ports"`
}

// DefaultConfig is the default of the Device Settings: not configured,
// DTR and RTS unchanged, no allowed serial port.
func DefaultConfig() Config {
	so := transport.DefaultSerialOptions()
	return Config{Transport: TransportNotConfigured, DTR: so.DTR, RTS: so.RTS, AllowedPorts: []string{}}
}

// ConfigFromStore reads the Device Settings.
func ConfigFromStore(ctx context.Context, st *store.Store) (Config, error) {
	c := DefaultConfig()
	for key, out := range map[string]any{
		store.SettingDeviceTransport:    &c.Transport,
		store.SettingDeviceSerial:       &c.SerialPort,
		store.SettingDeviceDTR:          &c.DTR,
		store.SettingDeviceRTS:          &c.RTS,
		store.SettingDeviceAllowedPorts: &c.AllowedPorts,
	} {
		s, err := st.GetSetting(ctx, key)
		if err != nil {
			return c, err
		}
		if err := json.Unmarshal(s.Value, out); err != nil {
			return c, fmt.Errorf("device: setting %s: %w", key, err)
		}
	}
	return c, nil
}

// Clock gives the time and waits. Tests use a virtual clock.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

// RealClock is the system clock.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time { return time.Now() }

// Sleep waits for d or until ctx is done.
func (RealClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Sender sends READ commands with the minimum gap between two requests.
type Sender struct {
	T     transport.Transport
	Clock Clock
	last  time.Time
}

// Send sends one READ command and returns the parsed value. The time of
// the request is the time after the gap.
func (s *Sender) Send(ctx context.Context, cmd string) (any, time.Time, error) {
	req, err := dialect.EncodeRead(cmd)
	if err != nil {
		return nil, time.Time{}, err
	}
	if !s.last.IsZero() {
		if wait := MinGap - s.Clock.Now().Sub(s.last); wait > 0 {
			if err := s.Clock.Sleep(ctx, wait); err != nil {
				return nil, time.Time{}, err
			}
		}
	}
	at := s.Clock.Now()
	s.last = at
	resp, err := s.T.Exchange(ctx, req)
	if err != nil {
		return nil, at, err
	}
	v, err := dialect.Parse(cmd, resp.Raw)
	return v, at, err
}

// ErrNoControl is returned when the Transport cannot send CONTROL commands.
var ErrNoControl = errors.New("device: the Transport cannot send CONTROL commands")

// SendControl sends one CONTROL command of internal/control with the
// minimum gap between two requests (VLT-IS-006 TASK 03-00-00-005 step 6).
// It returns the time of the request.
func (s *Sender) SendControl(ctx context.Context, c control.Command) (time.Time, error) {
	ct, ok := s.T.(transport.Controller)
	if !ok {
		return time.Time{}, ErrNoControl
	}
	if !s.last.IsZero() {
		if wait := MinGap - s.Clock.Now().Sub(s.last); wait > 0 {
			if err := s.Clock.Sleep(ctx, wait); err != nil {
				return time.Time{}, err
			}
		}
	}
	at := s.Clock.Now()
	s.last = at
	return at, ct.Control(ctx, c)
}

// Profile is the device profile (TASK 03-00-00-003 step 2.5).
type Profile struct {
	Transport string               `json:"transport"`
	Port      string               `json:"port"`
	Dialect   dialect.Dialect      `json:"dialect"`
	Supported []string             `json:"supported"`
	Nominal   *dialect.Rating      `json:"nominal,omitempty"`
	Info      *dialect.Information `json:"info,omitempty"`
	ModelData *dialect.ModelData   `json:"model_data,omitempty"`
	Firmware  *dialect.Firmware    `json:"firmware,omitempty"`
	// SerialNumber is "[REDACTED]" if QID answers. The value is never kept.
	SerialNumber string `json:"serial_number,omitempty"`
}

// Supports reports whether cmd gave a valid response at detection.
func (p Profile) Supports(cmd string) bool {
	for _, c := range p.Supported {
		if c == cmd {
			return true
		}
	}
	return false
}

// Probe sends the Probe List in order. A command without a valid response
// is skipped; the next command follows (step 2.3). It returns the valid
// responses by command, and the supported commands in Probe List order.
func Probe(ctx context.Context, s *Sender) (map[string]any, []string, error) {
	vals := map[string]any{}
	var supported []string
	for _, cmd := range ProbeList {
		v, _, err := s.Send(ctx, cmd)
		if ctx.Err() != nil {
			return vals, supported, ctx.Err()
		}
		if err != nil {
			continue
		}
		if cmd == "QID" {
			v = nil // personal data: do not keep it
		}
		vals[cmd] = v
		supported = append(supported, cmd)
	}
	return vals, supported, nil
}

// SelectDialect selects the Dialect from the supported commands (step 2.4):
// Voltronic if QGS answers, QS if QS answers, Q1 if only Q1 answers.
func SelectDialect(supported []string) (dialect.Dialect, bool) {
	has := map[string]bool{}
	for _, c := range supported {
		has[c] = true
	}
	switch {
	case has["QGS"]:
		return dialect.DialectVoltronic, true
	case has["QS"]:
		return dialect.DialectVoltronicQS, true
	case has["Q1"]:
		return dialect.DialectMegatec, true
	}
	return "", false
}

// IdentifyList is the list for the Dialect check after a reconnect.
var IdentifyList = []string{"Q1", "QS", "QGS"}

// Identify sends IdentifyList and returns the Dialect.
func Identify(ctx context.Context, s *Sender) (dialect.Dialect, error) {
	var ok []string
	for _, cmd := range IdentifyList {
		if _, _, err := s.Send(ctx, cmd); err == nil {
			ok = append(ok, cmd)
		} else if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	d, found := SelectDialect(ok)
	if !found {
		return "", ErrNoDevice
	}
	return d, nil
}

func makeProfile(c Candidate, vals map[string]any, supported []string, d dialect.Dialect) Profile {
	p := Profile{Transport: c.Transport, Port: c.Port, Dialect: d, Supported: supported}
	if r, ok := vals["QRI"].(dialect.Rating); ok {
		p.Nominal = &r
	} else if r, ok := vals["F"].(dialect.Rating); ok {
		p.Nominal = &r
	}
	if v, ok := vals["I"].(dialect.Information); ok {
		p.Info = &v
	}
	if v, ok := vals["QMD"].(dialect.ModelData); ok {
		p.ModelData = &v
	}
	if v, ok := vals["QVFW"].(dialect.Firmware); ok {
		p.Firmware = &v
	}
	if p.Supports("QID") {
		p.SerialNumber = redact.Redacted
	}
	return p
}

// Candidate is one Transport to try.
type Candidate struct {
	Transport string // hid, serial or sim
	Port      string
	New       func() transport.Transport
}

// Lister returns the candidates for a Config, in the order of step 2.
type Lister func(ctx context.Context, c Config) ([]Candidate, error)

// Detect tries each candidate in order. It returns the profile and the
// open Transport of the first candidate with a known Dialect.
func Detect(ctx context.Context, c Config, list Lister, clock Clock, log *slog.Logger) (Profile, transport.Transport, error) {
	if log == nil {
		log = slog.Default()
	}
	cands, err := list(ctx, c)
	if err != nil {
		return Profile{}, nil, err
	}
	busy := false
	serialPerm, hidPerm := false, false
	for _, cand := range cands {
		tr := cand.New()
		if err := tr.Open(ctx); err != nil {
			log.Info("device: cannot open", "transport", cand.Transport, "port", cand.Port, "err", err)
			if errors.Is(err, transport.ErrPortBusy) {
				busy = true
			}
			if transport.IsPermission(err) {
				switch {
				case cand.Transport == TransportHID || transport.IsLinuxHID(cand.Port):
					hidPerm = true
				case cand.Transport == TransportSerial && transport.IsLinuxUSBSerial(cand.Port):
					serialPerm = true
				case transport.IsLinuxUSBSerial(cand.Port):
					serialPerm = true
				}
			}
			continue
		}
		s := &Sender{T: tr, Clock: clock}
		vals, supported, err := Probe(ctx, s)
		if err != nil {
			tr.Close()
			return Profile{}, nil, err
		}
		d, ok := SelectDialect(supported)
		if !ok {
			log.Info("device: no known Dialect", "transport", cand.Transport, "port", cand.Port, "answers", len(supported))
			tr.Close()
			continue
		}
		p := makeProfile(cand, vals, supported, d)
		log.Info("device: UPS found", "transport", p.Transport, "port", p.Port, "dialect", string(p.Dialect), "supported", p.Supported)
		return p, tr, nil
	}
	if serialPerm && hidPerm {
		return Profile{}, nil, ErrPermissionDenied
	}
	if serialPerm {
		return Profile{}, nil, ErrPortPermission
	}
	if hidPerm {
		return Profile{}, nil, ErrHIDPermission
	}
	if busy {
		return Profile{}, nil, ErrPortBusy
	}
	return Profile{}, nil, ErrNoDevice
}
