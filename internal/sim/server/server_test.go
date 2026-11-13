// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/sim"
)

func startPipe(t *testing.T, s *Server) (net.Conn, context.CancelFunc) {
	t.Helper()
	a, b := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.serveOne(ctx, b)
		_ = b.Close()
	}()
	t.Cleanup(func() { cancel(); _ = a.Close(); _ = b.Close(); wg.Wait() })
	return a, cancel
}

func exchange(t *testing.T, c net.Conn, cmd string) []byte {
	t.Helper()
	raw, err := Exchange(context.Background(), c, cmd, 2*time.Second)
	if err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
	return raw
}

// writeCmd sends a CONTROL command. The UPS gives no response (VLT-REF-001 section 5).
func writeCmd(t *testing.T, c net.Conn, cmd string) {
	t.Helper()
	if _, err := c.Write([]byte(cmd + "\r")); err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
}

func TestResponseFormatsParse(t *testing.T) {
	// Each profile: supported commands parse with internal/dialect.
	// Q1, F and I have the exact lengths of VLT-REF-001 (47, 22, 39 with CR).
	for _, p := range []sim.Profile{sim.ProfileQ1, sim.ProfileQS, sim.ProfileVoltronic} {
		s := New(sim.NewModel(p))
		c, _ := startPipe(t, s)
		for _, cmd := range sim.ProbeList {
			if !s.Model.Supports(cmd) {
				continue
			}
			raw := exchange(t, c, cmd)
			if _, err := dialect.Parse(cmd, raw); err != nil {
				t.Errorf("%s %s %q: %v", p, cmd, raw, err)
			}
			switch cmd {
			case "Q1":
				if len(raw) != 47 {
					t.Errorf("%s Q1 length %d, want 47: %q", p, len(raw), raw)
				}
			case "F":
				if len(raw) != 22 {
					t.Errorf("%s F length %d, want 22: %q", p, len(raw), raw)
				}
			case "I":
				if len(raw) != 39 {
					t.Errorf("%s I length %d, want 39: %q", p, len(raw), raw)
				}
			}
		}
	}
}

func TestControlEffects(t *testing.T) {
	m := sim.NewModel(sim.ProfileQ1)
	s := New(m)
	c, _ := startPipe(t, s)

	writeCmd(t, c, "T")
	st, err := dialect.ParseQ1(exchange(t, c, "Q1"))
	if err != nil || !st.Flags.TestInProgress {
		t.Fatalf("T: %+v %v", st, err)
	}
	writeCmd(t, c, "CT")
	st, err = dialect.ParseQ1(exchange(t, c, "Q1"))
	if err != nil || st.Flags.TestInProgress {
		t.Fatalf("CT: %+v %v", st, err)
	}

	on := m.BeeperOn()
	writeCmd(t, c, "Q")
	st, err = dialect.ParseQ1(exchange(t, c, "Q1"))
	if err != nil || st.Flags.BeeperOn == on {
		t.Fatalf("Q status: %+v on=%v %v", st, on, err)
	}

	writeCmd(t, c, "S.2R0001")
	st, err = dialect.ParseQ1(exchange(t, c, "Q1"))
	if err != nil || !st.Flags.ShutdownActive {
		t.Fatalf("S.2R0001: %+v %v", st, err)
	}
	writeCmd(t, c, "C")
	st, err = dialect.ParseQ1(exchange(t, c, "Q1"))
	if err != nil || st.Flags.ShutdownActive {
		t.Fatalf("C: %+v %v", st, err)
	}
	if got := m.Controls(); len(got) < 4 || got[0] != "T" || got[1] != "CT" || got[2] != "Q" || got[3] != "S.2R0001" {
		t.Fatalf("controls %v", got)
	}
}

