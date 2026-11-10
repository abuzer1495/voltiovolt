// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package main

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/abuzer1495/voltiovolt/internal/ipc"
)

func testListen(t *testing.T) (func() (net.Listener, ipc.PeerCheck, error), string) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 8)
	rand.Read(b)
	name := `\\.\pipe\vltd-test-` + hex.EncodeToString(b)
	ep := ipc.Endpoint{Pipe: name, ExtraSIDs: []string{tu.User.Sid.String()}}
	return ep.Listen, name
}
