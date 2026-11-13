// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/demo"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/policy"
)

// TestDemoModeNoControlAndNoShutdown is VLT-IS-009 TASK 03-00-00-005
// step 4: demo mode sends no CONTROL bytes and does not call the OS
// executor.
func TestDemoModeNoControlAndNoShutdown(t *testing.T) {
	ctx := context.Background()
	ex := &fakeExecutor{ch: make(chan string, 4)}
	listed := 0
	d, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{dek: testOnlyDEK()}, func(c *Config) {
		c.Lister = func(context.Context, device.Config) ([]device.Candidate, error) {
			listed++
			t.Error("the OS lister must not run in demo mode")
			return nil, nil
		}
		c.MakeExecutor = func(func() bool, *audit.Log) policy.Executor { return ex }
	})
	c := dialTest(t, addr)
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{"device.transport": "demo"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 20*time.Second, "demo connected", func() bool {
		st, err := c.DeviceStatus(ctx)
		return err == nil && st.State == device.StateConnected && st.Config.Transport == device.TransportDemo
	})
	if listed != 0 {
		t.Fatalf("lister calls %d", listed)
	}
	sess := d.live.demo
	if sess == nil || !sess.Active() {
		t.Fatal("demo session not active")
	}
	before := append([]string(nil), sess.Sim().Received()...)
	if err := c.DemoPowerFail(ctx); err != nil {
		t.Fatal(err)
	}
	if sess.Model().MainsOn() {
		t.Fatal("power fail: still on mains")
	}
	res, err := c.UPSTestStart(ctx, wire.TestType10s, 0, wire.TestConfirmation)
	if err != nil || !res.DryRun || !res.Verified || res.Bytes != "T" {
		t.Fatalf("test start: %+v %v", res, err)
	}
	res, err = c.UPSBeeperSet(ctx, false)
	if err != nil || !res.DryRun {
		t.Fatalf("beeper: %+v %v", res, err)
	}
	if got := sess.Model().Controls(); len(got) != 0 {
		t.Fatalf("CONTROL reached the simulator: %v", got)
	}
	for _, cmd := range sess.Sim().Received()[len(before):] {
		if e, ok := dialect.LookupRead(cmd); !ok || e.Class != dialect.ClassRead {
			t.Fatalf("non-READ command in demo: %q", cmd)
		}
	}
	// Direct call through the demo executor path used by the Policy Engine.
	wrap := &demo.Executor{Real: ex, Sess: sess}
	if err := wrap.Shutdown(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	if sess.ShutdownMessage() != demo.ShutdownMessageEN {
		t.Fatalf("message %q", sess.ShutdownMessage())
	}
	if n := len(ex.Calls()); n != 0 {
		t.Fatalf("OS executor called in demo: %v", ex.Calls())
	}
}
