// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import (
	"strconv"
	"strings"
)

// Fixed-position parsers of the megatec Dialect (VLT-IS-006 TASK
// 03-00-00-003 step 2). Source: VLT-REF-001 section 6. The frame lengths,
// separator positions and fields are STATIC, PUBLIC. The selection of the
// response type by the first byte and the CR position is STATIC only:
// UNVERIFIED.

// Frame lengths with the CR.
const (
	Q1Len = 47 // VLT-REF-001 6.1
	FLen  = 22 // VLT-REF-001 6.2
	ILen  = 39 // VLT-REF-001 6.3
)

// span is one field: offset and length in the response, 1-based index.
type span struct{ off, n int }

var (
	// VLT-REF-001 6.1. Spaces at 6, 12, 18, 22, 27, 32 and 37.
	q1Spans  = []span{{1, 5}, {7, 5}, {13, 5}, {19, 3}, {23, 4}, {28, 4}, {33, 4}, {38, 8}}
	q1Spaces = []int{6, 12, 18, 22, 27, 32, 37}
	// VLT-REF-001 6.2. Spaces at 6, 10 and 16.
	fSpans  = []span{{1, 5}, {7, 3}, {11, 5}, {17, 4}}
	fSpaces = []int{6, 10, 16}
)

// fixedFrame checks the general frame rules (frame), the exact length and
// the separator positions. It returns the response without the CR.
func fixedFrame(cmd string, resp []byte, start byte, n int, spaces []int) (string, error) {
	if _, err := frame(cmd, resp, start); err != nil {
		return "", err
	}
	if len(resp) != n {
		return "", frameErr(cmd, "length %d with the CR, expected %d", len(resp), n)
	}
	for _, p := range spaces {
		if resp[p] != ' ' {
			return "", frameErr(cmd, "byte %d is not a space", p)
		}
	}
	return string(resp[:n-1]), nil
}

// isNumber reports whether s is digits with an optional decimal part:
// "004", "4.3", "024.0", "24.00".
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	digits := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			digits++
		case c == '.' && !dot && i > 0 && i < len(s)-1:
			dot = true
		default:
			return false
		}
	}
	return digits > 0
}

// noValueChars reports whether s has only "-", "@", "#" and "." and at
// least one of "-", "@" or "#" (for example "---.-").
func noValueChars(s string) bool {
	seen := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '-', '@', '#':
			seen = true
		case '.':
		default:
			return false
		}
	}
	return seen
}

// fixedQuantity parses one numeric field. Leading spaces are permitted
// (VLT-REF-001 6.1 NOTE, STATIC only: UNVERIFIED). A field with only spaces, or with only
// "-", "@" or "#" characters, has no value.
func fixedQuantity[U Unit](cmd string, idx int, raw string) (Quantity[U], error) {
	f := strings.TrimLeft(raw, " ")
	q := Quantity[U]{Raw: raw}
	switch {
	case f == "":
		return q, nil
	case noValueChars(f):
		return q, nil
	case !isNumber(f):
		return Quantity[U]{}, fieldErr(cmd, idx, "value %q is not a number", raw)
	}
	v, err := strconv.ParseFloat(f, 64)
	if err != nil {
		return Quantity[U]{}, fieldErr(cmd, idx, "number format: %q", raw)
	}
	q.Value, q.Valid = v, true
	return q, nil
}

func field(b string, s span) string { return b[s.off : s.off+s.n] }

// ParseQ1 parses the response of Q1: exactly 47 bytes with the CR
// (VLT-REF-001 6.1).
func ParseQ1(resp []byte) (Status, error) {
	const cmd = "Q1"
	b, err := fixedFrame(cmd, resp, '(', Q1Len, q1Spaces)
	if err != nil {
		return Status{}, err
	}
	s := Status{Command: cmd}
	if s.InputVoltage, err = fixedQuantity[unitVolt](cmd, 1, field(b, q1Spans[0])); err != nil {
		return Status{}, err
	}
	if s.InputFaultVolt, err = fixedQuantity[unitVolt](cmd, 2, field(b, q1Spans[1])); err != nil {
		return Status{}, err
	}
	if s.OutputVoltage, err = fixedQuantity[unitVolt](cmd, 3, field(b, q1Spans[2])); err != nil {
		return Status{}, err
	}
	if s.Load, err = fixedQuantity[unitPercent](cmd, 4, field(b, q1Spans[3])); err != nil {
		return Status{}, err
	}
	if s.InputFrequency, err = fixedQuantity[unitHertz](cmd, 5, field(b, q1Spans[4])); err != nil {
		return Status{}, err
	}
	bv := field(b, q1Spans[5])
	if s.BatteryVoltage, err = fixedQuantity[unitVolt](cmd, 6, bv); err != nil {
		return Status{}, err
	}
	switch t := strings.TrimLeft(bv, " "); {
	case matches(t, "d.dd"):
		s.BatteryForm = "S.SS"
	case matches(t, "dd.d"):
		s.BatteryForm = "SS.S"
	}
	if s.Temperature, err = fixedQuantity[unitCelsius](cmd, 7, field(b, q1Spans[6])); err != nil {
		return Status{}, err
	}
	raw, err := bitsField(cmd, 8, field(b, q1Spans[7]), 8)
	if err != nil {
		return Status{}, err
	}
	s.Bits = StatusBits{Raw: raw}
	s.Flags = q1Flags(s.Bits)
	return s, nil
}

// ParseF parses the response of F: exactly 22 bytes with the CR
// (VLT-REF-001 6.2). The number format in a field can change, for example
// "004" or "4.3" (VLT-REF-001 6.2 NOTE, STATIC only: UNVERIFIED).
func ParseF(resp []byte) (Rating, error) {
	const cmd = "F"
	b, err := fixedFrame(cmd, resp, '#', FLen, fSpaces)
	if err != nil {
		return Rating{}, err
	}
	r := Rating{Command: cmd}
	if r.Voltage, err = fixedQuantity[unitVolt](cmd, 1, field(b, fSpans[0])); err != nil {
		return Rating{}, err
	}
	if r.Current, err = fixedQuantity[unitAmpere](cmd, 2, field(b, fSpans[1])); err != nil {
		return Rating{}, err
	}
	if r.BatteryVoltage, err = fixedQuantity[unitVolt](cmd, 3, field(b, fSpans[2])); err != nil {
		return Rating{}, err
	}
	if r.Frequency, err = fixedQuantity[unitHertz](cmd, 4, field(b, fSpans[3])); err != nil {
		return Rating{}, err
	}
	return r, nil
}

// ParseI parses the response of I: exactly 39 bytes with the CR, spaces at
// 16 and 27 (VLT-REF-001 6.3). Spaces on the right of each field are
// removed. A field with only spaces has no value (empty string).
func ParseI(resp []byte) (Information, error) {
	const cmd = "I"
	b, err := fixedFrame(cmd, resp, '#', ILen, []int{1 + iCompanyWidth, 1 + iCompanyWidth + 1 + iModelWidth})
	if err != nil {
		return Information{}, err
	}
	c := b[1 : 1+iCompanyWidth]
	m := b[1+iCompanyWidth+1 : 1+iCompanyWidth+1+iModelWidth]
	v := b[1+iCompanyWidth+1+iModelWidth+1:]
	return Information{
		Company: strings.TrimRight(c, " "),
		Model:   strings.TrimRight(m, " "),
		Version: strings.TrimRight(v, " "),
	}, nil
}
