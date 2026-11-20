// SPDX-License-Identifier: AGPL-3.0-or-later

package device

import (
	"sort"
	"strings"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// VLT-IS-012 TASK 03-00-00-003: known USB-to-serial bridge chips and the
// UPS profiles. The mark "Probable UPS" comes only from the USB data that
// the OS gives (VID and PID). It sends no byte to a port and changes
// nothing else: no automatic selection, no automatic scan (SEC-099).

// Bridge is a known USB-to-serial bridge chip.
type Bridge struct {
	VID, PID, Name string
}

// Bridges is the list of known bridge chips (step 2).
var Bridges = []Bridge{
	{VID: "067B", PID: "2303", Name: "Prolific PL2303"},
	{VID: "0403", PID: "6001", Name: "FTDI FT232"},
	{VID: "10C4", PID: "EA60", Name: "Silicon Labs CP210x"},
	{VID: "1A86", PID: "7523", Name: "WCH CH340"},
	{VID: "0665", PID: "5161", Name: "Cypress USB to Serial"},
}

// BridgeOf returns the known bridge chip with the VID and PID. The
// comparison ignores the case ("067b" and "067B").
func BridgeOf(vid, pid string) (Bridge, bool) {
	for _, b := range Bridges {
		if strings.EqualFold(b.VID, strings.TrimSpace(vid)) && strings.EqualFold(b.PID, strings.TrimSpace(pid)) {
			return b, true
		}
	}
	return Bridge{}, false
}

// Profile IDs. ProfileAuto ("") is "Other UPS": the automatic detection.
const (
	ProfileAuto       = ""
	ProfileNewtechPro = "tuncmatik-newtech-pro-ii"
)

// UPSProfile is a fixed serial and Dialect setup for a known UPS model.
type UPSProfile struct {
	ID       string
	Name     string
	Baud     int
	DataBits int
	Parity   string // "N": none
	StopBits int
	DTR, RTS transport.Line
	Dialect  dialect.Dialect
	// Bridges are the bridge chips for which the wizard offers the
	// profile.
	Bridges []Bridge
}

// UPSProfiles are the known UPS profiles (step 3). The Reference UPS
// bridge chip is the Prolific PL2303, VID 067B, PID 2303 (FIELD, Owner
// test R-17).
var UPSProfiles = []UPSProfile{{
	ID:       ProfileNewtechPro,
	Name:     "Tunçmatik Newtech Pro II (Megatec)",
	Baud:     transport.SerialBaud,
	DataBits: 8,
	Parity:   "N",
	StopBits: 1,
	DTR:      transport.LineUnchanged,
	RTS:      transport.LineUnchanged,
	Dialect:  dialect.DialectMegatec,
	Bridges:  []Bridge{Bridges[0]},
}}

// ProfileByID returns the UPS profile with the ID.
func ProfileByID(id string) (UPSProfile, bool) {
	for _, p := range UPSProfiles {
		if p.ID == id {
			return p, true
		}
	}
	return UPSProfile{}, false
}

// ProfilesFor returns the UPS profiles that the wizard offers for a port
// with the VID and PID.
func ProfilesFor(vid, pid string) []UPSProfile {
	var out []UPSProfile
	for _, p := range UPSProfiles {
		for _, b := range p.Bridges {
			if strings.EqualFold(b.VID, strings.TrimSpace(vid)) && strings.EqualFold(b.PID, strings.TrimSpace(pid)) {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// MarkPorts sets the mark "Probable UPS" and the bridge name on each port
// with a known bridge chip and puts these ports at the top of the list.
// The order in each group stays the same. It opens no port.
func MarkPorts(l []transport.SerialPortInfo) []transport.SerialPortInfo {
	out := make([]transport.SerialPortInfo, len(l))
	copy(out, l)
	for i := range out {
		out[i].ProbableUPS, out[i].Bridge = false, ""
		if !out[i].IsUSB {
			continue
		}
		if b, ok := BridgeOf(out[i].VID, out[i].PID); ok {
			out[i].ProbableUPS, out[i].Bridge = true, b.Name
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].ProbableUPS && !out[b].ProbableUPS })
	return out
}

// probeListFor returns the commands of the Probe List that the Dialect
// has, in Probe List order.
func probeListFor(d dialect.Dialect) []string {
	var out []string
	for _, cmd := range ProbeList {
		e, ok := dialect.LookupRead(cmd)
		if !ok {
			continue
		}
		for _, x := range e.Dialects {
			if x == d {
				out = append(out, cmd)
				break
			}
		}
	}
	return out
}
