// SPDX-License-Identifier: AGPL-3.0-or-later

// Package control makes the CONTROL commands of the megatec, voltronic-qs
// and voltronic Dialects (VLT-IS-006 TASK 03-00-00-005, SEC-098, SEC-100).
//
// WARNING: CONTROL commands can cut power to the protected equipment.
//
// Rules:
//  1. Only this package makes CONTROL bytes (SEC-098). The go/ast test
//     TestOnlyControlMakesControlBytes enforces it.
//  2. Each encoder needs a Grant. Only two functions make a Grant:
//     IPCGrant (an IPC request in the elevated state with the confirmation
//     text) and PolicyGrant (the Policy Engine).
//  3. There is no encoder for a shutdown without restore. VLT never sends
//     "S.0" (SEC-100, VLT-REF-001 5.1 item 2).
//  4. An IPC Grant permits only the test, cancel test and beeper functions.
//     The shutdown and restore command needs the Policy Engine Grant. The
//     cancel shutdown command "C" (PUBLIC only, UNVERIFIED) is never sent
//     automatically and no IPC method sends it.
package control

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

// Func is one CONTROL function.
type Func string

// CONTROL functions (VLT-REF-001 section 5).
const (
	FuncTest10s         Func = "test.10s"         // T CR. STATIC, PUBLIC.
	FuncTestMinutes     Func = "test.minutes"     // "T" + nn CR. STATIC, PUBLIC.
	FuncTestUntilLow    Func = "test.until_low"   // TL CR. STATIC, PUBLIC.
	FuncCancelTest      Func = "test.cancel"      // CT CR. STATIC, PUBLIC.
	FuncBeeper          Func = "beeper"           // Q CR (toggle). STATIC, PUBLIC.
	FuncShutdownRestore Func = "shutdown.restore" // S...R.... CR (5.1). STATIC, PUBLIC.
	FuncCancelShutdown  Func = "shutdown.cancel"  // C CR. PUBLIC only, UNVERIFIED.
)

// Confirmation texts of the IPC requests (VLT-IS-006 TASK 03-00-00-007).
const (
	ConfirmStartTest    = "START TEST"
	ConfirmTestUntilLow = "TEST UNTIL LOW"
)

// Errors.
var (
	ErrNoGrant      = errors.New("control: no valid Grant")
	ErrNotPermitted = errors.New("control: the Grant does not permit this function")
	ErrNotElevated  = errors.New("control: the elevated state is necessary")
	ErrConfirmation = errors.New("control: the confirmation text is not correct")
	ErrRange        = errors.New("control: value out of range")
	ErrBytes        = errors.New("control: bytes are not a permitted CONTROL command")
)

// Source is the origin of a Grant.
type Source string

// Grant sources.
const (
	SourceIPC    Source = "ipc"
	SourcePolicy Source = "policy"
)

// Grant authorizes one CONTROL request. The zero value is not valid. Only
// IPCGrant and PolicyGrant make a valid Grant.
type Grant struct {
	src Source
	fn  Func // the one function that an IPC Grant permits
}

// Source returns the source of the Grant.
func (g Grant) Source() Source { return g.src }

// IPCGrant makes a Grant for an IPC request. Only internal/ipc calls it.
// The elevated state is necessary. The test functions need the
// confirmation text: "START TEST" for the 10 second and minutes test,
// "TEST UNTIL LOW" for the test until battery low. Cancel test and beeper
// need no text. An IPC Grant never permits the shutdown functions.
func IPCGrant(elevated bool, f Func, confirmation string) (Grant, error) {
	if !elevated {
		return Grant{}, ErrNotElevated
	}
	want, ok := ipcConfirm[f]
	if !ok {
		return Grant{}, fmt.Errorf("%w: %s", ErrNotPermitted, f)
	}
	if confirmation != want {
		return Grant{}, ErrConfirmation
	}
	return Grant{src: SourceIPC, fn: f}, nil
}

// ipcConfirm lists the functions of an IPC Grant and their confirmation.
var ipcConfirm = map[Func]string{
	FuncTest10s:      ConfirmStartTest,
	FuncTestMinutes:  ConfirmStartTest,
	FuncTestUntilLow: ConfirmTestUntilLow,
	FuncCancelTest:   "",
	FuncBeeper:       "",
}

// PolicyGrant makes the Grant of the Policy Engine (the UPS output off
// step). Only internal/policy calls it.
func PolicyGrant() Grant { return Grant{src: SourcePolicy} }

func (g Grant) check(f Func) error {
	switch g.src {
	case SourceIPC:
		if g.fn == f {
			return nil
		}
	case SourcePolicy:
		if f == FuncShutdownRestore || f == FuncCancelShutdown {
			return nil
		}
	default:
		return ErrNoGrant
	}
	return fmt.Errorf("%w: %s grant, %s", ErrNotPermitted, g.src, f)
}

// Command is the bytes of one CONTROL command. Only the encoders of this
// package make a Command.
type Command struct {
	fn  Func
	b   []byte
	src Source
}

// Func returns the function.
func (c Command) Func() Func { return c.fn }

// Source returns the source of the Grant that made the command.
func (c Command) Source() Source { return c.src }

// Bytes returns a copy of the bytes, CR included.
func (c Command) Bytes() []byte { return append([]byte(nil), c.b...) }

// Text returns the bytes without CR, for the audit log.
func (c Command) Text() string { return strings.TrimSuffix(string(c.b), "\r") }

// Valid reports whether the command came from an encoder and its bytes
// pass Check.
func (c Command) Valid() bool { return c.fn != "" && c.src != "" && Check(c.b) == nil }

