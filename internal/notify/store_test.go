// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

func TestStoreSettings(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "vlt.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.SetMACKey(make([]byte, 32)) // a test key: settings writes need it
	get := StoreSettings(st)
	// Defaults: desktop on, Telegram and e-mail off, all WARNING.
	if s := get(ctx, ChannelDesktop); !s.Enabled || s.MinSeverity != Warning {
		t.Fatalf("desktop %+v", s)
	}
	for _, ch := range []string{ChannelTelegram, ChannelEmail} {
		if s := get(ctx, ch); s.Enabled || s.MinSeverity != Warning {
			t.Fatalf("%s %+v", ch, s)
		}
	}
	if s := get(ctx, "x"); s.Enabled {
		t.Fatal("unknown channel is on")
	}
	en, sev, ok := SettingKeys(ChannelTelegram)
	if !ok {
		t.Fatal("no keys")
	}
	for k, v := range map[string]any{en: true, sev: "CRITICAL", store.SettingLanguage: "tr"} {
		b, _ := json.Marshal(v)
		if _, err := st.SetSetting(ctx, k, b, "test"); err != nil {
			t.Fatal(k, err)
		}
	}
	if s := get(ctx, ChannelTelegram); !s.Enabled || s.MinSeverity != Critical {
		t.Fatalf("telegram %+v", s)
	}
	b, _ := json.Marshal("ERROR")
	if _, err := st.SetSetting(ctx, sev, b, "test"); err == nil {
		t.Fatal("a wrong severity was accepted")
	}
	if l := StoreLanguage(st)(ctx); l != "tr" {
		t.Fatalf("language %q", l)
	}
}
