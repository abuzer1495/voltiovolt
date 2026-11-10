// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// SettingLanguage is the Setting of the GUI language (TASK 03-00-00-007
// step 2.1).
const SettingLanguage = "ui.language"

// ResetConfirmation is the text that the user types for a Vault reset.
const ResetConfirmation = "RESET VAULT"

// ResetVault calls vault.reset (elevated) with the typed confirmation text.
// vltd deletes all Secrets.
func (a *API) ResetVault(confirmation string) error {
	if confirmation != ResetConfirmation {
		return &Error{Code: wire.CodeInvalid}
	}
	_, err := a.raw(wire.MethodVaultReset, wire.VaultResetParams{Confirmation: confirmation})
	return err
}

// Audit calls audit.list (elevated): after (seq) and limit (1 to 500).
func (a *API) Audit(after int64, limit int) (json.RawMessage, error) {
	return a.raw(wire.MethodAuditList, wire.AuditListParams{After: after, Limit: limit})
}

// Settings calls settings.list.
func (a *API) Settings() (json.RawMessage, error) { return a.raw(wire.MethodSettingsList, nil) }

// SetLanguage stores "tr" or "en" in the Setting ui.language. vltd accepts
// it without the elevated state when a PIN is set (VLT-IS-006 Q-IA-1).
func (a *API) SetLanguage(lang string) error {
	if lang != "tr" && lang != "en" {
		return &Error{Code: wire.CodeInvalid}
	}
	c, cancel := ctx()
	defer cancel()
	err := a.m.call(c, func(cl *client.Client) error { _, err := cl.SetSetting(c, SettingLanguage, lang); return err })
	return wrap(err)
}

// OpenSource opens the link to the source code in the web browser of the
// OS. The VLT GUI window never loads remote content (SEC-095).
func (a *API) OpenSource() error {
	a.m.mu.Lock()
	f := a.m.openURL
	a.m.mu.Unlock()
	if f == nil {
		return &Error{Code: wire.CodeUnavailable}
	}
	if err := f(SourceURL); err != nil {
		return &Error{Code: "ipc_error"}
	}
	return nil
}

// SetOpenURL sets the function that opens a URL in the web browser.
func (m *Manager) SetOpenURL(f func(string) error) {
	m.mu.Lock()
	m.openURL = f
	m.mu.Unlock()
}
