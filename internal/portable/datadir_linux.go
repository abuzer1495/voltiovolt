// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package portable

import (
	"os"
	"path/filepath"
)

func profileDir() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "voltiovolt"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "voltiovolt"), nil
}
