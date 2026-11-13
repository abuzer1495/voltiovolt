// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package portable

import (
	"os"
	"path/filepath"
)

func profileDir() (string, error) {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "VoltioVolt"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "AppData", "Local", "VoltioVolt"), nil
}
