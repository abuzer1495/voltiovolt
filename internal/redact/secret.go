// SPDX-License-Identifier: AGPL-3.0-or-later

// Package redact contains the Secret type and the text redaction rules
// (VLT-IS-003 TASK 03-00-00-002).
package redact

import (
	"fmt"
	"log/slog"
)

// Redacted is the text that replaces a Secret value in all output.
const Redacted = "[REDACTED]"

// box holds the bytes behind a pointer. fmt prints a nested pointer as an
// address only, so a Secret in an unexported struct field does not show its
// bytes either.
type box struct {
	b []byte
}

// Secret holds Secret bytes. All text forms of a Secret are "[REDACTED]".
// The zero value is an empty Secret.
type Secret struct {
	p *box
}

// New returns a Secret with a copy of b.
func New(b []byte) Secret {
	c := make([]byte, len(b))
	copy(c, b)
	return Secret{p: &box{b: c}}
}

// NewString returns a Secret with the bytes of s.
func NewString(s string) Secret {
	return New([]byte(s))
}

// Expose returns the Secret bytes. The caller must not keep or log them.
//
// Use only in the Vault and in Channel code.
func (s Secret) Expose() []byte {
	if s.p == nil {
		return nil
	}
	return s.p.b
}

// IsZero reports whether the Secret has no bytes.
func (s Secret) IsZero() bool {
	return s.p == nil || len(s.p.b) == 0
}

// Len returns the number of bytes. The length is not a Secret.
func (s Secret) Len() int {
	if s.p == nil {
		return 0
	}
	return len(s.p.b)
}

// Wipe sets all bytes to zero.
func (s Secret) Wipe() {
	if s.p == nil {
		return
	}
	for i := range s.p.b {
		s.p.b[i] = 0
	}
}

// String returns "[REDACTED]".
func (Secret) String() string { return Redacted }

// GoString returns "[REDACTED]".
func (Secret) GoString() string { return Redacted }

// Format writes "[REDACTED]" for all verbs and flags.
func (Secret) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(Redacted))
}

// MarshalJSON returns the JSON string "[REDACTED]".
func (Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"` + Redacted + `"`), nil
}

// MarshalText returns "[REDACTED]".
func (Secret) MarshalText() ([]byte, error) {
	return []byte(Redacted), nil
}

// LogValue returns "[REDACTED]".
func (Secret) LogValue() slog.Value {
	return slog.StringValue(Redacted)
}

var (
	_ fmt.Stringer   = Secret{}
	_ fmt.GoStringer = Secret{}
	_ fmt.Formatter  = Secret{}
	_ slog.LogValuer = Secret{}
)
