// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// defaultPath is the Unix socket of OD-44: $XDG_RUNTIME_DIR/voltiovolt-simulator.sock,
// or /tmp/voltiovolt-simulator-<uid>.sock when XDG_RUNTIME_DIR is not set.
func defaultPath() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "voltiovolt-simulator.sock")
	}
	return fmt.Sprintf("/tmp/voltiovolt-simulator-%d.sock", os.Getuid())
}

// ListenAt listens on a Unix socket at path. It does not use TCP (OD-44, SEC-102).
func ListenAt(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && !os.IsExist(err) {
		// /tmp already exists; a missing runtime dir is an error.
		if filepath.Dir(path) != "/tmp" {
			return nil, err
		}
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// newTestPath is a socket for one test (VLT-IS-011 Q-1), in
// $XDG_RUNTIME_DIR (the tests set a temporary folder) or the temp folder.
func newTestPath(pid int, n uint64) string {
	d := os.Getenv("XDG_RUNTIME_DIR")
	if d == "" {
		d = os.TempDir()
	}
	return filepath.Join(d, fmt.Sprintf("vlt-sim-test-%d-%d.sock", pid, n))
}
