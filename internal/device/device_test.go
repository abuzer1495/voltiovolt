// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/redact"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// fakeClock is a virtual clock: Sleep moves the time forward at once.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sleeps = append(c.sleeps, d)
	if d > 0 {
		c.now = c.now.Add(d)
	}
	return nil
}

// testOpts: short real timing for the simulator.
var testOpts = transport.Options{Timeout: 30 * time.Millisecond, MinInterval: time.Nanosecond, FlushTime: time.Millisecond}

func simCandidate(t *testing.T, s *sim.Sim, kind string) Candidate {
	return Candidate{Transport: kind, Port: "sim0", New: func() transport.Transport {
		if kind == TransportHID {
			tr, err := s.HIDTransport(testOpts)
			if err != nil {
				t.Fatal(err)
			}
			return tr
		}
		return s.StreamTransport(testOpts)
	}}
}

func listOf(c ...Candidate) Lister {
	return func(context.Context, Config) ([]Candidate, error) { return c, nil }
}

// checkReadOnly is the recorder test of step 6: each received command is a
// READ command of the registry.
func checkReadOnly(t *testing.T, s *sim.Sim) {
	t.Helper()
	got := s.Received()
	if len(got) == 0 {
		t.Fatal("no command received")
	}
	for _, c := range got {
		e, ok := dialect.LookupRead(c)
		if !ok || e.Class != dialect.ClassRead {
			t.Errorf("command %q is not a READ command of the registry", c)
		}
	}
}

func TestProbeListIsVLTIS002(t *testing.T) {
	if !reflect.DeepEqual(ProbeList, sim.ProbeList) {
		t.Fatalf("%v != %v", ProbeList, sim.ProbeList)
	}
}

func TestSelectDialect(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want dialect.Dialect
		ok   bool
	}{
		{[]string{"Q1", "QS", "QGS"}, dialect.DialectVoltronic, true},
		{[]string{"QGS"}, dialect.DialectVoltronic, true},
		{[]string{"Q1", "QS"}, dialect.DialectVoltronicQS, true},
		{[]string{"Q1", "F", "I"}, dialect.DialectMegatec, true},
		{[]string{"F", "I"}, "", false},
		{nil, "", false},
	} {
		d, ok := SelectDialect(c.in)
		if d != c.want || ok != c.ok {
			t.Errorf("%v: %q %v", c.in, d, ok)
		}
	}
}

