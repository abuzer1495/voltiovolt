// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"
)

// Tray states (TASK 03-00-00-004 step 2).
const (
	TrayMains    = "mains"    // green circle
	TrayBattery  = "battery"  // yellow triangle
	TrayCritical = "critical" // red square: a CRITICAL Event is active or a countdown runs
	TrayOffline  = "offline"  // grey ring: no connection to vltd or comm lost
)

// TrayState is the state of the tray icon.
type TrayState struct {
	State   string `json:"state"`
	Runtime *int   `json:"runtime_minutes,omitempty"` // nil: no data
}

// statusPush is the part of the status push that the Go side reads.
type statusPush struct {
	Device struct {
		State string `json:"state"`
	} `json:"device"`
	UPS struct {
		OnBattery bool `json:"on_battery"`
		CommLost  bool `json:"comm_lost"`
	} `json:"ups"`
	Measurements map[string]struct {
		Value float64 `json:"value"`
	} `json:"measurements"`
	Countdown struct {
		Active bool `json:"active"`
	} `json:"countdown"`
}

// ComputeTray returns the tray state of a connection state, the last
// status push, the last countdown push and the active CRITICAL Events.
func ComputeTray(connected bool, status, countdown json.RawMessage, critical int) TrayState {
	if !connected || status == nil {
		return TrayState{State: TrayOffline}
	}
	var s statusPush
	if json.Unmarshal(status, &s) != nil {
		return TrayState{State: TrayOffline}
	}
	var ts TrayState
	if p, ok := s.Measurements["battery.runtime"]; ok {
		n := int(p.Value)
		ts.Runtime = &n
	}
	cd := s.Countdown.Active
	if countdown != nil {
		var c struct {
			Active bool `json:"active"`
		}
		if json.Unmarshal(countdown, &c) == nil {
			cd = cd || c.Active
		}
	}
	switch {
	case cd || critical > 0:
		ts.State = TrayCritical
	case s.UPS.CommLost || s.Device.State != "connected":
		ts.State = TrayOffline
	case s.UPS.OnBattery:
		ts.State = TrayBattery
	default:
		ts.State = TrayMains
	}
	return ts
}

func (m *Manager) updateTray() {
	m.mu.Lock()
	ts := ComputeTray(m.conn.State == ConnConnected, m.last["status"], m.last["countdown"], len(m.critical))
	f := m.onTray
	m.mu.Unlock()
	m.send(EventTray, ts)
	if f != nil {
		f(ts)
	}
}
