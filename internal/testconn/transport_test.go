// SPDX-License-Identifier: AGPL-3.0-or-later

package testconn

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

func uniqueRuntime(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Cleanup(server.SetTestPath(server.NewTestPath())) // VLT-IS-011 Q-1: own endpoint
}

func opts() transport.Options {
	return transport.Options{Timeout: device.ResponseTimeout, MinInterval: device.MinGap}
}

func startSim(t *testing.T) *sim.Model {
	t.Helper()
	uniqueRuntime(t)
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

func TestOpenFailsWhenSimulatorIsDown(t *testing.T) {
	uniqueRuntime(t)
	tr := New(opts())
	start := time.Now()
	err := tr.Open(context.Background())
	if !errors.Is(err, device.ErrSimulatorNotFound) {
		t.Fatalf("Open: %v, want ErrSimulatorNotFound", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("Open took %s, want a short timeout", time.Since(start))
	}
}

func TestExchangeQ1AndControl(t *testing.T) {
	startSim(t)
	tr := New(opts())
	if err := tr.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	resp, err := tr.Exchange(context.Background(), []byte("Q1\r"))
	if err != nil {
		t.Fatal(err)
	}
	v, err := dialect.Parse("Q1", resp.Raw)
	if err != nil {
		t.Fatalf("parse %q: %v", resp.Raw, err)
	}
	st, ok := v.(dialect.Status)
	if !ok || !st.InputVoltage.Valid || st.InputVoltage.Value < 229 || st.InputVoltage.Value > 231 {
		t.Fatalf("input voltage %+v, want ~230", v)
	}
	if _, ok := tr.(*transport.Stream); !ok {
		t.Fatalf("Transport is %T, want *transport.Stream (real Exchange path)", tr)
	}
}

func TestCandidatesAreTestOnly(t *testing.T) {
	uniqueRuntime(t)
	cands := Candidates(opts())
	if len(cands) != 1 || cands[0].Transport != device.TransportTest {
		t.Fatalf("%+v", cands)
	}
	if cands[0].Port != server.Path() {
		t.Fatalf("port %q want %q", cands[0].Port, server.Path())
	}
}

func TestNoTCP(t *testing.T) {
	uniqueRuntime(t)
	p := server.Path()
	if len(p) >= 4 && (p[:4] == "tcp:" || p[:6] == "tcp://" || p[0] == ':' || (len(p) > 0 && p[0] >= '0' && p[0] <= '9')) {
		t.Fatalf("Path looks like TCP: %q", p)
	}
}
