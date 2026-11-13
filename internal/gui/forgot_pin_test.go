// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// TestForgotPINAPI is the GUI side of VLT-IS-009 TASK 03-00-00-002:
// a wrong confirmation does not call vltd; "RESET" does.
func TestForgotPINAPI(t *testing.T) {
	f := newFake(t)
	f.handle(wire.MethodAuthStatus, func(json.RawMessage) (any, *wire.Error) {
		return wire.AuthStatusResult{PINSet: true}, nil
	})
	var got string
	f.handle(wire.MethodForgotPIN, func(p json.RawMessage) (any, *wire.Error) {
		var params wire.ForgotPINParams
		if err := json.Unmarshal(p, &params); err != nil {
			t.Fatal(err)
		}
		got = params.Confirmation
		return wire.OKResult{OK: true}, nil
	})
	_, a, _ := start(t, f)
	if err := a.ForgotPIN("reset"); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatalf("wrong text: %v", err)
	}
	if got != "" {
		t.Fatalf("vltd called with %q", got)
	}
	if err := a.ForgotPIN(wire.ForgotPINConfirmation); err != nil {
		t.Fatal(err)
	}
	if got != wire.ForgotPINConfirmation {
		t.Fatalf("confirmation %q", got)
	}
}

// TestForgotPINE2E is the end-to-end test of TASK 03-00-00-002 with the
// portable engine and the GUI Go side.
func TestForgotPINE2E(t *testing.T) {
	_, _, a, stop := startPortableGUI(t, "")
	defer stop()

	const pin = "k7Pq2Lm9"
	mustSetAndLock(t, a, pin)
	if err := a.Login(pin); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := a.SetLanguage("tr"); err != nil {
		t.Fatalf("SetLanguage: %v", err)
	}
	secret := []byte("integration-test-secret-value-WXYZ")
	if err := a.SetSecret("telegram.bot_token", secret); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	if _, err := a.SetPolicy(map[string]any{"countdown_seconds": 20}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}
	if raw, err := a.Simulate(); err != nil {
		t.Fatalf("Simulate: %v raw=%s", err, raw)
	}
	if err := a.ActivatePolicy(); err != nil {
		t.Fatalf("ActivatePolicy: %v", err)
	}
	if err := a.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	// The portable engine applies the IPC rate limit (20 requests each
	// second). The steps above already used many requests.
	time.Sleep(time.Second)

	if err := a.ForgotPIN("reset"); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatalf("wrong text: %v", err)
	}
	st, err := a.AuthStatus()
	if err != nil || !st.PINSet {
		t.Fatalf("still after wrong text: %+v %v", st, err)
	}
	if err := a.ForgotPIN(wire.ForgotPINConfirmation); err != nil {
		t.Fatalf("ForgotPIN: %v", err)
	}
	st, err = a.AuthStatus()
	if err != nil || st.PINSet || !st.Elevated {
		t.Fatalf("after forgot: %+v %v", st, err)
	}
	ss, err := a.SecretStatus("telegram.bot_token")
	if err != nil || ss.Set {
		t.Fatalf("secret still set: %+v %v", ss, err)
	}
	raw, err := a.Policy()
	if err != nil {
		t.Fatal(err)
	}
	var pol struct {
		Active bool `json:"active"`
	}
	if err := json.Unmarshal(raw, &pol); err != nil || pol.Active {
		t.Fatalf("policy: %s err=%v", raw, err)
	}
	sraw, err := a.Settings()
	if err != nil {
		t.Fatal(err)
	}
	found := strings.Contains(string(sraw), `"ui.language"`) && strings.Contains(string(sraw), `"tr"`)
	if !found {
		t.Fatalf("settings lost: %s", sraw)
	}
	if err := a.SetPIN("9q8w7e6r"); err != nil {
		t.Fatalf("SetPIN after forgot: %v", err)
	}
}
