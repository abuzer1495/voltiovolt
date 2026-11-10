// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// TestSecurityMethods is TASK 03-00-00-007.
func TestSecurityMethods(t *testing.T) {
	f := newFake(t)
	f.handle(wire.MethodVaultReset, ok)
	f.handle(wire.MethodAuditList, func(json.RawMessage) (any, *wire.Error) {
		return map[string]any{"rows": []any{map[string]any{"seq": 7, "action": "login"}}, "next": 0}, nil
	})
	f.handle(wire.MethodSettingsList, func(json.RawMessage) (any, *wire.Error) {
		return wire.SettingsListResult{Settings: []wire.Setting{{Key: "ui.language", Value: json.RawMessage(`"en"`), Default: true}}}, nil
	})
	f.handle(wire.MethodSettingsSet, func(json.RawMessage) (any, *wire.Error) { return map[string]any{"version": 2}, nil })
	f.handle(wire.MethodStatus, func(json.RawMessage) (any, *wire.Error) {
		return wire.StatusResult{Vault: wire.VaultState{State: "reduced protection", Reason: "no TPM"}}, nil
	})
	_, a, _ := start(t, f)
	if err := a.ResetVault("reset vault"); err == nil {
		t.Fatal("wrong confirmation accepted")
	}
	for _, m := range f.methods() {
		if m == wire.MethodVaultReset {
			t.Fatal("vault.reset sent with a wrong confirmation")
		}
	}
	if err := a.ResetVault("RESET VAULT"); err != nil {
		t.Fatal(err)
	}
	if p := f.last(wire.MethodVaultReset); string(p) != `{"confirmation":"RESET VAULT"}` {
		t.Fatalf("%s", p)
	}
	au, err := a.Audit(0, 50)
	if err != nil || !bytes.Contains(au, []byte(`"seq":7`)) {
		t.Fatalf("%s %v", au, err)
	}
	if p := f.last(wire.MethodAuditList); string(p) != `{"after":0,"limit":50}` {
		t.Fatalf("%s", p)
	}
	if s, err := a.Settings(); err != nil || !bytes.Contains(s, []byte("ui.language")) {
		t.Fatalf("%s %v", s, err)
	}
	if err := a.SetLanguage("de"); err == nil {
		t.Fatal("de accepted")
	}
	if err := a.SetLanguage("tr"); err != nil {
		t.Fatal(err)
	}
	if p := f.last(wire.MethodSettingsSet); string(p) != `{"key":"ui.language","value":"tr"}` {
		t.Fatalf("%s", p)
	}
	st, err := a.SystemStatus()
	if err != nil || st.Vault.State != "reduced protection" {
		t.Fatalf("%+v %v", st, err)
	}
	if err := a.OpenSource(); err == nil {
		t.Fatal("no browser function")
	}
	var opened string
	a.m.SetOpenURL(func(u string) error { opened = u; return nil })
	if err := a.OpenSource(); err != nil || opened != SourceURL {
		t.Fatal(opened, err)
	}
	ab := a.About()
	if ab.License != "AGPL-3.0-or-later" || ab.Source != "https://github.com/abuzer1495/voltiovolt" || ab.VltdVersion != "0.5.0-test" || ab.Protocol != 1 {
		t.Fatalf("%+v", ab)
	}
}
