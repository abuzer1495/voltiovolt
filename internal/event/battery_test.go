// SPDX-License-Identifier: AGPL-3.0-or-later

package event

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// VLT-IS-012 TASK 03-00-00-004 step 7: each case and each clear condition.

var day = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func TestMonthsBetween(t *testing.T) {
	d := func(s string) time.Time { x, _ := time.Parse(store.DateLayout, s); return x }
	for _, c := range []struct {
		a, b string
		n    int
	}{
		{"2023-11-20", "2026-11-20", 36}, {"2023-11-21", "2026-11-20", 35}, {"2023-03-31", "2023-04-30", 0},
		{"2023-03-31", "2023-05-01", 1}, {"2026-11-20", "2026-11-20", 0}, {"2027-01-01", "2026-11-20", 0},
	} {
		if got := MonthsBetween(d(c.a), d(c.b)); got != c.n {
			t.Errorf("%s to %s: %d, want %d", c.a, c.b, got, c.n)
		}
	}
}

func TestCheckBatteryCases(t *testing.T) {
	failed := &BatteryTest{Result: testFailed, End: day.Add(-time.Hour), InstalledOn: ""}
	passed := &BatteryTest{Result: testPassed, End: day.Add(-time.Hour)}
	for _, c := range []struct {
		name string
		in   BatteryInput
		want []string
	}{
		{"defaults: no signal", BatteryInput{Now: day, ReplaceAfterMonths: 36}, []string{}},
		// Case 2.1.
		{"2.1 the last test failed", BatteryInput{Now: day, ReplaceAfterMonths: 36, LastTest: failed}, []string{ReasonTestFailed}},
		{"2.1 clear: a new test passed", BatteryInput{Now: day, ReplaceAfterMonths: 36, LastTest: passed}, []string{}},
		{"2.1 clear: a new installed_on", BatteryInput{Now: day, InstalledOn: "2026-11-19", ReplaceAfterMonths: 36, LastTest: failed}, []string{}},
		// Case 2.2.
		{"2.2 age equal to the limit", BatteryInput{Now: day, InstalledOn: "2023-11-20", ReplaceAfterMonths: 36}, []string{ReasonAge}},
		{"2.2 age more than the limit", BatteryInput{Now: day, InstalledOn: "2020-01-01", ReplaceAfterMonths: 36}, []string{ReasonAge}},
		{"2.2 age less than the limit", BatteryInput{Now: day, InstalledOn: "2023-11-21", ReplaceAfterMonths: 36}, []string{}},
		{"2.2 the limit is a Setting", BatteryInput{Now: day, InstalledOn: "2025-11-20", ReplaceAfterMonths: 12}, []string{ReasonAge}},
		{"2.2 clear: a new installed_on", BatteryInput{Now: day, InstalledOn: "2026-11-20", ReplaceAfterMonths: 36}, []string{}},
		{"2.2 empty date: no age", BatteryInput{Now: day, InstalledOn: "", ReplaceAfterMonths: 12}, []string{}},
		// Both, in the order of step 2.
		{"2.1 and 2.2", BatteryInput{Now: day, InstalledOn: "2020-01-01", ReplaceAfterMonths: 36, LastTest: &BatteryTest{Result: testFailed, InstalledOn: "2020-01-01"}}, []string{ReasonTestFailed, ReasonAge}},
	} {
		st := CheckBattery(c.in)
		if !reflect.DeepEqual(st.Reasons, c.want) || st.Active != (len(c.want) > 0) {
			t.Errorf("%s: %+v, want %v", c.name, st, c.want)
		}
	}
	if st := CheckBattery(BatteryInput{Now: day, InstalledOn: "2023-11-20", ReplaceAfterMonths: 36}); st.AgeMonths == nil || *st.AgeMonths != 36 {
		t.Fatalf("age: %+v", st)
	}
}

// Case 2.3: no Dialect has a verified battery replace bit. QWS bits do
// not give BATTERY_REPLACE.
func TestBatteryReplaceNotFromQWS(t *testing.T) {
	e := New(nil, DefaultThresholds(), nil)
	for _, q := range []string{"0000000000000000000000000000000000000000000000000000000000000000", "1111111111111111111111111111111111111111111111111111111111111111", "1111111111111111111111111111111111111111111111111111111111111111"} {
		v := q
		for _, ev := range e.Observe(context.Background(), Observation{Time: day, QWS: &v}) {
			if ev.Type == BatteryReplace {
				t.Fatalf("QWS gave %+v", ev)
			}
		}
	}
	for _, a := range e.State().Active {
		if a == BatteryReplace {
			t.Fatal("BATTERY_REPLACE active from QWS")
		}
	}
}

