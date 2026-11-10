// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// TestSettingsSetUIPrefix: settings.set accepts a "ui." key without the
// elevated state and rejects all other keys without it. Each case writes
// an audit entry (VLT-IS-006 TASK 03-00-00-001 step 1).
func TestSettingsSetUIPrefix(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	c := typed.New(e.ln.dial(t))

	// No PIN: also "ui." keys are refused.
	if _, err := c.SetSetting(ctx, store.SettingLanguage, "tr"); client.Code(err) != wire.CodePINNotSet {
		t.Fatalf("ui. key before PIN: %v", err)
	}
	if err := c.SetPIN(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	// PIN set, not elevated.
	if v, err := c.SetSetting(ctx, store.SettingLanguage, "tr"); err != nil || v != 1 {
		t.Fatalf("ui. key without elevation: %d %v", v, err)
	}
	if _, err := c.SetSetting(ctx, store.SettingLanguage, "xx"); client.Code(err) != wire.CodeInvalid {
		t.Fatalf("ui. key with a wrong value: %v", err)
	}
	if _, err := c.SetSetting(ctx, store.SettingSessionIdleMinutes, 15); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("other key without elevation: %v", err)
	}
	if _, err := c.SetSetting(ctx, "uix.language", "tr"); client.Code(err) != wire.CodeNotElevated {
		t.Fatalf("a key without the exact prefix: %v", err)
	}
	// Elevated: the other key is accepted.
	if err := c.Login(ctx, testPIN); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetSetting(ctx, store.SettingSessionIdleMinutes, 15); err != nil {
		t.Fatalf("other key elevated: %v", err)
	}
	if _, err := c.SetSetting(ctx, store.SettingLanguage, "en"); err != nil {
		t.Fatalf("ui. key elevated: %v", err)
	}

	rows, err := e.au.List(ctx, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ target, result string }
	seen := map[key]int{}
	for _, r := range rows {
		if r.Action == "settings_set" {
			seen[key{r.Target, r.Result}]++
		}
	}
	for _, k := range []key{
		{store.SettingLanguage, "success"},           // ui. key without and with elevation
		{store.SettingLanguage, "failure"},           // wrong value
		{store.SettingSessionIdleMinutes, "failure"}, // not elevated
		{store.SettingSessionIdleMinutes, "success"}, // elevated
		{"uix.language", "failure"},                  // not elevated
	} {
		if seen[k] == 0 {
			t.Errorf("no audit entry %v (all: %v)", k, seen)
		}
	}
	if seen[key{store.SettingLanguage, "success"}] != 2 {
		t.Errorf("ui. key: %d success entries, want 2", seen[key{store.SettingLanguage, "success"}])
	}
	b, _ := json.Marshal(rows)
	if !bytes.Contains(b, []byte(`"elevated":false`)) || !bytes.Contains(b, []byte(`"elevated":true`)) {
		t.Error("the audit entries do not show the elevated state")
	}
}

func TestIsUISetting(t *testing.T) {
	for k, want := range map[string]bool{"ui.language": true, "ui.": true, "ui": false, "uix.a": false, "policy.action": false, "UI.language": false, "": false} {
		if IsUISetting(k) != want {
			t.Errorf("%q: %v", k, !want)
		}
	}
}
