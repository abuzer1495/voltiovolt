// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/engine"
	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/measure"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// selfTestPIN is a one-time PIN for --self-test --test-connection. It is
// not a Secret of a real installation (the data folder is temporary).
const selfTestPIN = "Vq7k2m9x"

// SelfTestConnectionResult is the result file of the test-connection
// self-test (VLT-IS-010 TASK 03-00-00-005).
const SelfTestConnectionResult = "self-test-connection-result.txt"

// SelfTestOpts are the checks of SelfTestConnection.
type SelfTestOpts struct {
	// ExpectDialect is "megatec" or "voltronic". Empty: "megatec".
	ExpectDialect string
	// ExpectEvent, when not empty, is an Event type to wait for (for
	// example "ON_BATTERY").
	ExpectEvent string
	// Timeout is the time limit of the whole check. 0: 60 seconds.
	Timeout time.Duration
}

// selfTestAfterVoltage, when set (tests), runs after the 230 V check and
// before the Event wait, so the test can drop the mains.
var selfTestAfterVoltage func()

// SelfTestConnection starts the portable engine with the test connection
// (OD-44), talks to a running vlt-simulator through the real Driver
// (Exchange, Probe, Poller), checks the Dialect and the Q1 input voltage,
// optionally waits for an Event, and checks that the OS shutdown is a
// dry-run (OD-46). No real hardware, no real shutdown (SEC-092).
func SelfTestConnection(dir string, opts SelfTestOpts) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if opts.ExpectDialect == "" {
		opts.ExpectDialect = "megatec"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ln := ipc.NewPipeListener()
	defer ln.Close()
	d, err := startEngine(dir, log, nil, func() (net.Listener, ipc.PeerCheck, error) {
		return ln, ipc.InProcessCheck, nil
	}, func(c *engine.Config) {
		// Always a dry-run executor: this self-test must never shut down
		// the computer (OD-46, default test.real_shutdown off).
		c.MakeExecutor = func(func() bool, *audit.Log) policy.Executor { return &policy.DryRun{} }
	})
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = d.Stop(ctx)
	}()

	conn, err := ln.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	cl := typed.New(conn)
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	if err := cl.SetPIN(ctx, selfTestPIN); err != nil {
		return fmt.Errorf("set PIN: %w", err)
	}
	if err := cl.Login(ctx, selfTestPIN); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if err := cl.SetDeviceSettings(ctx, map[string]any{"device.transport": device.TransportTest}); err != nil {
		return fmt.Errorf("device.transport=test: %w", err)
	}

	var st device.Status
	if err := waitFor(ctx, "test connection", func() error {
		var e error
		st, e = cl.DeviceStatus(ctx)
		if e != nil {
			return e
		}
		if st.State != device.StateConnected {
			return fmt.Errorf("state %s", st.State)
		}
		if st.Config.Transport != device.TransportTest {
			return fmt.Errorf("transport %s", st.Config.Transport)
		}
		if st.Profile == nil || string(st.Profile.Dialect) != opts.ExpectDialect {
			d := ""
			if st.Profile != nil {
				d = string(st.Profile.Dialect)
			}
			return fmt.Errorf("dialect %q, want %q", d, opts.ExpectDialect)
		}
		return nil
	}); err != nil {
		return err
	}

	var volts float64
	if err := waitFor(ctx, "input.voltage 230", func() error {
		_, rows, e := cl.Measurements(ctx, measure.InputVoltage, time.Now().Add(-time.Minute), time.Now().Add(time.Minute), "raw")
		if e != nil {
			return e
		}
		if len(rows) == 0 {
			return fmt.Errorf("no measurements")
		}
		volts = rows[len(rows)-1].Mean
		if volts < 229 || volts > 231 {
			return fmt.Errorf("input.voltage %v, want ~230", volts)
		}
		return nil
	}); err != nil {
		return err
	}

	if selfTestAfterVoltage != nil {
		selfTestAfterVoltage()
	}

	gotEvent := ""
	if opts.ExpectEvent != "" {
		if err := waitFor(ctx, "event "+opts.ExpectEvent, func() error {
			list, _, e := cl.Events(ctx, 0, 100)
			if e != nil {
				return e
			}
			for _, ev := range list {
				if ev.Type == opts.ExpectEvent {
					gotEvent = ev.Type
					return nil
				}
			}
			return fmt.Errorf("not yet")
		}); err != nil {
			return err
		}
	}

	// OD-46: test.real_shutdown stays at the default (off). The executor
	// is policy.DryRun. No real OS shutdown can happen.
	shutdown := "dry"
	s, err := cl.Settings(ctx)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	for _, row := range s {
		if row.Key == store.SettingTestRealShutdown && string(row.Value) != "false" && string(row.Value) != "false\n" {
			return fmt.Errorf("test.real_shutdown is %s, want false (no real OS shutdown)", row.Value)
		}
	}
	body := fmt.Sprintf("ok dialect=%s input=%.1f shutdown=%s origin=test", opts.ExpectDialect, volts, shutdown)
	if gotEvent != "" {
		body += " event=" + gotEvent
	}
	body += "\n"
	result := filepath.Join(dir, SelfTestConnectionResult)
	if err := os.WriteFile(result, []byte(body), 0o600); err != nil {
		return err
	}
	fmt.Print(body)
	return nil
}

func waitFor(ctx context.Context, what string, f func() error) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return fmt.Errorf("%s: %w (%v)", what, err, last)
			}
			return fmt.Errorf("%s: %w", what, err)
		}
		if err := f(); err == nil {
			return nil
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w (%v)", what, ctx.Err(), last)
		case <-time.After(200 * time.Millisecond):
		}
	}
}
