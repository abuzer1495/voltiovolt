// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// fakeOS records each Transport that the lister makes. It opens nothing.
type fakeOS struct {
	mu      sync.Mutex
	hidList int
	serial  []string
	hid     []string
}

func (f *fakeOS) funcs() OSFuncs {
	return OSFuncs{
		ListHID: func() ([]transport.HIDDevice, error) {
			f.mu.Lock()
			f.hidList++
			f.mu.Unlock()
			return []transport.HIDDevice{
				{Path: "/dev/hidraw0", VID: transport.VendorID, PID: transport.ProductID},
				{Path: "/dev/hidraw1", VID: 0x046d, PID: 0xc077}, // another device
			}, nil
		},
		NewHID: func(path string, _ transport.Options) transport.Transport {
			f.mu.Lock()
			f.hid = append(f.hid, path)
			f.mu.Unlock()
			return nil
		},
		NewSerial: func(name string, _ transport.SerialOptions, _ transport.Options) transport.Transport {
			f.mu.Lock()
			f.serial = append(f.serial, name)
			f.mu.Unlock()
			return nil
		},
	}
}

func ports(cs []Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Transport+":"+c.Port)
	}
	return out
}

// TestSEC099Lister: the candidates of each Transport value. No candidate
// is a serial port that the user did not select or allow (SEC-099,
// OD-27). The lister has no function that lists the OS serial ports.
func TestSEC099Lister(t *testing.T) {
	cases := []struct {
		name string
		cfg  func(*Config)
		want string
	}{
		{"default: not configured", func(*Config) {}, "[]"},
		{"serial without a port", func(c *Config) { c.Transport = TransportSerial }, "[]"},
		{"serial COM3", func(c *Config) { c.Transport = TransportSerial; c.SerialPort = "COM3" }, "[serial:COM3]"},
		{"serial ignores allowed ports", func(c *Config) {
			c.Transport = TransportSerial
			c.SerialPort = "COM3"
			c.AllowedPorts = []string{"COM4"}
		}, "[serial:COM3]"},
		{"auto, empty allowed list", func(c *Config) { c.Transport = TransportAuto; c.SerialPort = "COM9" }, "[hid:/dev/hidraw0]"},
		{"auto, allowed list", func(c *Config) {
			c.Transport = TransportAuto
			c.AllowedPorts = []string{"COM3", "/dev/ttyUSB0"}
		}, "[hid:/dev/hidraw0 serial:COM3 serial:/dev/ttyUSB0]"},
		{"hid", func(c *Config) { c.Transport = TransportHID; c.AllowedPorts = []string{"COM3"} }, "[hid:/dev/hidraw0]"},
	}
	for _, x := range cases {
		f := &fakeOS{}
		c := DefaultConfig()
		x.cfg(&c)
		cs, err := ListerFor(f.funcs(), transport.Options{})(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(ports(cs)); got != x.want && !(x.want == "[]" && len(cs) == 0) {
			t.Errorf("%s: %s, want %s", x.name, got, x.want)
		}
		if len(f.serial) != 0 || len(f.hid) != 0 {
			t.Errorf("%s: the lister made a Transport before Detect", x.name)
		}
		if (c.Transport == TransportNotConfigured || c.Transport == TransportSerial) && f.hidList != 0 {
			t.Errorf("%s: HID list read", x.name)
		}
	}
}

// TestSEC099NotConfigured: with the default Settings the Manager does not
// call the lister and opens no Transport. After the user selects a port
// (Restart), only that port gets bytes.
func TestSEC099NotConfigured(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	cfg := DefaultConfig()
	listed := 0
	s := sim.New(sim.NewModel(sim.ProfileQ1))
	polls := make(chan Profile, 2)
	m := &Manager{
		Config: func(context.Context) (Config, error) { mu.Lock(); defer mu.Unlock(); return cfg, nil },
		List: func(ctx context.Context, c Config) ([]Candidate, error) {
			mu.Lock()
			listed++
			mu.Unlock()
			if c.Transport != TransportSerial || c.SerialPort != "sim0" {
				t.Errorf("lister called with %+v", c)
				return nil, nil
			}
			return []Candidate{simCandidate(t, s, TransportSerial)}, nil
		},
		Poll: func(ctx context.Context, tr transport.Transport, p Profile, set func(string)) error {
			polls <- p
			<-ctx.Done()
			return ctx.Err()
		},
		Clock: newFakeClock(),
	}
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for m.Status().State != StateNotConfigured {
		if time.Now().After(deadline) {
			t.Fatalf("state %q", m.Status().State)
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if listed != 0 {
		t.Fatalf("lister called %d times while not configured", listed)
	}
	cfg.Transport, cfg.SerialPort = TransportSerial, "sim0"
	mu.Unlock()
	if n := len(s.Received()); n != 0 {
		t.Fatalf("%d commands before the selection", n)
	}
	m.Restart()
	select {
	case <-polls:
	case <-time.After(5 * time.Second):
		t.Fatal("no detection after the selection")
	}
	cancel()
	<-done
	checkReadOnly(t, s)
}

// busyTransport fails to open like a port that another program uses.
type busyTransport struct{ opened int }

func (b *busyTransport) Open(context.Context) error {
	b.opened++
	return fmt.Errorf("%w: COM3: Serial port busy", transport.ErrPortBusy)
}
func (b *busyTransport) Close() error { return nil }
func (b *busyTransport) Exchange(context.Context, []byte) (transport.Response, error) {
	panic("Exchange on a port that did not open")
}

// TestPortBusy: an open error "port busy" gives the device state
// "port busy" (VLT-IS-006 TASK 03-00-00-004 step 2.1).
func TestPortBusy(t *testing.T) {
	b := &busyTransport{}
	cand := Candidate{Transport: TransportSerial, Port: "COM3", New: func() transport.Transport { return b }}
	if _, _, err := Detect(context.Background(), Config{Transport: TransportSerial, SerialPort: "COM3"}, listOf(cand), newFakeClock(), nil); !errors.Is(err, ErrPortBusy) {
		t.Fatalf("Detect: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &Manager{
		Config: func(context.Context) (Config, error) {
			return Config{Transport: TransportSerial, SerialPort: "COM3"}, nil
		},
		List:  listOf(cand),
		Poll:  func(context.Context, transport.Transport, Profile, func(string)) error { t.Error("poll"); return nil },
		Clock: newFakeClock(),
	}
	go func() { _ = m.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for m.Status().State != StatePortBusy {
		if time.Now().After(deadline) {
			t.Fatalf("state %q", m.Status().State)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
