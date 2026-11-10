// SPDX-License-Identifier: AGPL-3.0-or-later

package dialect

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MaxResponseLen is the maximum length of a response, CR included.
const MaxResponseLen = 256

// ErrNotSupported is returned when the UPS answers "NAK", "(NAK" or an echo
// of the request. The result is "not supported".
var ErrNotSupported = errors.New("dialect: not supported")

// ParseError is a parser error. Field is the 1-based field index. Field 0
// means the frame (length, start character, end character or field count).
type ParseError struct {
	Command string
	Field   int
	Reason  string
}

func (e *ParseError) Error() string {
	if e.Field == 0 {
		return fmt.Sprintf("dialect: %s: frame: %s", e.Command, e.Reason)
	}
	return fmt.Sprintf("dialect: %s: field %d: %s", e.Command, e.Field, e.Reason)
}

func frameErr(cmd, format string, a ...any) error {
	return &ParseError{Command: cmd, Field: 0, Reason: fmt.Sprintf(format, a...)}
}

func fieldErr(cmd string, idx int, format string, a ...any) error {
	return &ParseError{Command: cmd, Field: idx, Reason: fmt.Sprintf(format, a...)}
}

// frame checks the general frame rules and returns the body: the bytes after
// the start character and before the CR.
func frame(cmd string, resp []byte, start byte) (string, error) {
	if len(resp) > MaxResponseLen {
		return "", frameErr(cmd, "length %d is more than %d", len(resp), MaxResponseLen)
	}
	if len(resp) == 0 {
		return "", frameErr(cmd, "empty response")
	}
	if resp[len(resp)-1] != '\r' {
		return "", frameErr(cmd, "no CR at the end")
	}
	body := resp[:len(resp)-1]
	if bytes.IndexByte(body, '\r') >= 0 {
		return "", frameErr(cmd, "CR before the end")
	}
	if isNotSupported(cmd, body) {
		return "", ErrNotSupported
	}
	for i, c := range body {
		if c < 0x20 || c > 0x7e {
			return "", frameErr(cmd, "byte %d is not printable ASCII", i)
		}
	}
	if len(body) == 0 || body[0] != start {
		return "", frameErr(cmd, "start character is not %q", start)
	}
	return string(body[1:]), nil
}

// isNotSupported detects "NAK", "(NAK" and an echo of the request.
func isNotSupported(cmd string, body []byte) bool {
	s := string(body)
	return s == "NAK" || s == "(NAK" || s == cmd
}

// pattern characters for a field:
//
//	d: one decimal digit
//	b: one bit character, "0" or "1"
//	.: a period
type fieldSpec struct {
	patterns []string // permitted patterns; all have the same length
}

func spec(p ...string) fieldSpec { return fieldSpec{patterns: p} }

// noValue reports whether a field has only "-", "@" or "#" characters.
// A period at a pattern period position is permitted (example "---.-").
func noValue(f string, pattern string) bool {
	if len(f) != len(pattern) || len(f) == 0 {
		return false
	}
	seen := false
	for i := 0; i < len(f); i++ {
		c := f[i]
		switch {
		case c == '.' && pattern[i] == '.':
		case c == '-' || c == '@' || c == '#':
			seen = true
		default:
			return false
		}
	}
	return seen
}

func matches(f, pattern string) bool {
	if len(f) != len(pattern) {
		return false
	}
	for i := 0; i < len(f); i++ {
		c := f[i]
		switch pattern[i] {
		case 'd':
			if c < '0' || c > '9' {
				return false
			}
		case 'b':
			if c != '0' && c != '1' {
				return false
			}
		default:
			if c != pattern[i] {
				return false
			}
		}
	}
	return true
}

// check validates one field. It returns valid=false for a no-value field.
func (s fieldSpec) check(cmd string, idx int, f string) (valid bool, err error) {
	for _, p := range s.patterns {
		if matches(f, p) {
			return true, nil
		}
	}
	for _, p := range s.patterns {
		if noValue(f, p) {
			return false, nil
		}
	}
	return false, fieldErr(cmd, idx, "value %q does not match %s", f, strings.Join(s.patterns, " or "))
}

// splitFields splits the body at single space characters and checks the
// field count.
func splitFields(cmd, body string, n int) ([]string, error) {
	fields := strings.Split(body, " ")
	if len(fields) != n {
		return nil, frameErr(cmd, "field count %d, expected %d", len(fields), n)
	}
	for i, f := range fields {
		if f == "" {
			return nil, fieldErr(cmd, i+1, "empty field")
		}
	}
	return fields, nil
}

func quantity[U Unit](cmd string, idx int, f string, s fieldSpec) (Quantity[U], error) {
	valid, err := s.check(cmd, idx, f)
	if err != nil {
		return Quantity[U]{}, err
	}
	q := Quantity[U]{Raw: f, Valid: valid}
	if valid {
		v, perr := strconv.ParseFloat(f, 64)
		if perr != nil {
			return Quantity[U]{}, fieldErr(cmd, idx, "number format: %q", f)
		}
		q.Value = v
	}
	return q, nil
}

func bitsField(cmd string, idx int, f string, n int) (string, error) {
	if len(f) != n {
		return "", fieldErr(cmd, idx, "bit field width %d, expected %d", len(f), n)
	}
	for i := 0; i < len(f); i++ {
		if f[i] != '0' && f[i] != '1' {
			return "", fieldErr(cmd, idx, "bit field %q has a character that is not 0 or 1", f)
		}
	}
	return f, nil
}
