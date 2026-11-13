// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !unix && !windows

package portable

import (
	"fmt"
	"os"
)

func shortcutName() string { return "voltiovolt.autostart" }

func startupDir() (string, error) { return os.TempDir(), nil }

func writeShortcut(path, target string) error {
	return os.WriteFile(path, []byte(fmt.Sprintf("target=%s\n", target)), 0o644)
}
