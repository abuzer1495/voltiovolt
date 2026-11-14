// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// TestPortableNoPINIsElevated is VLT-IS-008 TASK 03-00-00-003: if no PIN
// is set, the in-process connection is elevated at all times. A named
// (or test) connection without a PIN stays refused.
func TestPortableNoPINIsElevated(t *testing.T) {
	ctx := context.Background()
	e := newEnvWith(t, InProcessCheck)
	c := typed.New(e.ln.dial(t))
	as, err := c.AuthStatus(ctx)
	if err != nil || as.PINSet || !as.Elevated {
		t.Fatalf("no PIN in-process: %+v err=%v", as, err)
	}
	if _, err := c.Settings(ctx); err != nil {
		t.Fatalf("settings without PIN: %v", err)
	}
	if _, err := c.Policy(ctx); err != nil {
		t.Fatalf("policy.get without PIN: %v", err)
	}
	if _, err := c.SetSetting(ctx, store.SettingLanguage, "tr"); err != nil {
		t.Fatalf("settings.set without PIN: %v", err)
	}
	// A connection that is not in-process still needs a PIN.
	e2 := newEnv(t)
	ro := typed.New(e2.ln.dial(t))
	if _, err := ro.Settings(ctx); client.Code(err) != wire.CodePINNotSet {
		t.Fatalf("service no PIN: %v", err)
	}
}

func TestPortablePINSetNeedsLogin(t *testing.T) {
	ctx := context.Background()
	e := newEnvWith(t, InProcessCheck)
	c := typed.New(e.ln.dial(t))
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	// OD-50 (VLT-IS-011, R-08): the connection that sets the PIN keeps
	// the elevated state.
	as, err := c.AuthStatus(ctx)
	if err != nil || !as.PINSet || !as.Elevated {
		t.Fatalf("after set: %+v err=%v", as, err)
	}
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	as, err = c.AuthStatus(ctx)
	if err != nil || !as.PINSet || as.Elevated {
		t.Fatalf("after set and Lock: %+v err=%v", as, err)
	}
	if _, err := c.Policy(ctx); err != nil {
		t.Fatalf("policy.get is read-only: %v", err)
	}
	if _, err := c.Simulate(ctx); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("simulate without login: %v", err)
	}
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Simulate(ctx); err != nil {
		t.Fatalf("simulate after login: %v", err)
	}
}

func TestPortableRemovePIN(t *testing.T) {
	ctx := context.Background()
	e := newEnvWith(t, InProcessCheck)
	c := typed.New(e.ln.dial(t))
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if err := c.RemovePIN(ctx, "wrong1"); client.Code(err) != wire.CodePINIncorrect {
		t.Fatalf("wrong PIN: %v", err)
	}
	if err := c.RemovePIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	as, err := c.AuthStatus(ctx)
	if err != nil || as.PINSet || !as.Elevated {
		t.Fatalf("after remove: %+v err=%v", as, err)
	}
	if _, err := c.SetSetting(ctx, store.SettingLanguage, "en"); err != nil {
		t.Fatalf("elevated after remove: %v", err)
	}
}
