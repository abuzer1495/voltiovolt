// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import (
	"errors"
	"testing"
)

// Synthetic examples of VLT-REF-001 sections 6.1, 6.2 and 6.3 (CR added
// by cr()). They are synthetic. They are not captures.
var (
	refQ1 = []string{
		"(229.8 229.8 219.9 034 50.1 2.25 24.0 00000001", // on-line, mains, beeper on
		"(000.0 228.0 220.0 035 50.0 2.10 24.5 10000001", // on-line, on battery
		"(230.1 230.1 230.1   7 50.0 27.0 25.0 00001000", // standby, mains, load "  7"
	}
	refF = []string{
		"#220.0 004 024.0 50.0",
		"#230.0 4.3 24.00 50.0",
	}
	// The spaces are part of the response.
	refI = "#ACME POWER      ONL-1K     VER01.00  "
)

func TestRefVectorLengths(t *testing.T) {
	for _, v := range refQ1 {
		if n := len(cr(v)); n != Q1Len || Q1Len != 47 {
			t.Errorf("Q1 vector %q: %d bytes", v, n)
		}
	}
	for _, v := range refF {
		if n := len(cr(v)); n != FLen || FLen != 22 {
			t.Errorf("F vector %q: %d bytes", v, n)
		}
	}
	if n := len(cr(refI)); n != ILen || ILen != 39 {
		t.Errorf("I vector: %d bytes", n)
	}
}

func TestRefQ1(t *testing.T) {
	type want struct {
		in, fault, out, load, hz, batt, temp float64
		form, bits                           string
		flags                                Q1Flags
	}
	wants := []want{
		{229.8, 229.8, 219.9, 34, 50.1, 2.25, 24.0, "S.SS", "00000001", Q1Flags{BeeperOn: true}},
		{0, 228.0, 220.0, 35, 50.0, 2.10, 24.5, "S.SS", "10000001", Q1Flags{UtilityFail: true, BeeperOn: true}},
		{230.1, 230.1, 230.1, 7, 50.0, 27.0, 25.0, "SS.S", "00001000", Q1Flags{StandbyType: true}},
	}
	for i, v := range refQ1 {
		s, err := ParseQ1(cr(v))
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		w := wants[i]
		got := []float64{s.InputVoltage.Value, s.InputFaultVolt.Value, s.OutputVoltage.Value, s.Load.Value, s.InputFrequency.Value, s.BatteryVoltage.Value, s.Temperature.Value}
		exp := []float64{w.in, w.fault, w.out, w.load, w.hz, w.batt, w.temp}
		for j := range got {
			if got[j] != exp[j] {
				t.Errorf("%q field %d: %v, want %v", v, j+1, got[j], exp[j])
			}
		}
		for j, q := range []bool{s.InputVoltage.Valid, s.InputFaultVolt.Valid, s.OutputVoltage.Valid, s.Load.Valid, s.InputFrequency.Valid, s.BatteryVoltage.Valid, s.Temperature.Valid} {
			if !q {
				t.Errorf("%q field %d: no value", v, j+1)
			}
		}
		if s.BatteryForm != w.form || s.Bits.Raw != w.bits || s.Flags != w.flags {
			t.Errorf("%q: form %s bits %s flags %+v", v, s.BatteryForm, s.Bits.Raw, s.Flags)
		}
	}
}

func TestRefF(t *testing.T) {
	wants := [][4]float64{{220.0, 4, 24.0, 50.0}, {230.0, 4.3, 24.0, 50.0}}
	for i, v := range refF {
		r, err := ParseF(cr(v))
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		got := [4]float64{r.Voltage.Value, r.Current.Value, r.BatteryVoltage.Value, r.Frequency.Value}
		if got != wants[i] || !r.Voltage.Valid || !r.Current.Valid || !r.BatteryVoltage.Valid || !r.Frequency.Valid {
			t.Errorf("%q: %v", v, got)
		}
	}
}

func TestRefI(t *testing.T) {
	inf, err := ParseI(cr(refI))
	if err != nil {
		t.Fatal(err)
	}
	if inf.Company != "ACME POWER" || inf.Model != "ONL-1K" || inf.Version != "VER01.00" {
		t.Errorf("%+v", inf)
	}
}

func TestMegatecSpacesOnly(t *testing.T) {
	// A numeric field with only spaces has no value.
	s, err := ParseQ1(cr("(229.8 229.8 219.9     50.1 2.25      00000001"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Load.Valid || s.Temperature.Valid || !s.InputVoltage.Valid {
		t.Errorf("%+v", s)
	}
	r, err := ParseF(cr("#220.0     024.0 50.0"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Current.Valid || !r.Voltage.Valid {
		t.Errorf("%+v", r)
	}
	inf, err := ParseI(cr("#                ONL-1K     VER01.00  "))
	if err != nil || inf.Company != "" || inf.Model != "ONL-1K" {
		t.Errorf("%+v %v", inf, err)
	}
}

// TestRefNegative: one byte too short, one byte too long, a wrong
// separator position and a letter in a numeric field, for Q1, F and I.
func TestRefNegative(t *testing.T) {
	q1, f, i := refQ1[0], refF[0], refI
	cases := []struct {
		name  string
		parse func([]byte) error
		resp  string
		field int
	}{
		{"Q1 short", q1p, q1[:len(q1)-1], 0},
		{"Q1 long", q1p, q1 + "1", 0},
		{"Q1 separator", q1p, "(229.8 229.8 219.9 0340 50.1 2.25 24.000000001", 0},
		{"Q1 separator 2", q1p, "(229.8 229.8 219.9 034 50.1 2.25 24.0000000001", 0},
		{"Q1 letter", q1p, "(229.8 229.8 219.9 03A 50.1 2.25 24.0 00000001", 4},
		{"Q1 letter in volts", q1p, "(229.8 22x.8 219.9 034 50.1 2.25 24.0 00000001", 2},
		{"Q1 inner space", q1p, "(229.8 229.8 219.9 0 4 50.1 2.25 24.0 00000001", 4},
		{"F short", fp, f[:len(f)-1], 0},
		{"F long", fp, f + "0", 0},
		{"F separator", fp, "#220.0 0040024.0 50.0", 0},
		{"F letter", fp, "#220.0 00A 024.0 50.0", 2},
		{"I short", ip, i[:len(i)-1], 0},
		{"I long", ip, i + " ", 0},
		{"I separator", ip, "#ACME POWER      ONL-1K    xVER01.00  ", 0},
	}
	for _, c := range cases {
		err := c.parse(cr(c.resp))
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("%s: %v; want a ParseError", c.name, err)
			continue
		}
		if pe.Field != c.field {
			t.Errorf("%s: field %d, want %d (%v)", c.name, pe.Field, c.field, err)
		}
	}
}

func q1p(b []byte) error { _, err := ParseQ1(b); return err }
func fp(b []byte) error  { _, err := ParseF(b); return err }
func ip(b []byte) error  { _, err := ParseI(b); return err }
