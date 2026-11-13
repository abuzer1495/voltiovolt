// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
)

// PIN rules of VLT-IS-003 TASK 03-00-00-005 (internal/auth). The front end
// shows them; vltd checks them.
const (
	MinPINLength = 6
	MaxPINLength = 64
)

// PINRules are the PIN rules for the "Set PIN" screen.
type PINRules struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Rules returns the PIN rules.
func (a *API) Rules() PINRules { return PINRules{Min: MinPINLength, Max: MaxPINLength} }

// AuthStatus calls auth.status: PIN set, elevated, block time.
func (a *API) AuthStatus() (AuthInfo, error) {
	c, cancel := ctx()
	defer cancel()
	a.m.refreshAuth(c)
	a.m.mu.Lock()
	defer a.m.mu.Unlock()
	if a.m.c == nil {
		return AuthInfo{}, wrap(ErrNotConnected)
	}
	return a.m.auth, nil
}

// SetPIN calls auth.set_pin. At first start it needs no elevation; after
// that it is the PIN change and needs the elevated state. After the first
// PIN the GUI starts status.subscribe.
func (a *API) SetPIN(pin string) error {
	c, cancel := ctx()
	defer cancel()
	err := a.m.call(c, func(cl *client.Client) error { return cl.SetPIN(c, pin) })
	if err != nil {
		return wrap(err)
	}
	a.m.subscribe(c)
	a.m.refreshAuth(c)
	return nil
}

// RemovePIN calls auth.remove_pin. It needs the current PIN. After a
// success, no PIN is set (VLT-IS-008 TASK 03-00-00-003).
func (a *API) RemovePIN(pin string) error {
	c, cancel := ctx()
	defer cancel()
	err := a.m.call(c, func(cl *client.Client) error { return cl.RemovePIN(c, pin) })
	a.m.refreshAuth(c)
	return wrap(err)
}

// Login calls auth.login. The GUI is unlocked (elevated) after it.
func (a *API) Login(pin string) error {
	c, cancel := ctx()
	defer cancel()
	err := a.m.call(c, func(cl *client.Client) error { return cl.Login(c, pin) })
	if err != nil {
		a.m.refreshAuth(c)
		return wrap(err)
	}
	if cl := a.m.client(); cl != nil {
		a.m.readIdle(c, cl)
	}
	a.m.setElevated(true)
	a.m.refreshAuth(c)
	return nil
}

// Logout calls auth.logout (the "Lock" button and the tray menu).
func (a *API) Logout() error {
	c, cancel := ctx()
	defer cancel()
	err := a.m.call(c, func(cl *client.Client) error { return cl.Logout(c) })
	a.m.setElevated(false)
	a.m.armLock()
	return wrap(err)
}
