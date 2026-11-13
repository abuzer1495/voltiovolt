// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"net/url"
	"strings"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// OfficialHelpHosts are the only hosts that help links may open
// (VLT-IS-009 OD-42).
var OfficialHelpHosts = map[string]bool{
	"core.telegram.org":     true,
	"support.google.com":    true,
	"support.microsoft.com": true,
}

// OfficialHelpURLs is the list of help links in the product
// (VLT-IS-009 TASK 03-00-00-004). The CI link check uses the same list.
var OfficialHelpURLs = []string{
	"https://core.telegram.org/bots/tutorial",
	"https://support.google.com/accounts/answer/185833",
}

// AllowedHelpURL reports whether u is an https URL on an official help host.
func AllowedHelpURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Host == "" {
		return false
	}
	if u.Hostname() != u.Host && u.Port() != "" && u.Port() != "443" {
		return false
	}
	return OfficialHelpHosts[strings.ToLower(u.Hostname())]
}

// OpenHelp opens an official help URL in the default web browser of the
// user. The VLT GUI window never loads remote content (SEC-095).
func (a *API) OpenHelp(raw string) error {
	if !AllowedHelpURL(raw) {
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
