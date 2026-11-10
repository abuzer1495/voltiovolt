// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestParse(t *testing.T) {
	for _, ok := range []string{"dev-install", "dev-uninstall"} {
		if c, err := parse([]string{ok}); err != nil || c != ok {
			t.Errorf("%s: %q %v", ok, c, err)
		}
	}
	for _, bad := range [][]string{nil, {}, {"install"}, {"dev-install", "-x"}, {"--help"}, {"dev-install", "dev-uninstall"}} {
		if _, err := parse(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"status"}, strings.NewReader(""), &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage") {
		t.Fatalf("code %d, %q", code, errOut.String())
	}
}

func TestRunOtherOS(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Windows tool changes the system; it is not run in tests")
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"dev-install"}, strings.NewReader(""), &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "Windows only") {
		t.Fatalf("code %d, %q", code, errOut.String())
	}
}

func TestRequiredPrivileges(t *testing.T) {
	want := []string{"SeShutdownPrivilege", "SeChangeNotifyPrivilege"}
	if !reflect.DeepEqual(requiredPrivileges, want) {
		t.Fatalf("%v", requiredPrivileges)
	}
	ms := multiString(requiredPrivileges)
	want16 := utf16.Encode([]rune("SeShutdownPrivilege\x00SeChangeNotifyPrivilege\x00\x00"))
	if !reflect.DeepEqual(ms, want16) {
		t.Fatalf("multi-string %v", ms)
	}
	if got := multiString(nil); !reflect.DeepEqual(got, []uint16{0}) {
		t.Fatalf("empty %v", got)
	}
}

func TestRecoveryPlan(t *testing.T) {
	if len(recoveryPlan) != 3 {
		t.Fatalf("%d entries", len(recoveryPlan))
	}
	for i := 0; i < 2; i++ {
		if !recoveryPlan[i].restart || recoveryPlan[i].delay != 5*time.Second {
			t.Errorf("entry %d: %+v", i, recoveryPlan[i])
		}
	}
	if recoveryPlan[2].restart {
		t.Error("restart after the third failure")
	}
}

func TestDataDirSDDL(t *testing.T) {
	got := dataDirSDDL("S-1-5-80-1")
	want := "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;S-1-5-80-1)"
	if got != want {
		t.Fatal(got)
	}
}

func TestAsk(t *testing.T) {
	var out bytes.Buffer
	e := env{in: bufioReader("DELETE\r\n"), out: &out}
	if a := e.ask("? "); a != "DELETE" {
		t.Fatalf("%q", a)
	}
	if a := e.ask("? "); a != "" {
		t.Fatalf("EOF: %q", a)
	}
}
