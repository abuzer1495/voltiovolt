// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"context"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/measure"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// TestForgotPIN is VLT-IS-009 TASK 03-00-00-002 and OD-40: Forgot PIN in
// portable mode removes the PIN, deletes all Secrets, makes the Shutdown
// Policy inactive, and keeps Settings, Measurements and Events.
func TestForgotPIN(t *testing.T) {
	ctx := context.Background()
	e := newEnvWith(t, InProcessCheck)
	c := typed.New(e.ln.dial(t))

	if err := c.ForgotPIN(ctx, wire.ForgotPINConfirmation); client.Code(err) != wire.CodePINNotSet {
		t.Fatalf("no PIN: %v", err)
	}

	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetSetting(ctx, store.SettingLanguage, "tr"); err != nil {
		t.Fatalf("settings.set: %v", err)
	}
	secret := "integration-test-secret-value-WXYZ"
	if err := c.SetSecret(ctx, "telegram.bot_token", secret); err != nil {
		t.Fatalf("secret.set: %v", err)
	}
	in, err := c.SetPolicy(ctx, map[string]any{"countdown_seconds": 20})
	if err != nil {
		t.Fatalf("policy.set: %v", err)
	}
	r, err := c.Simulate(ctx)
	if err != nil || !r.Passed || r.Version != in.Version {
		t.Fatalf("policy.simulate: %+v %v", r, err)
	}
	if err := c.ActivatePolicy(ctx); err != nil {
		t.Fatalf("policy.activate: %v", err)
	}
	if in2, err := c.Policy(ctx); err != nil || !in2.Active {
		t.Fatalf("policy active: %+v %v", in2, err)
	}

	now := time.Now().UTC()
	if _, err := e.st.DB().ExecContext(ctx, "INSERT INTO events (ts, type, severity, data) VALUES (?, 'ON_BATTERY', 'WARNING', '{\"active\":true}')", now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	w := measure.Writer{DB: e.st.DB()}
	if err := w.WriteCycle(ctx, []measure.Sample{{Time: now.Add(-time.Second), Points: []measure.Point{{Name: measure.InputVoltage, Unit: "V", Value: 230, Source: "Q1"}}}}); err != nil {
		t.Fatal(err)
	}

	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.ForgotPIN(ctx, "reset"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("wrong confirmation: %v", err)
	}
	as, err := c.AuthStatus(ctx)
	if err != nil || !as.PINSet {
		t.Fatalf("PIN still after wrong text: %+v %v", as, err)
	}

	if err := c.ForgotPIN(ctx, wire.ForgotPINConfirmation); err != nil {
		t.Fatalf("ForgotPIN: %v", err)
	}
	as, err = c.AuthStatus(ctx)
	if err != nil || as.PINSet || !as.Elevated {
		t.Fatalf("after forgot: %+v %v", as, err)
	}
	ss, err := c.SecretStatus(ctx, "telegram.bot_token")
	if err != nil || ss.Set {
		t.Fatalf("secret still set: %+v %v", ss, err)
	}
	if in3, err := c.Policy(ctx); err != nil || in3.Active {
		t.Fatalf("policy still active: %+v %v", in3, err)
	}
	list, err := c.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range list {
		if s.Key == store.SettingLanguage && string(s.Value) == `"tr"` {
			found = true
		}
	}
	if !found {
		t.Fatalf("settings lost: %+v", list)
	}
	ev, _, err := c.Events(ctx, 0, 10)
	if err != nil || len(ev) != 1 || ev[0].Type != "ON_BATTERY" {
		t.Fatalf("events: %+v %v", ev, err)
	}
	unit, rows, err := c.Measurements(ctx, measure.InputVoltage, now.Add(-time.Hour), now.Add(time.Hour), measure.StepRaw)
	if err != nil || unit != "V" || len(rows) != 1 || rows[0].Mean != 230 {
		t.Fatalf("measurements: %s %+v %v", unit, rows, err)
	}
	al, err := c.Audit(ctx, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	var forgot, vaultReset, polOff bool
	for _, row := range al.Rows {
		switch row.Action {
		case "pin_forgot":
			forgot = row.Result == "success"
		case "vault_reset":
			vaultReset = row.Result == "success"
		case "policy_deactivate":
			polOff = row.Result == "success"
		}
	}
	if !forgot || !vaultReset || !polOff {
		t.Fatalf("audit: forgot=%v vault=%v policy=%v rows=%+v", forgot, vaultReset, polOff, al.Rows)
	}
	if err := c.SetPIN(ctx, "9q8w7e6r"); err != nil {
		t.Fatalf("SetPIN after forgot: %v", err)
	}
}

func TestForgotPINServiceMode(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	c := typed.New(e.ln.dial(t))
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if err := c.ForgotPIN(ctx, wire.ForgotPINConfirmation); client.Code(err) != wire.CodeUnavailable {
		t.Fatalf("service mode: %v", err)
	}
	as, err := c.AuthStatus(ctx)
	if err != nil || !as.PINSet {
		t.Fatalf("PIN removed in service mode: %+v %v", as, err)
	}
}
