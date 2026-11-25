// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"errors"
	"fmt"
	"testing"

	"go.bug.st/serial"
)

// TestSerialLines: "unchanged" is the default. On Windows VLT always
// passes InitialStatusBits (nil would turn DTR and RTS on). "unchanged"
// and "off" are off; "on" is on. On Linux the value is always nil
// (VLT-RPT-006). No port is opened.
func TestSerialLines(t *testing.T) {
	d := DefaultSerialOptions()
	if d.DTR != LineUnchanged || d.RTS != LineUnchanged {
		t.Fatalf("default %+v", d)
	}
	off := &serial.ModemOutputBits{DTR: false, RTS: false}
	cases := []struct {
		dtr, rts Line
		win      *serial.ModemOutputBits
	}{
		{LineUnchanged, LineUnchanged, off},
		{LineOn, LineUnchanged, &serial.ModemOutputBits{DTR: true, RTS: false}},
		{LineUnchanged, LineOff, off},
		{"", "", off},
		{LineOn, LineOff, &serial.ModemOutputBits{DTR: true, RTS: false}},
		{LineOff, LineOn, &serial.ModemOutputBits{DTR: false, RTS: true}},
		{LineOn, LineOn, &serial.ModemOutputBits{DTR: true, RTS: true}},
		{LineOff, LineOff, off},
	}
	for _, c := range cases {
		got := initialBits("windows", SerialOptions{DTR: c.dtr, RTS: c.rts})
		if got == nil || *got != *c.win {
			t.Errorf("windows %s/%s: %+v, want %+v", c.dtr, c.rts, got, c.win)
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

type fakeModem struct {
	dtr, rts *bool
	errDTR   error
	errRTS   error
}

func (f *fakeModem) SetDTR(on bool) error {
	if f.errDTR != nil {
		return f.errDTR
	}
	v := on
	f.dtr = &v
	return nil
}
func (f *fakeModem) SetRTS(on bool) error {
	if f.errRTS != nil {
		return f.errRTS
	}
	v := on
	f.rts = &v
	return nil
}

func TestApplyLinesAfterOpen(t *testing.T) {
	// Windows default "unchanged": both lines off (not left on).
	f := &fakeModem{}
	applyLinesAfterOpen(f, "windows", DefaultSerialOptions())
	if f.dtr == nil || *f.dtr || f.rts == nil || *f.rts {
		t.Fatalf("windows default: dtr=%v rts=%v", f.dtr, f.rts)
	}
	f = &fakeModem{}
	applyLinesAfterOpen(f, "windows", SerialOptions{DTR: LineOn, RTS: LineUnchanged})
	if f.dtr == nil || !*f.dtr || f.rts == nil || *f.rts {
		t.Fatalf("windows on/unchanged: dtr=%v rts=%v", f.dtr, f.rts)
	}
	// Linux "unchanged": do not call SetDTR/SetRTS.
	f = &fakeModem{}
	applyLinesAfterOpen(f, "linux", DefaultSerialOptions())
	if f.dtr != nil || f.rts != nil {
		t.Fatalf("linux default set lines: dtr=%v rts=%v", f.dtr, f.rts)
	}
	f = &fakeModem{}
	applyLinesAfterOpen(f, "linux", SerialOptions{DTR: LineOn, RTS: LineOff})
	if f.dtr == nil || !*f.dtr || f.rts == nil || *f.rts {
		t.Fatalf("linux on/off: dtr=%v rts=%v", f.dtr, f.rts)
	}
	var warns []string
	f = &fakeModem{errDTR: errors.New("no modem")}
	applyLinesAfterOpen(f, "linux", SerialOptions{DTR: LineOn, RTS: LineUnchanged, Warn: func(m string) { warns = append(warns, m) }})
	if len(warns) != 1 || f.rts != nil {
		t.Fatalf("warn %v rts %v", warns, f.rts)
	}
}

func TestIsBusy(t *testing.T) {
	if isBusy(errors.New("x")) || isBusy(nil) {
		t.Error("not busy")
	}
	if !errors.Is(fmt.Errorf("%w: COM3", ErrPortBusy), ErrPortBusy) {
		t.Error("wrap")
	}
}
