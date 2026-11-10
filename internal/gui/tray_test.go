// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"bytes"
	"encoding/json"
	"image/png"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

func st(device string, onBattery, commLost, countdown bool, runtime *float64) json.RawMessage {
	m := map[string]any{}
	if runtime != nil {
		m["battery.runtime"] = map[string]any{"value": *runtime}
	}
	b, _ := json.Marshal(map[string]any{"device": map[string]any{"state": device},
		"ups": map[string]any{"on_battery": onBattery, "comm_lost": commLost}, "measurements": m,
		"countdown": map[string]any{"active": countdown}})
	return b
}

// TestComputeTray is TASK 03-00-00-004 step 2.1.
func TestComputeTray(t *testing.T) {
	rt := 42.0
	for _, c := range []struct {
		name      string
		connected bool
		status    json.RawMessage
		cd        json.RawMessage
		critical  int
		want      string
	}{
		{"no connection", false, st("connected", false, false, false, nil), nil, 0, TrayOffline},
		{"no status", true, nil, nil, 0, TrayOffline},
		{"mains", true, st("connected", false, false, false, &rt), nil, 0, TrayMains},
		{"battery", true, st("connected", true, false, false, &rt), nil, 0, TrayBattery},
		{"comm lost", true, st("comm lost", true, true, false, nil), nil, 0, TrayOffline},
		{"no device", true, st("no device", false, false, false, nil), nil, 0, TrayOffline},
		{"countdown", true, st("connected", true, false, true, &rt), nil, 0, TrayCritical},
		{"countdown push", true, st("connected", true, false, false, &rt), json.RawMessage(`{"active":true}`), 0, TrayCritical},
		{"critical event", true, st("connected", false, false, false, &rt), nil, 1, TrayCritical},
	} {
		if got := ComputeTray(c.connected, c.status, c.cd, c.critical); got.State != c.want {
			t.Errorf("%s: %s", c.name, got.State)
		}
	}
	ts := ComputeTray(true, st("connected", true, false, false, &rt), nil, 0)
	if ts.Runtime == nil || *ts.Runtime != 42 {
		t.Fatal("runtime")
	}
	if got := Tooltip(ts, DefaultTrayTexts); got != "VLT: on battery, runtime 42 min" {
		t.Fatal(got)
	}
	tr := TrayTexts{Battery: "VLT: aküde", Runtime: "çalışma süresi {minutes} dk"}
	if got := Tooltip(ts, tr); got != "VLT: aküde, çalışma süresi 42 dk" {
		t.Fatal(got)
	}
	if got := Tooltip(TrayState{State: TrayOffline}, DefaultTrayTexts); got != "VLT: no connection" {
		t.Fatal(got)
	}
}

// TestTrayIcons is step 2.2: each state has its own shape (the opaque
// pixel masks differ), not only its own color.
func TestTrayIcons(t *testing.T) {
	masks := map[string]string{}
	for _, s := range []string{TrayMains, TrayBattery, TrayCritical, TrayOffline} {
		img, err := png.Decode(bytes.NewReader(TrayIcon(s)))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != iconSize {
			t.Fatal("size")
		}
		var m []byte
		for y := 0; y < iconSize; y++ {
			for x := 0; x < iconSize; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a > 0 {
					m = append(m, '1')
				} else {
					m = append(m, '0')
				}
			}
		}
		for other, om := range masks {
			if om == string(m) {
				t.Errorf("%s and %s have the same shape", s, other)
			}
		}
		masks[s] = string(m)
	}
	if TrayShape(TrayMains) == TrayShape(TrayBattery) || TrayShape(TrayCritical) == TrayShape(TrayOffline) {
		t.Fatal("shapes")
	}
}

// TestCriticalEventTray: an active CRITICAL Event gives the red state; its
// clear Event ends it.
func TestCriticalEventTray(t *testing.T) {
	f := newFake(t)
	m, a, _ := start(t, f)
	f.push(wire.PushStatus, map[string]any{"device": map[string]any{"state": "connected"}, "ups": map[string]any{}})
	waitFor(t, "status", func() bool { _, ok := a.Last()["status"]; return ok })
	f.push(wire.PushEvent, map[string]any{"type": "UPS_FAULT", "severity": "CRITICAL", "data": map[string]any{"active": true}})
	waitFor(t, "critical", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return len(m.critical) == 1
	})
	f.push(wire.PushEvent, map[string]any{"type": "UPS_FAULT", "severity": "INFO", "data": map[string]any{"active": false}})
	waitFor(t, "cleared", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return len(m.critical) == 0
	})
	a.SetTrayTexts(TrayTexts{Open: "Aç", Lock: "Kilitle", Quit: "Çık"})
	if m.TrayTexts().Open != "Aç" {
		t.Fatal("texts")
	}
}

// TestDashboardMethods: events.list, policy.get, shutdown.status and
// shutdown.cancel go to vltd with the correct parameters.
func TestDashboardMethods(t *testing.T) {
	f := newFake(t)
	f.handle(wire.MethodEventsList, func(json.RawMessage) (any, *wire.Error) {
		return map[string]any{"events": []any{map[string]any{"id": 1, "type": "ON_BATTERY"}}, "next": 0}, nil
	})
	f.handle(wire.MethodPolicyGet, func(json.RawMessage) (any, *wire.Error) {
		return map[string]any{"version": "v1", "active": true, "fallback": true}, nil
	})
	f.handle(wire.MethodShutdownStatus, func(json.RawMessage) (any, *wire.Error) {
		return map[string]any{"active": false, "remaining_seconds": 0}, nil
	})
	f.handle(wire.MethodShutdownCancel, func(json.RawMessage) (any, *wire.Error) {
		return nil, &wire.Error{Code: wire.CodeLowBattery, Message: "LOW_BATTERY"}
	})
	_, a, _ := start(t, f)
	ev, err := a.Events(0, 10)
	if err != nil || !bytes.Contains(ev, []byte("ON_BATTERY")) {
		t.Fatalf("%s %v", ev, err)
	}
	if p := f.last(wire.MethodEventsList); string(p) != `{"before":0,"limit":10}` {
		t.Fatalf("params %s", p)
	}
	pol, err := a.Policy()
	if err != nil || !bytes.Contains(pol, []byte(`"fallback":true`)) {
		t.Fatalf("%s %v", pol, err)
	}
	if _, err := a.ShutdownStatus(); err != nil {
		t.Fatal(err)
	}
	if err := a.CancelShutdown(); err == nil || err.Error() != wire.CodeLowBattery {
		t.Fatal(err)
	}
}
