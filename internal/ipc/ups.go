// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"errors"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// UPSControl is the UPS Control part of vltd (VLT-IS-006 TASK 03-00-00-007,
// *control.Controller). It has no shutdown function: no IPC method sends
// the shutdown and restore command or the cancel shutdown command
// (SEC-098, test TestNoIPCShutdown in internal/control).
type UPSControl interface {
	Capabilities() []control.Func
	LastTest() (control.TestResult, bool)
	StartTest(ctx context.Context, g control.Grant, kind control.Func, minutes int) (control.Result, error)
	CancelTest(ctx context.Context, g control.Grant) (control.Result, error)
	SetBeeper(ctx context.Context, g control.Grant, on bool) (control.Result, error)
}

func upsResult(r control.Result) wire.UPSControlResult {
	return wire.UPSControlResult{Function: string(r.Func), Bytes: r.Bytes, Sends: r.Sends, Verified: r.Verified, NoSend: r.NoSend, DryRun: r.DryRun}
}

// upsError maps a Controller error to a wire error. A not verified
// command gives the result with the error, so the GUI can show the sends.
func upsError(err error) *wire.Error {
	switch {
	case errors.Is(err, control.ErrOnBattery):
		return &wire.Error{Code: wire.CodeOnBattery, Message: "the UPS is on battery"}
	case errors.Is(err, control.ErrNotVerify):
		return &wire.Error{Code: wire.CodeNotVerified, Message: "the UPS status did not confirm the command"}
	case errors.Is(err, control.ErrNotSupport):
		return &wire.Error{Code: wire.CodeNotSupported, Message: "the UPS does not support this function"}
	case errors.Is(err, control.ErrNoLink), errors.Is(err, control.ErrNoStatus):
		return &wire.Error{Code: wire.CodeNoUPS, Message: "no UPS connected"}
	case errors.Is(err, control.ErrBusy):
		return &wire.Error{Code: wire.CodeControlBusy, Message: "another UPS Control request runs"}
	case errors.Is(err, control.ErrConfirmation):
		return invalid("confirmation text is not correct")
	case errors.Is(err, control.ErrRange):
		return &wire.Error{Code: wire.CodeInvalid, Message: err.Error()}
	case errors.Is(err, control.ErrNotElevated):
		return &wire.Error{Code: wire.CodeNotElevated, Message: "elevated state necessary"}
	}
	return &wire.Error{Code: wire.CodeInternal, Message: "UPS Control failed"}
}

func testResult(t control.TestResult) *wire.UPSTestResult {
	return &wire.UPSTestResult{Result: t.Result, Start: t.Start.UTC().Format(time.RFC3339), End: t.End.UTC().Format(time.RFC3339),
		UPSFailed: t.UPSFailed, BatteryLow: t.BatteryLow, Cancelled: t.Cancelled}
}

// dispatchUPS serves the UPS Control methods. Each Grant is made here, in
// the elevated state with the confirmation text (SEC-098).
func (cn *conn) dispatchUPS(ctx context.Context, req wire.Request, elev bool) (any, *wire.Error) {
	b := cn.s.b
	switch req.Method {
	case wire.MethodUPSControlCaps:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		if b.UPS == nil {
			return nil, unavailable("UPS Control")
		}
		caps := b.UPS.Capabilities()
		res := wire.UPSControlCapsResult{Connected: caps != nil, Functions: make([]string, 0, len(caps))}
		for _, f := range caps {
			res.Functions = append(res.Functions, string(f))
		}
		if t, ok := b.UPS.LastTest(); ok {
			res.LastTest = testResult(t)
		}
		return res, nil

	case wire.MethodUPSTestStart:
		var p wire.UPSTestStartParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if b.UPS == nil {
			return nil, unavailable("UPS Control")
		}
		var kind control.Func
		switch p.Type {
		case wire.TestType10s:
			kind = control.FuncTest10s
		case wire.TestTypeMinutes:
			kind = control.FuncTestMinutes
		case wire.TestTypeUntilLow:
			kind = control.FuncTestUntilLow
		default:
			return nil, invalid("type must be 10s, minutes or until_low")
		}
		d := map[string]any{"type": p.Type, "minutes": p.Minutes}
		// Step 1: no test when a countdown runs or the UPS is on battery.
		if b.Policy != nil && b.Policy.Countdown().Active {
			d["reason"] = "countdown active"
			cn.record(ctx, "ups_test_start", string(kind), "rejected", d)
			return nil, &wire.Error{Code: wire.CodeCountdown, Message: "a shutdown countdown runs"}
		}
		if b.Device != nil {
			if st := b.Device.Status(); st.State != "" && st.State != device.StateConnected {
				d["reason"] = "no UPS connected"
				cn.record(ctx, "ups_test_start", string(kind), "rejected", d)
				return nil, &wire.Error{Code: wire.CodeNoUPS, Message: "no UPS connected"}
			}
		}
		g, err := control.IPCGrant(elev, kind, p.Confirmation)
		if err != nil {
			d["reason"] = err.Error()
			cn.record(ctx, "ups_test_start", string(kind), "rejected", d)
			return nil, upsError(err)
		}
		r, err := b.UPS.StartTest(ctx, g, kind, p.Minutes)
		if err != nil {
			d["reason"] = err.Error()
			cn.record(ctx, "ups_test_start", string(kind), "failure", d)
			return nil, upsError(err)
		}
		d["bytes"], d["sends"], d["verified"] = r.Bytes, r.Sends, r.Verified
		cn.record(ctx, "ups_test_start", string(kind), "success", d)
		return upsResult(r), nil

	case wire.MethodUPSTestCancel:
		if err := wire.Strict(req.Params, &wire.NoParams{}); err != nil {
			return nil, bad(err)
		}
		if b.UPS == nil {
			return nil, unavailable("UPS Control")
		}
		g, err := control.IPCGrant(elev, control.FuncCancelTest, "")
		if err != nil {
			return nil, upsError(err)
		}
		r, err := b.UPS.CancelTest(ctx, g)
		if err != nil {
			cn.record(ctx, "ups_test_cancel", string(control.FuncCancelTest), "failure", map[string]any{"reason": err.Error()})
			return nil, upsError(err)
		}
		cn.record(ctx, "ups_test_cancel", string(control.FuncCancelTest), "success", map[string]any{"sends": r.Sends, "verified": r.Verified})
		return upsResult(r), nil

	case wire.MethodUPSBeeperSet:
		var p wire.UPSBeeperSetParams
		if err := wire.Strict(req.Params, &p); err != nil {
			return nil, bad(err)
		}
		if b.UPS == nil {
			return nil, unavailable("UPS Control")
		}
		g, err := control.IPCGrant(elev, control.FuncBeeper, "")
		if err != nil {
			return nil, upsError(err)
		}
		r, err := b.UPS.SetBeeper(ctx, g, p.On)
		if err != nil {
			cn.record(ctx, "ups_beeper_set", string(control.FuncBeeper), "failure", map[string]any{"on": p.On, "reason": err.Error()})
			return nil, upsError(err)
		}
		cn.record(ctx, "ups_beeper_set", string(control.FuncBeeper), "success", map[string]any{"on": p.On, "sends": r.Sends, "no_send": r.NoSend, "verified": r.Verified})
		return upsResult(r), nil
	}
	return nil, &wire.Error{Code: wire.CodeUnknownMethod, Message: "unknown method"}
}