func TestEngineBatteryEvents(t *testing.T) {
	e := New(nil, DefaultThresholds(), nil)
	var got []Event
	e.Subscribe(func(ev Event) { got = append(got, ev) })
	var kept []bool
	persist := func(a bool) { kept = append(kept, a) }
	ctx := context.Background()
	old := BatteryInput{Now: day, InstalledOn: "2022-01-10", ReplaceAfterMonths: 36}
	evs := e.Battery(ctx, old, persist)
	if len(evs) != 1 || evs[0].Type != BatteryReplace || evs[0].Severity != Warning || evs[0].Data["active"] != true || evs[0].Data["reason"] != ReasonAge || evs[0].Data["age_months"] != 58 {
		t.Fatalf("active: %+v", evs)
	}
	st := e.State()
	if st.Battery == nil || !st.Battery.Active || st.Battery.Reasons[0] != ReasonAge || st.Active[len(st.Active)-1] != BatteryReplace {
		t.Fatalf("state: %+v", st)
	}
	// The same state again: no new Event.
	if evs := e.Battery(ctx, old, persist); len(evs) != 0 {
		t.Fatalf("repeat: %+v", evs)
	}
	// Clear: the user sets a new installed_on.
	evs = e.Battery(ctx, BatteryInput{Now: day, InstalledOn: "2026-11-20", ReplaceAfterMonths: 36}, persist)
	if len(evs) != 1 || evs[0].Severity != Info || evs[0].Data["active"] != false {
		t.Fatalf("clear: %+v", evs)
	}
	if st := e.State(); st.Battery != nil {
		t.Fatalf("state after clear: %+v", st)
	}
	// Case 2.1, then the clear by a passed test.
	f := &BatteryTest{Result: testFailed, InstalledOn: "2026-11-20"}
	if evs := e.Battery(ctx, BatteryInput{Now: day, InstalledOn: "2026-11-20", ReplaceAfterMonths: 36, LastTest: f}, persist); len(evs) != 1 || evs[0].Data["reason"] != ReasonTestFailed {
		t.Fatalf("test failed: %+v", evs)
	}
	p := &BatteryTest{Result: testPassed, InstalledOn: "2026-11-20"}
	if evs := e.Battery(ctx, BatteryInput{Now: day, InstalledOn: "2026-11-20", ReplaceAfterMonths: 36, LastTest: p}, persist); len(evs) != 1 || evs[0].Data["active"] != false {
		t.Fatalf("test passed: %+v", evs)
	}
	if !reflect.DeepEqual(kept, []bool{true, false, true, false}) || len(got) != 4 {
		t.Fatalf("kept %v, published %d", kept, len(got))
	}
}

func TestBatteryStore(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "vlt.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.SetMACKey(bytes.Repeat([]byte{7}, 32)) // test key only
	in, err := BatteryFromStore(ctx, st, day)
	if err != nil || in.InstalledOn != "" || in.ReplaceAfterMonths != 36 || in.LastTest != nil {
		t.Fatalf("defaults: %+v %v", in, err)
	}
	// Setting checks (step 1).
	for _, d := range store.DefaultSettings() {
		switch d.Key {
		case store.SettingBatteryInstalledOn:
			for _, v := range []string{`""`, `"2023-11-20"`, `"1990-01-01"`} {
				if err := d.Check(json.RawMessage(v)); err != nil {
					t.Errorf("installed_on %s: %v", v, err)
				}
			}
			for _, v := range []string{`null`, `"2023-02-30"`, `"20.11.2023"`, `"2023-1-5"`, `"1989-12-31"`, `"2999-01-01"`, `5`} {
				if d.Check(json.RawMessage(v)) == nil {
					t.Errorf("installed_on %s accepted", v)
				}
			}
		case store.SettingBatteryReplaceAfter:
			for _, v := range []string{`12`, `36`, `120`} {
				if err := d.Check(json.RawMessage(v)); err != nil {
					t.Errorf("replace_after %s: %v", v, err)
				}
			}
			for _, v := range []string{`11`, `121`, `0`, `"36"`, `null`} {
				if d.Check(json.RawMessage(v)) == nil {
					t.Errorf("replace_after %s accepted", v)
				}
			}
		}
	}
	if _, err := st.SetSetting(ctx, store.SettingBatteryInstalledOn, json.RawMessage(`"2022-01-10"`), "gui"); err != nil {
		t.Fatal(err)
	}
	if err := SaveBatteryTest(ctx, st, testFailed, day); err != nil {
		t.Fatal(err)
	}
	in, err = BatteryFromStore(ctx, st, day)
	if err != nil || in.LastTest == nil || in.LastTest.Result != testFailed || in.LastTest.InstalledOn != "2022-01-10" {
		t.Fatalf("after test: %+v %v", in, err)
	}
	if s := CheckBattery(in); !reflect.DeepEqual(s.Reasons, []string{ReasonTestFailed, ReasonAge}) {
		t.Fatalf("reasons %v", s.Reasons)
	}
	// A new battery: both reasons clear.
	if _, err := st.SetSetting(ctx, store.SettingBatteryInstalledOn, json.RawMessage(`"2026-11-19"`), "gui"); err != nil {
		t.Fatal(err)
	}
	in, _ = BatteryFromStore(ctx, st, day)
	if s := CheckBattery(in); s.Active {
		t.Fatalf("after a new battery: %+v", s)
	}
	// A restart keeps the active state: no second Event.
	e := New(nil, DefaultThresholds(), nil)
	if err := st.SetMeta(ctx, MetaBatteryActive, []byte("1")); err != nil {
		t.Fatal(err)
	}
	e.LoadBatteryActive(ctx, st)
	if evs := e.Battery(ctx, BatteryInput{Now: day, InstalledOn: "2020-01-01", ReplaceAfterMonths: 36}, nil); len(evs) != 0 {
		t.Fatalf("restart gave %+v", evs)
	}
	// Only a test result is kept.
	if err := SaveBatteryTest(ctx, st, "CANCELLED", day); err != nil {
		t.Fatal(err)
	}
	in, _ = BatteryFromStore(ctx, st, day)
	if in.LastTest == nil || in.LastTest.Result != testFailed {
		t.Fatalf("non-result changed the last test: %+v", in.LastTest)
	}
}
