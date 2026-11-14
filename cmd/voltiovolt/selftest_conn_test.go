// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/event"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
)

func startTestSim(t *testing.T, p sim.Profile) *sim.Model {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Cleanup(server.SetTestPath(server.NewTestPath())) // VLT-IS-011 Q-1: own endpoint
	m := sim.NewConsoleModel(p)
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

func TestSelfTestConnectionMegatecAndOnBattery(t *testing.T) {
	m := startTestSim(t, sim.ProfileQ1)
	dir := t.TempDir()
	selfTestAfterVoltage = func() { m.SetMains(false) }
	t.Cleanup(func() { selfTestAfterVoltage = nil })
	if err := SelfTestConnection(dir, SelfTestOpts{ExpectDialect: "megatec", ExpectEvent: event.OnBattery, Timeout: 45 * time.Second}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, SelfTestConnectionResult))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"ok ", "dialect=megatec", "input=230.0", "event=ON_BATTERY", "shutdown=dry", "origin=test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("result %q, missing %q", got, want)
		}
	}
}

func TestSelfTestConnectionVoltronic(t *testing.T) {
	startTestSim(t, sim.ProfileVoltronic)
	dir := t.TempDir()
	if err := SelfTestConnection(dir, SelfTestOpts{ExpectDialect: "voltronic", Timeout: 45 * time.Second}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, SelfTestConnectionResult))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"ok ", "dialect=voltronic", "input=230.0", "shutdown=dry"} {
		if !strings.Contains(got, want) {
			t.Fatalf("result %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "event=") {
		t.Fatalf("no event expected: %q", got)
	}
}