func make1(g Grant, f Func, s string) (Command, error) {
	if err := g.check(f); err != nil {
		return Command{}, err
	}
	b := []byte(s + "\r")
	if err := Check(b); err != nil {
		return Command{}, err
	}
	return Command{fn: f, b: b, src: g.src}, nil
}

// EncodeTest10s makes "T" CR.
func EncodeTest10s(g Grant) (Command, error) { return make1(g, FuncTest10s, "T") }

// EncodeTestMinutes makes "T" + n with 2 digits + CR, n = 1 to 99.
func EncodeTestMinutes(g Grant, n int) (Command, error) {
	if n < 1 || n > 99 {
		return Command{}, fmt.Errorf("%w: test minutes %d (1 to 99)", ErrRange, n)
	}
	return make1(g, FuncTestMinutes, fmt.Sprintf("T%02d", n))
}

// EncodeTestUntilLow makes "TL" CR.
func EncodeTestUntilLow(g Grant) (Command, error) { return make1(g, FuncTestUntilLow, "TL") }

// EncodeCancelTest makes "CT" CR.
func EncodeCancelTest(g Grant) (Command, error) { return make1(g, FuncCancelTest, "CT") }

// EncodeBeeperToggle makes "Q" CR. The UPS changes the beeper from on to
// off or from off to on. Use Controller.SetBeeper: it reads b0 first.
func EncodeBeeperToggle(g Grant) (Command, error) { return make1(g, FuncBeeper, "Q") }

// Delay limits of SEC-100.
const (
	MinShortDelay = 12 * time.Second
	MaxShortDelay = 54 * time.Second
	ShortStep     = 6 * time.Second
	MinLongDelay  = time.Minute
	MaxLongDelay  = 10 * time.Minute
	MinRestore    = 1
	MaxRestore    = 9999
)

// ValidDelay reports whether d is a SEC-100 delay: 12 to 54 seconds in
// steps of 6 seconds (.2 to .9 minutes), or 1 to 10 whole minutes.
func ValidDelay(d time.Duration) bool {
	if d >= MinShortDelay && d <= MaxShortDelay {
		return d%ShortStep == 0
	}
	return d >= MinLongDelay && d <= MaxLongDelay && d%time.Minute == 0
}

// EncodeShutdownRestore makes the shutdown and restore command of
// VLT-REF-001 5.1. delay is a SEC-100 delay. restore is 1 to 9999 minutes.
//
//	delay < 60 s:  "S." + one digit (delay / 6 s) + "R" + restore with 4 digits + CR
//	delay >= 60 s: "S" + minutes with 2 digits + "R" + restore with 4 digits + CR
//
// There is no restore value 0: VLT always sends a restore time, and it
// never sends "S.0".
func EncodeShutdownRestore(g Grant, delay time.Duration, restore int) (Command, error) {
	if !ValidDelay(delay) {
		return Command{}, fmt.Errorf("%w: delay %s (SEC-100: 12 to 54 s in steps of 6 s, or 1 to 10 min)", ErrRange, delay)
	}
	if restore < MinRestore || restore > MaxRestore {
		return Command{}, fmt.Errorf("%w: restore %d (1 to 9999 minutes)", ErrRange, restore)
	}
	var s string
	if delay < time.Minute {
		s = fmt.Sprintf("S.%dR%04d", int(delay/ShortStep), restore)
	} else {
		s = fmt.Sprintf("S%02dR%04d", int(delay/time.Minute), restore)
	}
	return make1(g, FuncShutdownRestore, s)
}

// EncodeCancelShutdown makes "C" CR. PUBLIC only, UNVERIFIED: UPSilon 2000
// never sends it. VLT never sends it automatically, and no IPC method
// sends it.
func EncodeCancelShutdown(g Grant) (Command, error) { return make1(g, FuncCancelShutdown, "C") }

// Check returns nil only for the exact bytes of a permitted CONTROL
// command. It rejects "S.0", a shutdown without restore, a restore of 0
// and each delay outside SEC-100. The Transport calls it again before a
// write.
func Check(b []byte) error {
	if len(b) < 2 || b[len(b)-1] != '\r' || strings.IndexByte(string(b[:len(b)-1]), '\r') >= 0 {
		return ErrBytes
	}
	s := string(b[:len(b)-1])
	switch s {
	case "T", "TL", "CT", "Q", "C":
		return nil
	}
	if len(s) == 3 && s[0] == 'T' && isDigits(s[1:]) && s != "T00" {
		return nil
	}
	if strings.HasPrefix(s, "S.0") {
		return fmt.Errorf("%w: S.0 (SEC-100)", ErrBytes)
	}
	if len(s) == 8 && s[0] == 'S' && s[3] == 'R' && isDigits(s[4:]) && s[4:] != "0000" {
		d := s[1:3]
		switch {
		case d[0] == '.' && d[1] >= '2' && d[1] <= '9':
			return nil
		case isDigits(d) && d >= "01" && d <= "10":
			return nil
		}
	}
	return ErrBytes
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// Capabilities returns the CONTROL functions that VLT can use with a
// Dialect, for ups.control.capabilities. The shutdown functions are not in
// the list: no IPC method sends them. The bytes are the same in the three
// Dialects (VLT-REF-001 section 5, STATIC, PUBLIC; for voltronic-qs and
// voltronic the support of the Reference UPS is UNVERIFIED).
func Capabilities(d dialect.Dialect) []Func {
	switch d {
	case dialect.DialectMegatec, dialect.DialectVoltronicQS, dialect.DialectVoltronic:
		return []Func{FuncTest10s, FuncTestMinutes, FuncTestUntilLow, FuncCancelTest, FuncBeeper}
	}
	return nil
}
