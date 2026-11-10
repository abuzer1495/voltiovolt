// SPDX-License-Identifier: AGPL-3.0-or-later

package event

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func openDB(t *testing.T) (*store.Store, *sql.DB) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vlt.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st, st.DB()
}

// bits returns the flags of a status byte "b7b6b5b4b3b2b1b0".
func bits(t *testing.T, raw string) *dialect.Q1Flags {
	t.Helper()
	m := sim.NewModel(sim.ProfileQ1)
	resp := m.Respond("Q1")
	// Replace the 8 status bits at the end of the response.
	b := append([]byte(nil), resp...)
	copy(b[len(b)-9:len(b)-1], raw)
	v, err := dialect.Parse("Q1", b)
	if err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	f := v.(dialect.Status).Flags
	return &f
}

type run struct {
	t  *testing.T
	e  *Engine
	n  int
	ev []Event
}

func newRun(t *testing.T) *run {
	_, db := openDB(t)
	return &run{t: t, e: New(db, DefaultThresholds(), nil)}
}

func (r *run) obs(o Observation) []Event {
	r.n++
	o.Time = t0.Add(time.Duration(r.n) * 2 * time.Second)
	got := r.e.Observe(context.Background(), o)
	r.ev = append(r.ev, got...)
	return got
}

func (r *run) status(raw string) []Event { return r.obs(Observation{Flags: bits(r.t, raw)}) }

func one(t *testing.T, got []Event, typ, sev string, active bool) {
	t.Helper()
	if len(got) != 1 || got[0].Type != typ || got[0].Severity != sev {
		t.Fatalf("got %+v; want %s %s", got, typ, sev)
	}
	if typ != Online && typ != WarningBitsChanged && typ != CommLost && typ != CommRestored && got[0].Data["active"] != active {
		t.Fatalf("active %v", got[0].Data)
	}
}

func none(t *testing.T, got []Event) {
	t.Helper()
	if len(got) != 0 {
		t.Fatalf("got %+v; want no Event", got)
	}
}

// TestStatusBitRows tests each row of the status bit table (step 1) with
// the debounce of step 5.
func TestStatusBitRows(t *testing.T) {
	const base = "00000000"
	cases := []struct {
		name  string
		raw   string
		typ   string
		sev   string
		polls int
	}{
		{"bit 7 utility fail", "10000000", OnBattery, Warning, 1},
		{"bit 6 battery low", "01000000", LowBattery, Critical, 1},
		{"bit 5 and not bit 3: bypass", "00100000", BypassActive, Warning, 2},
		{"bit 5 and bit 3: boost or buck", "00101000", AVRActive, Info, 2},
		{"bit 4 UPS failed", "00010000", UPSFault, Critical, 2},
		{"bit 2 test", "00000100", TestStarted, Info, 2},
		{"bit 1 shutdown active", "00000010", UPSShutdownActive, Critical, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRun(t)
			none(t, r.status(base))
			for i := 1; i < c.polls; i++ {
				none(t, r.status(c.raw))
			}
			one(t, r.status(c.raw), c.typ, c.sev, true)
			none(t, r.status(c.raw)) // no repeat
			// Clear after 2 polls in sequence.
			none(t, r.status(base))
			if c.typ == OnBattery {
				one(t, r.status(base), Online, Info, false)
			} else {
				one(t, r.status(base), c.typ, Info, false)
			}
			// Only the expected Event types.
			for _, e := range r.ev {
				if e.Type != c.typ && e.Type != Online {
					t.Fatalf("other Event %+v", e)
				}
			}
		})
	}
	// Bit 3 alone (standby type) and bit 0 (beeper) make no Event.
	r := newRun(t)
	for i := 0; i < 4; i++ {
		none(t, r.status("00001001"))
	}
}

func TestDebounce(t *testing.T) {
	r := newRun(t)
	// One poll with bit 4 is not enough (2 polls in sequence).
	none(t, r.status("00010000"))
	none(t, r.status("00000000"))
	none(t, r.status("00010000"))
	one(t, r.status("00010000"), UPSFault, Critical, true)
	// One poll without the bit does not clear.
	none(t, r.status("00000000"))
	none(t, r.status("00010000"))
	none(t, r.status("00000000"))
	one(t, r.status("00000000"), UPSFault, Info, false)
	// ON_BATTERY after 1 poll; ONLINE after 2 polls.
	one(t, r.status("10000000"), OnBattery, Warning, true)
	none(t, r.status("00000000"))
	none(t, r.status("10000000"))
	none(t, r.status("00000000"))
	one(t, r.status("00000000"), Online, Info, false)
	// A cycle without status bits does not change the counters.
	none(t, r.status("00010000"))
	none(t, r.obs(Observation{}))
	one(t, r.status("00010000"), UPSFault, Critical, true)
}

func f(v float64) *float64 { return &v }