func TestFaultInjection(t *testing.T) {
	cases := []struct {
		fault Fault
		check func(*testing.T, []byte, error)
	}{
		{FaultNoResponse, func(t *testing.T, raw []byte, err error) {
			if !errors.Is(err, os.ErrDeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) && err == nil {
				t.Fatalf("no response: raw=%q err=%v", raw, err)
			}
		}},
		{FaultGarbage, func(t *testing.T, raw []byte, err error) {
			if err != nil {
				t.Fatal(err)
			}
			if _, perr := dialect.ParseQ1(raw); perr == nil {
				t.Fatalf("garbage parsed: %q", raw)
			}
		}},
		{FaultSplit, func(t *testing.T, raw []byte, err error) {
			if err != nil {
				t.Fatal(err)
			}
			if _, perr := dialect.ParseQ1(raw); perr != nil {
				t.Fatalf("split: %q %v", raw, perr)
			}
		}},
		{FaultWrongLength, func(t *testing.T, raw []byte, err error) {
			if err != nil {
				t.Fatal(err)
			}
			var pe *dialect.ParseError
			if _, perr := dialect.ParseQ1(raw); !errors.As(perr, &pe) {
				t.Fatalf("wrong length: %q %v", raw, perr)
			}
		}},
		{FaultOldBytes, func(t *testing.T, raw []byte, err error) {
			// The first CR ends the Exchange, so we see the old bytes.
			if err != nil {
				t.Fatal(err)
			}
			if _, perr := dialect.ParseQ1(raw); perr == nil {
				t.Fatalf("old bytes parsed as Q1: %q", raw)
			}
		}},
	}
	for _, c := range cases {
		t.Run(string(c.fault), func(t *testing.T) {
			s := New(sim.NewModel(sim.ProfileQ1))
			s.SetFault(c.fault)
			conn, _ := startPipe(t, s)
			wait := 2 * time.Second
			if c.fault == FaultSplit {
				wait = 3 * time.Second
			}
			if c.fault == FaultNoResponse {
				wait = 150 * time.Millisecond
			}
			raw, err := Exchange(context.Background(), conn, "Q1", wait)
			c.check(t, raw, err)
		})
	}
}

func TestOtherReply(t *testing.T) {
	s := New(sim.NewModel(sim.ProfileQ1))
	c, _ := startPipe(t, s)
	s.SetOther(OtherEcho)
	raw := exchange(t, c, "ZZ")
	if string(raw) != "ZZ\r" {
		t.Fatalf("echo %q", raw)
	}
	s.SetOther(OtherNAK)
	raw = exchange(t, c, "ZZ")
	if string(raw) != "(NAK\r" {
		t.Fatalf("nak %q", raw)
	}
	s.SetOther(OtherNone)
	_, err := Exchange(context.Background(), c, "ZZ", 150*time.Millisecond)
	if err == nil {
		t.Fatal("none: got a response")
	}
}

func TestLogKeepsLast200(t *testing.T) {
	s := New(sim.NewModel(sim.ProfileQ1))
	c, _ := startPipe(t, s)
	for i := 0; i < 205; i++ {
		exchange(t, c, "Q1")
	}
	log := s.Log()
	if len(log) != LogLimit {
		t.Fatalf("log %d, want %d", len(log), LogLimit)
	}
	if log[0].Request != "Q1" || log[0].Response == "" || log[0].Time.IsZero() {
		t.Fatalf("entry %+v", log[0])
	}
}

func TestSecondClientGetsNoConnection(t *testing.T) {
	path := testPath(t)
	ln, err := ListenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(sim.NewModel(sim.ProfileQ1))
	var statuses []string
	s.OnStatus = func(m string) { statuses = append(statuses, m) }
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	t.Cleanup(func() { cancel(); _ = ln.Close(); <-done })

	c1, err := DialPath(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()
	// Wait until the server marks the client.
	deadline := time.Now().Add(2 * time.Second)
	for !s.ClientConnected() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !s.ClientConnected() {
		t.Fatal("first client not accepted")
	}
	raw, err := Exchange(context.Background(), c1, "Q1", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dialect.ParseQ1(raw); err != nil {
		t.Fatal(err)
	}

	c2, err := DialPath(ctx, path)
	if err != nil {
		// A refused connect is "no connection".
		return
	}
	defer c2.Close()
	_, err = Exchange(context.Background(), c2, "Q1", 300*time.Millisecond)
	if err == nil {
		t.Fatal("second client got a response")
	}

	// The first client still works.
	if _, err := Exchange(context.Background(), c1, "F", 2*time.Second); err != nil {
		t.Fatalf("first client lost: %v", err)
	}
	if len(statuses) == 0 || statuses[0] != MsgConnected {
		t.Fatalf("status %v", statuses)
	}
}

func TestNoTCP(t *testing.T) {
	p := Path()
	if len(p) >= 4 && (p[:4] == "tcp:" || p[:5] == "tcp4:" || p[:6] == "http:") {
		t.Fatalf("Path looks like TCP: %s", p)
	}
	t.Logf("endpoint %s", p)
}

func testPath(t *testing.T) string {
	t.Helper()
	if os.PathSeparator == '\\' {
		return fmt.Sprintf(`\\.\pipe\voltiovolt-simulator-test-%d-%d`, os.Getpid(), time.Now().UnixNano()%1e9)
	}
	return fmt.Sprintf("%s/voltiovolt-simulator-test.sock", t.TempDir())
}
