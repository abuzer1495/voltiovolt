// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"strings"
)

// Lang is the user-visible text of vlt-simulator.
type Lang struct {
	Code        string
	Title       string
	AlreadyRuns string
	Connected   string
	Waiting     string
	On          string
	Off         string
	Auto        string
	ForcedOn    string
	Online      string
	Standby     string
	QuitAsk     string
	Yes         rune
	No          rune
	Keys        string
	LogTitle    string
	// Menu labels, in item order.
	Menu [18]string
	// Status panel.
	StInput, StFault, StOutput, StFreq, StLoad, StCharge, StBatt, StTemp, StRuntime, StBits, StOutputOn, StTimers string
	BitUtilFail, BitUtilOK, BitBattLow, BitBattOK, BitBypass, BitFault, BitTest, BitShut, BitBeep                 string
	None, Test, Shutdown, Restore                                                                                 string
	// Fault injection and other-reply labels.
	FaultNone, FaultNoResp, FaultGarbage, FaultSplit, FaultWrong, FaultOld string
	OtherEcho, OtherNAK, OtherNone                                         string
	Scenario                                                               [10]string
}

func langEN() Lang {
	return Lang{
		Code: "en", Title: "VoltioVolt UPS Simulator",
		AlreadyRuns: "The simulator already runs",
		Connected:   "connected", Waiting: "waiting",
		On: "on", Off: "off", Auto: "auto", ForcedOn: "forced on",
		Online: "on-line", Standby: "standby",
		QuitAsk: "Quit? [Y/N]", Yes: 'Y', No: 'N',
		Keys:     "Up/Down: select  Left/Right: +/- 1  PgUp/PgDn: +/- 10  Enter/Space: on/off  1-9: scenario  R: reset  L: log  Q: quit",
		LogTitle: "Full log  (L to close, Up/Down to scroll)",
		Menu: [18]string{
			"Mains power", "Input voltage", "Input frequency", "Output voltage (on mains)",
			"Load", "Battery charge", "Battery speed", "Temperature", "UPS type",
			"Battery low (b6)", "UPS fault (b4)", "Bypass or AVR (b5)", "Beeper (b0)",
			"Dialect", "Reply to other commands", "Response delay", "Fault injection",
			"Rated values (F)",
		},
		StInput: "Input voltage", StFault: "Input fault", StOutput: "Output voltage",
		StFreq: "Frequency", StLoad: "Load", StCharge: "Battery charge",
		StBatt: "Battery voltage", StTemp: "Temperature", StRuntime: "Runtime",
		StBits: "Status bits", StOutputOn: "Output", StTimers: "Timers",
		BitUtilFail: "utility fail", BitUtilOK: "utility ok",
		BitBattLow: "battery low", BitBattOK: "battery ok",
		BitBypass: "bypass/AVR", BitFault: "UPS fault",
		BitTest: "test", BitShut: "shutdown", BitBeep: "beeper",
		None: "none", Test: "test", Shutdown: "shutdown", Restore: "restore",
		FaultNone: "none", FaultNoResp: "no response", FaultGarbage: "garbage",
		FaultSplit: "split", FaultWrong: "wrong length", FaultOld: "old bytes",
		OtherEcho: "echo", OtherNAK: "NAK", OtherNone: "none",
		Scenario: [10]string{
			"",
			"Power failure now",
			"Power failure for 2 minutes, then mains on",
			"Low input voltage: 180 V",
			"High input voltage: 260 V",
			"Overload: 110 %",
			"Battery low: charge 15 % and mains off",
			"Communication loss for 30 seconds",
			"High temperature: 55 C",
			"Back to normal",
		},
	}
}

func langTR() Lang {
	return Lang{
		Code: "tr", Title: "VoltioVolt UPS Simülatörü",
		AlreadyRuns: "Simülatör zaten çalışıyor",
		Connected:   "bağlı", Waiting: "bekliyor",
		On: "açık", Off: "kapalı", Auto: "otomatik", ForcedOn: "zorunlu açık",
		Online: "on-line", Standby: "yedekleme",
		QuitAsk: "Çıkılsın mı? [E/H]", Yes: 'E', No: 'H',
		Keys:     "Yukarı/Aşağı: seç  Sol/Sağ: +/- 1  SyfYukarı/Aşağı: +/- 10  Enter/Boşluk: aç/kapa  1-9: senaryo  R: sıfırla  L: günlük  Q: çık",
		LogTitle: "Tam günlük  (L kapatır, Yukarı/Aşağı kaydırır)",
		Menu: [18]string{
			"Şebeke gücü", "Giriş gerilimi", "Giriş frekansı", "Çıkış gerilimi (şebekede)",
			"Yük", "Akü şarjı", "Akü hızı", "Sıcaklık", "UPS tipi",
			"Akü düşük (b6)", "UPS arızası (b4)", "Bypass veya AVR (b5)", "Sesli uyarı (b0)",
			"Diyalekt", "Diğer komutlara yanıt", "Yanıt gecikmesi", "Arıza enjeksiyonu",
			"Anma değerleri (F)",
		},
		StInput: "Giriş gerilimi", StFault: "Giriş arıza", StOutput: "Çıkış gerilimi",
		StFreq: "Frekans", StLoad: "Yük", StCharge: "Akü şarjı",
		StBatt: "Akü gerilimi", StTemp: "Sıcaklık", StRuntime: "Süre",
		StBits: "Durum bitleri", StOutputOn: "Çıkış", StTimers: "Zamanlayıcılar",
		BitUtilFail: "şebeke yok", BitUtilOK: "şebeke var",
		BitBattLow: "akü düşük", BitBattOK: "akü iyi",
		BitBypass: "bypass/AVR", BitFault: "UPS arızası",
		BitTest: "test", BitShut: "kapatma", BitBeep: "sesli uyarı",
		None: "yok", Test: "test", Shutdown: "kapatma", Restore: "geri açma",
		FaultNone: "yok", FaultNoResp: "yanıt yok", FaultGarbage: "çöp",
		FaultSplit: "bölünmüş", FaultWrong: "yanlış uzunluk", FaultOld: "eski baytlar",
		OtherEcho: "yankı", OtherNAK: "NAK", OtherNone: "yok",
		Scenario: [10]string{
			"",
			"Şimdi elektrik kesintisi",
			"2 dakikalık kesinti, sonra şebeke açık",
			"Düşük giriş gerilimi: 180 V",
			"Yüksek giriş gerilimi: 260 V",
			"Aşırı yük: % 110",
			"Akü düşük: şarj % 15 ve şebeke kapalı",
			"30 saniyelik haberleşme kaybı",
			"Yüksek sıcaklık: 55 C",
			"Normal değerlere dönüş",
		},
	}
}

func lookupLang(code string) Lang {
	if strings.EqualFold(code, "tr") {
		return langTR()
	}
	return langEN()
}

func defaultLangCode() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(k)
		if v == "" || v == "C" || strings.HasPrefix(v, "C.") || v == "POSIX" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(v), "tr") {
			return "tr"
		}
		return "en"
	}
	return defaultLangOS()
}
