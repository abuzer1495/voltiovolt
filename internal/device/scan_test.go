// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// TestScanOnlyMarkedPorts: the scan sends only "Q1" CR, and only to the
// marked ports (SEC-099). The ports are simulator ports; no real serial
// port is opened.
func TestScanOnlyMarkedPorts(t *testing.T) {
	ups := sim.New(sim.NewModel(sim.ProfileQ1))
	silent := sim.NewModel(sim.ProfileQ1)
	silent.SetFault(sim.FaultNoResponse, true)
	quiet := sim.New(silent)
	other := sim.New(sim.NewModel(sim.ProfileQ1)) // not marked
	busy := &busyTransport{}
	sims := map[string]*sim.Sim{"COM3": ups, "COM4": quiet, "COM5": other}
	opened := map[string]int{}
	sc := &Scanner{
		List: func() ([]transport.SerialPortInfo, error) {
			return []transport.SerialPortInfo{{Name: "COM1"}, {Name: "COM3", IsUSB: true, VID: "1a86", PID: "7523"}, {Name: "COM4"}, {Name: "COM5"}, {Name: "COM6"}}, nil
		},
		NewSerial: func(name string, so transport.SerialOptions, _ transport.Options) transport.Transport {
			opened[name]++
			if so.DTR != transport.LineUnchanged || so.RTS != transport.LineUnchanged {
				t.Errorf("%s: lines %s %s", name, so.DTR, so.RTS)
			}
			if name == "COM6" {
				return busy
			}
			return sims[name].StreamTransport(testOpts)
		},
	}
	res, err := sc.Scan(context.Background(), []string{"COM3", "COM4", "COM6", "COM1"}, transport.DefaultSerialOptions(), "COM1")
	if err != nil {
		t.Fatal(err)
	}
	want := []ScanResult{{"COM3", ScanResponse, true}, {"COM4", ScanNoResponse, false}, {"COM6", ScanBusy, false}, {"COM1", ScanBusy, false}}
	if len(res) != len(want) {
		t.Fatalf("%+v", res)
	}
	for i := range want {
		if res[i] != want[i] {
			t.Errorf("%d: %+v, want %+v", i, res[i], want[i])
		}
	}
	if opened["COM5"] != 0 || opened["COM1"] != 0 {
		t.Errorf("a port that is not marked or in use was opened: %v", opened)
	}
	if got := strings.Join(ups.Received(), ","); got != "Q1" {
		t.Errorf("COM3 got %q", got)
	}
	if got := strings.Join(quiet.Received(), ","); got != "Q1" {
		t.Errorf("COM4 got %q", got)
	}
	if n := len(other.Received()); n != 0 {
		t.Errorf("COM5 (not marked) got %d commands", n)
	}

	// Refused: not in the OS list, two times, none, too many.
	for _, m := range [][]string{{"COM9"}, {"/dev/ttyS0"}, {"COM3", "COM3"}, {}, make([]string, MaxScanPorts+1)} {
		if _, err := sc.Scan(context.Background(), m, transport.DefaultSerialOptions(), ""); !errors.Is(err, ErrScanPort) {
			t.Errorf("%q: %v", m, err)
		}
	}
	if n := len(ups.Received()); n != 1 {
		t.Errorf("a refused scan sent %d commands", n-1)
	}
	// Ports opens no port.
	before := len(opened)
	if l, err := sc.Ports(context.Background()); err != nil || len(l) != 5 || len(opened) != before {
		t.Errorf("ports %v %v", l, err)
	}
}
