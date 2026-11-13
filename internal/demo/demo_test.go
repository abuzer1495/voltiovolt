// SPDX-License-Identifier: AGPL-3.0-or-later

package demo

import (
	"context"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/policy"
)

func TestPowerFailRestoresMains(t *testing.T) {
	s := New()
	s.SetActive(true)
	if !s.Model().MainsOn() {
		t.Fatal("start on mains")
	}
	s.PowerFail(25 * time.Millisecond)
	if s.Model().MainsOn() {
		t.Fatal("want battery")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.Model().MainsOn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("mains did not return")
}

func TestExecutorIsDryRunWhenActive(t *testing.T) {
	real := &policy.DryRun{}
	s := New()
	e := &policy.Guarded{Real: real, Dry: s.Active, OnDry: func(string) { s.SetShutdownMessage(ShutdownMessageEN) }}
	if err := e.Shutdown(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if n := len(real.Calls()); n != 1 {
		t.Fatalf("inactive: real calls %d", n)
	}
	s.SetActive(true)
	if err := e.Shutdown(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if s.ShutdownMessage() != ShutdownMessageEN {
		t.Fatalf("message %q", s.ShutdownMessage())
	}
	if n := len(real.Calls()); n != 1 {
		t.Fatalf("active: real executor was called (%d)", n)
	}
}

func TestWithDemoListsOnlySimulator(t *testing.T) {
	called := false
	base := device.Lister(func(context.Context, device.Config) ([]device.Candidate, error) {
		called = true
		return []device.Candidate{{Transport: device.TransportSerial, Port: "COM3"}}, nil
	})
	s := New()
	list := device.WithDemo(base, s.Candidates)
	cands, err := list(context.Background(), device.Config{Transport: device.TransportDemo})
	if err != nil || called || len(cands) != 1 || cands[0].Port != "demo" || cands[0].Transport != device.TransportDemo {
		t.Fatalf("demo: %v called=%v %+v", err, called, cands)
	}
	cands, err = list(context.Background(), device.Config{Transport: device.TransportSerial, SerialPort: "COM3"})
	if err != nil || !called || len(cands) != 1 || cands[0].Port != "COM3" {
		t.Fatalf("serial: %v called=%v %+v", err, called, cands)
	}
}
