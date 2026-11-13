// SPDX-License-Identifier: AGPL-3.0-or-later

// Package portable has the portable-mode helpers (VLT-IS-008): the data
// folder and the start-at-logon shortcut. It does not install a service,
// change a user, a group or a user right, write to the registry or set an
// ACL (SEC-102).
package portable

import (
	"os"
	"path/filepath"
)

// DataDir returns the portable data folder (VLT-IS-008 TASK 03-00-00-002
// step 1). Default: the folder "data" next to the binary, if it is
// writable. If not, the profile folder of the user: Windows
// %LOCALAPPDATA%\VoltioVolt, Linux ~/.local/share/voltiovolt. The
// function does not check or change the ACL of the folder.
func DataDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return profileDir()
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}
	next := filepath.Join(filepath.Dir(exe), "data")
	if writable(next) {
		return next, nil
	}
	return profileDir()
}

// ProfileDir is the profile data folder of the user (the fallback of
// DataDir).
func ProfileDir() (string, error) { return profileDir() }

func writable(dir string) bool {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".w*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
