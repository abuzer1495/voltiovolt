// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import "fmt"

// Test vectors of VLT-IS-002 03-50-02. The CR is added by cr().
//
// Source A: Tunçmatik Newtech Pro 1 kVA, first generation (2010). Family
// sample. UNVERIFIED for the Reference UPS.
// https://lists.alioth.debian.org/pipermail/nut-upsuser/2010-February/005856.html
//
// Source B: standby UPS, Q1 over USB.
// https://gist.github.com/ao-kenji/7070940
//
// The QID value is fake. The source value is a real serial number.
var vectorsA = map[string]string{
	"Q1":    "(209.7 209.7 219.7 019 50.0 2.32 23.3 00000001",
	"QS":    "(209.7 209.7 219.7 020 50.0 41.8 23.3 00000001",
	"QGS":   "(209.7 50.0 219.7 50.0 000.8 019 374.9 374.3 041.8 ---.- 023.3 100000000001",
	"QBV":   "(041.8 03 01 100 058",
	"QRI":   "(220.0 004 036.0 50.0",
	"F":     "#220.0 004 036.0 50.0",
	"QMOD":  "(L",
	"QFLAG": "(EpbshcDrovegfjl",
	"QVFW":  "(VERFW:00019.03",
	"QWS":   "(0000000000000000000000000000000000000000000000000000000000000000",
	"QMD":   "(############G1K ###1000 80 1/1 220 220 03 12.0",
	"QID":   "(00000000000000",
	"Q":     "(NAK",
}

const vectorBQ1 = "(103.6 102.2 103.9 024 59.9 13.6 40.1 00101000"

// syntheticI is a synthetic I response made from the field widths of the
// public Megatec document (15, 10 and 10 characters). It is not a capture.
var syntheticI = fmt.Sprintf("#%-15s %-10s %-10s", "COMPANY", "MODEL1", "VER1")

func cr(s string) []byte { return []byte(s + "\r") }
