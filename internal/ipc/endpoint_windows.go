// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package ipc

import (
	"fmt"
	"net"

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

// Endpoint is the Windows IPC endpoint: a named pipe with a protected DACL
// (PipeSDDL).
// go-winio always rejects remote clients (PIPE_REJECT_REMOTE_CLIENTS).
type Endpoint struct {
	Pipe string // default DefaultPipe
	// ExtraSIDs get read and write access (GRGW; tests: the current user).
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

// SDDL returns the pipe security descriptor (see PipeSDDL).
func (e Endpoint) SDDL() string {
	var svc, ops string
	if !e.NoLookup {
		svc, _ = lookupSID(ServiceAccount)
		ops, _ = lookupSID(OperatorsGroup)
	}
	return PipeSDDL(svc, ops, e.ExtraSIDs)
}

// pipePeer returns "sid:<SID>" of the user of the client process.
func pipePeer(c net.Conn) (string, error) {
	f, ok := c.(interface{ Fd() uintptr })
	if !ok {
		return "", fmt.Errorf("pipe connection has no handle")
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(f.Fd()), &pid); err != nil {
		return "", fmt.Errorf("client process ID: %w", err)
	}
	ph, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", fmt.Errorf("open client process: %w", err)
	}
	defer windows.CloseHandle(ph)
	var tok windows.Token
	if err := windows.OpenProcessToken(ph, windows.TOKEN_QUERY, &tok); err != nil {
		return "", fmt.Errorf("client process token: %w", err)
	}
	defer tok.Close()
	tu, err := tok.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("client user: %w", err)
	}
	return "sid:" + tu.User.Sid.String(), nil
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
	return ln, pipePeer, nil
}
