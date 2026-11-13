// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"context"
	"encoding/json"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// Version is the VLT GUI version (set at build time with -ldflags).
var Version = "dev"

// SourceURL is the link to the source code (AGPL-3.0-or-later).
const SourceURL = "https://github.com/abuzer1495/voltiovolt"

// License is the license of VLT.
const License = "AGPL-3.0-or-later"

// callTimeout is the time limit of one front-end call.
const callTimeout = 30 * time.Second

// API is the service that the front end calls through the Wails bindings.
// Each exported method is one binding. No method returns a Secret value
// (SEC-096).
type API struct {
	m *Manager
}

// NewAPI returns the API of m.
func NewAPI(m *Manager) *API { return &API{m: m} }

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), callTimeout)
}

// Error is an error for the front end. Its text is a code; the front end
// shows the text of the code from the i18n files.
type Error struct {
	Code string
}

func (e *Error) Error() string { return e.Code }

// wrap gives an *Error with the IPC error code.
func wrap(err error) error {
	if err == nil {
		return nil
	}
	if err == ErrNotConnected {
		return &Error{Code: "not_connected"}
	}
	if c := client.Code(err); c != "" {
		return &Error{Code: c}
	}
	return &Error{Code: "ipc_error"}
}

// Connection returns the connection state.
func (a *API) Connection() ConnInfo {
	a.m.mu.Lock()
	defer a.m.mu.Unlock()
	return a.m.conn
}

// AboutInfo is the About text of the General screen.
type AboutInfo struct {
	Version     string `json:"version"`
	VltdVersion string `json:"vltd_version"`
	Protocol    int    `json:"protocol"`
	GUIProtocol int    `json:"gui_protocol"`
	License     string `json:"license"`
	Source      string `json:"source"`
	Mode        string `json:"mode"`
}

// About returns the versions, the license and the source link.
func (a *API) About() AboutInfo {
	ci := a.Connection()
	return AboutInfo{Version: Version, VltdVersion: ci.Version, Protocol: ci.Protocol, GUIProtocol: wire.Version, License: License, Source: SourceURL, Mode: a.m.Mode()}
}

// Last returns the last push messages ("status", "event", "countdown") for
// the first render of a screen.
func (a *API) Last() map[string]json.RawMessage {
	a.m.mu.Lock()
	defer a.m.mu.Unlock()
	out := make(map[string]json.RawMessage, len(a.m.last))
	for k, v := range a.m.last {
		out[k] = v
	}
	return out
}

// SystemStatus calls system.status (Vault state and reason).
func (a *API) SystemStatus() (r wire.StatusResult, err error) {
	c, cancel := ctx()
	defer cancel()
	err = a.m.call(c, func(cl *client.Client) error { r, err = cl.Status(c); return err })
	return r, wrap(err)
}

// StartAtLogon reports whether the start-at-logon shortcut is present
// (VLT-IS-008 TASK 03-00-00-001 step 6, OD-39).
func (a *API) StartAtLogon() (bool, error) {
	a.m.mu.Lock()
	s := a.m.autostart
	a.m.mu.Unlock()
	if s == nil {
		return false, &Error{Code: wire.CodeUnavailable}
	}
	on, err := s.Enabled()
	return on, wrap(err)
}

// SetStartAtLogon makes or deletes the start-at-logon shortcut.
func (a *API) SetStartAtLogon(on bool) error {
	a.m.mu.Lock()
	s := a.m.autostart
	a.m.mu.Unlock()
	if s == nil {
		return &Error{Code: wire.CodeUnavailable}
	}
	return wrap(s.Set(on))
}
