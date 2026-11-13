// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package shutdown

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// systemd-logind D-Bus names (org.freedesktop.login1(5)).
const (
	login1Dest        = "org.freedesktop.login1"
	login1Path        = dbus.ObjectPath("/org/freedesktop/login1")
	login1Manager     = "org.freedesktop.login1.Manager"
	methodList        = login1Manager + ".ListInhibitors"
	methodCanHib      = login1Manager + ".CanHibernate"
	methodCanPowerOff = login1Manager + ".CanPowerOff"
	methodPowerOff    = login1Manager + ".PowerOff"
	methodHibernate   = login1Manager + ".Hibernate"
)

// busObject is the part of dbus.BusObject that the executor uses. Tests
// give a fake object; the fake never calls D-Bus.
type busObject interface {
	CallWithContext(ctx context.Context, method string, flags dbus.Flags, args ...interface{}) *dbus.Call
}

// linuxOS connects to the system bus for each call.
type linuxOS struct {
	connect func() (busObject, func(), error)
}

func defaultOS() osAPI {
	return &linuxOS{connect: func() (busObject, func(), error) {
		c, err := dbus.ConnectSystemBus()
		if err != nil {
			return nil, nil, fmt.Errorf("shutdown: system D-Bus: %w", err)
		}
		return c.Object(login1Dest, login1Path), func() { c.Close() }, nil
	}}
}

// Inhibitor is one entry of ListInhibitors: a(ssssuu).
type Inhibitor struct {
	What string
	Who  string
	Why  string
	Mode string
	UID  uint32
	PID  uint32
}

func listInhibitors(ctx context.Context, o busObject) ([]Inhibitor, error) {
	var out []Inhibitor
	if err := o.CallWithContext(ctx, methodList, 0).Store(&out); err != nil {
		return nil, fmt.Errorf("ListInhibitors: %w", err)
	}
	return out, nil
}

func inhibitorData(l []Inhibitor, err error) any {
	if err != nil {
		return "error: " + err.Error()
	}
	out := make([]map[string]any, 0, len(l))
	for _, i := range l {
		out = append(out, map[string]any{"what": i.What, "who": i.Who, "why": i.Why, "mode": i.Mode, "uid": i.UID, "pid": i.PID})
	}
	return out
}

func (l *linuxOS) call(ctx context.Context, e *Executor, action, method, reason string, extra map[string]any) error {
	o, closeFn, err := l.connect()
	if err != nil {
		e.record(ctx, "os_"+action+"_request", "failure", map[string]any{"error": err.Error()})
		return err
	}
	defer closeFn()
	return l.callOn(ctx, o, e, action, method, reason, extra)
}

func (l *linuxOS) callOn(ctx context.Context, o busObject, e *Executor, action, method, reason string, extra map[string]any) error {
	inh, ierr := listInhibitors(ctx, o)
	d := map[string]any{"action": action, "message": reason, "inhibitors": inhibitorData(inh, ierr)}
	for k, v := range extra {
		d[k] = v
	}
	// interactive=false: polkit does not ask a user (50-vltd.rules).
	if err := o.CallWithContext(ctx, method, 0, false).Err; err != nil {
		d["error"] = err.Error()
		e.record(ctx, "os_"+action+"_request", "failure", d)
		return fmt.Errorf("shutdown: %s: %w", method, err)
	}
	e.record(ctx, "os_"+action+"_request", "success", d)
	return nil
}

func (l *linuxOS) shutdown(ctx context.Context, e *Executor, reason string) error {
	return l.call(ctx, e, ActionShutdown, methodPowerOff, reason, nil)
}

func (l *linuxOS) hibernate(ctx context.Context, e *Executor, reason string) error {
	o, closeFn, err := l.connect()
	if err != nil {
		e.record(ctx, "os_hibernate_request", "failure", map[string]any{"error": err.Error()})
		return err
	}
	defer closeFn()
	var can string
	cerr := o.CallWithContext(ctx, methodCanHib, 0).Store(&can)
	if cerr != nil || can != "yes" {
		why := "CanHibernate: " + can
		if cerr != nil {
			why = "CanHibernate: " + cerr.Error()
		}
		e.record(ctx, "os_hibernate_fallback", "success", map[string]any{"reason": why, "action": ActionShutdown})
		return l.callOn(ctx, o, e, ActionShutdown, methodPowerOff, reason, map[string]any{"fallback_from": ActionHibernate})
	}
	return l.callOn(ctx, o, e, ActionHibernate, methodHibernate, reason, nil)
}

func (l *linuxOS) canShutdown(ctx context.Context, _ *Executor) error {
	o, closeFn, err := l.connect()
	if err != nil {
		return ErrCannotShutdown
	}
	defer closeFn()
	var can string
	if err := o.CallWithContext(ctx, methodCanPowerOff, 0).Store(&can); err != nil {
		return ErrCannotShutdown
	}
	// "yes": permitted. "challenge" would need a polkit prompt; portable
	// mode does not install a polkit rule and uses interactive=false, so
	// a challenge would fail. "no" and "na" are a refusal.
	if can != "yes" {
		return ErrCannotShutdown
	}
	return nil
}
