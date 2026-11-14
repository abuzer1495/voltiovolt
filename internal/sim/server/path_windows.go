// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package server

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
)

// defaultPath is the Windows named pipe of OD-44 (the product endpoint).
func defaultPath() string { return `\\.\pipe\voltiovolt-simulator` }

// newTestPath is a named pipe for one test (VLT-IS-011 Q-1).
func newTestPath(pid int, n uint64) string {
	return fmt.Sprintf(`\\.\pipe\voltiovolt-simulator-test-%d-%d`, pid, n)
}

// ListenAt listens on a named pipe. go-winio sets PIPE_REJECT_REMOTE_CLIENTS
// (OD-44). No TCP and no change to the operating system (SEC-102).
func ListenAt(path string) (net.Listener, error) {
	return winio.ListenPipe(path, &winio.PipeConfig{
		InputBufferSize:  4096,
		OutputBufferSize: 4096,
	})
}
