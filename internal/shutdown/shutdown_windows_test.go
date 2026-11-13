// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package shutdown

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

// TestReasonCode checks the constant against Microsoft Learn, "System
// Shutdown Reason Codes".
func TestReasonCode(t *testing.T) {
	if ReasonMajorPower != 0x00060000 || ReasonMinorEnvironment != 0x0000000c {
		t.Fatal("reason code parts changed")
	}
	if ReasonPowerFailure != 0x0006000c {
		t.Fatalf("reason 0x%08x, want 0x0006000c", ReasonPowerFailure)
	}
	if ReasonPowerFailure&0x80000000 != 0 {
		t.Fatal("SHTDN_REASON_FLAG_PLANNED must not be set")
	}
}

type winRec struct {
	priv, suspend int
	shut          []struct {
		msg     string
		timeout uint32
		force   bool
		reboot  bool
		reason  uint32
	}
}

// fakeWin returns fake Windows functions. They never call Windows.
func fakeWin(r *winRec, allowed bool, suspendErr error) winCalls {
	return winCalls{
		enablePrivilege: func() error { r.priv++; return nil },
		initiateShutdown: func(m string, to uint32, f, rb bool, rs uint32) error {
			r.shut = append(r.shut, struct {
				msg     string
				timeout uint32
				force   bool
				reboot  bool
				reason  uint32
			}{m, to, f, rb, rs})
			return nil
		},
		hibernateAllowed: func() bool { return allowed },
		setSuspendState:  func(bool) error { r.suspend++; return suspendErr },
	}
}

func TestWindowsShutdown(t *testing.T) {
	for _, force := range []bool{true, false} {
		r := &winRec{}
		a := &fakeAudit{}
		e := New(Options{Audit: a, ForceClose: func() bool { return force }})
		e.os = &winOS{c: fakeWin(r, true, nil)}
		if err := e.Shutdown(context.Background(), "UPS power failure"); err != nil {
			t.Fatal(err)
		}
		if r.priv != 1 || len(r.shut) != 1 {
			t.Fatalf("priv %d, shutdown calls %d", r.priv, len(r.shut))
		}
		s := r.shut[0]
		if s.msg != "UPS power failure" || s.timeout != 0 || s.force != force || s.reboot || s.reason != 0x0006000c {
			t.Fatalf("call %+v", s)
		}
		if len(a.entries) != 1 || a.entries[0].action != "os_shutdown_request" || a.entries[0].details["reason_code"] != "0x0006000c" {
			t.Fatalf("audit %+v", a.entries)
		}
	}
}

func TestWindowsPrivilegeFails(t *testing.T) {
	r := &winRec{}
	c := fakeWin(r, true, nil)
	c.enablePrivilege = func() error { return errors.New("no privilege") }
	e := New(Options{})
	e.os = &winOS{c: c}
	if err := e.Shutdown(context.Background(), "x"); err == nil || len(r.shut) != 0 {
		t.Fatal("shutdown without the privilege")
	}
}

func TestWindowsHibernate(t *testing.T) {
	r := &winRec{}
	a := &fakeAudit{}
	e := New(Options{Audit: a})
	e.os = &winOS{c: fakeWin(r, true, nil)}
	if err := e.Hibernate(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if r.suspend != 1 || len(r.shut) != 0 {
		t.Fatalf("suspend %d shutdown %d", r.suspend, len(r.shut))
	}
}

func TestWindowsHibernateFallback(t *testing.T) {
	for _, c := range []struct {
		allowed bool
		err     error
	}{{false, nil}, {true, errors.New("fail")}} {
		r := &winRec{}
		a := &fakeAudit{}
		e := New(Options{Audit: a})
		e.os = &winOS{c: fakeWin(r, c.allowed, c.err)}
		if err := e.Hibernate(context.Background(), "x"); err != nil {
			t.Fatal(err)
		}
		if len(r.shut) != 1 {
			t.Fatalf("%+v: no fallback shutdown", c)
		}
		if len(a.entries) != 2 || a.entries[0].action != "os_hibernate_fallback" {
			t.Fatalf("%+v: audit %+v", c, a.entries)
		}
	}
}

// TestPrivilegeResult: a nonzero return with ERROR_NOT_ALL_ASSIGNED is an
// error (Microsoft Learn, AdjustTokenPrivileges).
func TestPrivilegeResult(t *testing.T) {
	if err := privilegeResult(1, windows.ERROR_SUCCESS); err != nil {
		t.Fatal(err)
	}
	if err := privilegeResult(1, windows.ERROR_NOT_ALL_ASSIGNED); !errors.Is(err, errNotAllAssigned) {
		t.Fatalf("not all assigned: %v", err)
	}
	if err := privilegeResult(0, windows.ERROR_ACCESS_DENIED); err == nil {
		t.Fatal("zero return without an error")
	}
}

func TestWindowsCanShutdown(t *testing.T) {
	r := &winRec{}
	e := New(Options{})
	e.os = &winOS{c: fakeWin(r, true, nil)}
	if err := e.CanShutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.priv != 1 || len(r.shut) != 0 {
		t.Fatalf("priv %d shut %d, want a privilege check and no shutdown", r.priv, len(r.shut))
	}
}

func TestWindowsCanShutdownNoPrivilege(t *testing.T) {
	c := fakeWin(&winRec{}, true, nil)
	c.enablePrivilege = func() error { return errNotAllAssigned }
	e := New(Options{})
	e.os = &winOS{c: c}
	err := e.CanShutdown(context.Background())
	if !errors.Is(err, ErrCannotShutdown) {
		t.Fatalf("%v", err)
	}
	if err.Error() != "This user cannot shut down the computer." {
		t.Fatalf("text %q", err.Error())
	}
}
