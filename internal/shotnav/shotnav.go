// SPDX-License-Identifier: AGPL-3.0-or-later

// Package shotnav prepares a data directory and a front-end hash for
// GUI screenshots in test builds (VLT-IS-009).
package shotnav

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/gui"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// Options are the screenshot / demo-capture settings.
type Options struct {
	Demo      bool
	Screen    string
	Lang      string
	Mode      string
	ReadyFile string
}

const (
	// Width and Height are the capture window size (VLT-IS-009).
	Width  = 1280
	Height = 720
)

// PrepareDir writes the wizard-completed marker so the first-start dialog
// does not cover the screenshot.
func PrepareDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return (gui.FileFirstRun{Dir: dir}).SetCompleted()
}

// ApplySettings writes Demo transport, language and UI mode. Call it after
// the Vault has set the settings MAC key (engine start).
func ApplySettings(ctx context.Context, st *store.Store, o Options) error {
	set := func(key, raw string) error {
		_, err := st.SetSetting(ctx, key, json.RawMessage(raw), audit.ActorSystem)
		return err
	}
	if o.Demo || o.Screen != "" {
		if err := set(store.SettingDeviceTransport, `"demo"`); err != nil {
			return err
		}
	}
	if o.Lang == "en" || o.Lang == "tr" {
		b, err := json.Marshal(o.Lang)
		if err != nil {
			return err
		}
		if err := set(store.SettingLanguage, string(b)); err != nil {
			return err
		}
	}
	mode := o.Mode
	if mode == "" {
		if o.Screen == "simple" {
			mode = "simple"
		} else if o.Screen == "expert" {
			mode = "expert"
		}
	}
	if mode == "simple" || mode == "expert" {
		b, err := json.Marshal(mode)
		if err != nil {
			return err
		}
		if err := set(store.SettingUIMode, string(b)); err != nil {
			return err
		}
	}
	return nil
}

// Hash returns the location hash target for --demo-screen (no leading #).
func Hash(screen string) string {
	s := strings.TrimSpace(strings.TrimPrefix(screen, "#"))
	if s == "" {
		return "dashboard"
	}
	return s
}

// NavJS is the script the test-build window runs after it is ready.
func NavJS(screen string) string {
	h := Hash(screen)
	help := "false"
	if h == "help" {
		help = "true"
	}
	return fmt.Sprintf(`window.__VLT_DEMO_NAV__=true;window.__VLT_DEMO_HELP__=%s;window.location.hash=%q;`, help, "#"+h)
}

// ReadyTouch writes the ready file for the capture script.
func ReadyTouch(path string) {
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte("ready\n"), 0o600)
}
