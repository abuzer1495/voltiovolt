// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package portable

import (
	"fmt"
	"os"
	"path/filepath"
)

func shortcutName() string { return "voltiovolt.desktop" }

func startupDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "autostart"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart"), nil
}

func writeShortcut(path, target string) error {
	body := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=VoltioVolt\nExec=%q\nX-GNOME-Autostart-enabled=true\n", target)
	return os.WriteFile(path, []byte(body), 0o644)
}
