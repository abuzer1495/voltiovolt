// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import (
	"errors"
	"strings"
	"testing"
)

func TestEncoderRejects(t *testing.T) {
	rejected := []string{"Q", "D", "T", "TL", "S01", "S01R0001", "C", "CS", "SOFF", "BZOFF", "PEE", "PDB", "", "\r", "Q1 ", "q1"}
	for _, s := range rejected {
		b, err := EncodeRead(s)
		if err == nil || b != nil {
			t.Errorf("EncodeRead(%q) = %q, %v; want rejection", s, b, err)
		}
		if !errors.Is(err, ErrNotRead) {
			t.Errorf("EncodeRead(%q) error %v is not ErrNotRead", s, err)
		}
	}
}

func TestEncoderReadCommands(t *testing.T) {
	probe := []string{"Q1", "QS", "QGS", "QMOD", "QBV", "QWS", "QFLAG", "QRI", "F", "I", "QID", "QVFW", "QMD"}
	for _, s := range probe {
		b, err := EncodeRead(s)
		if err != nil {
			t.Fatalf("EncodeRead(%q): %v", s, err)
		}
		if string(b) != s+"\r" {
			t.Errorf("EncodeRead(%q) = %q", s, b)
		}
	}
}

func TestRegistryRows(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Registry() {
		if seen[e.Name] {
			t.Errorf("duplicate row %q", e.Name)
		}
		seen[e.Name] = true
		switch e.Class {
		case ClassRead:
			if e.Response == "" || len(e.Dialects) == 0 {
				t.Errorf("READ row %q has no response type or Dialect", e.Name)
			}
		case ClassControl, ClassConfig:
			if e.Response != "" || len(e.Dialects) != 0 {
				t.Errorf("%s row %q has more than name and class", e.Class, e.Name)
			}
		default:
			t.Errorf("row %q has unknown class %q", e.Name, e.Class)
		}
	}
	if seen["D"] {
		t.Error(`"D" must be in no class`)
	}
	if e := findRow("Q"); e == nil || e.Class != ClassControl {
		t.Error(`"Q" must be a CONTROL row`)
	}
}

func findRow(name string) *Entry {
	for _, e := range Registry() {
		if e.Name == name {
			return &e
		}
	}
	return nil
}

func TestVectorsParse(t *testing.T) {
	for cmd, v := range vectorsA {
		if cmd == "Q" {
			continue
		}
		if _, err := Parse(cmd, cr(v)); err != nil {
			t.Errorf("Parse(%s, %q): %v", cmd, v, err)
		}
	}
}

func TestQ1SourceA(t *testing.T) {
	s, err := ParseQ1(cr(vectorsA["Q1"]))
	if err != nil {
		t.Fatal(err)
	}
	if s.InputVoltage.Value != 209.7 || s.InputVoltage.Unit() != "V" {
		t.Errorf("input voltage %+v", s.InputVoltage)
	}
	if s.OutputVoltage.Value != 219.7 || s.Load.Value != 19 || s.Load.Unit() != "%" {
		t.Errorf("output %+v load %+v", s.OutputVoltage, s.Load)
	}
	if s.InputFrequency.Value != 50.0 || s.InputFrequency.Unit() != "Hz" {
		t.Errorf("frequency %+v", s.InputFrequency)
	}
	if s.BatteryVoltage.Value != 2.32 || s.BatteryForm != "S.SS" {
		t.Errorf("battery %+v form %q (must stay as received)", s.BatteryVoltage, s.BatteryForm)
	}
	if s.Temperature.Value != 23.3 || s.Temperature.Unit() != "°C" {
		t.Errorf("temperature %+v", s.Temperature)
	}
	if s.Bits.Raw != "00000001" || !s.Flags.BeeperOn || s.Flags.UtilityFail || s.Flags.StandbyType {
		t.Errorf("bits %+v flags %+v", s.Bits, s.Flags)
	}
}

