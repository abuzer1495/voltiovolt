// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package server

import (
	"net"

	"github.com/Microsoft/go-winio"
)

// Path is the Windows named pipe of OD-44.
func Path() string { return `\\.\pipe\voltiovolt-simulator` }

// ListenAt listens on a named pipe. go-winio sets PIPE_REJECT_REMOTE_CLIENTS
// (OD-44). No TCP and no change to the operating system (SEC-102).
func ListenAt(path string) (net.Listener, error) {
	return winio.ListenPipe(path, &winio.PipeConfig{
		InputBufferSize:  4096,
		OutputBufferSize: 4096,
	})
}
