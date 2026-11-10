// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package shutdown

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
)

// Shutdown reason code (Microsoft Learn, "System Shutdown Reason Codes",
// https://learn.microsoft.com/en-us/windows/win32/shutdown/system-shutdown-reason-codes):
// SHTDN_REASON_MAJOR_POWER (0x00060000, "Power failure") with
// SHTDN_REASON_MINOR_ENVIRONMENT (0x0000000c). The system knows this
// combination as "Power Failure: Environment" ("There was a power outage").
// No SHTDN_REASON_FLAG_PLANNED: a power failure is not planned.
const (
	ReasonMajorPower       = 0x00060000
	ReasonMinorEnvironment = 0x0000000c
	ReasonPowerFailure     = ReasonMajorPower | ReasonMinorEnvironment // 0x0006000c
)

var (
	modpowrprof               = windows.NewLazySystemDLL("powrprof.dll")
	procSetSuspendState       = modpowrprof.NewProc("SetSuspendState")
	procIsPwrHibernateAllowed = modpowrprof.NewProc("IsPwrHibernateAllowed")
)

// winCalls are the Windows functions. Tests replace them.
type winCalls struct {
	enablePrivilege  func() error
	initiateShutdown func(message string, timeout uint32, force, reboot bool, reason uint32) error
	hibernateAllowed func() bool
	setSuspendState  func(hibernate bool) error
}

type winOS struct{ c winCalls }

func defaultOS() osAPI {
	return &winOS{c: winCalls{
		enablePrivilege:  enableShutdownPrivilege,
		initiateShutdown: initiateShutdown,
		hibernateAllowed: hibernateAllowed,
		setSuspendState:  setSuspendState,
	}}
}

// enableShutdownPrivilege enables SE_SHUTDOWN_NAME in the process token
// (AdjustTokenPrivileges).
func enableShutdownPrivilege() error {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &tok); err != nil {
		return fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer tok.Close()
	name, err := windows.UTF16PtrFromString("SeShutdownPrivilege") // SE_SHUTDOWN_NAME
	if err != nil {
		return err
	}
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		return fmt.Errorf("LookupPrivilegeValue: %w", err)
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	if err := windows.AdjustTokenPrivileges(tok, false, &tp, 0, nil, nil); err != nil {
		return fmt.Errorf("AdjustTokenPrivileges: %w", err)
	}
	// AdjustTokenPrivileges succeeds with ERROR_NOT_ALL_ASSIGNED when the
	// token does not have the privilege. x/sys/windows returns it as an
	// error, so a nil error means that the privilege is enabled.
	return nil
}

func initiateShutdown(message string, timeout uint32, force, reboot bool, reason uint32) error {
	m, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return err
	}
	if err := windows.InitiateSystemShutdownEx(nil, m, timeout, force, reboot, reason); err != nil {
		return fmt.Errorf("InitiateSystemShutdownEx: %w", err)
	}
	return nil
}

func hibernateAllowed() bool {
	if procIsPwrHibernateAllowed.Find() != nil {
		return false
	}
	r, _, _ := procIsPwrHibernateAllowed.Call()
	return r != 0
}

func setSuspendState(hibernate bool) error {
	if err := procSetSuspendState.Find(); err != nil {
		return err
	}
	h := uintptr(0)
	if hibernate {
		h = 1
	}
	// SetSuspendState(bHibernate, bForce, bWakeupEventsDisabled).
	r, _, e := procSetSuspendState.Call(h, 0, 0)
	if r == 0 {
		return fmt.Errorf("SetSuspendState: %w", e)
	}
	return nil
}

func (w *winOS) shutdown(ctx context.Context, e *Executor, reason string) error {
	force := e.forceClose()
	d := map[string]any{"action": ActionShutdown, "reason_code": fmt.Sprintf("0x%08x", ReasonPowerFailure), "force_close_applications": force, "message": reason}
	if err := w.c.enablePrivilege(); err != nil {
		d["error"] = err.Error()
		e.record(ctx, "os_shutdown_request", "failure", d)
		return err
	}
	// Timeout 0: no dialog box, no AbortSystemShutdown. No restart.
	if err := w.c.initiateShutdown(reason, 0, force, false, ReasonPowerFailure); err != nil {
		d["error"] = err.Error()
		e.record(ctx, "os_shutdown_request", "failure", d)
		return err
	}
	e.record(ctx, "os_shutdown_request", "success", d)
	return nil
}

func (w *winOS) hibernate(ctx context.Context, e *Executor, reason string) error {
	if err := w.c.enablePrivilege(); err != nil {
		e.record(ctx, "os_hibernate_request", "failure", map[string]any{"error": err.Error()})
		return err
	}
	if !w.c.hibernateAllowed() {
		e.record(ctx, "os_hibernate_fallback", "success", map[string]any{"reason": "hibernation not available", "action": ActionShutdown})
		return w.shutdown(ctx, e, reason)
	}
	if err := w.c.setSuspendState(true); err != nil {
		e.record(ctx, "os_hibernate_fallback", "success", map[string]any{"reason": "SetSuspendState failed", "error": err.Error(), "action": ActionShutdown})
		return w.shutdown(ctx, e, reason)
	}
	e.record(ctx, "os_hibernate_request", "success", map[string]any{"action": ActionHibernate, "message": reason})
	return nil
}
