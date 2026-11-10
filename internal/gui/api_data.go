// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"

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
