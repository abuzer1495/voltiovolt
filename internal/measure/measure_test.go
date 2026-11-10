// SPDX-License-Identifier: AGPL-3.0-or-later

package measure

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "vlt.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st.DB()
}

func count(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func parse(t *testing.T, cmd string, m *sim.Model) any {
	t.Helper()
	v, err := dialect.Parse(cmd, m.Respond(cmd))
	if err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
	return v
}

func names(ps []Point) map[string]Point {
	out := map[string]Point{}
	for _, p := range ps {
		out[p.Name] = p
	}
	return out
}

// TestFromValue checks the table of step 1 with the simulator responses.
func TestFromValue(t *testing.T) {
	v := sim.NewModel(sim.ProfileVoltronic)
	qgs := names(FromValue("QGS", parse(t, "QGS", v)))
	for _, n := range []string{InputVoltage, InputFrequency, OutputVoltage, OutputFrequency, OutputCurrent, LoadPercent, BatteryVoltage, UPSTemperature} {
		if p, ok := qgs[n]; !ok || p.Source != "QGS" || p.Unit != Units[n] {
			t.Errorf("QGS %s: %+v", n, p)
		}
	}
	if _, ok := qgs[InputFaultVoltage]; ok || len(qgs) != 8 {
		t.Errorf("QGS: %v", qgs)
	}
	qbv := names(FromValue("QBV", parse(t, "QBV", v)))
	if len(qbv) != 3 || qbv[BatteryCharge].Value != 100 || qbv[BatteryRuntime].Unit != "min" {
		t.Errorf("QBV: %v", qbv)
	}
	q1 := names(FromValue("Q1", parse(t, "Q1", sim.NewModel(sim.ProfileQ1))))
	for _, n := range []string{InputVoltage, InputFaultVoltage, InputFrequency, OutputVoltage, LoadPercent, BatteryVoltage, UPSTemperature} {
		if p, ok := q1[n]; !ok || p.Source != "Q1" {
			t.Errorf("Q1 %s: %+v", n, p)
		}
	}
	if q1[BatteryVoltage].Value != 2.32 {
		t.Errorf("Q1 battery voltage as received: %v", q1[BatteryVoltage])
	}
	qs := names(FromValue("QS", parse(t, "QS", sim.NewModel(sim.ProfileQS))))
	if qs[LoadPercent].Source != "QS" || len(qs) != 7 {
		t.Errorf("QS: %v", qs)
	}
	for _, cmd := range []string{"QMOD", "QWS", "QFLAG", "QRI", "QVFW", "QMD"} {
		if ps := FromValue(cmd, parse(t, cmd, v)); len(ps) != 0 {
			t.Errorf("%s gives %v", cmd, ps)
		}
	}
}

// TestNoValueFields is step 2 and 5: a field with no value is not stored,
// and zero is not stored in its place.
func TestNoValueFields(t *testing.T) {
	st := dialect.Status{
		Command:      "Q1",
		InputVoltage: dialect.Volts{Value: 230, Valid: true},
		Load:         dialect.Percent{Raw: "---"},            // no value
		Temperature:  dialect.Celsius{Value: 0, Raw: "--.-"}, // no value
	}
	ps := FromValue("Q1", st)
	if len(ps) != 1 || ps[0].Name != InputVoltage {
		t.Fatalf("%v", ps)
	}
	db := openDB(t)
	w := &Writer{DB: db}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if err := w.WriteCycle(context.Background(), []Sample{{Time: now, Points: ps}}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "SELECT count(*) FROM measurements"); n != 1 {
		t.Fatalf("%d rows", n)
	}
	if n := count(t, db, "SELECT count(*) FROM measurements WHERE value = 0 OR value IS NULL"); n != 0 {
		t.Fatalf("%d zero rows", n)
	}
	// NaN is refused.
	if err := w.WriteCycle(context.Background(), []Sample{{Time: now, Points: []Point{{Name: LoadPercent, Unit: "%", Value: math.NaN()}}}}); err == nil {
		t.Fatal("NaN accepted")
	}
	if err := w.WriteCycle(context.Background(), []Sample{{Time: now, Points: []Point{{Name: "x", Unit: "V", Value: 1}}}}); err == nil {
		t.Fatal("unknown name accepted")
	}
}

// TestOneTransaction is step 3: if one row fails, no row of the cycle is
// stored.
func TestOneTransaction(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`CREATE TEMP TRIGGER fail_temp BEFORE INSERT ON main.measurements
		WHEN NEW.name = 'ups.temperature' BEGIN SELECT RAISE(ABORT, 'test'); END`); err != nil {
		t.Fatal(err)
	}
	ps := FromValue("QGS", parse(t, "QGS", sim.NewModel(sim.ProfileVoltronic)))
	now := time.Now()
	err := (&Writer{DB: db}).WriteCycle(context.Background(), []Sample{{Time: now, Points: ps}})
	if err == nil {
		t.Fatal("no error")
	}
	if n := count(t, db, "SELECT count(*) FROM measurements"); n != 0 {
		t.Fatalf("%d rows after a failed cycle", n)
	}
	if _, err := db.Exec("DROP TRIGGER fail_temp"); err != nil {
		t.Fatal(err)
	}
	if err := (&Writer{DB: db}).WriteCycle(context.Background(), []Sample{{Time: now, Points: ps}}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "SELECT count(*) FROM measurements WHERE ts = ?", now.UnixMilli()); n != len(ps) {
		t.Fatalf("%d rows", n)
	}
}

