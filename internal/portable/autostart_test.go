// SPDX-License-Identifier: AGPL-3.0-or-later

package portable

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAutostartShortcut(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VLT_STARTUP_DIR", dir)
	t.Setenv("VLT_AUTOSTART_TARGET", filepath.Join(dir, "VoltioVolt-fake"))
	on, err := AutostartEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("default is on")
	}
	if err := SetAutostart(true); err != nil {
		t.Fatal(err)
	}
	on, err = AutostartEnabled()
	if err != nil || !on {
		t.Fatalf("enabled %v %v", on, err)
	}
	p, err := ShortcutPath()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if !strings.Contains(string(b), "VoltioVolt-fake") {
			t.Fatalf("shortcut body %q", b)
		}
		if !strings.Contains(string(b), "[Desktop Entry]") && runtime.GOOS == "linux" {
			t.Fatal("not a desktop file")
		}
	} else {
		if len(b) < 16 {
			t.Fatalf("lnk too small: %d", len(b))
		}
	}
	if err := SetAutostart(false); err != nil {
		t.Fatal(err)
	}
	on, err = AutostartEnabled()
	if err != nil || on {
		t.Fatalf("still enabled %v %v", on, err)
	}
	// A second off is not an error.
	if err := SetAutostart(false); err != nil {
		t.Fatal(err)
	}
}

func TestShortcutName(t *testing.T) {
	n := ShortcutName()
	if runtime.GOOS == "windows" && n != "VoltioVolt.lnk" {
		t.Fatal(n)
	}
	if runtime.GOOS == "linux" && n != "voltiovolt.desktop" {
		t.Fatal(n)
	}
}
