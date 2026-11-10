// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"fmt"
	"strings"
	"time"
)

// Status is the data of the Telegram commands /status and /durum
// (VLT-IS-007 TASK 03-00-00-005 step 5). No Secret and no serial number.
type Status struct {
	Snapshot
	OutputVoltage *float64
	PolicyActive  bool
	PolicyAction  string // shutdown, hibernate
	LastEvent     *Item  // nil: no Event
}

type statusText struct {
	header, outputV, policy, on, off, lastEvent, noEvent string
}

var statusTexts = map[string]statusText{
	"en": {header: "VLT status", outputV: "Output voltage", policy: "Shutdown Policy", on: "active", off: "not active", lastEvent: "Last Event", noEvent: "none"},
	"tr": {header: "VLT durumu", outputV: "Çıkış gerilimi", policy: "Kapatma Politikası", on: "etkin", off: "etkin değil", lastEvent: "Son Olay", noEvent: "yok"},
}

var policyActions = map[string]map[string]string{
	"en": {"shutdown": "shut down", "hibernate": "hibernate"},
	"tr": {"shutdown": "kapat", "hibernate": "hazırda beklet"},
}

// RenderStatus makes the text of /status and /durum in the language l.
func RenderStatus(l string, s Status, now time.Time, loc *time.Location) string {
	tx := lang(l)
	st, ok := statusTexts[l]
	if !ok {
		st = statusTexts["en"]
		l = "en"
	}
	if loc == nil {
		loc = time.Local
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)\n", st.header, now.In(loc).Format(timeLayout))
	mode := s.Mode
	if mode == "" {
		mode = ModeUnknown
	}
	fmt.Fprintf(&b, "%s: %s\n", tx.mode, tx.modes[mode])
	if s.InputVoltage != nil {
		fmt.Fprintf(&b, "%s: %s V\n", tx.inputV, num(*s.InputVoltage))
	}
	if s.OutputVoltage != nil {
		fmt.Fprintf(&b, "%s: %s V\n", st.outputV, num(*s.OutputVoltage))
	}
	if s.Load != nil {
		fmt.Fprintf(&b, "%s: %s %%\n", tx.load, num(*s.Load))
	}
	if s.Charge != nil {
		fmt.Fprintf(&b, "%s: %s %%\n", tx.charge, num(*s.Charge))
	}
	if s.Runtime != nil {
		fmt.Fprintf(&b, "%s: %s %s\n", tx.runtime, num(*s.Runtime), tx.minutes)
	}
	if s.PolicyActive {
		a := policyActions[l][s.PolicyAction]
		if a == "" {
			a = s.PolicyAction
		}
		fmt.Fprintf(&b, "%s: %s (%s)\n", st.policy, st.on, a)
	} else {
		fmt.Fprintf(&b, "%s: %s\n", st.policy, st.off)
	}
	if s.LastEvent != nil {
		fmt.Fprintf(&b, "%s: %s, %s\n", st.lastEvent, tx.title(*s.LastEvent), s.LastEvent.Time.In(loc).Format(timeLayout))
	} else {
		fmt.Fprintf(&b, "%s: %s\n", st.lastEvent, st.noEvent)
	}
	return strings.TrimRight(b.String(), "\n")
}