func TestQ1SourceB(t *testing.T) {
	s, err := ParseQ1(cr(vectorBQ1))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Flags.StandbyType {
		t.Error("bit 3 must be standby type")
	}
	if !s.Flags.BypassBoostBuck {
		t.Error("bit 5 must be set")
	}
	if s.BatteryVoltage.Value != 13.6 || s.BatteryForm != "SS.S" {
		t.Errorf("battery %+v form %q", s.BatteryVoltage, s.BatteryForm)
	}
	if s.Temperature.Value != 40.1 || s.InputFrequency.Value != 59.9 {
		t.Errorf("temperature %+v frequency %+v", s.Temperature, s.InputFrequency)
	}
}

func TestQS(t *testing.T) {
	s, err := ParseQS(cr(vectorsA["QS"]))
	if err != nil {
		t.Fatal(err)
	}
	if s.Command != "QS" || s.Load.Value != 20 || s.BatteryVoltage.Value != 41.8 || !s.Flags.BeeperOn {
		t.Errorf("%+v", s)
	}
}

func TestQGS(t *testing.T) {
	s, err := ParseQGS(cr(vectorsA["QGS"]))
	if err != nil {
		t.Fatal(err)
	}
	if s.OutputCurrent.Value != 0.8 || s.OutputCurrent.Unit() != "A" {
		t.Errorf("current %+v", s.OutputCurrent)
	}
	if s.Unknown10 != "---.-" {
		t.Errorf("field 10 %q", s.Unknown10)
	}
	if s.BatteryVoltage.Value != 41.8 || s.Temperature.Value != 23.3 || s.OutputFrequency.Value != 50 {
		t.Errorf("%+v", s)
	}
	if s.Bits.Raw != "100000000001" {
		t.Errorf("bits %q", s.Bits.Raw)
	}
}

func TestQBV(t *testing.T) {
	b, err := ParseQBV(cr(vectorsA["QBV"]))
	if err != nil {
		t.Fatal(err)
	}
	if b.Voltage.Value != 41.8 || b.SeriesCount != 3 || b.ParallelCount != 1 || b.Charge.Value != 100 {
		t.Errorf("%+v", b)
	}
	if b.RuntimeEstimate.Value != 58 || b.RuntimeEstimate.Unit() != "min" {
		t.Errorf("runtime %+v", b.RuntimeEstimate)
	}
}

func TestRatings(t *testing.T) {
	for _, c := range []struct {
		name string
		fn   func([]byte) (Rating, error)
	}{{"QRI", ParseQRI}, {"F", ParseF}} {
		r, err := c.fn(cr(vectorsA[c.name]))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if r.Voltage.Value != 220 || r.Current.Value != 4 || r.BatteryVoltage.Value != 36 || r.Frequency.Value != 50 {
			t.Errorf("%s: %+v", c.name, r)
		}
	}
	if _, err := ParseF(cr(vectorsA["QRI"])); err == nil {
		t.Error(`F must start with "#"`)
	}
	if _, err := ParseQRI(cr(vectorsA["F"])); err == nil {
		t.Error(`QRI must start with "("`)
	}
}

func TestSmallParsers(t *testing.T) {
	m, err := ParseQMOD(cr(vectorsA["QMOD"]))
	if err != nil || m.Code != 'L' {
		t.Errorf("QMOD %+v %v", m, err)
	}
	f, err := ParseQFLAG(cr(vectorsA["QFLAG"]))
	if err != nil || f.Enabled != "pbshc" || f.Disabled != "rovegfjl" {
		t.Errorf("QFLAG %+v %v", f, err)
	}
	v, err := ParseQVFW(cr(vectorsA["QVFW"]))
	if err != nil || v.Version != "00019.03" {
		t.Errorf("QVFW %+v %v", v, err)
	}
	w, err := ParseQWS(cr(vectorsA["QWS"]))
	if err != nil || len(w.Bits.Raw) != 64 {
		t.Errorf("QWS %+v %v", w, err)
	}
	id, err := ParseQID(cr(vectorsA["QID"]))
	if err != nil || id.Value != "00000000000000" {
		t.Errorf("QID %+v %v", id, err)
	}
	md, err := ParseQMD(cr(vectorsA["QMD"]))
	if err != nil || md.Model != "G1K" || md.RatedVA != 1000 || md.PowerFactor.Value != 80 ||
		md.Phases != "1/1" || md.NominalInput.Value != 220 || md.BatteryCount != 3 || md.BatteryVoltage.Value != 12 {
		t.Errorf("QMD %+v %v", md, err)
	}
	inf, err := ParseI(cr(syntheticI))
	if err != nil || inf.Company != "COMPANY" || inf.Model != "MODEL1" || inf.Version != "VER1" {
		t.Errorf("I %+v %v", inf, err)
	}
}

