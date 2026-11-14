// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// ownerPIN is a digits PIN for the Owner sequence of VLT-IS-011 TASK
// 03-00-00-001 step 1. The instruction example "4826" is shorter than the
// PIN rules (minimum 6, VLT-IS-003), so the test uses 6 digits.
const ownerPIN = "482619"

func mustElevated(t *testing.T, a *API, step string) {
	t.Helper()
	st, err := a.AuthStatus()
	if err != nil {
		t.Fatalf("%s: auth.status: %v", step, err)
	}
	if !st.PINSet || !st.Elevated {
		t.Fatalf("%s: want PIN set and elevated (no lock screen), got %+v", step, st)
	}
}

func notLocked(t *testing.T, step string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var e *Error
	if errors.As(err, &e) && (e.Code == wire.CodeNotElevated || e.Code == wire.CodePINNotSet) {
		t.Fatalf("%s: a PIN request (lock screen): %v", step, err)
	}
	t.Fatalf("%s: %v", step, err)
}

// ownerSequence is steps 1.2 to 1.7 after a fresh start (step 1.1).
// setPIN is the place where the PIN is set (wizard or General screen):
// both call the same API.SetPIN.
func ownerSequence(t *testing.T, a *API) {
	t.Helper()
	// 1.2 Set a PIN.
	if err := a.SetPIN(ownerPIN); err != nil {
		t.Fatalf("1.2 set PIN: %v", err)
	}
	// 1.3 "Next": the port step opens without the lock screen (OD-50).
	mustElevated(t, a, "1.3 after set PIN")
	// 1.4 Test connection (simulator), save: device.settings.set.
	notLocked(t, "1.4 device.settings.set", a.SetDeviceSettings(map[string]any{"device.transport": device.TransportTest}))
	// 1.5 Shutdown Policy step: Simulation, then activation.
	raw, err := a.Simulate()
	notLocked(t, "1.5 policy.simulate", err)
	var sim struct {
		Passed bool `json:"passed"`
	}
	_ = json.Unmarshal(raw, &sim)
	if err := a.ActivatePolicy(); err != nil {
		var e *Error
		// Activation can be refused for a reason of the policy (for
		// example no_simulation). It must not be refused for the lock.
		if errors.As(err, &e) && (e.Code == wire.CodeNotElevated || e.Code == wire.CodePINNotSet) {
			t.Fatalf("1.5 policy.activate: a PIN request (lock screen): %v", err)
		}
		t.Logf("1.5 policy.activate refused for a policy reason (not the lock): %v (simulation passed=%v)", err, sim.Passed)
	}
	// 1.6 Complete: the Dashboard opens, unlocked.
	mustElevated(t, a, "1.6 after the wizard")
	// 1.7 Lock, then unlock with the PIN.
	if err := a.Logout(); err != nil {
		t.Fatalf("1.7 Lock: %v", err)
	}
	st, err := a.AuthStatus()
	if err != nil || st.Elevated {
		t.Fatalf("1.7 after Lock: %+v err=%v", st, err)
	}
	if err := a.Login(ownerPIN); err != nil {
		t.Fatalf("1.7 Unlock with %s: %v", ownerPIN, err)
	}
	mustElevated(t, a, "1.7 after Unlock")
}

// TestOwnerSequenceWizardPIN is VLT-IS-011 TASK 03-00-00-001 step 1 (R-08):
// a new empty data folder, the wizard sets the first PIN, and the next
// wizard steps need no PIN.
func TestOwnerSequenceWizardPIN(t *testing.T) {
	_, _, a, stop := startPortableGUI(t, "") // 1.1 new empty data folder
	defer stop()
	st, err := a.AuthStatus()
	if err != nil || st.PINSet || !st.Elevated {
		t.Fatalf("1.1 fresh start: want no PIN and elevated (OD-37), got %+v err=%v", st, err)
	}
	ownerSequence(t, a)
}

// TestOwnerSequenceGeneralPIN is step 2: the same sequence with the PIN
// set in the General screen (no wizard).
func TestOwnerSequenceGeneralPIN(t *testing.T) {
	_, _, a, stop := startPortableGUI(t, "")
	defer stop()
	ownerSequence(t, a)
}

// TestPINChangeKeepsElevated is OD-50 for a PIN change: the connection
// that changes the PIN keeps the elevated state, and the new PIN unlocks.
func TestPINChangeKeepsElevated(t *testing.T) {
	_, _, a, stop := startPortableGUI(t, "")
	defer stop()
	if err := a.SetPIN(ownerPIN); err != nil {
		t.Fatal(err)
	}
	mustElevated(t, a, "after first PIN")
	const newPIN = "k7Pq2Lm9"
	if err := a.SetPIN(newPIN); err != nil {
		t.Fatalf("change PIN: %v", err)
	}
	mustElevated(t, a, "after PIN change")
	if err := a.Logout(); err != nil {
		t.Fatal(err)
	}
	if err := a.Login(ownerPIN); err == nil {
		t.Fatal("the old PIN must not unlock")
	}
	if err := a.Login(newPIN); err != nil {
		t.Fatalf("new PIN: %v", err)
	}
}

// TestTouchThrottle is OD-52 and TASK 03-00-00-001 step 4: after user
// input the GUI sends auth.touch, at most one time in each 30 seconds, and
// only in the elevated state.
func TestTouchThrottle(t *testing.T) {
	f := allFake(t)
	elevated := false
	f.handle(wire.MethodAuthStatus, func(json.RawMessage) (any, *wire.Error) {
		return wire.AuthStatusResult{PINSet: true, Elevated: elevated, IdleLeft: 600}, nil
	})
	f.handle(wire.MethodAuthTouch, func(json.RawMessage) (any, *wire.Error) {
		return wire.AuthStatusResult{PINSet: true, Elevated: elevated, IdleLeft: 600}, nil
	})
	m, a, _ := start(t, f)
	clock := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	m.mu.Lock()
	m.now = func() time.Time { return clock }
	m.mu.Unlock()
	count := func() int {
		n := 0
		for _, s := range f.methods() {
			if s == wire.MethodAuthTouch {
				n++
			}
		}
		return n
	}
	a.Touch()
	if n := count(); n != 0 {
		t.Fatalf("locked: auth.touch sent %d times, want 0", n)
	}
	elevated = true
	m.setElevated(true)
	a.Touch()
	a.Touch()
	if n := count(); n != 1 {
		t.Fatalf("two inputs in 1 s: auth.touch %d times, want 1", n)
	}
	m.mu.Lock()
	clock = clock.Add(29 * time.Second)
	m.mu.Unlock()
	a.Touch()
	if n := count(); n != 1 {
		t.Fatalf("after 29 s: auth.touch %d times, want 1", n)
	}
	m.mu.Lock()
	clock = clock.Add(2 * time.Second)
	m.mu.Unlock()
	a.Touch()
	if n := count(); n != 2 {
		t.Fatalf("after 31 s: auth.touch %d times, want 2", n)
	}
}
