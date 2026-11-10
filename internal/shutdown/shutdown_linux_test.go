// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package shutdown

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
)

// fakeBus is a fake system bus object. It records the method calls and
// never connects to D-Bus.
type fakeBus struct {
	calls        []string
	args         [][]interface{}
	canHibernate string
	failPowerOff bool
}

func (b *fakeBus) CallWithContext(_ context.Context, method string, _ dbus.Flags, args ...interface{}) *dbus.Call {
	b.calls = append(b.calls, method)
	b.args = append(b.args, args)
	c := &dbus.Call{Method: method, Args: args}
	switch method {
	case methodList:
		// The wire form of a(ssssuu).
		c.Body = []interface{}{[][]interface{}{{"shutdown", "Editor", "Unsaved file", "delay", uint32(1000), uint32(4242)}}}
	case methodCanHib:
		c.Body = []interface{}{b.canHibernate}
	case methodPowerOff:
		if b.failPowerOff {
			c.Err = errors.New("org.freedesktop.DBus.Error.AccessDenied")
		}
	}
	return c
}

func newLinux(b *fakeBus) (*Executor, *fakeAudit) {
	a := &fakeAudit{}
	e := New(Options{Audit: a})
	e.os = &linuxOS{connect: func() (busObject, func(), error) { return b, func() {}, nil }}
	return e, a
}

func TestLinuxPowerOff(t *testing.T) {
	b := &fakeBus{}
	e, a := newLinux(b)
	if err := e.Shutdown(context.Background(), "UPS power failure"); err != nil {
		t.Fatal(err)
	}
	if len(b.calls) != 2 || b.calls[0] != "org.freedesktop.login1.Manager.ListInhibitors" || b.calls[1] != "org.freedesktop.login1.Manager.PowerOff" {
		t.Fatalf("calls %v", b.calls)
	}
	if len(b.args[1]) != 1 || b.args[1][0] != false {
		t.Fatalf("PowerOff args %v, want [false]", b.args[1])
	}
	if len(a.entries) != 1 || a.entries[0].action != "os_shutdown_request" || a.entries[0].result != "success" {
		t.Fatalf("audit %+v", a.entries)
	}
	inh, ok := a.entries[0].details["inhibitors"].([]map[string]any)
	if !ok || len(inh) != 1 || inh[0]["who"] != "Editor" || inh[0]["pid"] != uint32(4242) {
		t.Fatalf("inhibitors in the audit entry: %#v", a.entries[0].details["inhibitors"])
	}
}

func TestLinuxPowerOffFails(t *testing.T) {
	b := &fakeBus{failPowerOff: true}
	e, a := newLinux(b)
	if err := e.Shutdown(context.Background(), "x"); err == nil {
		t.Fatal("no error")
	}
	if len(a.entries) != 1 || a.entries[0].result != "failure" {
		t.Fatalf("audit %+v", a.entries)
	}
}

func TestLinuxHibernate(t *testing.T) {
	b := &fakeBus{canHibernate: "yes"}
	e, a := newLinux(b)
	if err := e.Hibernate(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	want := []string{methodCanHib, methodList, methodHibernate}
	if len(b.calls) != 3 || b.calls[0] != want[0] || b.calls[1] != want[1] || b.calls[2] != want[2] {
		t.Fatalf("calls %v", b.calls)
	}
	if b.args[2][0] != false {
		t.Fatalf("Hibernate args %v", b.args[2])
	}
	if len(a.entries) != 1 || a.entries[0].action != "os_hibernate_request" {
		t.Fatalf("audit %+v", a.entries)
	}
}

func TestLinuxHibernateFallback(t *testing.T) {
	for _, can := range []string{"no", "na", "challenge", ""} {
		b := &fakeBus{canHibernate: can}
		e, a := newLinux(b)
		if err := e.Hibernate(context.Background(), "x"); err != nil {
			t.Fatal(err)
		}
		if b.calls[len(b.calls)-1] != methodPowerOff {
			t.Fatalf("%q: calls %v, want PowerOff last", can, b.calls)
		}
		for _, c := range b.calls {
			if c == methodHibernate {
				t.Fatalf("%q: Hibernate called", can)
			}
		}
		if len(a.entries) != 2 || a.entries[0].action != "os_hibernate_fallback" || a.entries[1].action != "os_shutdown_request" {
			t.Fatalf("%q: audit %+v", can, a.entries)
		}
	}
}
