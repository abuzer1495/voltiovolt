// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/event"
)

// TestBatteryReplaceWiring is VLT-IS-012 TASK 03-00-00-004 step 7: a
// failed real UPS battery test gives BATTERY_REPLACE; a passed test clears
// it; an old battery.installed_on gives it; a new date clears it. No UPS
// and no port is used (no Lister).
func TestBatteryReplaceWiring(t *testing.T) {
	ctx := context.Background()
	d, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{dek: testOnlyDEK()})
	c := dialTest(t, addr)
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	l := d.live
	active := func() bool {
		st := l.events.State()
		return st.Battery != nil && st.Battery.Active
	}
	if active() {
		t.Fatal("active at start")
	}
	l.ctl.OnTestEnd(control.TestResult{Result: control.TestFailed, End: time.Now()})
	if st := l.events.State(); st.Battery == nil || st.Battery.Reasons[0] != event.ReasonTestFailed {
		t.Fatalf("after a failed test: %+v", st)
	}
	l.ctl.OnTestEnd(control.TestResult{Result: control.TestPassed, End: time.Now()})
	if active() {
		t.Fatal("still active after a passed test")
	}
	old := time.Now().AddDate(-4, 0, 0).Format("2006-01-02")
	if _, err := c.SetSetting(ctx, "battery.installed_on", old); err != nil {
		t.Fatal(err)
	}
	l.checkBattery(ctx)
	if st := l.events.State(); st.Battery == nil || st.Battery.Reasons[0] != event.ReasonAge {
		t.Fatalf("old battery: %+v", st)
	}
	if b, err := d.st.GetMeta(ctx, event.MetaBatteryActive); err != nil || string(b) != "1" {
		t.Fatalf("meta %q %v", b, err)
	}
	if _, err := c.SetSetting(ctx, "battery.installed_on", time.Now().Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	l.checkBattery(ctx)
	if active() {
		t.Fatal("still active after a new battery date")
	}
	// A test of the test connection is not kept (only the real UPS).
	l.tmode.SetActive(true)
	l.ctl.OnTestEnd(control.TestResult{Result: control.TestFailed, End: time.Now()})
	l.tmode.SetActive(false)
	l.checkBattery(ctx)
	if active() {
		t.Fatal("a test connection result counted")
	}
}
