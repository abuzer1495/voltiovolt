// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sim is a UPS simulator for tests. It is not a model of the
// Reference UPS. The base values are the test vectors of VLT-IS-002
// 03-50-02 (family sample, UNVERIFIED for the Reference UPS).
package sim

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Profile is a Dialect profile of the simulator.
type Profile string

// Profiles.
const (
	ProfileQ1        Profile = "q1"
	ProfileQS        Profile = "qs"
	ProfileVoltronic Profile = "voltronic"
)

// Fault is one fault injection.
type Fault string

// Faults.
const (
	FaultStale       Fault = "stale"         // old bytes in the buffer after each response
	FaultSplit       Fault = "split"         // responses come in parts of 3 bytes
	FaultBrokenRpt   Fault = "broken-report" // HID reports with missing bytes
	FaultNAK         Fault = "nak"           // all responses are "(NAK"
	FaultEcho        Fault = "echo"          // all responses are an echo of the request
	FaultNoResponse  Fault = "no-response"   // no response
	FaultWrongLength Fault = "wrong-length"  // the last field is removed
	FaultUPSFailed   Fault = "ups-failed"    // status bit 4 is set
)

// Model is the state of the simulated UPS.
type Model struct {
	mu          sync.Mutex
	Profile     Profile
	Mains       bool
	Charge      float64 // battery charge, percent
	Load        int     // percent
	Temperature float64 // °C
	faults      map[Fault]bool

	// CONTROL state (VLT-IS-006 TASK 03-00-00-005 step 9). Now gives the
	// time of the model; nil: time.Now.
	Now       func() time.Time
	beeperOff bool
	testEnd   time.Time // end of a timed test
	testLow   bool      // test until battery low
	lowSeen   bool
	shutAt    time.Time // output off time; zero: no shutdown active
	restoreIn time.Duration
	outputOff bool
	restoreAt time.Time
	controls  []string
}

// SimTestLowCharge is the charge at which a test until low ends.
const SimTestLowCharge = 20

func (m *Model) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// tick updates the CONTROL state at the model time. m.mu is held.
func (m *Model) tick() {
	now := m.now()
	if !m.testEnd.IsZero() && !now.Before(m.testEnd) {
		m.testEnd = time.Time{}
	}
	// A test until low ends one response after the battery is low, so one
	// status shows b2 = 1 with b6 = 1.
	if m.testLow && m.Charge < SimTestLowCharge {
		if m.lowSeen {
			m.testLow, m.lowSeen = false, false
		} else {
			m.lowSeen = true
		}
	}
	if !m.shutAt.IsZero() && !now.Before(m.shutAt) {
		m.shutAt = time.Time{}
		m.outputOff = true
		m.restoreAt = now.Add(m.restoreIn)
	}
	// Output on after the restore time if mains power is present. Without
	// mains power the model waits for mains (VLT-REF-001 5.1 item 4: the
	// real behavior is UNVERIFIED).
	if m.outputOff && !now.Before(m.restoreAt) && m.Mains {
		m.outputOff = false
	}
}

// testing reports whether a test runs. m.mu is held.
func (m *Model) testing() bool { return !m.testEnd.IsZero() || m.testLow }

// OutputOn reports whether the UPS output is on.
func (m *Model) OutputOn() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tick()
	return !m.outputOff
}

// Controls returns the CONTROL commands that the model received.
func (m *Model) Controls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.controls...)
}

// BeeperOn reports the beeper state (b0).
func (m *Model) BeeperOn() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.beeperOff
}

// control reacts to a CONTROL command. It returns false if cmd is not a
// CONTROL command. m.mu is held.
func (m *Model) control(cmd string) bool {
	now := m.now()
	switch {
	case cmd == "T":
		m.testEnd = now.Add(10 * time.Second)
	case cmd == "TL":
		m.testLow, m.lowSeen = true, false
	case len(cmd) == 3 && cmd[0] == 'T' && isNum(cmd[1:]):
		n, _ := strconv.Atoi(cmd[1:])
		m.testEnd = now.Add(time.Duration(n) * time.Minute)
	case cmd == "CT":
		m.testEnd, m.testLow = time.Time{}, false
	case cmd == "Q":
		m.beeperOff = !m.beeperOff
	case cmd == "C":
		m.shutAt = time.Time{}
	case len(cmd) == 8 && cmd[0] == 'S' && cmd[3] == 'R' && isNum(cmd[4:]):
		var d time.Duration
		if cmd[1] == '.' && isNum(cmd[2:3]) {
			n, _ := strconv.Atoi(cmd[2:3])
			d = time.Duration(n) * 6 * time.Second
		} else if isNum(cmd[1:3]) {
			n, _ := strconv.Atoi(cmd[1:3])
			d = time.Duration(n) * time.Minute
		} else {
			return false
		}
		r, _ := strconv.Atoi(cmd[4:])
		m.shutAt = now.Add(d)
		m.restoreIn = time.Duration(r) * time.Minute
	default:
		return false
	}
	m.controls = append(m.controls, cmd)
	return true
}

