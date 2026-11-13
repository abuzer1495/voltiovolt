// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// methodCase is one GUI method: its arguments and the vltd method that it
// calls ("" = no vltd call).
type methodCase struct {
	args []any
	ipc  string
}

// methodCases has each exported method of API (TASK 03-00-00-008 step 3).
// TestEachMethodWithFakeServer fails if a method is missing here.
func methodCases(secret []byte) map[string]methodCase {
	return map[string]methodCase{
		"Connection":             {nil, ""},
		"About":                  {nil, ""},
		"Last":                   {nil, ""},
		"Rules":                  {nil, ""},
		"SetTrayTexts":           {[]any{DefaultTrayTexts}, ""},
		"OpenSource":             {nil, ""},
		"SystemStatus":           {nil, wire.MethodStatus},
		"AuthStatus":             {nil, wire.MethodAuthStatus},
		"SetPIN":                 {[]any{"1234"}, wire.MethodSetPIN},
		"Login":                  {[]any{"1234"}, wire.MethodLogin},
		"Logout":                 {nil, wire.MethodLogout},
		"Events":                 {[]any{int64(0), 50}, wire.MethodEventsList},
		"Policy":                 {nil, wire.MethodPolicyGet},
		"ShutdownStatus":         {nil, wire.MethodShutdownStatus},
		"CancelShutdown":         {nil, wire.MethodShutdownCancel},
		"DeviceStatus":           {nil, wire.MethodDeviceStatus},
		"SetDeviceSettings":      {[]any{map[string]any{"device.buzzer": true}}, wire.MethodDeviceSettingsSet},
		"Measurements":           {[]any{"battery.capacity", int64(0), int64(3_600_000), "1m"}, wire.MethodMeasurements},
		"SetPolicy":              {[]any{map[string]any{"policy.low_battery.percent": 20}}, wire.MethodPolicySet},
		"Simulate":               {nil, wire.MethodPolicySimulate},
		"ActivatePolicy":         {nil, wire.MethodPolicyActivate},
		"DeactivatePolicy":       {nil, wire.MethodPolicyDeactivate},
		"SetSecret":              {[]any{"channel.test.token", secret}, wire.MethodSecretSet},
		"SecretStatus":           {[]any{"channel.test.token"}, wire.MethodSecretStatus},
		"ResetVault":             {[]any{ResetConfirmation}, wire.MethodVaultReset},
		"Audit":                  {[]any{int64(0), 50}, wire.MethodAuditList},
		"Settings":               {nil, wire.MethodSettingsList},
		"SetLanguage":            {[]any{"tr"}, wire.MethodSettingsSet},
		"DevicePorts":            {nil, wire.MethodDevicePorts},
		"DeviceScan":             {[]any{[]string{"COM3"}, ScanConfirmation}, wire.MethodDeviceScan},
		"UPSControlCapabilities": {nil, wire.MethodUPSControlCaps},
		"UPSTestStart":           {[]any{"10s", 0, TestConfirmation}, wire.MethodUPSTestStart},
		"UPSTestCancel":          {nil, wire.MethodUPSTestCancel},
		"UPSBeeperSet":           {[]any{false}, wire.MethodUPSBeeperSet},
		"Channels":               {nil, wire.MethodChannelsList},
		"SetChannel":             {[]any{"email", true, "WARNING", map[string]any{"port": 465}}, wire.MethodChannelsSettingsSet},
		"ChannelTest":            {[]any{"desktop"}, wire.MethodChannelsTest},
		"TelegramPairStart":      {nil, wire.MethodTelegramPairStart},
		"TelegramChats":          {nil, wire.MethodTelegramChatsList},
		"TelegramChatRemove":     {[]any{int64(100)}, wire.MethodTelegramChatsRemove},
	}
}

