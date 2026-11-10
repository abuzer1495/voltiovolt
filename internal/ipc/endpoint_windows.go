// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package ipc

import (
	"fmt"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// DefaultPipe is the vltd pipe name (step 6.2).
const DefaultPipe = `\\.\pipe\vltd`

// Account names in the pipe DACL.
const (
	ServiceAccount = `NT SERVICE\vltd`
	OperatorsGroup = "VLT Operators"
)

// Endpoint is the Windows IPC endpoint: a named pipe with a protected DACL.
// go-winio always rejects remote clients (PIPE_REJECT_REMOTE_CLIENTS).
type Endpoint struct {
	Pipe string // default DefaultPipe
	// ExtraSIDs get read and write access (tests: the current user).
	ExtraSIDs []string
	// NoLookup: do not look up the service account and the operators group
	// (tests).
	NoLookup bool
}

// DefaultEndpoint returns the vltd endpoint.
func DefaultEndpoint() Endpoint { return Endpoint{Pipe: DefaultPipe} }

func lookupSID(name string) (string, bool) {
	sid, _, _, err := windows.LookupSID("", name)
	if err != nil {
		return "", false
	}
	return sid.String(), true
}

// SDDL returns the pipe security descriptor. D:P is a protected DACL (no
// inherited entries). SYSTEM, Administrators and NT SERVICE\vltd get full
// access (the service must make new pipe instances). "VLT Operators" (if
// the group exists) gets read and write access.
func (e Endpoint) SDDL() string {
	var b strings.Builder
	b.WriteString("D:P(A;;GA;;;SY)(A;;GA;;;BA)")
	if !e.NoLookup {
		if s, ok := lookupSID(ServiceAccount); ok {
			fmt.Fprintf(&b, "(A;;GA;;;%s)", s)
		}
		if s, ok := lookupSID(OperatorsGroup); ok {
			fmt.Fprintf(&b, "(A;;GRGW;;;%s)", s)
		}
	}
	for _, s := range e.ExtraSIDs {
		fmt.Fprintf(&b, "(A;;GA;;;%s)", s)
	}
	return b.String()
}

// Listen makes the pipe.
func (e Endpoint) Listen() (net.Listener, PeerCheck, error) {
	if e.Pipe == "" {
		e.Pipe = DefaultPipe
	}
	ln, err := winio.ListenPipe(e.Pipe, &winio.PipeConfig{
		SecurityDescriptor: e.SDDL(),
		InputBufferSize:    64 << 10,
		OutputBufferSize:   64 << 10,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("ipc: pipe %s: %w", e.Pipe, err)
	}
	return ln, nil, nil
}