func isNum(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// NewModel returns a model with the base values of the test vectors.
func NewModel(p Profile) *Model {
	return &Model{Profile: p, Mains: true, Charge: 100, Load: 19, Temperature: 23.3, faults: map[Fault]bool{}}
}

// SetFault turns a fault on or off.
func (m *Model) SetFault(f Fault, on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.faults[f] = on
}

// HasFault reports whether a fault is on.
func (m *Model) HasFault(f Fault) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.faults[f]
}

// SetCharge sets the battery charge in percent. Below 20 % the model sets
// status bit 6 (battery low).
func (m *Model) SetCharge(p float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Charge = p
}

// SetMains sets the mains state.
func (m *Model) SetMains(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Mains = on
}

func (m *Model) supports(cmd string) bool {
	switch m.Profile {
	case ProfileQ1:
		return cmd == "Q1" || cmd == "F" || cmd == "I"
	case ProfileQS:
		return cmd == "Q1" || cmd == "QS" || cmd == "F" || cmd == "I"
	case ProfileVoltronic:
		switch cmd {
		case "Q1", "QS", "QGS", "QMOD", "QBV", "QWS", "QFLAG", "QRI", "F", "QID", "QVFW", "QMD":
			return true
		}
	}
	return false
}

func (m *Model) bits() string {
	b := []byte("00000001") // bit 0: beeper on (vector value)
	if m.beeperOff {
		b[7] = '0'
	}
	if m.testing() {
		b[5] = '1' // bit 2: test in progress
	}
	if !m.shutAt.IsZero() || m.outputOff {
		b[6] = '1' // bit 1: shutdown active
	}
	if !m.Mains {
		b[0] = '1' // bit 7: utility fail
	}
	if m.Charge < 20 {
		b[1] = '1' // bit 6: battery low
	}
	if m.faults[FaultUPSFailed] {
		b[3] = '1' // bit 4: UPS failed
	}
	return string(b)
}

// Respond returns the response bytes (CR included) for one command, or nil
// for no response. Unknown commands get "(NAK" in the Voltronic profile and
// an echo in the other profiles.
func (m *Model) Respond(cmd string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.faults[FaultNoResponse]:
		return nil
	case m.faults[FaultNAK]:
		return []byte("(NAK\r")
	case m.faults[FaultEcho]:
		return []byte(cmd + "\r")
	}
	m.tick()
	// CONTROL commands: the response is not read (VLT-REF-001 section 5).
	// The model gives no response.
	if m.control(cmd) {
		return nil
	}
	if !m.supports(cmd) {
		if m.Profile == ProfileVoltronic {
			return []byte("(NAK\r")
		}
		return []byte(cmd + "\r")
	}
	in := 209.7
	if !m.Mains {
		in = 0
	}
	out := 219.7
	if m.outputOff {
		out = 0
	}
	battCell := 2.0 + 0.32*m.Charge/100
	battTotal := 36.0 + 5.8*m.Charge/100
	runtime := int(58 * m.Charge / 100)
	var s string
	switch cmd {
	case "Q1":
		s = fmt.Sprintf("(%05.1f %05.1f %05.1f %03d 50.0 %4.2f %04.1f %s", in, in, out, m.Load, battCell, m.Temperature, m.bits())
	case "QS":
		s = fmt.Sprintf("(%05.1f %05.1f %05.1f %03d 50.0 %04.1f %04.1f %s", in, in, out, m.Load+1, battTotal, m.Temperature, m.bits())
	case "QGS":
		qgsBits := "100000000001"
		if !m.Mains {
			qgsBits = "110000000001" // raw bit pattern only; meaning UNVERIFIED
		}
		s = fmt.Sprintf("(%05.1f 50.0 219.7 50.0 000.8 %03d 374.9 374.3 %05.1f ---.- %05.1f %s", in, m.Load, battTotal, m.Temperature, qgsBits)
	case "QMOD":
		if m.Mains {
			s = "(L"
		} else {
			s = "(B"
		}
	case "QBV":
		s = fmt.Sprintf("(%05.1f 03 01 %03d %03d", battTotal, int(m.Charge), runtime)
	case "QWS":
		s = "(" + strings.Repeat("0", 64)
	case "QFLAG":
		s = "(EpbshcDrovegfjl"
	case "QRI":
		s = "(220.0 004 036.0 50.0"
	case "F":
		s = "#220.0 004 036.0 50.0"
	case "I":
		s = fmt.Sprintf("#%-15s %-10s %-10s", "SIMULATOR", "VLT-SIM", "0.0")
	case "QID":
		s = "(SIMQID00000001" // fake value
	case "QVFW":
		s = "(VERFW:00019.03"
	case "QMD":
		s = "(############G1K ###1000 80 1/1 220 220 03 12.0"
	}
	if m.faults[FaultWrongLength] {
		if i := strings.LastIndexByte(s, ' '); i > 0 {
			s = s[:i]
		} else if len(s) > 1 {
			s = s[:len(s)-1]
		}
	}
	return []byte(s + "\r")
}
