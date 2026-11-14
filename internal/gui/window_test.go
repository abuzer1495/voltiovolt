// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import "testing"

// TestScaledMinSize: the minimum window size scales with the interface
// size (VLT-IS-011 TASK 03-00-00-004).
func TestScaledMinSize(t *testing.T) {
	for _, c := range []struct{ p, w, h int }{
		{100, 1024, 680}, {80, 819, 544}, {125, 1280, 850}, {200, 2048, 1360}, {10, 512, 340}, {900, 4096, 2720},
	} {
		w, h := ScaledMinSize(c.p)
		if w != c.w || h != c.h {
			t.Errorf("%d: %dx%d want %dx%d", c.p, w, h, c.w, c.h)
		}
	}
	m := &Manager{}
	a := &API{m: m}
	a.SetInterfaceScale(150) // no function: no panic
	var gw, gh int
	m.SetMinSizeFunc(func(w, h int) { gw, gh = w, h })
	a.SetInterfaceScale(150)
	if gw != 1536 || gh != 1020 {
		t.Fatalf("%dx%d", gw, gh)
	}
}
