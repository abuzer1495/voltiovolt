// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"fmt"
	"strings"
	"time"
)

// text is one language of the templates.
type text struct {
	titles    map[string]string
	ended     string // "%s: ended"
	severity  map[string]string
	modes     map[string]string
	time      string
	mode      string
	inputV    string
	load      string
	charge    string
	runtime   string
	minutes   string
	combined  string // "%d times in 60 seconds"
	late      string // "Late message. Original time: %s"
	countdown string // "Shutdown in %d seconds (%s)."
	actions   map[string]string
	test      string
}

var templates = map[string]text{
	"en": {
		titles: map[string]string{
			"ON_BATTERY":               "On battery",
			"ONLINE":                   "Mains power",
			"LOW_BATTERY":              "Battery low",
			"BYPASS_ACTIVE":            "Bypass active",
			"AVR_ACTIVE":               "AVR active",
			"UPS_FAULT":                "UPS fault",
			"TEST_STARTED":             "Battery test",
			"UPS_SHUTDOWN_ACTIVE":      "UPS shutdown active",
			"UPS_WARNING_BITS_CHANGED": "UPS warning bits changed",
			"OVERLOAD":                 "Overload",
			"OVER_TEMPERATURE":         "Over temperature",
			"RUNTIME_LOW":              "Runtime low",
			"COMM_LOST":                "No communication with the UPS",
			"COMM_RESTORED":            "Communication with the UPS restored",
			TypeCountdownStart:         "Shutdown countdown started",
			TypeCountdownCancel:        "Shutdown countdown cancelled",
			TypeShutdownRequest:        "Shutdown requested",
			TypeTest:                   "Test message",
		},
		ended:     "%s: ended",
		severity:  map[string]string{Info: "INFO", Warning: "WARNING", Critical: "CRITICAL"},
		modes:     map[string]string{ModeMains: "Mains power", ModeBattery: "On battery", ModeCommLost: "No communication", ModeUnknown: "Unknown"},
		time:      "Time",
		mode:      "Mode",
		inputV:    "Input voltage",
		load:      "Load",
		charge:    "Battery charge",
		runtime:   "Runtime",
		minutes:   "min",
		combined:  "%d times in 60 seconds:",
		late:      "Late message. Original time: %s",
		countdown: "The computer %s in %d seconds.",
		actions:   map[string]string{"shutdown": "shuts down", "hibernate": "hibernates", "": "shuts down"},
		test:      "This is a test message from VoltioVolt.",
	},
	"tr": {
		titles: map[string]string{
			"ON_BATTERY":               "Akü ile çalışıyor",
			"ONLINE":                   "Şebeke gücü",
			"LOW_BATTERY":              "Akü düşük",
			"BYPASS_ACTIVE":            "Bypass etkin",
			"AVR_ACTIVE":               "AVR etkin",
			"UPS_FAULT":                "UPS arızası",
			"TEST_STARTED":             "Akü testi",
			"UPS_SHUTDOWN_ACTIVE":      "UPS kapatma etkin",
			"UPS_WARNING_BITS_CHANGED": "UPS uyarı bitleri değişti",
			"OVERLOAD":                 "Aşırı yük",
			"OVER_TEMPERATURE":         "Aşırı sıcaklık",
			"RUNTIME_LOW":              "Çalışma süresi düşük",
			"COMM_LOST":                "UPS ile iletişim yok",
			"COMM_RESTORED":            "UPS ile iletişim yeniden kuruldu",
			TypeCountdownStart:         "Kapatma geri sayımı başladı",
			TypeCountdownCancel:        "Kapatma geri sayımı iptal edildi",
			TypeShutdownRequest:        "Kapatma istendi",
			TypeTest:                   "Test mesajı",
		},
		ended:     "%s: sona erdi",
		severity:  map[string]string{Info: "BİLGİ", Warning: "UYARI", Critical: "KRİTİK"},
		modes:     map[string]string{ModeMains: "Şebeke gücü", ModeBattery: "Akü ile çalışıyor", ModeCommLost: "İletişim yok", ModeUnknown: "Bilinmiyor"},
		time:      "Zaman",
		mode:      "Mod",
		inputV:    "Giriş gerilimi",
		load:      "Yük",
		charge:    "Akü doluluğu",
		runtime:   "Çalışma süresi",
		minutes:   "dk",
		combined:  "60 saniyede %d kez:",
		late:      "Geç gönderilen mesaj. Asıl zaman: %s",
		countdown: "Bilgisayar %[2]d saniye sonra %[1]s.",
		actions:   map[string]string{"shutdown": "kapanacak", "hibernate": "hazırda bekletilecek", "": "kapanacak"},
		test:      "Bu, VoltioVolt'tan bir test mesajıdır.",
	},
}

// Languages are the template languages.
var Languages = []string{"en", "tr"}

const timeLayout = "2006-01-02 15:04:05"

func lang(l string) text {
	if t, ok := templates[l]; ok {
		return t
	}
	return templates["en"]
}

func (tx text) title(it Item) string {
	s, ok := tx.titles[it.Type]
	if !ok {
		s = it.Type
	}
	if !it.Active && it.Type != TypeCountdownCancel {
		s = fmt.Sprintf(tx.ended, s)
	}
	return s
}

func num(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(s, ".0")
}

// Render makes the text of a message in the language l ("en" or "tr").
// The subject has the format "[VLT][severity] Event".
func Render(l string, m Message, loc *time.Location) Notification {
	tx := lang(l)
	if loc == nil {
		loc = time.Local
	}
	first := Item{Type: m.Type, Severity: m.Severity, Active: true}
	if len(m.Items) > 0 {
		first = m.Items[0]
	}
	sev := tx.severity[m.Severity]
	if sev == "" {
		sev = m.Severity
	}
	title := tx.title(first)
	var b strings.Builder
	fmt.Fprintf(&b, "[VLT][%s] %s\n", sev, title)
	if m.Late {
		fmt.Fprintf(&b, tx.late+"\n", m.Time().In(loc).Format(timeLayout))
	}
	if len(m.Items) > 1 {
		fmt.Fprintf(&b, tx.combined+"\n", len(m.Items))
		for _, it := range m.Items {
			fmt.Fprintf(&b, "- %s %s\n", it.Time.In(loc).Format(timeLayout), tx.title(it))
		}
	} else {
		fmt.Fprintf(&b, "%s: %s\n", tx.time, m.Time().In(loc).Format(timeLayout))
	}
	if first.Type == TypeCountdownStart {
		fmt.Fprintf(&b, tx.countdown+"\n", tx.actions[first.Action], first.Remaining)
	}
	if first.Type == TypeTest {
		b.WriteString(tx.test + "\n")
	}
	s := m.Snapshot
	if mode, ok := tx.modes[s.Mode]; ok && s.Mode != ModeUnknown {
		fmt.Fprintf(&b, "%s: %s\n", tx.mode, mode)
	}
	if s.InputVoltage != nil {
		fmt.Fprintf(&b, "%s: %s V\n", tx.inputV, num(*s.InputVoltage))
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
	return Notification{
		Subject:  fmt.Sprintf("[VLT][%s] %s", m.Severity, title),
		Text:     strings.TrimRight(b.String(), "\n"),
		Severity: m.Severity,
		Type:     m.Type,
		Time:     m.Time(),
	}
}
