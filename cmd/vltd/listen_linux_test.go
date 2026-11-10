// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc"
)

func testListen(t *testing.T) (func() (net.Listener, ipc.PeerCheck, error), string) {
	dir := filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vltd.sock")
	ep := ipc.Endpoint{Path: path, GID: os.Getgid(), SetDirMode: true}
	return ep.Listen, path
}
