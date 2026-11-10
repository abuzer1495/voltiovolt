// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"errors"
	"fmt"
	"testing"

	"go.bug.st/serial"
)

// TestSerialLines: "unchanged" is the default. The library gets no initial
// line values (nil) unless both lines are set (VLT-IS-006 TASK 03-00-00-004
// step 1). No port is opened.
func TestSerialLines(t *testing.T) {
	d := DefaultSerialOptions()
	if d.DTR != LineUnchanged || d.RTS != LineUnchanged {
		t.Fatalf("default %+v", d)
	}
	cases := []struct {
		dtr, rts Line
		want     *serial.ModemOutputBits
	}{
		{LineUnchanged, LineUnchanged, nil},
		{LineOn, LineUnchanged, nil},
		{LineUnchanged, LineOff, nil},
		{"", "", nil},
		{LineOn, LineOff, &serial.ModemOutputBits{DTR: true, RTS: false}},
		{LineOff, LineOn, &serial.ModemOutputBits{DTR: false, RTS: true}},
	}
	for _, c := range cases {
		got := initialBits("windows", SerialOptions{DTR: c.dtr, RTS: c.rts})
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s/%s: %+v, want %+v", c.dtr, c.rts, got, c.want)
		}
		if l := initialBits("linux", SerialOptions{DTR: c.dtr, RTS: c.rts}); l != nil {
			t.Errorf("linux %s/%s: %+v, want nil", c.dtr, c.rts, l)
		}
	}
	for _, s := range []string{"unchanged", "on", "off"} {
		if l, err := ParseLine(s); err != nil || string(l) != s {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range []string{"", "true", "ON", "1"} {
		if _, err := ParseLine(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestIsBusy(t *testing.T) {
	if isBusy(errors.New("x")) || isBusy(nil) {
		t.Error("not busy")
	}
	// The PortError code is set by the library only; check the wrapping.
	if !errors.Is(fmt.Errorf("%w: COM3", ErrPortBusy), ErrPortBusy) {
		t.Error("wrap")
	}
}
