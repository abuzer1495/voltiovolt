// SPDX-License-Identifier: AGPL-3.0-or-later

package event

import (
	"context"
	"encoding/json"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// VLT-IS-012 TASK 03-00-00-004 (Q-1): the Event BATTERY_REPLACE.

// BatteryReplace is the battery replace Event (WARNING).
const BatteryReplace = "BATTERY_REPLACE"

// Reasons of BATTERY_REPLACE, in the order of step 2.
const (
	ReasonTestFailed = "test_failed" // step 2.1: the last UPS battery test failed
	ReasonAge        = "age"         // step 2.2: the battery age
	// Step 2.3 (a verified battery replace bit of the Dialect) has no
	// reason: no Dialect has a verified bit now. QWS bits are not used.
)

// Battery test results (the same text as control.TestPassed and
// control.TestFailed; the package control is not imported here).
const (
	testPassed = "TEST_PASSED"
	testFailed = "TEST_FAILED"
)

// BatteryTest is the last UPS battery test that counts for
// BATTERY_REPLACE. It is kept in the meta table (MetaBatteryTest).
type BatteryTest struct {
	Result string    `json:"result"` // TEST_PASSED or TEST_FAILED
	End    time.Time `json:"end"`
	// InstalledOn is battery.installed_on at the end of the test. If the
	// user sets a new date later, the failed test is for the old battery
	// (step 3).
	InstalledOn string `json:"installed_on"`
}

// Meta keys.
const (
	MetaBatteryTest   = "battery.last_test"
	MetaBatteryActive = "battery.replace_active"
)

// BatteryInput is the input of the BATTERY_REPLACE check.
type BatteryInput struct {
	Now                time.Time
	InstalledOn        string // "" or YYYY-MM-DD
	ReplaceAfterMonths int
	LastTest           *BatteryTest
}

// BatteryState is the result of the check.
type BatteryState struct {
	Active  bool     `json:"active"`
	Reasons []string `json:"reasons"`
	// AgeMonths is the battery age in full months (only with InstalledOn).
	AgeMonths          *int   `json:"age_months,omitempty"`
	ReplaceAfterMonths int    `json:"replace_after_months"`
	InstalledOn        string `json:"installed_on,omitempty"`
}

// MonthsBetween returns the number of full months from a to b (0 if b is
// before a). 2023-03-31 to 2023-04-30 is 0 months; to 2023-05-01 is 1.
func MonthsBetween(a, b time.Time) int {
	if b.Before(a) {
		return 0
	}
	n := (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
	if b.Day() < a.Day() {
		n--
	}
	if n < 0 {
		n = 0
	}
	return n
}

// CheckBattery evaluates the conditions of step 2.
func CheckBattery(in BatteryInput) BatteryState {
	st := BatteryState{ReplaceAfterMonths: in.ReplaceAfterMonths, InstalledOn: in.InstalledOn, Reasons: []string{}}
	// Step 2.1 and step 3: a failed test counts until a test passes or
	// the user sets a new battery.installed_on.
	if t := in.LastTest; t != nil && t.Result == testFailed && t.InstalledOn == in.InstalledOn {
		st.Reasons = append(st.Reasons, ReasonTestFailed)
	}
	// Step 2.2.
	if in.InstalledOn != "" && in.ReplaceAfterMonths > 0 {
		if d, err := time.ParseInLocation(store.DateLayout, in.InstalledOn, in.Now.Location()); err == nil {
			age := MonthsBetween(d, in.Now)
			st.AgeMonths = &age
			if age >= in.ReplaceAfterMonths {
				st.Reasons = append(st.Reasons, ReasonAge)
			}
		}
	}
	st.Active = len(st.Reasons) > 0
	return st
}

// BatteryFromStore reads the input of the check from the Settings and the
// meta table.
func BatteryFromStore(ctx context.Context, st *store.Store, now time.Time) (BatteryInput, error) {
	in := BatteryInput{Now: now, ReplaceAfterMonths: 36}
	for key, out := range map[string]any{store.SettingBatteryInstalledOn: &in.InstalledOn, store.SettingBatteryReplaceAfter: &in.ReplaceAfterMonths} {
		s, err := st.GetSetting(ctx, key)
		if err != nil {
			return in, err
		}
		if err := json.Unmarshal(s.Value, out); err != nil {
			return in, err
		}
	}
	if b, err := st.GetMeta(ctx, MetaBatteryTest); err == nil && len(b) > 0 {
		var t BatteryTest
		if json.Unmarshal(b, &t) == nil && (t.Result == testPassed || t.Result == testFailed) {
			in.LastTest = &t
		}
	}
	return in, nil
}

// SaveBatteryTest keeps the result of a real UPS battery test (step 2.1)
// with the current battery.installed_on.
func SaveBatteryTest(ctx context.Context, st *store.Store, result string, end time.Time) error {
	if result != testPassed && result != testFailed {
		return nil
	}
	in, err := BatteryFromStore(ctx, st, end)
	if err != nil {
		return err
	}
	b, err := json.Marshal(BatteryTest{Result: result, End: end.UTC(), InstalledOn: in.InstalledOn})
	if err != nil {
		return err
	}
	return st.SetMeta(ctx, MetaBatteryTest, b)
}

// LoadBatteryActive sets the BATTERY_REPLACE state that was active at the
// last stop, so that a restart does not give the Event again.
func (e *Engine) LoadBatteryActive(ctx context.Context, st *store.Store) {
	b, err := st.GetMeta(ctx, MetaBatteryActive)
	if err != nil {
		return
	}
	e.mu.Lock()
	e.battery.Active = string(b) == "1"
	e.mu.Unlock()
}

// Battery evaluates BATTERY_REPLACE (step 2 and 3). It gives the Event
// when the state becomes active (WARNING) and the clear Event (INFO,
// "active": false) when it clears. persist, if not nil, keeps the active
// state.
func (e *Engine) Battery(ctx context.Context, in BatteryInput, persist func(active bool)) []Event {
	n := CheckBattery(in)
	e.mu.Lock()
	was := e.battery.Active
	e.battery = n
	e.mu.Unlock()
	if n.Active == was {
		return nil
	}
	if persist != nil {
		persist(n.Active)
	}
	d := map[string]any{"active": n.Active, "reasons": n.Reasons, "replace_after_months": n.ReplaceAfterMonths}
	if n.InstalledOn != "" {
		d["installed_on"] = n.InstalledOn
	}
	if n.AgeMonths != nil {
		d["age_months"] = *n.AgeMonths
	}
	if len(n.Reasons) > 0 {
		d["reason"] = n.Reasons[0]
	}
	sev := Warning
	if !n.Active {
		sev = Info
	}
	out := []Event{{Time: in.Now, Type: BatteryReplace, Severity: sev, Data: d}}
	e.publish(ctx, out)
	return out
}
