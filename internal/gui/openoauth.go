// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/notify/email/oauth"
)

// OpenOAuthURL opens a Google authorization URL in the default web
// browser of the user. The VLT GUI window never loads remote content
// (SEC-095). Only https://accounts.google.com/o/oauth2/… is allowed.
func (a *API) OpenOAuthURL(raw string) error {
	if !oauth.AllowedAuthURL(raw) {
		return &Error{Code: wire.CodeInvalid}
	}
	a.m.mu.Lock()
	f := a.m.openURL
	a.m.mu.Unlock()
	if f == nil {
		return &Error{Code: wire.CodeUnavailable}
	}
	if err := f(raw); err != nil {
		return &Error{Code: "ipc_error"}
	}
	return nil
}
