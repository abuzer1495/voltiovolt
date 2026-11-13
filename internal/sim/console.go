// SPDX-License-Identifier: AGPL-3.0-or-later

package sim

import (
	"fmt"
	"strings"
	"time"
)

// UPSType is the simulated UPS topology. It selects the battery-voltage
// formula of VLT-IS-010 TASK 03-00-00-003 step 7.
type UPSType int

// Topologies.
const (
	UPSOnline UPSType = iota
	UPSStandby
)

// Console defaults (VLT-IS-010 TASK 03-00-00-003 step 5).
const (
	ConsoleInputV    = 230.0
	ConsoleOutputV   = 220.0
	ConsoleFreq      = 50.0
	ConsoleLoad      = 30
	ConsoleCharge    = 100.0
	ConsoleTemp      = 25.0
	ConsoleRatedV    = 220.0
	ConsoleRatedBatt = 24.0
	ConsoleRatedHz   = 50.0
	ConsoleSpeed     = 1
	// ConsoleFullRuntime is the runtime in minutes at 100 % charge and
	// 100 % load. Used for QBV and for the discharge rate.
	ConsoleFullRuntime = 10.0
	// ConsoleChargeHours is the time on mains to go from 0 % to 100 % at
	// battery speed x1.
	ConsoleChargeHours = 2.0
)

// NewConsoleModel returns a model with the default values of the
// vlt-simulator console, not the test-vector bases of NewModel.
func NewConsoleModel(p Profile) *Model {
	m := NewModel(p)
	m.ApplyConsoleDefaults()
	return m
}

// ApplyConsoleDefaults resets the model to the console defaults
// (230 V input, 30 % load, 25 °C, mains on, and so on).
func (m *Model) ApplyConsoleDefaults() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.console = true
	m.Mains = true
	m.Charge = ConsoleCharge
	m.Load = ConsoleLoad
	m.Temperature = ConsoleTemp
	m.inputV = ConsoleInputV
	m.inputFreq = ConsoleFreq
	m.outputV = ConsoleOutputV
	m.lastInputV = ConsoleInputV
	m.battSpeed = ConsoleSpeed
	m.upsType = UPSOnline
	m.battLowForce = false
	m.bypass = false
	m.ratedV = ConsoleRatedV
	m.ratedBatt = ConsoleRatedBatt
	m.ratedHz = ConsoleRatedHz
	m.beeperOff = false
	m.testEnd = time.Time{}
	m.testLow = false
	m.lowSeen = false
	m.shutAt = time.Time{}
	m.restoreIn = 0
	m.outputOff = false
	m.restoreAt = time.Time{}
	m.liveAt = time.Time{}
	m.faults = map[Fault]bool{}
	m.controls = nil
}

// Tick updates CONTROL timers and, in console mode, the charge.
func (m *Model) Tick() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tick()
}

// consoleTickLocked updates the charge from the load, the mains state and
// the battery speed. m.mu is held. NewModel leaves battSpeed at 0, so
// existing tests are not changed.
func (m *Model) consoleTickLocked() {
	if !m.console || m.battSpeed <= 0 {
		return
	}
	now := m.now()
	if m.liveAt.IsZero() {
		m.liveAt = now
		return
	}
	dt := now.Sub(m.liveAt).Seconds()
	m.liveAt = now
	if dt <= 0 {
		return
	}
	speed := float64(m.battSpeed)
	if !m.Mains && !m.outputOff {
		load := float64(m.Load)
		if load < 1 {
			load = 1
		}
		// Percent per second at speed x1: load / (full runtime in seconds).
		m.Charge = max(0, m.Charge-dt*speed*load/(ConsoleFullRuntime*60))
	} else if m.Mains {
		m.Charge = min(100, m.Charge+dt*speed*(100.0/(ConsoleChargeHours*3600)))
	}
}

