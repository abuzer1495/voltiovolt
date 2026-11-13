// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

func TestAllowedHelpURL(t *testing.T) {
	ok := []string{
		"https://core.telegram.org/bots/tutorial",
		"https://support.google.com/accounts/answer/185833",
		"https://support.microsoft.com/windows",
	}
	for _, u := range ok {
		if !AllowedHelpURL(u) {
			t.Errorf("allowed: %s", u)
		}
	}
	bad := []string{
		"http://core.telegram.org/bots/tutorial",
		"https://example.com/",
		"https://evil.core.telegram.org/",
		"https://core.telegram.org.evil.com/",
		"https://user:pass@core.telegram.org/",
		"javascript:alert(1)",
		"",
		"/local",
	}
	for _, u := range bad {
		if AllowedHelpURL(u) {
			t.Errorf("not allowed: %s", u)
		}
	}
}

func TestOpenHelp(t *testing.T) {
	f := allFake(t)
	_, a, _ := start(t, f)
	if err := a.OpenHelp("https://example.com/"); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatalf("bad url: %v", err)
	}
	if err := a.OpenHelp(OfficialHelpURLs[0]); err == nil {
		t.Fatal("want unavailable without OpenURL")
	}
	var opened string
	a.m.SetOpenURL(func(u string) error { opened = u; return nil })
	if err := a.OpenHelp(OfficialHelpURLs[0]); err != nil || opened != OfficialHelpURLs[0] {
		t.Fatalf("open: %v %q", err, opened)
	}
	if err := a.OpenHelp("https://github.com/"); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatalf("github: %v", err)
	}
}

func TestOfficialHelpURLsAreAllowed(t *testing.T) {
	if len(OfficialHelpURLs) == 0 {
		t.Fatal("no official help URLs")
	}
	for _, u := range OfficialHelpURLs {
		if !AllowedHelpURL(u) {
			t.Errorf("product link not allowed: %s", u)
		}
	}
}