func write(t *testing.T, db *sql.DB, ts time.Time, name string, v float64) {
	t.Helper()
	if err := (&Writer{DB: db}).WriteCycle(context.Background(), []Sample{{Time: ts, Points: []Point{{Name: name, Unit: Units[name], Value: v, Source: "QGS"}}}}); err != nil {
		t.Fatal(err)
	}
}

// TestSummaries is step 5: minimum, maximum and mean.
func TestSummaries(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	base := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	// Minute 0: 10, 20, 30. Minute 1: 40. Minute 61 (next hour): 5.
	write(t, db, base.Add(2*time.Second), LoadPercent, 10)
	write(t, db, base.Add(20*time.Second), LoadPercent, 20)
	write(t, db, base.Add(59*time.Second), LoadPercent, 30)
	write(t, db, base.Add(70*time.Second), LoadPercent, 40)
	write(t, db, base.Add(61*time.Minute), LoadPercent, 5)
	write(t, db, base.Add(5*time.Second), UPSTemperature, 25)

	// At 10:01:30, only minute 0 is complete.
	if err := Maintain(ctx, db, base.Add(90*time.Second)); err != nil {
		t.Fatal(err)
	}
	_, rows, err := Query(ctx, db, LoadPercent, base, base.Add(3*time.Hour), StepMinute)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Min != 10 || rows[0].Max != 30 || rows[0].Mean != 20 || rows[0].Count != 3 || !rows[0].TS.Equal(base) {
		t.Fatalf("1m %+v", rows)
	}
	// Run again later: the minutes and the first hour are complete.
	if err := Maintain(ctx, db, base.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := Maintain(ctx, db, base.Add(2*time.Hour)); err != nil { // the same again: no change
		t.Fatal(err)
	}
	_, rows, _ = Query(ctx, db, LoadPercent, base, base.Add(3*time.Hour), StepMinute)
	if len(rows) != 3 || rows[1].Mean != 40 || rows[2].Mean != 5 {
		t.Fatalf("1m %+v", rows)
	}
	unit, hrs, err := Query(ctx, db, LoadPercent, base, base.Add(3*time.Hour), StepHour)
	if err != nil || unit != "%" {
		t.Fatal(unit, err)
	}
	// Hour 10: values 10, 20, 30, 40: min 10, max 40, mean 25 (weighted).
	if len(hrs) != 2 || hrs[0].Min != 10 || hrs[0].Max != 40 || hrs[0].Mean != 25 || hrs[0].Count != 4 || hrs[1].Mean != 5 {
		t.Fatalf("1h %+v", hrs)
	}
	_, tr, _ := Query(ctx, db, UPSTemperature, base, base.Add(3*time.Hour), StepHour)
	if len(tr) != 1 || tr[0].Mean != 25 {
		t.Fatalf("temperature %+v", tr)
	}
	_, raw, _ := Query(ctx, db, LoadPercent, base, base.Add(time.Minute), StepRaw)
	if len(raw) != 3 || raw[2].Mean != 30 || raw[0].Source != "QGS" {
		t.Fatalf("raw %+v", raw)
	}
	for _, bad := range []struct {
		name, step string
		from, to   time.Time
	}{
		{"x", StepRaw, base, base.Add(time.Hour)},
		{LoadPercent, "5m", base, base.Add(time.Hour)},
		{LoadPercent, StepRaw, base, base},
	} {
		if _, _, err := Query(ctx, db, bad.name, bad.from, bad.to, bad.step); err == nil {
			t.Errorf("%+v: no error", bad)
		}
	}
}

// TestRetention is step 4 and 5.
func TestRetention(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	old := now.Add(-RawRetention - time.Hour) // 7 days and 1 hour ago
	write(t, db, old, LoadPercent, 50)
	write(t, db, now.Add(-time.Hour), LoadPercent, 60)
	// An old 1-minute value and an old 1-hour value (more than 400 days).
	veryOld := now.Add(-MinuteRetention - 24*time.Hour).UnixMilli()
	veryOld -= veryOld % hourMs
	for _, tbl := range []string{"measurements_1m", "measurements_1h"} {
		if _, err := db.Exec("INSERT INTO "+tbl+" (ts, name, unit, min, max, mean, count) VALUES (?, 'load.percent', '%', 1, 1, 1, 1)", veryOld); err != nil {
			t.Fatal(err)
		}
	}
	if err := Maintain(ctx, db, now); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "SELECT count(*) FROM measurements"); n != 1 {
		t.Fatalf("raw rows %d; want 1 (the old value is deleted)", n)
	}
	// The old raw value is in the summaries before it is deleted.
	if n := count(t, db, "SELECT count(*) FROM measurements_1m WHERE ts = ?", old.UnixMilli()-old.UnixMilli()%minuteMs); n != 1 {
		t.Fatalf("1m of the old value: %d", n)
	}
	if n := count(t, db, "SELECT count(*) FROM measurements_1m WHERE ts = ?", veryOld); n != 0 {
		t.Fatal("1-minute value older than 400 days is not deleted")
	}
	if n := count(t, db, "SELECT count(*) FROM measurements_1h WHERE ts = ?", veryOld); n != 1 {
		t.Fatal("1-hour value deleted (no limit)")
	}
}

type fakeClock struct {
	now   time.Time
	n     int
	limit int
}

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	c.n++
	c.now = c.now.Add(d)
	if c.n >= c.limit {
		return context.Canceled
	}
	return nil
}

func TestRunJobHourly(t *testing.T) {
	db := openDB(t)
	base := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	write(t, db, base.Add(time.Second), LoadPercent, 10)
	clk := &fakeClock{now: base.Add(30 * time.Second), limit: 3}
	RunJob(context.Background(), db, clk, nil)
	if clk.n != 3 || !clk.now.Equal(base.Add(30*time.Second+3*time.Hour)) {
		t.Fatalf("%d sleeps, %v", clk.n, clk.now)
	}
	if n := count(t, db, "SELECT count(*) FROM measurements_1h"); n != 1 {
		t.Fatalf("1h rows %d", n)
	}
}
