// SPDX-License-Identifier: AGPL-3.0-or-later

package portable

import (
	"os"
	"path/filepath"
)

// ShortcutName is the file name of the start-at-logon shortcut, without
// the directory. Windows: VoltioVolt.lnk. Linux: voltiovolt.desktop.
func ShortcutName() string { return shortcutName() }

// StartupDir is the startup folder of the current user (Windows:
// shell:startup; Linux: ~/.config/autostart). Tests can set
// VLT_STARTUP_DIR to override it. The function does not write to the
// registry (SEC-102, OD-39).
func StartupDir() (string, error) {
	if d := os.Getenv("VLT_STARTUP_DIR"); d != "" {
		return d, nil
	}
	return startupDir()
}

// TargetPath is the executable that the shortcut starts. Tests can set
// VLT_AUTOSTART_TARGET to override it.
func TargetPath() (string, error) {
	if t := os.Getenv("VLT_AUTOSTART_TARGET"); t != "" {
		return t, nil
	}
	return os.Executable()
}

// ShortcutPath is the full path of the start-at-logon shortcut.
func ShortcutPath() (string, error) {
	dir, err := StartupDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ShortcutName()), nil
}

// AutostartEnabled reports whether the shortcut is present.
func AutostartEnabled() (bool, error) {
	p, err := ShortcutPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// SetAutostart makes or deletes the start-at-logon shortcut (OD-39).
// On: make the shortcut in the startup folder of the user. Off: delete it.
// Default of a new install is off (no shortcut). No administrator rights,
// no registry write.
func SetAutostart(on bool) error {
	if !on {
		p, err := ShortcutPath()
		if err != nil {
			return err
		}
		err = os.Remove(p)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dir, err := StartupDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	target, err := TargetPath()
	if err != nil {
		return err
	}
	return writeShortcut(filepath.Join(dir, ShortcutName()), target)
}
