// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
)

const (
	screenW = 100
	screenH = 30
	menuW   = 48
)

func (ss *session) loop(ctx context.Context, sc tcell.Screen) int {
	if err := sc.Init(); err != nil {
		return 1
	}
	defer sc.Fini()
	if simu, ok := sc.(tcell.SimulationScreen); ok {
		simu.SetSize(screenW, screenH)
	}
	sc.DisableMouse()
	sc.HideCursor()

	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				sc.PostEvent(tcell.NewEventInterrupt(nil))
				return
			case <-tick.C:
				ss.m.Tick()
				sc.PostEvent(tcell.NewEventInterrupt(nil))
			}
		}
	}()

	ss.draw(sc)
	for {
		ev := sc.PollEvent()
		if ctx.Err() != nil {
			return 0
		}
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ss.handle(ev) {
				return 0
			}
			ss.draw(sc)
		case *tcell.EventResize:
			sc.Sync()
			ss.draw(sc)
		case *tcell.EventInterrupt:
			ss.draw(sc)
		}
	}
}

// handle returns true when the user confirmed quit.
func (ss *session) handle(ev *tcell.EventKey) bool {
	if ss.quitting {
		r := ev.Rune()
		if ev.Key() == tcell.KeyEscape || r == ss.lang.No || r == 'n' || r == 'N' || r == 'h' || r == 'H' {
			ss.quitting = false
			return false
		}
		if r == ss.lang.Yes || r == 'y' || r == 'Y' || r == 'e' || r == 'E' {
			return true
		}
		return false
	}
	if ss.fullLog {
		switch ev.Key() {
		case tcell.KeyUp:
			ss.logScroll++
			return false
		case tcell.KeyDown:
			if ss.logScroll > 0 {
				ss.logScroll--
			}
			return false
		case tcell.KeyPgUp:
			ss.logScroll += 10
			return false
		case tcell.KeyPgDn:
			ss.logScroll -= 10
			if ss.logScroll < 0 {
				ss.logScroll = 0
			}
			return false
		case tcell.KeyEscape:
			ss.fullLog = false
			return false
		}
		if ev.Rune() == 'l' || ev.Rune() == 'L' {
			ss.fullLog = false
			return false
		}
		// Fall through so 1-9, Q, R still work.
	}
	switch ev.Key() {
	case tcell.KeyUp:
		if ss.cursor > 0 {
			ss.cursor--
		}
		return false
	case tcell.KeyDown:
		if ss.cursor < itemCount-1 {
			ss.cursor++
		}
		return false
	case tcell.KeyLeft:
		ss.change(-1)
		return false
	case tcell.KeyRight:
		ss.change(1)
		return false
	case tcell.KeyPgUp:
		ss.change(10)
		return false
	case tcell.KeyPgDn:
		ss.change(-10)
		return false
	case tcell.KeyEnter:
		ss.activate()
		return false
	case tcell.KeyEscape:
		ss.quitting = true
		return false
	}
	switch ev.Rune() {
	case ' ':
		ss.activate()
	case 'q', 'Q':
		ss.quitting = true
	case 'r', 'R':
		ss.reset()
	case 'l', 'L':
		ss.fullLog = !ss.fullLog
		ss.logScroll = 0
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		ss.applyScenario(int(ev.Rune() - '0'))
	}
	return false
}

func (ss *session) draw(sc tcell.Screen) {
	sc.Clear()
	w, h := sc.Size()
	if w < 2 || h < 2 {
		sc.Show()
		return
	}
	st := tcell.StyleDefault
	sel := tcell.StyleDefault.Reverse(true)
	bold := tcell.StyleDefault.Bold(true)
	L := ss.lang
	sn := ss.m.Snap()

	client := L.Waiting
	if ss.s.ClientConnected() {
		client = L.Connected
	}
	title := fmt.Sprintf("%s  |  %s  |  %s  |  %s", L.Title, client, sn.Profile.DisplayName(), ss.endpoint)
	put(sc, 0, 0, w, title, bold)
	hline(sc, 0, 1, w, st)

	if ss.fullLog {
		ss.drawFullLog(sc, w, h, st, bold)
		ss.drawFooter(sc, w, h, st, L)
		ss.show(sc)
		return
	}

	for i := 0; i < itemCount; i++ {
		y := 2 + i
		if y >= h-10 {
			break
		}
		style := st
		if i == ss.cursor {
			style = sel
		}
		// Temporarily set cursor to i to reuse valueText.
		cur := ss.cursor
		ss.cursor = i
		val := ss.valueText(sn)
		ss.cursor = cur
		mark := "  "
		if i == ss.cursor {
			mark = "> "
		}
		left := fmt.Sprintf("%s%-26s %s", mark, L.Menu[i], val)
		put(sc, 0, y, menuW, left, style)
	}

	ss.drawStatus(sc, menuW+1, 2, w, st, bold, L, sn)
	hline(sc, 0, h-10, w, st)
	ss.drawLog(sc, h-9, 8, w, st)
	ss.drawFooter(sc, w, h, st, L)
	ss.show(sc)
}

