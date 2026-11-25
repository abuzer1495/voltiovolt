// SPDX-License-Identifier: AGPL-3.0-or-later

package shotnav

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/gui"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

func TestHashAndNavJS(t *testing.T) {
	if Hash("") != "dashboard" || Hash("#wizard/pin") != "wizard/pin" {
		t.Fatalf("hash %q %q", Hash(""), Hash("#wizard/pin"))
	}
	js := NavJS("help")
	if !strings.Contains(js, "window.__VLT_DEMO_NAV__=true") || !strings.Contains(js, "__VLT_DEMO_HELP__=true") || !strings.Contains(js, "#help") {
		t.Fatalf("js %s", js)
	}
	js = NavJS("device")
	if strings.Contains(js, "__VLT_DEMO_HELP__=true") || !strings.Contains(js, "#device") {
		t.Fatalf("js %s", js)
	}
}

func TestPrepareDirAndApplySettings(t *testing.T) {
	dir := t.TempDir()
	if err := PrepareDir(dir); err != nil {
		t.Fatal(err)
	}
	ok, err := (gui.FileFirstRun{Dir: dir}).Completed()
	if err != nil || !ok {
		t.Fatalf("wizard completed: %v %v", ok, err)
	}
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(dir, store.DBFileName), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.SetMACKey(bytes.Repeat([]byte{9}, 32))
	if err := ApplySettings(ctx, st, Options{Demo: true, Lang: "tr", Mode: "expert"}); err != nil {
		t.Fatal(err)
	}
	check := func(key, want string) {
		t.Helper()
		s, err := st.GetSetting(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		var v string
		if err := json.Unmarshal(s.Value, &v); err != nil || v != want {
			t.Fatalf("%s = %s, want %s (%v)", key, s.Value, want, err)
		}
	}
	check(store.SettingDeviceTransport, "demo")
	check(store.SettingLanguage, "tr")
	check(store.SettingUIMode, "expert")
	ready := filepath.Join(dir, "ready")
	ReadyTouch(ready)
	if _, err := os.Stat(ready); err != nil {
		t.Fatal(err)
	}
}
