// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
)

const (
	itemMains = iota
	itemInputV
	itemInputHz
	itemOutputV
	itemLoad
	itemCharge
	itemSpeed
	itemTemp
	itemType
	itemBattLow
	itemUPSFault
	itemBypass
	itemBeeper
	itemDialect
	itemOther
	itemDelay
	itemInj
	itemRated
	itemCount
)

// session is the shared state of the console and of --headless.
type session struct {
	m        *sim.Model
	s        *server.Server
	r        *Runner
	lang     Lang
	endpoint string

	cursor     int
	ratedField int // 0 voltage, 1 battery, 2 frequency
	fullLog    bool
	logScroll  int
	quitting   bool
	scenario   int // last scenario, 0 none
	delay      time.Duration
	fault      server.Fault
	other      server.OtherReply

	// onDraw, if set, is called from the UI goroutine after each draw
	// (tests capture the screen without a data race on the tcell buffer).
	onDraw func(string)
}

func newSession(m *sim.Model, s *server.Server, r *Runner, lang Lang, endpoint string) *session {
	s.SetDelay(50 * time.Millisecond)
	s.SetOther(server.OtherEcho)
	s.SetFault(server.FaultNone)
	return &session{
		m: m, s: s, r: r, lang: lang, endpoint: endpoint,
		delay: 50 * time.Millisecond, fault: server.FaultNone, other: server.OtherEcho,
	}
}

func (ss *session) reset() {
	ss.r.Cancel()
	ss.m.ApplyConsoleDefaults()
	ss.delay = 50 * time.Millisecond
	ss.fault = server.FaultNone
	ss.other = server.OtherEcho
	ss.s.SetDelay(ss.delay)
	ss.s.SetFault(ss.fault)
	ss.s.SetOther(ss.other)
	ss.ratedField = 0
	ss.scenario = 0
}

func (ss *session) applyScenario(n int) {
	if n < 1 || n > 9 {
		return
	}
	if n == 9 {
		ss.reset()
		ss.scenario = 9
		return
	}
	applyScenario(ss.m, ss.s, ss.r, n)
	if n == 7 {
		ss.fault = server.FaultNoResponse
	} else if n == 1 || n == 2 || n == 3 || n == 4 || n == 6 {
		ss.fault = server.FaultNone
		ss.s.SetFault(server.FaultNone)
	}
	ss.scenario = n
}

func (ss *session) valueText(sn sim.Snapshot) string {
	L := ss.lang
	switch ss.cursor {
	case itemMains:
		return onOff(L, sn.Mains)
	case itemInputV:
		return fmt.Sprintf("%.0f V", ss.inputSetting(sn))
	case itemInputHz:
		return fmt.Sprintf("%.1f Hz", sn.Freq)
	case itemOutputV:
		return fmt.Sprintf("%.0f V", outputSetting(sn))
	case itemLoad:
		return fmt.Sprintf("%d %%", sn.Load)
	case itemCharge:
		return fmt.Sprintf("%.0f %%", sn.Charge)
	case itemSpeed:
		return fmt.Sprintf("x%d", speedOf(sn))
	case itemTemp:
		return fmt.Sprintf("%.0f C", sn.Temperature)
	case itemType:
		if sn.UPSType == sim.UPSStandby {
			return L.Standby
		}
		return L.Online
	case itemBattLow:
		if sn.BattLowForce {
			return L.ForcedOn
		}
		return L.Auto
	case itemUPSFault:
		return onOff(L, sn.UPSFault)
	case itemBypass:
		return onOff(L, sn.Bypass)
	case itemBeeper:
		return onOff(L, sn.Beeper)
	case itemDialect:
		return sn.Profile.DisplayName()
	case itemOther:
		return otherText(L, ss.other)
	case itemDelay:
		return fmt.Sprintf("%d ms", int(ss.delay/time.Millisecond))
	case itemInj:
		return faultText(L, ss.fault)
	case itemRated:
		return ratedText(sn, ss.ratedField)
	}
	return ""
}

func onOff(L Lang, on bool) string {
	if on {
		return L.On
	}
	return L.Off
}

func speedOf(sn sim.Snapshot) int {
	if sn.BattSpeed <= 0 {
		return 1
	}
	return sn.BattSpeed
}

func outputSetting(sn sim.Snapshot) float64 {
	if sn.OutputOn && sn.Output != 0 {
		return sn.Output
	}
	if sn.Output == 0 {
		return sim.ConsoleOutputV
	}
	return sn.Output
}

func (ss *session) inputSetting(sn sim.Snapshot) float64 {
	if sn.Mains {
		return sn.Input
	}
	return sn.Fault
}

func otherText(L Lang, o server.OtherReply) string {
	switch o {
	case server.OtherNAK:
		return L.OtherNAK
	case server.OtherNone:
		return L.OtherNone
	default:
		return L.OtherEcho
	}
}

func faultText(L Lang, f server.Fault) string {
	switch f {
	case server.FaultNoResponse:
		return L.FaultNoResp
	case server.FaultGarbage:
		return L.FaultGarbage
	case server.FaultSplit:
		return L.FaultSplit
	case server.FaultWrongLength:
		return L.FaultWrong
	case server.FaultOldBytes:
		return L.FaultOld
	default:
		return L.FaultNone
	}
}

