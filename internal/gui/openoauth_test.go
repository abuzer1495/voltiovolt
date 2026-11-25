// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

func TestOpenOAuthURL(t *testing.T) {
	var opened string
	a := &API{m: &Manager{openURL: func(u string) error { opened = u; return nil }}}
	good := "https://accounts.google.com/o/oauth2/v2/auth?client_id=test"
	if err := a.OpenOAuthURL(good); err != nil || opened != good {
		t.Fatalf("good: %v %q", err, opened)
	}
	if err := a.OpenOAuthURL("https://evil.example/o/oauth2/v2/auth"); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatalf("evil: %v", err)
	}
	if err := a.OpenOAuthURL("http://accounts.google.com/o/oauth2/v2/auth"); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatalf("http: %v", err)
	}
	a.m.openURL = nil
	if err := a.OpenOAuthURL(good); err == nil || err.Error() != wire.CodeUnavailable {
		t.Fatalf("no opener: %v", err)
	}
}
