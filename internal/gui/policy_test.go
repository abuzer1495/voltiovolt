// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"encoding/json"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// TestPolicyMethods is TASK 03-00-00-006: the methods go to vltd with the
// correct parameters, and the error codes of vltd go to the front end.
func TestPolicyMethods(t *testing.T) {
	f := newFake(t)
	f.handle(wire.MethodPolicySet, func(p json.RawMessage) (any, *wire.Error) {
		var x wire.SettingsMapParams
		_ = json.Unmarshal(p, &x)
		if string(x.Settings["countdown_seconds"]) == "5" {
			return nil, &wire.Error{Code: wire.CodeInvalid, Message: "out of range"}
		}
		return map[string]any{"version": "v2", "active": false}, nil
	})
	f.handle(wire.MethodPolicySimulate, func(json.RawMessage) (any, *wire.Error) {
		return map[string]any{"version": "v2", "passed": true, "scenarios": []any{}}, nil
	})
	activated := false
	f.handle(wire.MethodPolicyActivate, func(json.RawMessage) (any, *wire.Error) {
		if !activated {
			activated = true
			return nil, &wire.Error{Code: wire.CodeNoSimulation, Message: "no passed Simulation"}
		}
		return wire.OKResult{OK: true}, nil
	})
	f.handle(wire.MethodPolicyDeactivate, func(json.RawMessage) (any, *wire.Error) {
		return nil, &wire.Error{Code: wire.CodeVaultLocked, Message: "fallback"}
	})
	_, a, _ := start(t, f)
	if _, err := a.SetPolicy(map[string]any{"countdown_seconds": 5}); err == nil || err.Error() != wire.CodeInvalid {
		t.Fatal(err)
	}
	r, err := a.SetPolicy(map[string]any{"countdown_seconds": 90})
	if err != nil || string(r) != `{"active":false,"version":"v2"}` {
		t.Fatalf("%s %v", r, err)
	}
	if p := f.last(wire.MethodPolicySet); string(p) != `{"settings":{"countdown_seconds":90}}` {
		t.Fatalf("params %s", p)
	}
	if _, err := a.Simulate(); err != nil {
		t.Fatal(err)
	}
	if err := a.ActivatePolicy(); err == nil || err.Error() != wire.CodeNoSimulation {
		t.Fatal(err)
	}
	if err := a.ActivatePolicy(); err != nil {
		t.Fatal(err)
	}
	if err := a.DeactivatePolicy(); err == nil || err.Error() != wire.CodeVaultLocked {
		t.Fatal(err)
	}
}
