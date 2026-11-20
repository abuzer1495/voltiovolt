// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/store"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// VLT-IS-012 TASK 03-00-00-003 step 5.

func TestBridgeOf(t *testing.T) {
	for _, c := range []struct{ vid, pid, name string }{
		{"067B", "2303", "Prolific PL2303"}, {"067b", "2303", "Prolific PL2303"},
		{"0403", "6001", "FTDI FT232"}, {"10C4", "EA60", "Silicon Labs CP210x"}, {"10c4", "ea60", "Silicon Labs CP210x"},
		{"1A86", "7523", "WCH CH340"}, {"0665", "5161", "Cypress USB to Serial"},
	} {
		b, ok := BridgeOf(c.vid, c.pid)
		if !ok || b.Name != c.name {
			t.Errorf("BridgeOf(%s, %s) = %+v %v", c.vid, c.pid, b, ok)
		}
	}
	for _, c := range [][2]string{{"067B", "2304"}, {"0403", "6015"}, {"", ""}, {"1234", "5678"}} {
		if b, ok := BridgeOf(c[0], c[1]); ok {
			t.Errorf("BridgeOf(%s, %s) = %+v, want none", c[0], c[1], b)
		}
	}
}

func TestMarkPortsOrderAndNothingElse(t *testing.T) {
	in := []transport.SerialPortInfo{
		{Name: "COM1"},
		{Name: "COM5", IsUSB: true, VID: "2341", PID: "0043", Product: "Arduino"},
		{Name: "COM3", IsUSB: true, VID: "067B", PID: "2303", Product: "Prolific USB-to-Serial Comm Port"},
		{Name: "COM2"},
		{Name: "COM7", IsUSB: true, VID: "0403", PID: "6001"},
		// Not USB: the VID is not used.
		{Name: "COM9", VID: "067B", PID: "2303"},
	}
	orig := make([]transport.SerialPortInfo, len(in))
	copy(orig, in)
	out := MarkPorts(in)
	want := []string{"COM3", "COM7", "COM1", "COM5", "COM2", "COM9"}
	if len(out) != len(want) {
		t.Fatalf("len %d", len(out))
	}
	for i, n := range want {
		if out[i].Name != n {
			t.Fatalf("order %d: %s, want %s (%+v)", i, out[i].Name, n, out)
		}
	}
	if !out[0].ProbableUPS || out[0].Bridge != "Prolific PL2303" || !out[1].ProbableUPS || out[1].Bridge != "FTDI FT232" {
		t.Fatalf("marks: %+v", out[:2])
	}
	for _, p := range out[2:] {
		if p.ProbableUPS || p.Bridge != "" {
			t.Fatalf("unexpected mark: %+v", p)
		}
	}
	// The other fields stay the same; the input is not changed.
	for i := range in {
		if in[i] != orig[i] {
			t.Fatalf("input changed: %+v", in[i])
		}
	}
	if out[0].Product != "Prolific USB-to-Serial Comm Port" || out[0].VID != "067B" {
		t.Fatalf("fields changed: %+v", out[0])
	}
}

// The mark sends no byte to a port (SEC-099): Ports opens no Transport.
func TestPortsMarkSendsNothing(t *testing.T) {
	opened := 0
	s := &Scanner{
		List: func() ([]transport.SerialPortInfo, error) {
			return []transport.SerialPortInfo{{Name: "COM1"}, {Name: "COM3", IsUSB: true, VID: "067b", PID: "2303"}}, nil
		},
		NewSerial: func(string, transport.SerialOptions, transport.Options) transport.Transport {
			opened++
			return nil
		},
	}
	l, err := s.Ports(context.Background())
	if err != nil || len(l) != 2 || l[0].Name != "COM3" || !l[0].ProbableUPS {
		t.Fatalf("%+v %v", l, err)
	}
	if opened != 0 {
		t.Fatalf("Ports opened %d ports", opened)
	}
}

