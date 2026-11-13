// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package server

import (
	"fmt"
	"net"
)

// ListenExclusive listens on the named pipe of OD-44. go-winio ListenPipe
// requires that the pipe does not already exist (FILE_FLAG_FIRST_PIPE_INSTANCE),
// so a second vlt-simulator receives ErrAlreadyRunning. No change to the
// operating system (SEC-102).
func ListenExclusive() (net.Listener, error) {
	ln, err := ListenAt(Path())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAlreadyRunning, err)
	}
	return ln, nil
}