// allFake answers each vltd method of the GUI.
func allFake(t *testing.T) *fakeServer {
	f := newFake(t)
	for _, m := range []string{wire.MethodStatus, wire.MethodSetPIN, wire.MethodLogin, wire.MethodLogout,
		wire.MethodEventsList, wire.MethodPolicyGet, wire.MethodShutdownStatus, wire.MethodShutdownCancel,
		wire.MethodDeviceStatus, wire.MethodDeviceSettingsSet, wire.MethodMeasurements, wire.MethodPolicySet,
		wire.MethodPolicySimulate, wire.MethodPolicyActivate, wire.MethodPolicyDeactivate, wire.MethodSecretSet,
		wire.MethodVaultReset, wire.MethodAuditList, wire.MethodSettingsList, wire.MethodSettingsSet,
		wire.MethodDevicePorts, wire.MethodDeviceScan,
		wire.MethodUPSControlCaps, wire.MethodUPSTestStart, wire.MethodUPSTestCancel, wire.MethodUPSBeeperSet,
		wire.MethodChannelsList, wire.MethodChannelsSettingsSet, wire.MethodChannelsTest, wire.MethodTelegramPairStart,
		wire.MethodTelegramChatsList, wire.MethodTelegramChatsRemove} {
		f.handle(m, func(json.RawMessage) (any, *wire.Error) { return map[string]any{"ok": true}, nil })
	}
	f.handle(wire.MethodSecretStatus, func(json.RawMessage) (any, *wire.Error) {
		return wire.SecretStatusResult{ID: "channel.test.token", Set: true, Hint: "…ue"}, nil
	})
	return f
}

// invoke calls an API method by its name with reflection.
func invoke(t *testing.T, a *API, name string, args []any) []reflect.Value {
	t.Helper()
	m := reflect.ValueOf(a).MethodByName(name)
	if !m.IsValid() {
		t.Fatalf("no method %s", name)
	}
	in := make([]reflect.Value, len(args))
	for i, v := range args {
		in[i] = reflect.ValueOf(v)
	}
	return m.Call(in)
}

func exported(a *API) []string {
	var out []string
	tp := reflect.TypeOf(a)
	for i := range tp.NumMethod() {
		out = append(out, tp.Method(i).Name)
	}
	return out
}

// TestEachMethodWithFakeServer calls each GUI method on the fake IPC server
// and checks the vltd method that it sends.
func TestEachMethodWithFakeServer(t *testing.T) {
	f := allFake(t)
	m, a, _ := start(t, f)
	m.SetOpenURL(func(string) error { return nil })
	cases := methodCases([]byte("value"))
	for _, name := range exported(a) {
		c, ok := cases[name]
		if !ok {
			t.Errorf("method %s has no test case", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			before := len(f.methods())
			out := invoke(t, a, name, c.args)
			if n := len(out); n > 0 {
				if e, ok := out[n-1].Interface().(error); ok && e != nil {
					t.Fatalf("%s: %v", name, e)
				}
			}
			got := f.methods()[before:]
			if c.ipc == "" {
				if slices.ContainsFunc(got, func(s string) bool { return s != wire.MethodAuthStatus }) {
					t.Fatalf("%s: unexpected vltd calls %v", name, got)
				}
				return
			}
			if !slices.Contains(got, c.ipc) {
				t.Fatalf("%s: calls %v, want %s", name, got, c.ipc)
			}
		})
	}
}