func TestReferenceProfile(t *testing.T) {
	p, ok := ProfileByID(ProfileNewtechPro)
	if !ok || p.Name != "Tunçmatik Newtech Pro II (Megatec)" || p.Baud != 2400 || p.DataBits != 8 || p.Parity != "N" || p.StopBits != 1 ||
		p.DTR != transport.LineUnchanged || p.RTS != transport.LineUnchanged || p.Dialect != dialect.DialectMegatec {
		t.Fatalf("%+v %v", p, ok)
	}
	if _, ok := ProfileByID(ProfileAuto); ok {
		t.Fatal("Other UPS must be the automatic detection, not a profile")
	}
	if l := ProfilesFor("067b", "2303"); len(l) != 1 || l[0].ID != ProfileNewtechPro {
		t.Fatalf("ProfilesFor 067b:2303 = %+v", l)
	}
	for _, c := range [][2]string{{"0403", "6001"}, {"1A86", "7523"}, {"", ""}} {
		if l := ProfilesFor(c[0], c[1]); len(l) != 0 {
			t.Fatalf("ProfilesFor %v = %+v", c, l)
		}
	}
	if got := probeListFor(dialect.DialectMegatec); len(got) != 3 || got[0] != "Q1" || got[1] != "F" || got[2] != "I" {
		t.Fatalf("megatec probe list %v", got)
	}
}

// With the profile, Detect sends only the megatec commands and selects
// the Dialect megatec, also for a UPS that answers more commands.
func TestDetectWithProfile(t *testing.T) {
	for _, prof := range []sim.Profile{sim.ProfileQ1, sim.ProfileVoltronic} {
		s := sim.New(sim.NewModel(prof))
		list := listOf(simCandidate(t, s, TransportSerial))
		c := DefaultConfig()
		c.Transport, c.SerialPort, c.Profile = TransportSerial, "COM3", ProfileNewtechPro
		p, tr, err := Detect(context.Background(), c, list, newFakeClock(), nil)
		if err != nil {
			t.Fatalf("%s: %v", prof, err)
		}
		tr.Close()
		if p.Dialect != dialect.DialectMegatec {
			t.Fatalf("%s: dialect %s", prof, p.Dialect)
		}
		for _, r := range s.Received() {
			if r != "Q1" && r != "F" && r != "I" {
				t.Fatalf("%s: sent %q with the megatec profile", prof, r)
			}
		}
	}
	// Other UPS: the automatic detection finds the Voltronic Dialect.
	s := sim.New(sim.NewModel(sim.ProfileVoltronic))
	list := listOf(simCandidate(t, s, TransportSerial))
	c := DefaultConfig()
	c.Transport, c.SerialPort = TransportSerial, "COM3"
	p, tr, err := Detect(context.Background(), c, list, newFakeClock(), nil)
	if err != nil || p.Dialect != dialect.DialectVoltronic {
		t.Fatalf("auto: %+v %v", p, err)
	}
	tr.Close()
}

// The profile Setting gives DTR and RTS unchanged, also when the stored
// line Settings are on or off.
func TestConfigFromStoreProfile(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "vlt.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.SetMACKey(bytes.Repeat([]byte{7}, 32)) // test key only
	for k, v := range map[string]string{store.SettingDeviceDTR: `"on"`, store.SettingDeviceRTS: `"off"`, store.SettingDeviceProfile: `"tuncmatik-newtech-pro-ii"`} {
		if _, err := st.SetSetting(ctx, k, json.RawMessage(v), "test"); err != nil {
			t.Fatal(err)
		}
	}
	c, err := ConfigFromStore(ctx, st)
	if err != nil || c.Profile != ProfileNewtechPro || c.DTR != transport.LineUnchanged || c.RTS != transport.LineUnchanged {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := st.SetSetting(ctx, store.SettingDeviceProfile, json.RawMessage(`""`), "test"); err != nil {
		t.Fatal(err)
	}
	c, err = ConfigFromStore(ctx, st)
	if err != nil || c.Profile != ProfileAuto || c.DTR != transport.LineOn || c.RTS != transport.LineOff {
		t.Fatalf("Other UPS: %+v %v", c, err)
	}
}
