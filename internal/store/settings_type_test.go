// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/redact"
)

// TestNoSecretSetting is VLT-IS-004 TASK 03-00-00-001 step 5 (QI-6): the
// test fails if a Setting key is registered with the Secret type.
func TestNoSecretSetting(t *testing.T) {
	defs := DefaultSettings()
	if len(defs) == 0 {
		t.Fatal("no Settings")
	}
	for _, d := range defs {
		if d.Type == nil {
			t.Errorf("%s: no type", d.Key)
			continue
		}
		if containsSecret(d.Type, map[reflect.Type]bool{}) {
			t.Errorf("%s: the Setting has the Secret type", d.Key)
		}
	}
	if err := CheckDefs(defs); err != nil {
		t.Fatal(err)
	}

	// The check finds a Secret type, also inside other types.
	type withSecret struct {
		Name  string
		Token redact.Secret
	}
	for _, typ := range []reflect.Type{
		reflect.TypeOf(redact.Secret{}),
		reflect.TypeOf(&redact.Secret{}),
		reflect.TypeOf([]redact.Secret{}),
		reflect.TypeOf(map[string]redact.Secret{}),
		reflect.TypeOf(withSecret{}),
		reflect.TypeOf((*any)(nil)).Elem(),
	} {
		bad := append(DefaultSettings(), SettingDef{Key: "x.token", Type: typ, Default: json.RawMessage(`{}`)})
		if err := CheckDefs(bad); !errors.Is(err, ErrSecretSetting) {
			t.Errorf("%v: %v", typ, err)
		}
	}
	// Open refuses the definition.
	bad := append(DefaultSettings(), SettingDef{Key: "x.token", Type: reflect.TypeOf(redact.Secret{}), Default: json.RawMessage(`{}`)})
	_, err := Open(context.Background(), filepath.Join(t.TempDir(), "vlt.db"), Options{Settings: bad})
	if !errors.Is(err, ErrSecretSetting) {
		t.Fatalf("Open: %v", err)
	}
	// Other definition errors.
	for name, d := range map[string]SettingDef{
		"no type":    {Key: "a.b", Default: json.RawMessage(`1`)},
		"no key":     {Type: TypeInt, Default: json.RawMessage(`1`)},
		"wrong type": {Key: "a.b", Type: TypeInt, Default: json.RawMessage(`"x"`)},
		"check":      {Key: "a.b", Type: TypeInt, Default: json.RawMessage(`0`), Check: intRange(1, 2)},
	} {
		if err := CheckDefs([]SettingDef{d}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if err := CheckDefs(append(DefaultSettings(), DefaultSettings()[0])); err == nil {
		t.Error("duplicate key: no error")
	}
}

func TestSeriesVisibilitySetting(t *testing.T) {
	def := DefaultSettings()
	var found *SettingDef
	for i := range def {
		if def[i].Key == SettingHistorySeriesVisibility {
			found = &def[i]
			break
		}
	}
	if found == nil || found.Check == nil {
		t.Fatal("ui.history.series_visibility missing")
	}
	if err := found.Check(json.RawMessage(`{"input.voltage":false,"output.voltage":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := found.Check(json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := found.Check(json.RawMessage(`{"BAD":true}`)); err == nil {
		t.Fatal("uppercase key accepted")
	}
	if err := found.Check(json.RawMessage(`{"input.voltage":"no"}`)); err == nil {
		t.Fatal("non-boolean accepted")
	}
}

// TestUIModeSetting: ui.mode is "simple" by default (VLT-IS-011 OD-53).
func TestUIModeSetting(t *testing.T) {
	for _, d := range DefaultSettings() {
		if d.Key != SettingUIMode {
			continue
		}
		if string(d.Default) != `"simple"` {
			t.Fatalf("default %s", d.Default)
		}
		for v, ok := range map[string]bool{`"simple"`: true, `"expert"`: true, `"Simple"`: false, `""`: false, `1`: false} {
			if err := d.Check(json.RawMessage(v)); (err == nil) != ok {
				t.Errorf("%s: %v", v, err)
			}
		}
		return
	}
	t.Fatal("ui.mode missing")
}