// TestSEC096 checks that a Secret goes to vltd with secret.set, that the
// Go side clears the buffer, and that no GUI method or push event returns
// the Secret value to the front end.
func TestSEC096(t *testing.T) {
	const value = "S3cr3t-Wert-ÇĞİÖŞÜ-\"\\\x01"
	f := allFake(t)
	var got wire.SecretSetParams
	f.handle(wire.MethodSecretSet, func(p json.RawMessage) (any, *wire.Error) {
		if err := json.Unmarshal(p, &got); err != nil {
			return nil, &wire.Error{Code: wire.CodeInvalid, Message: err.Error()}
		}
		return wire.OKResult{OK: true}, nil
	})
	m, a, rec := start(t, f)
	m.SetOpenURL(func(string) error { return nil })

	buf := []byte(value)
	if err := a.SetSecret("channel.test.token", buf); err != nil {
		t.Fatal(err)
	}
	if got.ID != "channel.test.token" || got.Value != value {
		t.Fatalf("vltd got %+v", got)
	}
	if !bytes.Equal(buf, make([]byte, len(value))) {
		t.Fatal("SetSecret did not clear the buffer")
	}
	// The buffer is cleared after an error too.
	f.handle(wire.MethodSecretSet, func(json.RawMessage) (any, *wire.Error) {
		return nil, &wire.Error{Code: wire.CodeNotElevated, Message: "x"}
	})
	buf = []byte(value)
	if err := a.SetSecret("channel.test.token", buf); err == nil {
		t.Fatal("want error")
	}
	if !bytes.Equal(buf, make([]byte, len(value))) {
		t.Fatal("buffer not cleared after an error")
	}

	// After secret.set, no GUI method and no push event gives the value
	// back: the Go side keeps no copy of it.
	secretJSON, _ := json.Marshal(value)
	needle := []byte(value)
	cases := methodCases([]byte("other"))
	for _, name := range exported(a) {
		c := cases[name]
		if name == "SetSecret" {
			continue
		}
		for _, r := range invoke(t, a, name, c.args) {
			b, err := json.Marshal(r.Interface())
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if bytes.Contains(b, needle) || bytes.Contains(b, secretJSON[1:len(secretJSON)-1]) {
				t.Fatalf("%s returns the Secret", name)
			}
		}
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, e := range rec.evs {
		b, _ := json.Marshal(e.data)
		if bytes.Contains(b, secretJSON[1:len(secretJSON)-1]) {
			t.Fatalf("event %s has the Secret", e.name)
		}
	}
}

// TestSEC096Types checks that no result type of a GUI method has a field
// that holds a Secret value (secret.set params or a "value" in a secret
// type).
func TestSEC096Types(t *testing.T) {
	bad := reflect.TypeOf(wire.SecretSetParams{})
	seen := map[reflect.Type]bool{}
	var walk func(tp reflect.Type, path string)
	walk = func(tp reflect.Type, path string) {
		if seen[tp] {
			return
		}
		seen[tp] = true
		if tp == bad {
			t.Fatalf("%s returns %v", path, tp)
		}
		switch tp.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			walk(tp.Elem(), path)
		case reflect.Struct:
			for i := range tp.NumField() {
				walk(tp.Field(i).Type, path+"."+tp.Field(i).Name)
			}
		}
	}
	tp := reflect.TypeOf(&API{})
	for i := range tp.NumMethod() {
		mt := tp.Method(i).Type
		for j := range mt.NumOut() {
			walk(mt.Out(j), tp.Method(i).Name)
		}
	}
	if _, ok := reflect.TypeOf(wire.SecretStatusResult{}).FieldByName("Value"); ok {
		t.Fatal("secret.status result has a Value field")
	}
}

// TestUPSTestStartChecks: the GUI refuses a wrong confirmation text or a
// wrong minutes value before it sends a request (VLT-IS-006).
func TestUPSTestStartChecks(t *testing.T) {
	f := allFake(t)
	_, a, _ := start(t, f)
	before := len(f.methods())
	for _, c := range []struct {
		typ  string
		min  int
		conf string
	}{
		{"10s", 0, "start test"}, {"until_low", 0, TestConfirmation}, {"minutes", 0, TestConfirmation},
		{"minutes", 100, TestConfirmation}, {"shutdown", 0, TestConfirmation}, {"10s", 0, TestUntilLowConfirmation},
	} {
		if _, err := a.UPSTestStart(c.typ, c.min, c.conf); err == nil {
			t.Errorf("%+v accepted", c)
		}
	}
	if got := f.methods()[before:]; len(got) != 0 {
		t.Fatalf("sent %v", got)
	}
	if _, err := a.UPSTestStart("until_low", 0, TestUntilLowConfirmation); err != nil {
		t.Fatal(err)
	}
}
