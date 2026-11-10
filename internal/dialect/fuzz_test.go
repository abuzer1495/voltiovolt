// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import "testing"

// Each fuzz target checks that the parser never panics.

func seed(f *testing.F, cmd string) {
	f.Add(cr(vectorsA[cmd]))
	f.Add(cr("NAK"))
	f.Add(cr(cmd))
	f.Add([]byte{})
}

func FuzzParseQ1(f *testing.F) {
	seed(f, "Q1")
	f.Add(cr(vectorBQ1))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQ1(b) })
}

func FuzzParseQS(f *testing.F) {
	seed(f, "QS")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQS(b) })
}

func FuzzParseQGS(f *testing.F) {
	seed(f, "QGS")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQGS(b) })
}

func FuzzParseQBV(f *testing.F) {
	seed(f, "QBV")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQBV(b) })
}

func FuzzParseQMOD(f *testing.F) {
	seed(f, "QMOD")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQMOD(b) })
}

func FuzzParseQWS(f *testing.F) {
	seed(f, "QWS")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQWS(b) })
}

func FuzzParseQFLAG(f *testing.F) {
	seed(f, "QFLAG")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQFLAG(b) })
}

func FuzzParseQRI(f *testing.F) {
	seed(f, "QRI")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQRI(b) })
}

func FuzzParseF(f *testing.F) {
	seed(f, "F")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseF(b) })
}

func FuzzParseI(f *testing.F) {
	f.Add(cr(syntheticI))
	f.Add(cr("NAK"))
	f.Add(cr("I"))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseI(b) })
}

func FuzzParseQID(f *testing.F) {
	seed(f, "QID")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQID(b) })
}

func FuzzParseQVFW(f *testing.F) {
	seed(f, "QVFW")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQVFW(b) })
}

func FuzzParseQMD(f *testing.F) {
	seed(f, "QMD")
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseQMD(b) })
}
