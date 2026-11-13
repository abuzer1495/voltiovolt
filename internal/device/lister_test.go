// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"
	"errors"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/transport"
)

func TestWithTestDoesNotListRealPorts(t *testing.T) {
	called := 0
	base := Lister(func(context.Context, Config) ([]Candidate, error) {
		called++
		t.Error("base lister must not run")
		return []Candidate{{Transport: TransportSerial, Port: "COM1"}}, nil
	})
	got, err := WithTest(base, func() []Candidate {
		return []Candidate{{Transport: TransportTest, Port: "sim"}}
	})(context.Background(), Config{Transport: TransportTest})
	if err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Fatalf("base called %d", called)
	}
	if len(got) != 1 || got[0].Transport != TransportTest || got[0].Port != "sim" {
		t.Fatalf("%+v", got)
	}
}

func TestWithTestFallsThrough(t *testing.T) {
	base := Lister(func(context.Context, Config) ([]Candidate, error) {
		return []Candidate{{Transport: TransportSerial, Port: "COM1"}}, nil
	})
	got, err := WithTest(base, func() []Candidate {
		t.Error("test candidates must not run")
		return nil
	})(context.Background(), Config{Transport: TransportSerial, SerialPort: "COM1"})
	if err != nil || len(got) != 1 || got[0].Port != "COM1" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestDetectSimulatorNotFound(t *testing.T) {
	list := Lister(func(context.Context, Config) ([]Candidate, error) {
		return []Candidate{{
			Transport: TransportTest,
			Port:      "missing",
			New: func() transport.Transport {
				return failOpen{err: ErrSimulatorNotFound}
			},
		}}, nil
	})
	_, _, err := Detect(context.Background(), Config{Transport: TransportTest}, list, newFakeClock(), nil)
	if !errors.Is(err, ErrSimulatorNotFound) {
		t.Fatalf("Detect: %v, want ErrSimulatorNotFound", err)
	}
}

func TestOriginOf(t *testing.T) {
	if OriginOf(TransportTest) != "test" || OriginOf(TransportDemo) != "demo" || OriginOf(TransportHID) != "real" {
		t.Fatal(OriginOf(TransportTest), OriginOf(TransportDemo), OriginOf(TransportHID))
	}
}

type failOpen struct{ err error }

func (f failOpen) Open(context.Context) error { return f.err }
func (f failOpen) Close() error               { return nil }
func (f failOpen) Exchange(context.Context, []byte) (transport.Response, error) {
	return transport.Response{}, f.err
}
