// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// raw calls a method and returns its JSON result unchanged. The Go side
// does not decode the result types of vltd, so it does not link their
// packages (TASK 03-00-00-002 step 4.1).
func (a *API) raw(method string, params any) (json.RawMessage, error) {
	c, cancel := ctx()
	defer cancel()
	var r json.RawMessage
	err := a.m.call(c, func(cl *client.Client) error { return cl.Call(c, method, params, &r) })
	if err != nil {
		return nil, wrap(err)
	}
	return r, nil
}

// Events calls events.list: before (0: the newest) and limit (1 to 500).
func (a *API) Events(before int64, limit int) (json.RawMessage, error) {
	return a.raw(wire.MethodEventsList, wire.EventsListParams{Before: before, Limit: limit})
}

// Policy calls policy.get (with the "fallback" field).
func (a *API) Policy() (json.RawMessage, error) { return a.raw(wire.MethodPolicyGet, nil) }

// ShutdownStatus calls shutdown.status.
func (a *API) ShutdownStatus() (json.RawMessage, error) { return a.raw(wire.MethodShutdownStatus, nil) }

// CancelShutdown calls shutdown.cancel (elevated). vltd refuses it while
// LOW_BATTERY is active ("low_battery").
func (a *API) CancelShutdown() error {
	_, err := a.raw(wire.MethodShutdownCancel, nil)
	return err
}

// SetTrayTexts sets the tray texts from the i18n files of the front end.
func (a *API) SetTrayTexts(tx TrayTexts) {
	a.m.mu.Lock()
	a.m.texts = tx
	a.m.mu.Unlock()
	a.m.updateTray()
}

// TrayTexts returns the tray texts.
func (m *Manager) TrayTexts() TrayTexts {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.texts.Open == "" {
		return DefaultTrayTexts
	}
	return m.texts
}

// DeviceStatus calls device.status. The serial number (QID) is
// "[REDACTED]" from vltd.
func (a *API) DeviceStatus() (json.RawMessage, error) { return a.raw(wire.MethodDeviceStatus, nil) }

// SetDeviceSettings calls device.settings.set (elevated) after the
// confirmation in the front end.
func (a *API) SetDeviceSettings(values map[string]any) error {
	m, err := client.RawMap(values)
	if err != nil {
		return &Error{Code: wire.CodeInvalid}
	}
	_, err = a.raw(wire.MethodDeviceSettingsSet, wire.SettingsMapParams{Settings: m})
	return err
}

// ScanConfirmation is the text that the user types for a port scan.
const ScanConfirmation = wire.ScanConfirmation

// DevicePorts calls device.ports: the serial ports of the OS with the USB
// data. vltd opens no port.
func (a *API) DevicePorts() (json.RawMessage, error) { return a.raw(wire.MethodDevicePorts, nil) }

// DeviceScan calls device.scan (elevated) with the marked ports. vltd
// sends only "Q1" CR to each marked port (VLT-IS-006 TASK 03-00-00-004).
func (a *API) DeviceScan(ports []string, confirmation string) (json.RawMessage, error) {
	if confirmation != ScanConfirmation || len(ports) == 0 {
		return nil, &Error{Code: wire.CodeInvalid}
	}
	return a.raw(wire.MethodDeviceScan, wire.DeviceScanParams{Ports: ports, Confirmation: confirmation})
}

// Step lengths of measurements.query.
var stepLen = map[string]time.Duration{"raw": 2 * time.Second, "1m": time.Minute, "1h": time.Hour}

// chunkRows is the number of rows of one query; it is less than the row
// limit of vltd (10000), so that a long range is not cut.
const chunkRows = 9000

// Series is the result of Measurements.
type Series struct {
	Name string            `json:"name"`
	Unit string            `json:"unit"`
	Step string            `json:"step"`
	Rows []json.RawMessage `json:"rows"`
}

// Measurements calls measurements.query from fromMs to toMs (Unix
// milliseconds) with step raw, 1m or 1h. A long range is queried in parts.
func (a *API) Measurements(name string, fromMs, toMs int64, step string) (Series, error) {
	d, ok := stepLen[step]
	if !ok || toMs <= fromMs {
		return Series{}, &Error{Code: wire.CodeInvalid}
	}
	out := Series{Name: name, Step: step, Rows: []json.RawMessage{}}
	from, to := time.UnixMilli(fromMs).UTC(), time.UnixMilli(toMs).UTC()
	span := d * chunkRows
	for t := from; t.Before(to); t = t.Add(span) {
		end := t.Add(span)
		if end.After(to) {
			end = to
		}
		raw, err := a.raw(wire.MethodMeasurements, wire.MeasurementsParams{Name: name,
			From: t.Format(time.RFC3339Nano), To: end.Format(time.RFC3339Nano), Step: step})
		if err != nil {
			return Series{}, err
		}
		var r struct {
			Unit string            `json:"unit"`
			Rows []json.RawMessage `json:"rows"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return Series{}, &Error{Code: "ipc_error"}
		}
		out.Unit = r.Unit
		out.Rows = append(out.Rows, r.Rows...)
	}
	return out, nil
}
