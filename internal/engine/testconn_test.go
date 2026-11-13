// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/measure"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
)

func startConsoleSim(t *testing.T) *sim.Model {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	m := sim.NewConsoleModel(sim.ProfileQ1)
	ln, err := server.ListenExclusive()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.New(m).Serve(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})
	return m
}

func TestTestConnectionUsesRealDriverAndControl(t *testing.T) {
	m := startConsoleSim(t)
	ctx := context.Background()
	ex := &fakeExecutor{ch: make(chan string, 4)}
	listed := 0
	d, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{dek: testOnlyDEK()}, func(c *Config) {
		c.Lister = func(context.Context, device.Config) ([]device.Candidate, error) {
			listed++
			t.Error("the OS lister must not run in the test connection")
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
	if err := c.SetDeviceSettings(ctx, map[string]any{"device.transport": "test"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 20*time.Second, "test connected", func() bool {
		st, err := c.DeviceStatus(ctx)
		return err == nil && st.State == device.StateConnected && st.Config.Transport == device.TransportTest
	})
	if listed != 0 {
		t.Fatalf("lister calls %d", listed)
	}
	st, err := c.DeviceStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Profile == nil || st.Profile.Dialect != dialect.DialectMegatec {
		t.Fatalf("profile %+v, want megatec (real Probe)", st.Profile)
	}
	waitFor(t, 15*time.Second, "input.voltage ~230", func() bool {
		_, rows, err := c.Measurements(ctx, measure.InputVoltage, time.Now().Add(-time.Minute), time.Now().Add(time.Minute), "raw")
		if err != nil || len(rows) == 0 {
			return false
		}
		v := rows[len(rows)-1].Mean
		return v >= 229 && v <= 231
	})
	res, err := c.UPSTestStart(ctx, wire.TestType10s, 0, wire.TestConfirmation)
	if err != nil {
		t.Fatal(err)
	}
	if res.DryRun {
		t.Fatalf("CONTROL was a dry-run (OD-47 sends CONTROL to the simulator): %+v", res)
	}
	waitFor(t, 5*time.Second, "CONTROL T on the simulator", func() bool {
		for _, x := range m.Controls() {
			if x == "T" || strings.HasPrefix(x, "T") {
				return true
			}
		}
		return false
	})
	if !d.live.tmode.Active() {
		t.Fatal("test mode not active")
	}
	wrap := &policy.Guarded{Real: ex, Dry: func() bool {
		if !d.live.tmode.Active() {
			return false
		}
		return !realShutdownOn(d.st)
	}, OnDry: func(string) { d.live.tmode.SetMessage(TestShutdownMessage) }}
	if err := wrap.Shutdown(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	if d.live.tmode.Message() != TestShutdownMessage {
		t.Fatalf("message %q", d.live.tmode.Message())
	}
	if n := len(ex.Calls()); n != 0 {
		t.Fatalf("OS executor called in a test: %v", ex.Calls())
	}
}

func TestSimulatorNotFound(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	ctx := context.Background()
	_, addr := startTestDaemon(t, dataDir(t), &testOnlyDEKProvider{dek: testOnlyDEK()}, func(c *Config) {
		c.Lister = func(context.Context, device.Config) ([]device.Candidate, error) {
			t.Error("the OS lister must not run in the test connection")
			return nil, nil
		}
	})
	c := dialTest(t, addr)
	if err := c.SetPIN(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, pin1); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDeviceSettings(ctx, map[string]any{"device.transport": "test"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 20*time.Second, "simulator not found", func() bool {
		st, err := c.DeviceStatus(ctx)
		return err == nil && st.State == device.StateSimulatorNotFound && st.Config.Transport == device.TransportTest
	})
}

func TestRetryWaitSimulatorNotFoundIs5s(t *testing.T) {
	if g := device.RetryWait(device.StateSimulatorNotFound, 0); g != 5*time.Second {
		t.Fatalf("RetryWait = %s, want 5s", g)
	}
	if g := device.RetryWait(device.StateSimulatorNotFound, 20); g != 5*time.Second {
		t.Fatalf("RetryWait stays 5s, got %s", g)
	}
	if g := device.RetryWait(device.StateNoDevice, 0); g != device.BackoffAt(0) {
		t.Fatalf("other states use BackoffAt, got %s", g)
	}
}
