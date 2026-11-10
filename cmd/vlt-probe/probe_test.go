// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// fakeClock makes the 500 ms waits and the 2 s cycles instant.
type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time { return f.t }
func (f *fakeClock) sleep(ctx context.Context, d time.Duration) error {
	f.t = f.t.Add(d)
	return ctx.Err()
}

func testRunner(t *testing.T, s *sim.Sim, hid bool) (*runner, transport.Transport) {
	t.Helper()
	opts := transport.Options{Timeout: 100 * time.Millisecond, MinInterval: time.Microsecond, FlushTime: time.Millisecond}
	var tr transport.Transport = s.StreamTransport(opts)
	if hid {
		h, err := s.HIDTransport(opts)
		if err != nil {
			t.Fatal(err)
		}
		tr = h
	}
	if err := tr.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	clk := &fakeClock{t: time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)}
	return &runner{t: tr, now: clk.now, sleep: clk.sleep}, tr
}

func TestListsMatchInstruction(t *testing.T) {
	if strings.Join(probeList[:], ",") != strings.Join(sim.ProbeList, ",") {
		t.Errorf("Probe List %v", probeList)
	}
	if strings.Join(watchList[:], ",") != strings.Join(sim.WatchList, ",") {
		t.Errorf("Watch List %v", watchList)
	}
}

// TestProbeSendsOnlyProbeList proves that vlt-probe sends only Probe List
// commands, in order, for each simulator profile and both Transports.
func TestProbeSendsOnlyProbeList(t *testing.T) {
	for _, p := range []sim.Profile{sim.ProfileQ1, sim.ProfileQS, sim.ProfileVoltronic} {
		for _, hid := range []bool{false, true} {
			s := sim.New(sim.NewModel(p))
			r, _ := testRunner(t, s, hid)
			entries, err := r.probe(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if bad := s.NotIn(sim.ProbeList); len(bad) != 0 {
				t.Fatalf("%s hid=%v: commands not in the Probe List: %q", p, hid, bad)
			}
			got := s.Received()
			if strings.Join(got, ",") != strings.Join(sim.ProbeList, ",") {
				t.Errorf("%s hid=%v: order %q", p, hid, got)
			}
			if len(entries) != len(sim.ProbeList) {
				t.Errorf("%d entries", len(entries))
			}
		}
	}
}

// TestWatchSendsOnlyWatchList proves that watch mode sends only Watch List
// commands and stops a command after two "not supported" results.
func TestWatchSendsOnlyWatchList(t *testing.T) {
	for _, p := range []sim.Profile{sim.ProfileQ1, sim.ProfileVoltronic} {
		s := sim.New(sim.NewModel(p))
		r, _ := testRunner(t, s, false)
		entries, err := r.watch(context.Background(), 1)
		if err != nil {
			t.Fatal(err)
		}
		if bad := s.NotIn(sim.WatchList); len(bad) != 0 {
			t.Fatalf("%s: commands not in the Watch List: %q", p, bad)
		}
		n := map[string]int{}
		for _, c := range s.Received() {
			n[c]++
		}
		if p == sim.ProfileQ1 {
			for _, c := range []string{"QS", "QGS", "QMOD", "QBV", "QWS"} {
				if n[c] != notSupportedLimit {
					t.Errorf("q1: %s sent %d times; want %d", c, n[c], notSupportedLimit)
				}
			}
		}
		if n["Q1"] < 10 {
			t.Errorf("%s: Q1 sent %d times in 1 minute", p, n["Q1"])
		}
		if len(entries) != len(s.Received()) {
			t.Errorf("entries %d, received %d", len(entries), len(s.Received()))
		}
	}
	if _, err := (&runner{}).watch(context.Background(), 31); err == nil {
		t.Error("31 minutes must fail")
	}
}

func TestCaptureRedaction(t *testing.T) {
	s := sim.New(sim.NewModel(sim.ProfileVoltronic))
	r, tr := testRunner(t, s, false)
	dir := t.TempDir()
	var out bytes.Buffer
	c := config{mode: "probe", transport: "hid"}
	info := func() *transport.HIDDevice {
		return &transport.HIDDevice{Path: "/dev/hidraw9", VID: 0x0665, PID: 0x5161, Product: "FAKE", Serial: "FAKE-SERIAL-123",
			ReportDescriptor: []byte{0x75, 0x08}, Lengths: sim.HIDLengths}
	}
	_ = tr.Close()
	if err := session(context.Background(), c, tr, info, &out, dir, r, nil); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "vlt-capture-*.vltcap.json"))
	if len(files) != 1 {
		t.Fatalf("files %v", files)
	}
	if filepath.Base(files[0]) != "vlt-capture-20261110T090000Z.vltcap.json" {
		t.Errorf("name %s", files[0])
	}
	raw, _ := os.ReadFile(files[0])
	if bytes.Contains(raw, []byte("FAKE-SERIAL-123")) {
		t.Error("USB serial number not redacted")
	}
	if bytes.Contains(raw, []byte("SIMQID")) || bytes.Contains(raw, []byte(hex.EncodeToString([]byte("SIMQID")))) {
		t.Error("QID data not redacted")
	}
	var capt Capture
	if err := json.Unmarshal(raw, &capt); err != nil {
		t.Fatal(err)
	}
	if capt.Device == nil || capt.Device.Serial != "REDACTED (15 bytes)" || capt.Device.ReportDescriptor != "7508" {
		t.Errorf("device %+v", capt.Device)
	}
	for _, e := range capt.Entries {
		if e.Command == "QID" && e.ResponseHex != "REDACTED (16 bytes)" {
			t.Errorf("QID entry %+v", e)
		}
		if e.Command == "Q1" && (e.Result != resultData || e.RequestHex != "51310d") {
			t.Errorf("Q1 entry %+v", e)
		}
	}
	var keys map[string]any
	_ = json.Unmarshal(raw, &keys)
	for k := range keys {
		if strings.Contains(strings.ToLower(k), "host") || strings.Contains(strings.ToLower(k), "user") {
			t.Errorf("key %q in Capture", k)
		}
	}
	if !strings.Contains(out.String(), "Summary:") {
		t.Errorf("no summary: %s", out.String())
	}
}

func TestParseArgs(t *testing.T) {
	var sink bytes.Buffer
	good := [][]string{
		{"version"},
		{"scan"},
		{"probe"},
		{"probe", "--transport", "serial", "--port", "/dev/ttyS0", "--dtr=off", "--rts=on"},
		{"probe", "--transport", "serial", "--port", "COM3", "--dtr", "unchanged"},
		{"watch", "--minutes", "30"},
	}
	for _, a := range good {
		if _, err := parseArgs(a, &sink); err != nil {
			t.Errorf("%q: %v", a, err)
		}
	}
	bad := [][]string{
		{},
		{"send"},
		{"probe", "--command", "Q"},
		{"probe", "--raw", "S01"},
		{"probe", "Q"},
		{"probe", "--transport", "serial", "--port", "COM3", "--dtr=true"},
		{"probe", "--transport", "serial", "--port", "COM3", "--rts", "high"},
		{"probe", "--port", "/dev/ttyS0"},
		{"probe", "--transport", "serial"},
		{"probe", "--minutes", "5"},
		{"watch", "--minutes", "0"},
		{"watch", "--minutes", "31"},
		{"scan", "--transport", "serial"},
		{"probe", "--transport", "usb"},
		{"version", "--port", "COM3"},
	}
	for _, a := range bad {
		if _, err := parseArgs(a, &sink); err == nil {
			t.Errorf("%q: want error", a)
		}
	}
}