func (ss *session) drawStatus(sc tcell.Screen, x, y, w int, st, bold tcell.Style, L Lang, sn sim.Snapshot) {
	rw := w - x
	if rw < 10 {
		return
	}
	line := func(s string) {
		put(sc, x+1, y, rw-1, s, st)
		y++
	}
	line(fmt.Sprintf("%s  %6.1f V", L.StInput, sn.Input))
	line(fmt.Sprintf("%s  %6.1f V", L.StFault, sn.Fault))
	line(fmt.Sprintf("%s  %6.1f V", L.StOutput, sn.Output))
	line(fmt.Sprintf("%s  %6.1f Hz", L.StFreq, sn.Freq))
	line(fmt.Sprintf("%s  %6d %%", L.StLoad, sn.Load))
	line(fmt.Sprintf("%s  %6.1f %%", L.StCharge, sn.Charge))
	line(fmt.Sprintf("%s  %5.2f V  (%0.1f V)", L.StBatt, sn.BattCell, sn.BattTotal))
	line(fmt.Sprintf("%s  %6.1f C", L.StTemp, sn.Temperature))
	line(fmt.Sprintf("%s  %6d min", L.StRuntime, sn.Runtime))
	line(fmt.Sprintf("%s  %s", L.StBits, sn.Bits))
	util := L.BitUtilOK
	if sn.UtilityFail {
		util = L.BitUtilFail
	}
	batt := L.BitBattOK
	if sn.BatteryLow {
		batt = L.BitBattLow
	}
	line("  " + util + ", " + batt)
	line(fmt.Sprintf("  %s %s, %s %s", L.BitBypass, onOff(L, sn.Bypass), L.BitFault, onOff(L, sn.UPSFault)))
	line(fmt.Sprintf("  %s %s, %s %s, %s %s", L.BitTest, onOff(L, sn.Test), L.BitShut, onOff(L, sn.Shutdown), L.BitBeep, onOff(L, sn.Beeper)))
	line(fmt.Sprintf("%s: %s", L.StOutputOn, onOff(L, sn.OutputOn)))
	timers := ss.timerText(L, sn)
	line(fmt.Sprintf("%s: %s", L.StTimers, timers))
	if ss.scenario >= 1 && ss.scenario <= 9 {
		line(fmt.Sprintf("%d: %s", ss.scenario, L.Scenario[ss.scenario]))
	}
}

func (ss *session) timerText(L Lang, sn sim.Snapshot) string {
	var parts []string
	if sn.TestLeft > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", L.Test, sn.TestLeft.Truncate(time.Second)))
	}
	if sn.ShutLeft > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", L.Shutdown, sn.ShutLeft.Truncate(time.Second)))
	}
	if sn.RestoreLeft > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", L.Restore, sn.RestoreLeft.Truncate(time.Second)))
	}
	if d, lab := ss.r.Left(); d > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", lab, d.Truncate(time.Second)))
	}
	if len(parts) == 0 {
		return L.None
	}
	return strings.Join(parts, ", ")
}

func (ss *session) drawLog(sc tcell.Screen, y, n, w int, st tcell.Style) {
	log := ss.s.Log()
	start := 0
	if len(log) > n {
		start = len(log) - n
	}
	row := 0
	for i := start; i < len(log) && row < n; i++ {
		put(sc, 0, y+row, w, formatEntry(log[i]), st)
		row++
	}
}

func (ss *session) drawFullLog(sc tcell.Screen, w, h int, st, bold tcell.Style) {
	put(sc, 0, 2, w, ss.lang.LogTitle, bold)
	log := ss.s.Log()
	avail := h - 5
	if avail < 1 {
		return
	}
	if ss.logScroll > len(log)-avail {
		ss.logScroll = len(log) - avail
	}
	if ss.logScroll < 0 {
		ss.logScroll = 0
	}
	end := len(log) - ss.logScroll
	start := end - avail
	if start < 0 {
		start = 0
	}
	row := 3
	for i := start; i < end && i < len(log); i++ {
		put(sc, 0, row, w, formatEntry(log[i]), st)
		row++
	}
}

func formatEntry(e server.Entry) string {
	return fmt.Sprintf("%s  %s  %s", e.Time.Local().Format("15:04:05"), e.Request, e.Response)
}

func (ss *session) drawFooter(sc tcell.Screen, w, h int, st tcell.Style, L Lang) {
	hline(sc, 0, h-2, w, st)
	help := L.Keys
	if ss.quitting {
		help = L.QuitAsk
	}
	put(sc, 0, h-1, w, help, st)
}

func (ss *session) show(sc tcell.Screen) {
	sc.Show()
	if ss.onDraw != nil {
		ss.onDraw(capture(sc))
	}
}

func capture(sc tcell.Screen) string {
	w, h := sc.Size()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, _, _, _ := sc.GetContent(x, y)
			if r == 0 {
				r = ' '
			}
			b.WriteRune(r)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func put(sc tcell.Screen, x, y, maxw int, text string, st tcell.Style) {
	if maxw <= 0 {
		return
	}
	n := 0
	for _, r := range text {
		if n >= maxw {
			break
		}
		sc.SetContent(x+n, y, r, nil, st)
		n++
	}
}

func hline(sc tcell.Screen, x, y, w int, st tcell.Style) {
	for i := 0; i < w; i++ {
		sc.SetContent(x+i, y, '-', nil, st)
	}
}