// TestDetectProfiles is step 7.1: detection for each Dialect profile, on
// the byte-stream and the HID transport.
func TestDetectProfiles(t *testing.T) {
	cases := []struct {
		p         sim.Profile
		dialect   dialect.Dialect
		supported []string
		nominal   string
		qid       bool
	}{
		{sim.ProfileQ1, dialect.DialectMegatec, []string{"Q1", "F", "I"}, "F", false},
		{sim.ProfileQS, dialect.DialectVoltronicQS, []string{"Q1", "F", "I", "QS"}, "F", false},
		{sim.ProfileVoltronic, dialect.DialectVoltronic, []string{"Q1", "F", "QS", "QGS", "QMOD", "QBV", "QWS", "QFLAG", "QRI", "QID", "QVFW", "QMD"}, "QRI", true},
	}
	for _, kind := range []string{TransportSerial, TransportHID} {
		for _, c := range cases {
			t.Run(kind+"/"+string(c.p), func(t *testing.T) {
				s := sim.New(sim.NewModel(c.p))
				clk := newFakeClock()
				p, tr, err := Detect(context.Background(), DefaultConfig(), listOf(simCandidate(t, s, kind)), clk, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer tr.Close()
				if p.Dialect != c.dialect || !reflect.DeepEqual(p.Supported, c.supported) || p.Transport != kind {
					t.Fatalf("profile %+v", p)
				}
				if p.Nominal == nil || p.Nominal.Command != c.nominal {
					t.Fatalf("nominal %+v", p.Nominal)
				}
				if (p.SerialNumber == redact.Redacted) != c.qid {
					t.Fatalf("serial number %q", p.SerialNumber)
				}
				if c.p == sim.ProfileVoltronic && (p.ModelData == nil || p.Firmware == nil || p.Info != nil) {
					t.Fatalf("model data %+v %+v %+v", p.ModelData, p.Firmware, p.Info)
				}
				if c.p != sim.ProfileVoltronic && p.Info == nil {
					t.Fatal("no I data")
				}
				// The Probe List in order, each command one time.
				if got := s.Received(); !reflect.DeepEqual(got, ProbeList) {
					t.Fatalf("sent %v", got)
				}
				// The minimum gap between requests (virtual time).
				for i, d := range clk.sleeps {
					if d != MinGap {
						t.Fatalf("gap %d: %v", i, d)
					}
				}
				if len(clk.sleeps) != len(ProbeList)-1 {
					t.Fatalf("%d gaps", len(clk.sleeps))
				}
				// QID is redacted in the profile JSON (IPC result).
				b, _ := json.Marshal(p)
				if c.qid && !strings.Contains(string(b), `"serial_number":"[REDACTED]"`) {
					t.Fatalf("json %s", b)
				}
				checkReadOnly(t, s)
			})
		}
	}
}

func TestDetectNextCandidate(t *testing.T) {
	dead := sim.NewModel(sim.ProfileVoltronic)
	dead.SetFault(sim.FaultNoResponse, true)
	ds := sim.New(dead)
	echo := sim.New(sim.NewModel(sim.ProfileQ1))
	echo.Model.SetFault(sim.FaultEcho, true)
	good := sim.New(sim.NewModel(sim.ProfileQS))
	p, tr, err := Detect(context.Background(), DefaultConfig(),
		listOf(simCandidate(t, ds, TransportHID), simCandidate(t, echo, TransportSerial), simCandidate(t, good, TransportSerial)), newFakeClock(), nil)
	if err != nil {
		t.Fatal(err)
	}
	tr.Close()
	if p.Dialect != dialect.DialectVoltronicQS {
		t.Fatalf("%+v", p)
	}
	for _, s := range []*sim.Sim{ds, echo, good} {
		checkReadOnly(t, s)
	}
	// No device at all.
	ds2 := sim.New(dead)
	if _, _, err := Detect(context.Background(), DefaultConfig(), listOf(simCandidate(t, ds2, TransportHID)), newFakeClock(), nil); !errors.Is(err, ErrNoDevice) {
		t.Fatalf("err %v", err)
	}
}

func TestConfigFromStore(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "vlt.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c, err := ConfigFromStore(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Transport: TransportNotConfigured, DTR: transport.LineUnchanged, RTS: transport.LineUnchanged, AllowedPorts: []string{}}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("%+v", c)
	}
	for _, bad := range []struct{ k, v string }{
		{store.SettingDeviceTransport, `"usb"`},
		{store.SettingDeviceSerial, `"../etc/passwd"`},
		{store.SettingDeviceSerial, `"/dev/tty USB0"`},
		{store.SettingDeviceDTR, `"yes"`},
		{store.SettingDeviceDTR, `true`}, // the old type
		{store.SettingDeviceRTS, `false`},
		{store.SettingDeviceAllowedPorts, `null`},
		{store.SettingDeviceAllowedPorts, `[""]`},
		{store.SettingDeviceAllowedPorts, `["COM3","COM3"]`},
		{store.SettingDeviceAllowedPorts, `"COM3"`},
		{store.SettingDeviceAllowedPorts, `["COM1","COM2","COM3","COM4","COM5","COM6","COM7","COM8","COM9","COM10","COM11","COM12","COM13","COM14","COM15","COM16","COM17"]`},
	} {
		for _, d := range store.DefaultSettings() {
			if d.Key == bad.k && d.Check(json.RawMessage(bad.v)) == nil {
				t.Errorf("%s=%s accepted", bad.k, bad.v)
			}
		}
	}
	for _, good := range []struct{ k, v string }{
		{store.SettingDeviceTransport, `"serial"`},
		{store.SettingDeviceSerial, `"/dev/ttyUSB0"`},
		{store.SettingDeviceSerial, `"COM3"`},
		{store.SettingDeviceRTS, `"on"`},
		{store.SettingDeviceDTR, `"off"`},
		{store.SettingDeviceDTR, `"unchanged"`},
		{store.SettingDeviceTransport, `"not_configured"`},
		{store.SettingDeviceAllowedPorts, `[]`},
		{store.SettingDeviceAllowedPorts, `["COM3","/dev/ttyUSB0"]`},
	} {
		for _, d := range store.DefaultSettings() {
			if d.Key == good.k {
				if err := d.Check(json.RawMessage(good.v)); err != nil {
					t.Errorf("%s=%s: %v", good.k, good.v, err)
				}
			}
		}
	}
}

func TestBackoff(t *testing.T) {
	want := []time.Duration{1, 2, 5, 10, 30, 30, 30}
	for i, w := range want {
		if got := BackoffAt(i); got != w*time.Second {
			t.Errorf("attempt %d: %v", i, got)
		}
	}
}

// TestManager: no device, then a device; Restart starts a new detection
// with the current Config.
func TestManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clk := newFakeClock()
	var mu sync.Mutex
	present := false
	s := sim.New(sim.NewModel(sim.ProfileQ1))
	polls := make(chan Profile, 4)
	m := &Manager{
		Config: func(context.Context) (Config, error) {
			c := DefaultConfig()
			c.Transport = TransportSerial
			c.SerialPort = "sim0"
			return c, nil
		},
		List: func(ctx context.Context, c Config) ([]Candidate, error) {
			mu.Lock()
			defer mu.Unlock()
			if !present {
				present = true // the next detection finds the device
				return nil, nil
			}
			return []Candidate{simCandidate(t, s, TransportSerial)}, nil
		},
		Poll: func(ctx context.Context, tr transport.Transport, p Profile, set func(string)) error {
			polls <- p
			<-ctx.Done()
			return ctx.Err()
		},
		Clock: clk,
	}
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()
	p := <-polls
	if p.Dialect != dialect.DialectMegatec || m.Status().State != StateConnected || m.Status().Profile == nil {
		t.Fatalf("%+v %+v", p, m.Status())
	}
	if len(clk.sleeps) == 0 || clk.sleeps[0] != time.Second {
		t.Fatalf("no-device wait %v", clk.sleeps)
	}
	m.Restart()
	select {
	case p = <-polls:
	case <-time.After(5 * time.Second):
		t.Fatal("no new detection after Restart")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if m.Status().State != StateStopped {
		t.Fatalf("%+v", m.Status())
	}
	checkReadOnly(t, s)
}
