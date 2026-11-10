// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"strings"
	"testing"
	"time"
)

func TestTemplatesEnglish(t *testing.T) {
	m := Message{Type: "ON_BATTERY", Severity: Warning, Items: []Item{{Time: t0, Type: "ON_BATTERY", Severity: Warning, Active: true}},
		Snapshot: Snapshot{Mode: ModeBattery, InputVoltage: f64(0), Load: f64(23), Charge: f64(91.5), Runtime: f64(12)}}
	n := Render("en", m, time.UTC)
	if n.Subject != "[VLT][WARNING] On battery" {
		t.Errorf("subject %q", n.Subject)
	}
	want := "[VLT][WARNING] On battery\nTime: 2026-11-10 12:00:00\nMode: On battery\nInput voltage: 0 V\nLoad: 23 %\nBattery charge: 91.5 %\nRuntime: 12 min"
	if n.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", n.Text, want)
	}
}

func TestTemplatesTurkish(t *testing.T) {
	m := Message{Type: "LOW_BATTERY", Severity: Critical, Late: true, Items: []Item{{Time: t0, Type: "LOW_BATTERY", Severity: Critical, Active: true}},
		Snapshot: Snapshot{Mode: ModeBattery, Charge: f64(18)}}
	n := Render("tr", m, time.UTC)
	for _, s := range []string{"[VLT][KRİTİK] Akü düşük", "Geç gönderilen mesaj. Asıl zaman: 2026-11-10 12:00:00", "Mod: Akü ile çalışıyor", "Akü doluluğu: 18 %"} {
		if !strings.Contains(n.Text, s) {
			t.Errorf("no %q in:\n%s", s, n.Text)
		}
	}
	// Values that are not available are not in the text.
	if strings.Contains(n.Text, "Yük") || strings.Contains(n.Text, "Giriş gerilimi") {
		t.Errorf("text has a missing value:\n%s", n.Text)
	}
	cd := Message{Type: TypeCountdownStart, Severity: Critical, Items: []Item{{Time: t0, Type: TypeCountdownStart, Severity: Critical, Active: true, Remaining: 60, Action: "shutdown"}}}
	if n := Render("tr", cd, time.UTC); !strings.Contains(n.Text, "Bilgisayar 60 saniye sonra kapanacak.") {
		t.Errorf("countdown:\n%s", n.Text)
	}
	// An unknown language gives English.
	if n := Render("de", m, time.UTC); !strings.Contains(n.Text, "Battery low") {
		t.Errorf("fallback:\n%s", n.Text)
	}
}

// TestTemplatesComplete: each type has a title in each language, and the
// two languages have the same keys.
func TestTemplatesComplete(t *testing.T) {
	types := []string{"ON_BATTERY", "ONLINE", "LOW_BATTERY", "BYPASS_ACTIVE", "AVR_ACTIVE", "UPS_FAULT", "TEST_STARTED", "UPS_SHUTDOWN_ACTIVE",
		"UPS_WARNING_BITS_CHANGED", "OVERLOAD", "OVER_TEMPERATURE", "RUNTIME_LOW", "COMM_LOST", "COMM_RESTORED",
		TypeCountdownStart, TypeCountdownCancel, TypeShutdownRequest, TypeTest}
	for _, l := range Languages {
		tx := templates[l]
		if len(tx.titles) != len(types) {
			t.Errorf("%s: %d titles, want %d", l, len(tx.titles), len(types))
		}
		for _, ty := range types {
			if tx.titles[ty] == "" {
				t.Errorf("%s: no title for %s", l, ty)
			}
		}
		for _, s := range []string{Info, Warning, Critical} {
			if tx.severity[s] == "" {
				t.Errorf("%s: no severity %s", l, s)
			}
		}
	}
}

// TestNoEventData: the text has only the Snapshot fields. Event data (for
// example a serial number) is never in the message.
func TestNoEventData(t *testing.T) {
	m := Message{Type: "UPS_WARNING_BITS_CHANGED", Severity: Info, Items: []Item{{Time: t0, Type: "UPS_WARNING_BITS_CHANGED", Severity: Info, Active: true}}}
	n := Render("en", m, time.UTC)
	if strings.Count(n.Text, "\n") != 1 {
		t.Fatalf("text:\n%s", n.Text)
	}
}