func (m *Model) battCellLocked() float64 {
	if !m.console {
		return 2.0 + 0.32*m.Charge/100
	}
	// On-line, per cell: 1.667 V at 0 % to 2.267 V at 100 %.
	cell := 1.667 + 0.6*m.Charge/100
	if m.upsType == UPSStandby {
		// Q1 has a 4-character cell field. Report the equivalent cell
		// voltage so the response length stays 47 bytes.
		blocks := m.blocksLocked()
		if blocks <= 0 {
			return cell
		}
		return m.battTotalLocked() / (blocks * 6)
	}
	return cell
}

func (m *Model) battTotalLocked() float64 {
	if !m.console {
		return 36.0 + 5.8*m.Charge/100
	}
	blocks := m.blocksLocked()
	if m.upsType == UPSStandby {
		// Standby, total: blocks × 10.0 V at 0 % to blocks × 13.6 V at 100 %.
		return blocks * (10.0 + 3.6*m.Charge/100)
	}
	return (1.667 + 0.6*m.Charge/100) * blocks * 6
}

func (m *Model) blocksLocked() float64 {
	rated := m.ratedBatt
	if rated == 0 {
		rated = ConsoleRatedBatt
	}
	return rated / 12
}

func (m *Model) runtimeLocked() int {
	if !m.console {
		return int(58 * m.Charge / 100)
	}
	load := float64(m.Load)
	if load < 1 {
		load = 1
	}
	return int(ConsoleFullRuntime * m.Charge / load)
}

func (m *Model) ratedLocked() (v, batt, hz float64) {
	v, batt, hz = 220.0, 36.0, 50.0
	if m.ratedV != 0 {
		v = m.ratedV
	}
	if m.ratedBatt != 0 {
		batt = m.ratedBatt
	}
	if m.ratedHz != 0 {
		hz = m.ratedHz
	}
	return
}

// SetProfile sets the Dialect profile.
func (m *Model) SetProfile(p Profile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Profile = p
}

// SetInputVoltage sets the input voltage in volts (0 to 300).
func (m *Model) SetInputVoltage(v float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputV = clamp(v, 0, 300)
	if m.Mains {
		m.lastInputV = m.inputV
	}
}

// SetInputFrequency sets the input frequency in hertz (45 to 65).
func (m *Model) SetInputFrequency(f float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inputFreq = clamp(f, 45, 65)
}

// SetOutputVoltage sets the output voltage when the output is on (200 to 240).
func (m *Model) SetOutputVoltage(v float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputV = clamp(v, 200, 240)
}

// SetLoadPercent sets the load in percent (0 to 150).
func (m *Model) SetLoadPercent(p int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p < 0 {
		p = 0
	}
	if p > 150 {
		p = 150
	}
	m.Load = p
}

// SetTemperature sets the temperature in °C (0 to 80).
func (m *Model) SetTemperature(c float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Temperature = clamp(c, 0, 80)
}

// SetBattSpeed sets the discharge and charge speed (1, 10 or 60).
func (m *Model) SetBattSpeed(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch n {
	case 1, 10, 60:
		m.battSpeed = n
	}
}

// SetUPSType sets the topology used for the battery-voltage formula.
func (m *Model) SetUPSType(t UPSType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsType = t
}

// SetBattLowForce, when on, sets status bit 6 regardless of the charge.
func (m *Model) SetBattLowForce(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.battLowForce = on
}

// SetBypass sets status bit 5 (bypass or AVR).
func (m *Model) SetBypass(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bypass = on
}

// SetUPSFault sets status bit 4 (UPS failed).
func (m *Model) SetUPSFault(on bool) {
	m.SetFault(FaultUPSFailed, on)
}

// SetBeeper sets status bit 0.
func (m *Model) SetBeeper(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beeperOff = !on
}

// SetRated sets the rated values of the F response (voltage, battery
// voltage, frequency). Voltage is 220 or 230, battery 24 or 36,
// frequency 50 or 60.
func (m *Model) SetRated(v, batt, hz float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v == 220 || v == 230 {
		m.ratedV = v
	}
	if batt == 24 || batt == 36 {
		m.ratedBatt = batt
	}
	if hz == 50 || hz == 60 {
		m.ratedHz = hz
	}
}