func ratedText(sn sim.Snapshot, field int) string {
	parts := [3]string{
		fmt.Sprintf("%.0f V", sn.RatedV),
		fmt.Sprintf("%.0f V", sn.RatedBatt),
		fmt.Sprintf("%.0f Hz", sn.RatedHz),
	}
	s := parts[0] + ", " + parts[1] + ", " + parts[2]
	if field >= 0 && field < 3 {
		// The selected subfield is marked with [ ].
		switch field {
		case 0:
			s = "[" + parts[0] + "], " + parts[1] + ", " + parts[2]
		case 1:
			s = parts[0] + ", [" + parts[1] + "], " + parts[2]
		case 2:
			s = parts[0] + ", " + parts[1] + ", [" + parts[2] + "]"
		}
	}
	return s
}

// delta is +1 or -1 for Left/Right, +10 or -10 for Page.
func (ss *session) change(delta int) {
	sn := ss.m.Snap()
	switch ss.cursor {
	case itemMains:
		ss.m.SetMains(!sn.Mains)
	case itemInputV:
		ss.m.SetInputVoltage(ss.inputSetting(sn) + float64(delta))
	case itemInputHz:
		ss.m.SetInputFrequency(sn.Freq + 0.1*float64(delta))
	case itemOutputV:
		ss.m.SetOutputVoltage(outputSetting(sn) + float64(delta))
	case itemLoad:
		ss.m.SetLoadPercent(sn.Load + delta)
	case itemCharge:
		ss.m.SetChargeConsole(sn.Charge + float64(delta))
	case itemSpeed:
		ss.cycleSpeed(delta)
	case itemTemp:
		ss.m.SetTemperature(sn.Temperature + float64(delta))
	case itemType:
		if delta != 0 {
			if sn.UPSType == sim.UPSOnline {
				ss.m.SetUPSType(sim.UPSStandby)
			} else {
				ss.m.SetUPSType(sim.UPSOnline)
			}
		}
	case itemBattLow:
		ss.m.SetBattLowForce(!sn.BattLowForce)
	case itemUPSFault:
		ss.m.SetUPSFault(!sn.UPSFault)
	case itemBypass:
		ss.m.SetBypass(!sn.Bypass)
	case itemBeeper:
		ss.m.SetBeeper(!sn.Beeper)
	case itemDialect:
		ss.cycleDialect(delta)
	case itemOther:
		ss.cycleOther(delta)
	case itemDelay:
		ms := int(ss.delay/time.Millisecond) + 50*delta
		if ms < 0 {
			ms = 0
		}
		if ms > 2000 {
			ms = 2000
		}
		ss.delay = time.Duration(ms) * time.Millisecond
		ss.s.SetDelay(ss.delay)
	case itemInj:
		ss.cycleFault(delta)
	case itemRated:
		ss.changeRated(sn, delta)
	}
}

func (ss *session) activate() {
	// Enter/Space: toggle on/off, or cycle the rated subfield.
	switch ss.cursor {
	case itemMains, itemType, itemBattLow, itemUPSFault, itemBypass, itemBeeper, itemDialect, itemOther, itemInj, itemSpeed:
		ss.change(1)
	case itemRated:
		ss.ratedField = (ss.ratedField + 1) % 3
	}
}

func (ss *session) cycleSpeed(delta int) {
	sn := ss.m.Snap()
	speeds := []int{1, 10, 60}
	i := 0
	for j, v := range speeds {
		if v == speedOf(sn) {
			i = j
			break
		}
	}
	if delta < 0 {
		i = (i + len(speeds) - 1) % len(speeds)
	} else {
		i = (i + 1) % len(speeds)
	}
	ss.m.SetBattSpeed(speeds[i])
}

func (ss *session) cycleDialect(delta int) {
	order := []sim.Profile{sim.ProfileQ1, sim.ProfileQS, sim.ProfileVoltronic}
	i := 0
	p := ss.m.Snap().Profile
	for j, v := range order {
		if v == p {
			i = j
			break
		}
	}
	if delta < 0 {
		i = (i + len(order) - 1) % len(order)
	} else {
		i = (i + 1) % len(order)
	}
	ss.m.SetProfile(order[i])
}

func (ss *session) cycleOther(delta int) {
	order := []server.OtherReply{server.OtherEcho, server.OtherNAK, server.OtherNone}
	i := 0
	for j, v := range order {
		if v == ss.other {
			i = j
			break
		}
	}
	if delta < 0 {
		i = (i + len(order) - 1) % len(order)
	} else {
		i = (i + 1) % len(order)
	}
	ss.other = order[i]
	ss.s.SetOther(ss.other)
}

func (ss *session) cycleFault(delta int) {
	order := []server.Fault{
		server.FaultNone, server.FaultNoResponse, server.FaultGarbage,
		server.FaultSplit, server.FaultWrongLength, server.FaultOldBytes,
	}
	i := 0
	for j, v := range order {
		if v == ss.fault {
			i = j
			break
		}
	}
	if delta < 0 {
		i = (i + len(order) - 1) % len(order)
	} else {
		i = (i + 1) % len(order)
	}
	ss.fault = order[i]
	ss.s.SetFault(ss.fault)
}

func (ss *session) changeRated(sn sim.Snapshot, delta int) {
	v, b, hz := sn.RatedV, sn.RatedBatt, sn.RatedHz
	switch ss.ratedField {
	case 0:
		if v == 220 {
			v = 230
		} else {
			v = 220
		}
	case 1:
		if b == 24 {
			b = 36
		} else {
			b = 24
		}
	case 2:
		if hz == 50 {
			hz = 60
		} else {
			hz = 50
		}
	}
	ss.m.SetRated(v, b, hz)
	_ = delta
}
