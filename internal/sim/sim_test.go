// SPDX-License-Identifier: AGPL-3.0-or-later

package sim

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

func fastOpts() transport.Options {
	return transport.Options{Timeout: 100 * time.Millisecond, MinInterval: time.Millisecond, FlushTime: 5 * time.Millisecond}
}

func exchange(t *testing.T, tr transport.Transport, cmd string) (transport.Response, error) {
	t.Helper()
	req, err := dialect.EncodeRead(cmd)
	if err != nil {
		t.Fatal(err)
	}
	return tr.Exchange(context.Background(), req)
}

func TestProfilesParse(t *testing.T) {
	for _, p := range []Profile{ProfileQ1, ProfileQS, ProfileVoltronic} {
		s := New(NewModel(p))
		tr := s.StreamTransport(fastOpts())
		if err := tr.Open(context.Background()); err != nil {
			t.Fatal(err)
		}
		for _, cmd := range ProbeList {
			r, err := exchange(t, tr, cmd)
			if err != nil {
				t.Fatalf("%s %s: %v", p, cmd, err)
			}
			_, perr := dialect.Parse(cmd, r.Raw)
			if s.Model.supports(cmd) && perr != nil {
				t.Errorf("%s %s %q: %v", p, cmd, r.Raw, perr)
			}
			if !s.Model.supports(cmd) && !errors.Is(perr, dialect.ErrNotSupported) {
				t.Errorf("%s %s %q: %v; want not supported", p, cmd, r.Raw, perr)
			}
		}
		if bad := s.NotIn(ProbeList); len(bad) != 0 {
			t.Errorf("not in Probe List: %q", bad)
		}
	}
}

func TestMainsOff(t *testing.T) {
	m := NewModel(ProfileVoltronic)
	m.SetMains(false)
	m.Charge = 10
	st, err := dialect.ParseQ1(m.Respond("Q1"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Flags.UtilityFail || !st.Flags.BatteryLow || st.InputVoltage.Value != 0 {
		t.Errorf("%+v", st)
	}
	mode, _ := dialect.ParseQMOD(m.Respond("QMOD"))
	if mode.Code != 'B' {
		t.Errorf("mode %q", mode.Code)
	}
}

func TestRecorderDetectsViolation(t *testing.T) {
	s := New(NewModel(ProfileVoltronic))
	// A tool that sends a command that is not in the Probe List MUST fail
	// the check. The simulator records all bytes, so it can see this.
	s.Feed([]byte("Q1\rQ\rS01R0001\r"))
	bad := s.NotIn(ProbeList)
	if len(bad) != 2 || bad[0] != "Q" || bad[1] != "S01R0001" {
		t.Fatalf("violations %q", bad)
	}
}

func TestFaultInjection(t *testing.T) {
	cases := []struct {
		fault Fault
		check func(t *testing.T, r transport.Response, err error)
	}{
		{FaultNAK, func(t *testing.T, r transport.Response, err error) {
			if _, perr := dialect.ParseQ1(r.Raw); !errors.Is(perr, dialect.ErrNotSupported) {
				t.Errorf("NAK: %v", perr)
			}
		}},
		{FaultEcho, func(t *testing.T, r transport.Response, err error) {
			if _, perr := dialect.ParseQ1(r.Raw); !errors.Is(perr, dialect.ErrNotSupported) {
				t.Errorf("echo: %q %v", r.Raw, perr)
			}
		}},
		{FaultNoResponse, func(t *testing.T, r transport.Response, err error) {
			if !errors.Is(err, transport.ErrTimeout) {
				t.Errorf("no response: %v", err)
			}
		}},
		{FaultWrongLength, func(t *testing.T, r transport.Response, err error) {
			var pe *dialect.ParseError
			if _, perr := dialect.ParseQ1(r.Raw); !errors.As(perr, &pe) {
				t.Errorf("wrong length: %v", perr)
			}
		}},
		{FaultSplit, func(t *testing.T, r transport.Response, err error) {
			if _, perr := dialect.ParseQ1(r.Raw); perr != nil {
				t.Errorf("split: %q %v", r.Raw, perr)
			}
		}},
		{FaultStale, func(t *testing.T, r transport.Response, err error) {
			if _, perr := dialect.ParseQ1(r.Raw); perr != nil {
				t.Errorf("stale: %q %v", r.Raw, perr)
			}
		}},
	}
	for _, c := range cases {
		s := New(NewModel(ProfileVoltronic))
		s.Model.SetFault(c.fault, true)
		tr := s.StreamTransport(fastOpts())
		_ = tr.Open(context.Background())
		// Two exchanges: the second one shows that old bytes are removed.
		for i := 0; i < 2; i++ {
			r, err := exchange(t, tr, "Q1")
			c.check(t, r, err)
		}
	}
}

func TestStaleBeforeRequest(t *testing.T) {
	s := New(NewModel(ProfileQ1))
	tr := s.StreamTransport(fastOpts())
	_ = tr.Open(context.Background())
	s.InjectStale([]byte("(OLD 1 2 3\r"))
	r, err := exchange(t, tr, "Q1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dialect.ParseQ1(r.Raw); err != nil {
		t.Errorf("%q %v", r.Raw, err)
	}
}

func TestHIDReports(t *testing.T) {
	s := New(NewModel(ProfileVoltronic))
	tr, err := s.HIDTransport(fastOpts())
	if err != nil {
		t.Fatal(err)
	}
	_ = tr.Open(context.Background())
	r, err := exchange(t, tr, "QGS")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dialect.ParseQGS(r.Raw); err != nil {
		t.Errorf("%q %v", r.Raw, err)
	}
	if got := s.Received(); len(got) != 1 || got[0] != "QGS" {
		t.Errorf("received %q", got)
	}

	s.Model.SetFault(FaultBrokenRpt, true)
	r, err = exchange(t, tr, "Q1")
	if err == nil {
		if _, perr := dialect.ParseQ1(r.Raw); perr == nil {
			t.Errorf("broken report not detected: %q", r.Raw)
		}
	}
}
