// SPDX-License-Identifier: AGPL-3.0-or-later

package measure

import (
	"math"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

func q1(t *testing.T, s string) dialect.Status {
	t.Helper()
	st, err := dialect.ParseQ1([]byte(s + "\r"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func fr(t *testing.T, s string) *dialect.Rating {
	t.Helper()
	r, err := dialect.ParseF([]byte(s + "\r"))
	if err != nil {
		t.Fatal(err)
	}
	return &r
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestChargeEstimate(t *testing.T) {
	c := DefaultEstimateConfig()
	f24 := fr(t, "#220.0 004 024.0 50.0")
	cases := []struct {
		name  string
		q1    string
		rated *dialect.Rating
		want  float64
		ok    bool
	}{
		// On-line (b3 = 0): per cell, 1.667 V = 0 %, 2.267 V = 100 %.
		{"on-line 2.25 V", "(229.8 229.8 219.9 034 50.1 2.25 24.0 00000001", nil, 100 * (2.25 - 1.667) / 0.6, true},
		{"on-line 2.10 V", "(000.0 228.0 220.0 035 50.0 2.10 24.5 10000001", nil, 100 * (2.10 - 1.667) / 0.6, true},
		// Above 90 %: not rounded to 100.
		{"on-line 2.22 V", "(229.8 229.8 219.9 034 50.1 2.22 24.0 00000001", nil, 100 * (2.22 - 1.667) / 0.6, true},
		{"on-line above full", "(229.8 229.8 219.9 034 50.1 2.30 24.0 00000001", nil, 100, true},
		{"on-line below empty", "(229.8 229.8 219.9 034 50.1 1.50 24.0 00000001", nil, 0, true},
		// Standby (b3 = 1): F 24.0 V = 2 blocks, 20.0 V = 0 %, 27.2 V = 100 %.
		{"standby 27.0 V", "(230.1 230.1 230.1   7 50.0 27.0 25.0 00001000", f24, 100 * (27.0 - 20.0) / 7.2, true},
		{"standby F 24.00", "(230.1 230.1 230.1   7 50.0 25.0 25.0 00001000", fr(t, "#230.0 4.3 24.00 50.0"), 100 * 5.0 / 7.2, true},
		{"standby no F", "(230.1 230.1 230.1   7 50.0 27.0 25.0 00001000", nil, 0, false},
		{"standby F no value", "(230.1 230.1 230.1   7 50.0 27.0 25.0 00001000", fr(t, "#220.0 004       50.0"), 0, false},
		{"no battery value", "(229.8 229.8 219.9 034 50.1      24.0 00000001", nil, 0, false},
	}
	for _, x := range cases {
		got, ok := ChargeEstimate(q1(t, x.q1), x.rated, c)
		if ok != x.ok || (ok && !near(got, x.want)) {
			t.Errorf("%s: %v %v, want %v %v", x.name, got, ok, x.want, x.ok)
		}
	}
	// 2.22 V gives 92.17 %: not 100 (no rounding above 90).
	if v, _ := ChargeEstimate(q1(t, "(229.8 229.8 219.9 034 50.1 2.22 24.0 00000001"), nil, c); v >= 100 || v <= 90 {
		t.Errorf("2.22 V: %v", v)
	}
	// A wrong Setting pair (empty >= full) gives no value.
	bad := c
	bad.CellEmptyV = bad.CellFullV
	if _, ok := ChargeEstimate(q1(t, "(229.8 229.8 219.9 034 50.1 2.25 24.0 00000001"), nil, bad); ok {
		t.Error("empty = full gives a value")
	}
	if Units[BatteryChargeEstimate] != "%" {
		t.Error("unit")
	}
}
