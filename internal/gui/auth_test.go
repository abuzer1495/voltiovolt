// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// TestFirstStartSetPIN is TASK 03-00-00-003 step 2: no PIN; SetPIN, then
// status.subscribe.
func TestFirstStartSetPIN(t *testing.T) {
	f := newFake(t)
	var pinSet atomic.Bool
	f.handle(wire.MethodAuthStatus, func(json.RawMessage) (any, *wire.Error) {
		return wire.AuthStatusResult{PINSet: pinSet.Load()}, nil
	})
	f.handle(wire.MethodSubscribe, func(json.RawMessage) (any, *wire.Error) {
		if !pinSet.Load() {
			return nil, &wire.Error{Code: wire.CodePINNotSet, Message: "set a PIN first"}
		}
		return wire.OKResult{OK: true}, nil
	})
	f.handle(wire.MethodSetPIN, func(p json.RawMessage) (any, *wire.Error) {
		var x wire.PINParams
		_ = json.Unmarshal(p, &x)
		if x.PIN == "111111" {
			return nil, &wire.Error{Code: wire.CodeInvalid, Message: "auth: PIN is too simple"}
		}
		pinSet.Store(true)
		return wire.OKResult{OK: true}, nil
	})
	_, a, _ := start(t, f)
	if st, err := a.AuthStatus(); err != nil || st.PINSet {
		t.Fatalf("%+v %v", st, err)
	}
	if err := a.SetPIN("111111"); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatalf("weak PIN: %v", err)
	}
	if err := a.SetPIN("k7Pq2Lm9"); err != nil {
		t.Fatal(err)
	}
	if st, _ := a.AuthStatus(); !st.PINSet {
		t.Fatal("PIN not set")
	}
	n := strings.Count(strings.Join(f.methods(), ","), wire.MethodSubscribe)
	if n < 2 {
		t.Fatalf("subscribe after SetPIN: %v", f.methods())
	}
	if r := a.Rules(); r.Min != 6 || r.Max != 64 {
		t.Fatalf("%+v", r)
	}
}

// TestLoginLockLogout is step 3 and 4: login, block time, the lock at the
// end of the idle time, the Lock button.
func TestLoginLockLogout(t *testing.T) {
	f := newFake(t)
	var elevated atomic.Bool
	f.handle(wire.MethodAuthStatus, func(json.RawMessage) (any, *wire.Error) {
		r := wire.AuthStatusResult{PINSet: true, Elevated: elevated.Load()}
		if !r.Elevated {
			r.BlockUntil = "2026-11-10T12:00:00Z"
		}
		return r, nil
	})
	f.handle(wire.MethodLogin, func(p json.RawMessage) (any, *wire.Error) {
		var x wire.PINParams
		_ = json.Unmarshal(p, &x)
		if x.PIN != "k7Pq2Lm9" {
			return nil, &wire.Error{Code: wire.CodePINIncorrect, Message: "PIN incorrect"}
		}
		elevated.Store(true)
		return wire.OKResult{OK: true}, nil
	})
	f.handle(wire.MethodLogout, func(json.RawMessage) (any, *wire.Error) { elevated.Store(false); return wire.OKResult{OK: true}, nil })
	f.handle(wire.MethodSettingsList, func(json.RawMessage) (any, *wire.Error) {
		return wire.SettingsListResult{Settings: []wire.Setting{{Key: "auth.session_idle_minutes", Value: json.RawMessage("5")}}}, nil
	})
	m, a, rec := start(t, f)
	if err := a.Login("wrong1"); err == nil || err.Error() != wire.CodePINIncorrect {
		t.Fatalf("wrong PIN: %v", err)
	}
	if st, _ := a.AuthStatus(); st.BlockUntil == "" {
		t.Fatal("no block time")
	}
	if err := a.Login("k7Pq2Lm9"); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	idle := m.idle
	m.mu.Unlock()
	if idle != 5*time.Minute {
		t.Fatalf("idle %v", idle)
	}
	if st, _ := a.AuthStatus(); !st.Elevated {
		t.Fatal("not elevated")
	}
	// The end of the idle time: vltd ends the elevated state; the timer
	// asks auth.status and the GUI locks.
	elevated.Store(false)
	m.mu.Lock()
	m.idle = 10 * time.Millisecond
	m.mu.Unlock()
	m.armLock()
	waitFor(t, "locked by the idle time", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return !m.auth.Elevated
	})
	if !rec.find(EventAuth, func(d any) bool { return !d.(AuthInfo).Elevated }) {
		t.Fatal("no auth event")
	}
	// Lock button.
	if err := a.Login("k7Pq2Lm9"); err != nil {
		t.Fatal(err)
	}
	if err := a.Logout(); err != nil {
		t.Fatal(err)
	}
	if st, _ := a.AuthStatus(); st.Elevated {
		t.Fatal("elevated after Logout")
	}
}

// TestNotElevatedLocks: a "not_elevated" answer locks the GUI.
func TestNotElevatedLocks(t *testing.T) {
	f := newFake(t)
	f.handle(wire.MethodStatus, func(json.RawMessage) (any, *wire.Error) {
		return nil, &wire.Error{Code: wire.CodeNotElevated, Message: "enter the PIN first"}
	})
	m, a, _ := start(t, f)
	m.setElevated(true)
	if _, err := a.SystemStatus(); err == nil || err.Error() != wire.CodeNotElevated {
		t.Fatal(err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.auth.Elevated {
		t.Fatal("still elevated")
	}
}
