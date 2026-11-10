// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// WARNING: These methods control the automatic shutdown of the computer
// (TASK 03-00-00-006). vltd checks each rule again: the range of each
// Setting, a passed Simulation of the current version before the
// activation, and "vault_locked" while the Fallback Policy is in use.

// SetPolicy calls policy.set (elevated). The new version is inactive.
func (a *API) SetPolicy(values map[string]any) (json.RawMessage, error) {
	m, err := client.RawMap(values)
	if err != nil {
		return nil, &Error{Code: wire.CodeInvalid}
	}
	return a.raw(wire.MethodPolicySet, wire.SettingsMapParams{Settings: m})
}

// Simulate calls policy.simulate (elevated). The Simulation uses a virtual
// clock and a dry-run executor in vltd; it never turns off the computer.
func (a *API) Simulate() (json.RawMessage, error) { return a.raw(wire.MethodPolicySimulate, nil) }

// ActivatePolicy calls policy.activate (elevated). vltd refuses it without
// a passed Simulation of the current version ("no_simulation").
func (a *API) ActivatePolicy() error {
	_, err := a.raw(wire.MethodPolicyActivate, nil)
	return err
}

// DeactivatePolicy calls policy.deactivate (elevated).
func (a *API) DeactivatePolicy() error {
	_, err := a.raw(wire.MethodPolicyDeactivate, nil)
	return err
}