func TestNotSupported(t *testing.T) {
	for _, r := range []string{"NAK", "(NAK"} {
		for _, cmd := range []string{"Q1", "QS", "QGS", "QMOD", "QBV", "QWS", "QFLAG", "QRI", "F", "I", "QID", "QVFW", "QMD"} {
			if _, err := Parse(cmd, cr(r)); !errors.Is(err, ErrNotSupported) {
				t.Errorf("%s %q: %v; want not supported", cmd, r, err)
			}
			// Echo of the request.
			if _, err := Parse(cmd, cr(cmd)); !errors.Is(err, ErrNotSupported) {
				t.Errorf("%s echo: %v; want not supported", cmd, err)
			}
		}
	}
	// Vector of Source A for "Q" is "(NAK".
	if _, err := ParseQ1(cr(vectorsA["Q"])); !errors.Is(err, ErrNotSupported) {
		t.Error(`"(NAK" must be not supported`)
	}
}

func TestNoValueIsNotZero(t *testing.T) {
	s, err := ParseQ1(cr("(209.7 @@@.@ 219.7 019 50.0 --.- 23.3 00000001"))
	if err != nil {
		t.Fatal(err)
	}
	if s.InputFaultVolt.Valid || s.BatteryVoltage.Valid {
		t.Errorf("no-value fields must be not valid: %+v %+v", s.InputFaultVolt, s.BatteryVoltage)
	}
	if s.InputFaultVolt.Raw != "@@@.@" {
		t.Errorf("raw %q", s.InputFaultVolt.Raw)
	}
	s, err = ParseQ1(cr("(209.7 209.7 219.7 ### 50.0 2.32 ##.# 00000001"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Load.Valid || s.Temperature.Valid {
		t.Error("# fields must be not valid")
	}
}

func TestFrameErrors(t *testing.T) {
	q1 := vectorsA["Q1"]
	cases := []struct {
		name  string
		resp  []byte
		field int
	}{
		{"no CR", []byte(q1), 0},
		{"wrong start", cr("#" + q1[1:]), 0},
		{"too long", cr("(" + strings.Repeat("1", MaxResponseLen)), 0},
		{"field count", cr("(209.7 209.7 219.7 019 50.0 2.32 23.3"), 0},
		{"field width", cr("(209.70 209.7 219.7 019 50.0 2.32 23.3 00000001"), 1},
		{"number format", cr("(209.7 209.7 219.7 0x9 50.0 2.32 23.3 00000001"), 4},
		{"bits", cr("(209.7 209.7 219.7 019 50.0 2.32 23.3 0000002"), 8},
		{"double space", cr("(209.7  209.7 219.7 019 50.0 2.32 23.3 00000001"), 0},
		{"CR inside", []byte("(209.7\r209.7 219.7 019 50.0 2.32 23.3 00000001\r"), 0},
		{"non-ASCII", cr("(209.7 209.7 219.7 019 50.0 2.32 23.3 0000000\xff"), 0},
	}
	for _, c := range cases {
		_, err := ParseQ1(c.resp)
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("%s: error %v is not a ParseError", c.name, err)
			continue
		}
		if pe.Field != c.field {
			t.Errorf("%s: field %d, want %d (%v)", c.name, pe.Field, c.field, err)
		}
	}
}

func TestParseUnknownCommand(t *testing.T) {
	if _, err := Parse("Q", cr("(NAK")); !errors.Is(err, ErrNotRead) {
		t.Errorf("Parse(Q): %v", err)
	}
}
