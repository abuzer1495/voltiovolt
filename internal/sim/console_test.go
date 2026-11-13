// SPDX-License-Identifier: AGPL-3.0-or-later

package sim

import (
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

func TestNewModelVectorsUnchanged(t *testing.T) {
	m := NewModel(ProfileQ1)
	raw := m.Respond("Q1")
	if len(raw) != 47 {
		t.Fatalf("Q1 length %d: %q", len(raw), raw)
	}
	st, err := dialect.ParseQ1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if st.InputVoltage.Value != 209.7 || st.OutputVoltage.Value != 219.7 || st.Load.Value != 19 || st.Temperature.Value != 23.3 {
		t.Fatalf("%+v", st)
	}
	if st.BatteryVoltage.Value != 2.32 {
		t.Fatalf("cell %v", st.BatteryVoltage.Value)
	}
	f := m.Respond("F")
	if len(f) != 22 || string(f) != "#220.0 004 036.0 50.0\r" {
		t.Fatalf("F %q len %d", f, len(f))
	}
	i := m.Respond("I")
	if len(i) != 39 {
		t.Fatalf("I length %d: %q", len(i), i)
	}
}

func TestConsoleDefaults230(t *testing.T) {
	m := NewConsoleModel(ProfileQ1)
	raw := m.Respond("Q1")
	if len(raw) != 47 {
		t.Fatalf("Q1 length %d: %q", len(raw), raw)
	}
	st, err := dialect.ParseQ1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if st.InputVoltage.Value != 230 || st.OutputVoltage.Value != 220 || st.Load.Value != 30 || st.Temperature.Value != 25 || st.InputFrequency.Value != 50 {
		t.Fatalf("%+v raw=%q", st, raw)
	}
	// On-line, 100 %, 24 V rated: cell 2.267, total 2.267*12 = 27.204
	if d := st.BatteryVoltage.Value - 2.267; d > 0.005 || d < -0.005 {
		t.Fatalf("cell %v, want 2.267", st.BatteryVoltage.Value)
	}
	fraw := m.Respond("F")
	if len(fraw) != 22 {
		t.Fatalf("F length %d: %q", len(fraw), fraw)
	}
	fr, err := dialect.ParseF(fraw)
	if err != nil {
		t.Fatal(err)
	}
	if fr.Voltage.Value != 220 || fr.BatteryVoltage.Value != 24 || fr.Frequency.Value != 50 {
		t.Fatalf("F %+v %q", fr, fraw)
	}
}

func TestBatteryVoltageFormulas(t *testing.T) {
	m := NewConsoleModel(ProfileQ1)
	m.SetChargeConsole(0)
	if c := m.Snap().BattCell; abs(c-1.667) > 0.0005 {
		t.Fatalf("online 0%% cell %v", c)
	}
	m.SetChargeConsole(100)
	if c := m.Snap().BattCell; abs(c-2.267) > 0.0005 {
		t.Fatalf("online 100%% cell %v", c)
	}
	if tot := m.Snap().BattTotal; abs(tot-2.267*12) > 0.01 {
		t.Fatalf("online 100%% total %v", tot)
	}

	m.SetUPSType(UPSStandby)
	m.SetRated(220, 24, 50)
	m.SetChargeConsole(0)
	if tot := m.Snap().BattTotal; abs(tot-20.0) > 0.01 {
		t.Fatalf("standby 24V 0%% %v", tot)
	}
	m.SetChargeConsole(100)
	if tot := m.Snap().BattTotal; abs(tot-27.2) > 0.01 {
		t.Fatalf("standby 24V 100%% %v", tot)
	}
	m.SetRated(220, 36, 50)
	m.SetChargeConsole(0)
	if tot := m.Snap().BattTotal; abs(tot-30.0) > 0.01 {
		t.Fatalf("standby 36V 0%% %v", tot)
	}
	m.SetChargeConsole(100)
	if tot := m.Snap().BattTotal; abs(tot-40.8) > 0.01 {
		t.Fatalf("standby 36V 100%% %v", tot)
	}
	// Q1 length still 47 with standby (cell field, not the total).
	if raw := m.Respond("Q1"); len(raw) != 47 {
		t.Fatalf("standby Q1 length %d: %q", len(raw), raw)
	}
}

func TestChargeRatesAndRuntime(t *testing.T) {
	m := NewConsoleModel(ProfileQ1)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.Now = func() time.Time { return now }
	m.SetBattSpeed(60)
	m.SetLoadPercent(100)
	m.SetMains(false)
	m.Tick() // records liveAt
	now = now.Add(5 * time.Second)
	m.Tick()
	// x60, 100 % load: 60*100/(10*60) = 10 %/s; 5 s → 50 %.
	got := m.Snap().Charge
	if abs(got-50) > 0.05 {
		t.Fatalf("discharge charge %v, want 50", got)
	}
	// Runtime at 50 % charge, 100 % load: 10*50/100 = 5 min.
	if r := m.Snap().Runtime; r != 5 {
		t.Fatalf("runtime %d", r)
	}

	m.SetMains(true)
	m.Tick()
	now = now.Add(12 * time.Second)
	m.Tick()
	// On mains, x60: 60 * 100/(2*3600) = 60/72 %/s ≈ 0.833 %/s; 12 s ≈ 10 %.
	got = m.Snap().Charge
	if abs(got-60) > 0.1 {
		t.Fatalf("charge-up %v, want 60", got)
	}

	m.SetLoadPercent(30)
	m.SetChargeConsole(100)
	// 10 * 100 / 30 = 33
	if r := m.Snap().Runtime; r != 33 {
		t.Fatalf("runtime 30%% load %d", r)
	}
}

func TestOnBatteryInputFault(t *testing.T) {
	m := NewConsoleModel(ProfileQ1)
	m.SetInputVoltage(230)
	m.SetMains(false)
	raw := m.Respond("Q1")
	st, err := dialect.ParseQ1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if st.InputVoltage.Value != 0 || st.InputFaultVolt.Value != 230 {
		t.Fatalf("input %v fault %v raw=%q", st.InputVoltage.Value, st.InputFaultVolt.Value, raw)
	}
	if !st.Flags.UtilityFail {
		t.Fatal("utility fail bit")
	}
}

func TestForcedBatteryLowAndBypass(t *testing.T) {
	m := NewConsoleModel(ProfileQ1)
	sn := m.Snap()
	if sn.BatteryLow || sn.Bypass {
		t.Fatalf("defaults %+v", sn)
	}
	m.SetBattLowForce(true)
	m.SetBypass(true)
	m.SetUPSFault(true)
	sn = m.Snap()
	if !sn.BatteryLow || !sn.Bypass || !sn.UPSFault || sn.Charge != 100 {
		t.Fatalf("forced %+v", sn)
	}
}

func TestParseProfile(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"megatec", "megatec"},
		{"q1", "megatec"},
		{"voltronic-qs", "voltronic-qs"},
		{"qs", "voltronic-qs"},
		{"voltronic", "voltronic"},
	} {
		p, err := ParseProfile(c.in)
		if err != nil || p.DisplayName() != c.want {
			t.Errorf("%s: %s %v", c.in, p.DisplayName(), err)
		}
	}
	if _, err := ParseProfile("tcp"); err == nil {
		t.Fatal("tcp must not be a dialect")
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
