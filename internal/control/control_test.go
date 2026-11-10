// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

func ipc(t *testing.T, f Func) Grant {
	t.Helper()
	conf := map[Func]string{FuncTest10s: ConfirmStartTest, FuncTestMinutes: ConfirmStartTest, FuncTestUntilLow: ConfirmTestUntilLow}[f]
	g, err := IPCGrant(true, f, conf)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestEncoderVectors: the bytes of each encoder (TASK 03-00-00-005 step 1,
// VLT-REF-001 section 5 and 5.1).
func TestEncoderVectors(t *testing.T) {
	pg := PolicyGrant()
	cases := []struct {
		name string
		enc  func() (Command, error)
		want string
	}{
		{"T", func() (Command, error) { return EncodeTest10s(ipc(t, FuncTest10s)) }, "T\r"},
		{"T01", func() (Command, error) { return EncodeTestMinutes(ipc(t, FuncTestMinutes), 1) }, "T01\r"},
		{"T09", func() (Command, error) { return EncodeTestMinutes(ipc(t, FuncTestMinutes), 9) }, "T09\r"},
		{"T99", func() (Command, error) { return EncodeTestMinutes(ipc(t, FuncTestMinutes), 99) }, "T99\r"},
		{"TL", func() (Command, error) { return EncodeTestUntilLow(ipc(t, FuncTestUntilLow)) }, "TL\r"},
		{"CT", func() (Command, error) { return EncodeCancelTest(ipc(t, FuncCancelTest)) }, "CT\r"},
		{"Q", func() (Command, error) { return EncodeBeeperToggle(ipc(t, FuncBeeper)) }, "Q\r"},
		// VLT-REF-001 5.1 examples with a restore time.
		{"D30 R1", func() (Command, error) { return EncodeShutdownRestore(pg, 30*time.Second, 1) }, "S.5R0001\r"},
		{"D120 R10", func() (Command, error) { return EncodeShutdownRestore(pg, 120*time.Second, 10) }, "S02R0010\r"},
		{"D12 R1", func() (Command, error) { return EncodeShutdownRestore(pg, 12*time.Second, 1) }, "S.2R0001\r"},
		{"D54 R9999", func() (Command, error) { return EncodeShutdownRestore(pg, 54*time.Second, 9999) }, "S.9R9999\r"},
		{"D60 R1", func() (Command, error) { return EncodeShutdownRestore(pg, time.Minute, 1) }, "S01R0001\r"},
		{"D300 R1", func() (Command, error) { return EncodeShutdownRestore(pg, 5*time.Minute, 1) }, "S05R0001\r"},
		{"D600 R100", func() (Command, error) { return EncodeShutdownRestore(pg, 10*time.Minute, 100) }, "S10R0100\r"},
		{"C", func() (Command, error) { return EncodeCancelShutdown(pg) }, "C\r"},
	}
	for _, c := range cases {
		cmd, err := c.enc()
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if string(cmd.Bytes()) != c.want || !cmd.Valid() {
			t.Errorf("%s: %q, want %q", c.name, cmd.Bytes(), c.want)
		}
	}
}

// TestShutdownDelaySteps: each SEC-100 delay below 60 s gives ".2" to ".9".
func TestShutdownDelaySteps(t *testing.T) {
	for i, d := range []int{12, 18, 24, 30, 36, 42, 48, 54} {
		cmd, err := EncodeShutdownRestore(PolicyGrant(), time.Duration(d)*time.Second, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := "S." + string(rune('2'+i)) + "R0001\r"
		if string(cmd.Bytes()) != want {
			t.Errorf("D=%d: %q, want %q", d, cmd.Bytes(), want)
		}
	}
}

// TestShutdownNegative: the negative vectors. The UPSilon 2000 defect
// values D = 57, 58, 59 (VLT-REF-001 5.1 item 2), delays outside SEC-100,
// restore 0 and restore more than 9999 are refused.
func TestShutdownNegative(t *testing.T) {
	pg := PolicyGrant()
	for _, d := range []time.Duration{0, time.Second, 6 * time.Second, 11 * time.Second, 13 * time.Second, 57 * time.Second, 58 * time.Second, 59 * time.Second, 90 * time.Second, 11 * time.Minute, 99 * time.Minute, 5940 * time.Second, -time.Minute} {
		if _, err := EncodeShutdownRestore(pg, d, 1); !errors.Is(err, ErrRange) {
			t.Errorf("delay %s: %v, want ErrRange", d, err)
		}
	}
	for _, r := range []int{0, -1, 10000} {
		if _, err := EncodeShutdownRestore(pg, time.Minute, r); !errors.Is(err, ErrRange) {
			t.Errorf("restore %d: %v, want ErrRange", r, err)
		}
	}
	for _, n := range []int{0, -1, 100} {
		if _, err := EncodeTestMinutes(ipc(t, FuncTestMinutes), n); !errors.Is(err, ErrRange) {
			t.Errorf("test minutes %d: %v", n, err)
		}
	}
}

// TestNeverSendsS0 (SEC-100): no input of the shutdown encoder gives
// "S.0" or a shutdown without "R". Check refuses "S.0" and each shutdown
// without restore.
func TestNeverSendsS0(t *testing.T) {
	pg := PolicyGrant()
	for ds := -10; ds <= 6000; ds++ {
		for _, r := range []int{-1, 0, 1, 2, 10, 9999, 10000} {
			cmd, err := EncodeShutdownRestore(pg, time.Duration(ds)*time.Second, r)
			if err != nil {
				continue
			}
			b := string(cmd.Bytes())
			if strings.HasPrefix(b, "S.0") || !strings.Contains(b, "R") || strings.HasSuffix(b, "R0000\r") {
				t.Fatalf("D=%d R=%d: %q", ds, r, b)
			}
		}
	}
	for _, b := range []string{"S.0\r", "S.0R0001\r", "S.5\r", "S05\r", "S.1R0001\r", "S00R0001\r", "S11R0001\r", "S.5R0000\r", "S\r", "S.5R001\r"} {
		if err := Check([]byte(b)); err == nil {
			t.Errorf("Check accepted %q", b)
		}
	}
}

// TestNoShutdownWithoutRestore (SEC-100): the package has no encoder for
// a shutdown without restore. The exported encoders are exactly this list.
func TestNoShutdownWithoutRestore(t *testing.T) {
	got := exportedFuncs(t, "Encode")
	want := []string{"EncodeBeeperToggle", "EncodeCancelShutdown", "EncodeCancelTest", "EncodeShutdownRestore", "EncodeTest10s", "EncodeTestMinutes", "EncodeTestUntilLow"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("encoders %v, want %v", got, want)
	}
}

func TestCheck(t *testing.T) {
	for _, b := range []string{"T\r", "T01\r", "T99\r", "TL\r", "CT\r", "Q\r", "C\r", "S.2R0001\r", "S.9R9999\r", "S01R0001\r", "S10R9999\r"} {
		if err := Check([]byte(b)); err != nil {
			t.Errorf("%q: %v", b, err)
		}
	}
	for _, b := range []string{"", "\r", "T", "T00\r", "T100\r", "Q1\r", "QS\r", "F\r", "I\r", "Q\rQ\r", "TL", "CT\r\r", "S.2R0001", "X\r"} {
		if err := Check([]byte(b)); err == nil {
			t.Errorf("accepted %q", b)
		}
	}
}

// TestGrants: the IPC Grant needs the elevated state and the confirmation
// text, permits only its function and never the shutdown functions. The
// zero Grant permits nothing.
func TestGrants(t *testing.T) {
	if _, err := IPCGrant(false, FuncTest10s, ConfirmStartTest); !errors.Is(err, ErrNotElevated) {
		t.Errorf("not elevated: %v", err)
	}
	if _, err := IPCGrant(true, FuncTest10s, "start test"); !errors.Is(err, ErrConfirmation) {
		t.Errorf("confirmation: %v", err)
	}
	if _, err := IPCGrant(true, FuncTestUntilLow, ConfirmStartTest); !errors.Is(err, ErrConfirmation) {
		t.Errorf("until low with START TEST: %v", err)
	}
	for _, f := range []Func{FuncShutdownRestore, FuncCancelShutdown, "x"} {
		if _, err := IPCGrant(true, f, ""); !errors.Is(err, ErrNotPermitted) {
			t.Errorf("IPC grant %s: %v", f, err)
		}
	}
	g := ipc(t, FuncTest10s)
	if _, err := EncodeShutdownRestore(g, time.Minute, 1); !errors.Is(err, ErrNotPermitted) {
		t.Errorf("IPC grant shutdown: %v", err)
	}
	if _, err := EncodeCancelShutdown(g); !errors.Is(err, ErrNotPermitted) {
		t.Errorf("IPC grant C: %v", err)
	}
	if _, err := EncodeTestUntilLow(g); !errors.Is(err, ErrNotPermitted) {
		t.Errorf("IPC grant of T for TL: %v", err)
	}
	if _, err := EncodeTest10s(PolicyGrant()); !errors.Is(err, ErrNotPermitted) {
		t.Errorf("policy grant test: %v", err)
	}
	var zero Grant
	for _, enc := range []func() (Command, error){
		func() (Command, error) { return EncodeTest10s(zero) },
		func() (Command, error) { return EncodeBeeperToggle(zero) },
		func() (Command, error) { return EncodeShutdownRestore(zero, time.Minute, 1) },
	} {
		if _, err := enc(); !errors.Is(err, ErrNoGrant) {
			t.Errorf("zero grant: %v", err)
		}
	}
	var c Command
	if c.Valid() {
		t.Error("zero Command is valid")
	}
}

func TestCapabilities(t *testing.T) {
	for _, d := range []dialect.Dialect{dialect.DialectMegatec, dialect.DialectVoltronicQS, dialect.DialectVoltronic} {
		got := Capabilities(d)
		if len(got) != 5 {
			t.Errorf("%s: %v", d, got)
		}
		for _, f := range got {
			if f == FuncShutdownRestore || f == FuncCancelShutdown {
				t.Errorf("%s: capability %s", d, f)
			}
		}
	}
	if Capabilities("other") != nil {
		t.Error("unknown Dialect has capabilities")
	}
}
