// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// TestDeviceMethods is TASK 03-00-00-005 step 1.
func TestDeviceMethods(t *testing.T) {
	f := newFake(t)
	f.handle(wire.MethodDeviceStatus, func(json.RawMessage) (any, *wire.Error) {
		return map[string]any{"state": "connected", "profile": map[string]any{"serial_number": "[REDACTED]"}}, nil
	})
	f.handle(wire.MethodDeviceSettingsSet, ok)
	_, a, _ := start(t, f)
	d, err := a.DeviceStatus()
	if err != nil || !bytes.Contains(d, []byte(`"[REDACTED]"`)) {
		t.Fatalf("%s %v", d, err)
	}
	if err := a.SetDeviceSettings(map[string]any{"device.transport": "serial", "device.serial_dtr": true}); err != nil {
		t.Fatal(err)
	}
	var p wire.SettingsMapParams
	if err := json.Unmarshal(f.last(wire.MethodDeviceSettingsSet), &p); err != nil || string(p.Settings["device.transport"]) != `"serial"` || string(p.Settings["device.serial_dtr"]) != "true" {
		t.Fatalf("%+v %v", p, err)
	}
}

// TestMeasurementsChunks is step 2: a long range is queried in parts of
// less than 10000 rows; each range of the History screen is permitted.
func TestMeasurementsChunks(t *testing.T) {
	f := newFake(t)
	var calls []wire.MeasurementsParams
	f.handle(wire.MethodMeasurements, func(p json.RawMessage) (any, *wire.Error) {
		var x wire.MeasurementsParams
		_ = json.Unmarshal(p, &x)
		calls = append(calls, x)
		return map[string]any{"name": x.Name, "unit": "V", "step": x.Step, "rows": []any{map[string]any{"ts": x.From, "mean": 230}}}, nil
	})
	_, a, _ := start(t, f)
	to := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		d     time.Duration
		step  string
		parts int
	}{
		{time.Hour, "raw", 1},
		{24 * time.Hour, "1m", 1},
		{7 * 24 * time.Hour, "1m", 2},
		{30 * 24 * time.Hour, "1h", 1},
		{365 * 24 * time.Hour, "1h", 1},
	} {
		calls = nil
		s, err := a.Measurements("input.voltage", to.Add(-c.d).UnixMilli(), to.UnixMilli(), c.step)
		if err != nil || s.Unit != "V" || len(s.Rows) != c.parts || len(calls) != c.parts {
			t.Fatalf("%v %s: %+v %v %d", c.d, c.step, s, err, len(calls))
		}
		if calls[0].From != to.Add(-c.d).Format(time.RFC3339Nano) || calls[len(calls)-1].To != to.Format(time.RFC3339Nano) {
			t.Fatalf("range %+v", calls)
		}
	}
	if _, err := a.Measurements("input.voltage", 2, 1, "raw"); err == nil {
		t.Fatal("bad range")
	}
	if _, err := a.Measurements("input.voltage", 1, 2, "5m"); err == nil {
		t.Fatal("bad step")
	}
}