func TestThresholdsAndHysteresis(t *testing.T) {
	r := newRun(t)
	// OVERLOAD: 100 % or more; clears below 95 %.
	none(t, r.obs(Observation{Load: f(99.9)}))
	none(t, r.obs(Observation{Load: f(100)}))
	one(t, r.obs(Observation{Load: f(100)}), Overload, Warning, true)
	none(t, r.obs(Observation{Load: f(96)}))
	none(t, r.obs(Observation{Load: f(95)})) // 95 is not below 95
	none(t, r.obs(Observation{Load: f(94.9)}))
	one(t, r.obs(Observation{Load: f(94.9)}), Overload, Info, false)

	// OVER_TEMPERATURE: 50 °C or more; clears below 47.5 °C.
	none(t, r.obs(Observation{Temp: f(50)}))
	one(t, r.obs(Observation{Temp: f(51)}), OverTemperature, Warning, true)
	none(t, r.obs(Observation{Temp: f(48)}))
	none(t, r.obs(Observation{Temp: f(48)}))
	none(t, r.obs(Observation{Temp: f(47.4)}))
	one(t, r.obs(Observation{Temp: f(47.4)}), OverTemperature, Info, false)

	// RUNTIME_LOW: less than 5 minutes, only on battery; clears at 5.25.
	none(t, r.obs(Observation{Runtime: f(3)}))
	none(t, r.obs(Observation{Runtime: f(3)}))
	one(t, r.status("10000000"), OnBattery, Warning, true)
	none(t, r.obs(Observation{Runtime: f(4.9)}))
	got := r.obs(Observation{Runtime: f(4.9)})
	one(t, got, RuntimeLow, Warning, true)
	if got[0].Data["runtime_minutes"] != 4.9 {
		t.Fatalf("%v", got[0].Data)
	}
	none(t, r.obs(Observation{Runtime: f(5.2)}))
	none(t, r.obs(Observation{Runtime: f(5.2)}))
	none(t, r.obs(Observation{Runtime: f(5.25)}))
	one(t, r.obs(Observation{Runtime: f(5.25)}), RuntimeLow, Info, false)
}

func TestThresholdSettings(t *testing.T) {
	st, _ := openDB(t)
	th, err := ThresholdsFromStore(context.Background(), st)
	if err != nil || th != DefaultThresholds() {
		t.Fatalf("%+v %v", th, err)
	}
	r := newRun(t)
	r.e.SetThresholds(Thresholds{OverloadPercent: 80, OverTemperatureC: 50, RuntimeLowMinutes: 5})
	none(t, r.obs(Observation{Load: f(80)}))
	one(t, r.obs(Observation{Load: f(80)}), Overload, Warning, true)
}

func TestQWSChange(t *testing.T) {
	r := newRun(t)
	a, b := "0000", "0100"
	none(t, r.obs(Observation{QWS: &a})) // first value
	none(t, r.obs(Observation{QWS: &a}))
	none(t, r.obs(Observation{QWS: &b}))
	got := r.obs(Observation{QWS: &b})
	one(t, got, WarningBitsChanged, Info, false)
	if got[0].Data["old"] != a || got[0].Data["new"] != b {
		t.Fatalf("%v", got[0].Data)
	}
	none(t, r.obs(Observation{QWS: &b}))
	// A one-poll change is debounced.
	none(t, r.obs(Observation{QWS: &a}))
	none(t, r.obs(Observation{QWS: &b}))
}

// TestQMODAndQGSAreDataOnly is step 2.
func TestQMODAndQGSAreDataOnly(t *testing.T) {
	m := sim.NewModel(sim.ProfileVoltronic)
	m.SetMains(false)
	m.SetFault(sim.FaultUPSFailed, true)
	var o Observation
	for _, cmd := range []string{"QGS", "QMOD"} {
		v, err := dialect.Parse(cmd, m.Respond(cmd))
		if err != nil {
			t.Fatal(err)
		}
		o.Add(cmd, v)
	}
	if o.Flags != nil || o.Load == nil || o.Temp == nil {
		t.Fatalf("%+v", o)
	}
	r := newRun(t)
	for i := 0; i < 3; i++ {
		none(t, r.obs(o))
	}
	// QS gives the flags.
	v, _ := dialect.Parse("QS", m.Respond("QS"))
	o.Add("QS", v)
	if o.Flags == nil || !o.Flags.UtilityFail {
		t.Fatalf("%+v", o.Flags)
	}
}

func TestCommLost(t *testing.T) {
	ctx := context.Background()
	r := newRun(t)
	one(t, r.e.CommState(ctx, t0, true), CommLost, Warning, false)
	none(t, r.e.CommState(ctx, t0, true))
	one(t, r.e.CommState(ctx, t0, false), CommRestored, Info, false)
	// On battery: CRITICAL.
	one(t, r.status("10000000"), OnBattery, Warning, true)
	got := r.e.CommState(ctx, t0, true)
	one(t, got, CommLost, Critical, false)
	if got[0].Data["on_battery"] != true || !r.e.State().CommLost || !r.e.State().OnBattery {
		t.Fatalf("%+v %+v", got, r.e.State())
	}
}

func TestStoreListSubscribe(t *testing.T) {
	ctx := context.Background()
	_, db := openDB(t)
	e := New(db, DefaultThresholds(), nil)
	var got []Event
	cancel := e.Subscribe(func(ev Event) { got = append(got, ev) })
	r := &run{t: t, e: e}
	r.status("10000000")
	r.status("01000000") // LOW_BATTERY; ON_BATTERY not yet clear
	e.CommState(ctx, t0, true)
	cancel()
	e.CommState(ctx, t0, false)
	if len(got) != 3 {
		t.Fatalf("subscriber got %d", len(got))
	}
	all, err := List(ctx, db, 0, 10)
	if err != nil || len(all) != 4 {
		t.Fatalf("%d %v", len(all), err)
	}
	if all[0].Type != CommRestored || all[3].Type != OnBattery || all[3].Severity != Warning {
		t.Fatalf("%+v", all)
	}
	page, _ := List(ctx, db, all[1].ID, 2)
	if len(page) != 2 || page[0].ID != all[2].ID {
		t.Fatalf("page %+v", page)
	}
	if _, err := List(ctx, db, 0, 0); err == nil {
		t.Fatal("limit 0 accepted")
	}
	if s := e.State(); !s.OnBattery || !s.LowBattery {
		t.Fatalf("%+v", s)
	}
}
