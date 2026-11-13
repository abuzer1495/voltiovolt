// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"errors"
	"net"
	"sync"
	"time"
)

// PipeListener is an in-process net.Listener. Each Dial makes a net.Pipe
// pair. Portable mode uses this so that VoltioVolt opens no named pipe and
// no Unix socket (VLT-IS-008 TASK 03-00-00-001 step 3).
type PipeListener struct {
	ch     chan net.Conn
	once   sync.Once
	closed chan struct{}
}

// NewPipeListener returns a listener that is not yet closed.
func NewPipeListener() *PipeListener {
	return &PipeListener{ch: make(chan net.Conn), closed: make(chan struct{})}
}

// Accept waits for a Dial.
func (l *PipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

// Close stops Accept and Dial.
func (l *PipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

// Addr returns a dummy address.
func (l *PipeListener) Addr() net.Addr { return inProcessAddr{} }

type inProcessAddr struct{}

func (inProcessAddr) Network() string { return "pipe" }
func (inProcessAddr) String() string  { return "in-process" }

// Dial connects to the listener with net.Pipe. It does not use a named
// pipe or a Unix socket.
func (l *PipeListener) Dial() (net.Conn, error) {
	a, b := net.Pipe()
	select {
	case l.ch <- b:
		return a, nil
	case <-l.closed:
		a.Close()
		b.Close()
		return nil, net.ErrClosed
	case <-time.After(10 * time.Second):
		a.Close()
		b.Close()
		return nil, errors.New("pipe dial: no accept")
	}
}

// PeerInProcess is the peer identity of InProcessCheck (portable mode).
const PeerInProcess = "in-process"

// InProcessCheck is the PeerCheck of portable mode: the peer is the same
// process, so the check always passes.
func InProcessCheck(net.Conn) (string, error) { return PeerInProcess, nil }
