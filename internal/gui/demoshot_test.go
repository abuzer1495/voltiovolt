// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import "testing"

func TestDemoNavOffByDefault(t *testing.T) {
	a := &API{m: NewManager("", nil, nil)}
	if got := a.DemoNav(); got.Active || got.Screen != "" {
		t.Fatalf("default %+v", got)
	}
	var called bool
	a.DemoShotReady()
	if called {
		t.Fatal("ready callback without SetDemoShotReady")
	}
	a.m.SetDemoNav("wizard/pin")
	a.m.SetDemoShotReady(func() { called = true })
	got := a.DemoNav()
	if !got.Active || got.Screen != "wizard/pin" {
		t.Fatalf("nav %+v", got)
	}
	a.DemoShotReady()
	if !called {
		t.Fatal("ready not called")
	}
}