// SetChargeConsole sets the charge and records a live-tick origin so the
// next Tick uses a short dt.
func (m *Model) SetChargeConsole(p float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Charge = clamp(p, 0, 100)
	m.liveAt = m.now()
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ParseProfile accepts the console names (megatec, voltronic-qs,
// voltronic) and the short model names (q1, qs, voltronic).
func ParseProfile(name string) (Profile, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "megatec", "q1":
		return ProfileQ1, nil
	case "voltronic-qs", "qs":
		return ProfileQS, nil
	case "voltronic":
		return ProfileVoltronic, nil
	default:
		return "", fmt.Errorf("unknown dialect %q", name)
	}
}

// DisplayName is the console name of a profile.
func (p Profile) DisplayName() string {
	switch p {
	case ProfileQ1:
		return "megatec"
	case ProfileQS:
		return "voltronic-qs"
	case ProfileVoltronic:
		return "voltronic"
	default:
		return string(p)
	}
}

// Snapshot is a consistent view of the model for the console screen.
type Snapshot struct {
	Profile      Profile
	Mains        bool
	Input        float64
	Fault        float64
	Output       float64
	Freq         float64
	Load         int
	Charge       float64
	BattCell     float64
	BattTotal    float64
	Temperature  float64
	Runtime      int
	BattSpeed    int
	UPSType      UPSType
	BattLowForce bool
	Bypass       bool
	UPSFault     bool
	Beeper       bool
	RatedV       float64
	RatedBatt    float64
	RatedHz      float64
	Bits         string
	UtilityFail  bool
	BatteryLow   bool
	Test         bool
	Shutdown     bool
	OutputOn     bool
	TestLeft     time.Duration
	ShutLeft     time.Duration
	RestoreLeft  time.Duration
}

// Snap returns a consistent view of the model.
func (m *Model) Snap() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tick()
	in := 209.7
	if m.inputV != 0 {
		in = m.inputV
	}
	fault := in
	if !m.Mains {
		if m.lastInputV != 0 {
			fault = m.lastInputV
		}
		in = 0
	}
	out := 219.7
	if m.outputV != 0 {
		out = m.outputV
	}
	if m.outputOff {
		out = 0
	}
	freq := 50.0
	if m.inputFreq != 0 {
		freq = m.inputFreq
	}
	bits := m.bits()
	now := m.now()
	var testLeft, shutLeft, restoreLeft time.Duration
	if !m.testEnd.IsZero() && m.testEnd.After(now) {
		testLeft = m.testEnd.Sub(now)
	}
	if !m.shutAt.IsZero() && m.shutAt.After(now) {
		shutLeft = m.shutAt.Sub(now)
	}
	if m.outputOff && m.restoreAt.After(now) {
		restoreLeft = m.restoreAt.Sub(now)
	}
	rv, rb, rh := m.ratedLocked()
	return Snapshot{
		Profile:      m.Profile,
		Mains:        m.Mains,
		Input:        in,
		Fault:        fault,
		Output:       out,
		Freq:         freq,
		Load:         m.Load,
		Charge:       m.Charge,
		BattCell:     m.battCellLocked(),
		BattTotal:    m.battTotalLocked(),
		Temperature:  m.Temperature,
		Runtime:      m.runtimeLocked(),
		BattSpeed:    m.battSpeed,
		UPSType:      m.upsType,
		BattLowForce: m.battLowForce,
		Bypass:       m.bypass,
		UPSFault:     m.faults[FaultUPSFailed],
		Beeper:       !m.beeperOff,
		RatedV:       rv,
		RatedBatt:    rb,
		RatedHz:      rh,
		Bits:         bits,
		UtilityFail:  bits[0] == '1',
		BatteryLow:   bits[1] == '1',
		Test:         bits[5] == '1',
		Shutdown:     bits[6] == '1',
		OutputOn:     !m.outputOff,
		TestLeft:     testLeft,
		ShutLeft:     shutLeft,
		RestoreLeft:  restoreLeft,
	}
}
