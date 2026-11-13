// SPDX-License-Identifier: AGPL-3.0-or-later

// Package testconn is the "Test connection (simulator)" Transport of
// VoltioVolt (VLT-IS-010 TASK 03-00-00-004, RQ-02, OD-44, OD-45, OD-47).
//
// It dials the local stream of vlt-simulator (a Windows named pipe or a
// Unix socket, never TCP) and wraps the connection as a transport.Stream,
// so Exchange is the normal READ path (SEC-037) and Stream.Control is the
// CONTROL path. CONTROL reaches the simulator only, never a real port.
package testconn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// DialTimeout is the time Open waits for the simulator. On Windows a
// missing named pipe would otherwise block. After a failure the Device
// Manager retries every device.SimulatorRetry (5 seconds).
const DialTimeout = time.Second

// New returns a Transport that dials the default OD-44 path.
func New(opts transport.Options) transport.Transport {
	return NewAt(server.Path(), opts)
}

// NewAt returns a Transport that dials path.
func NewAt(path string, opts transport.Options) transport.Transport {
	return transport.NewStream(func(ctx context.Context) (transport.Port, error) {
		return dialPort(ctx, path)
	}, opts)
}

// Candidates returns the single test-connection candidate of OD-44. No
// real port is listed (SEC-099).
func Candidates(opts transport.Options) []device.Candidate {
	path := server.Path()
	return []device.Candidate{{
		Transport: device.TransportTest,
		Port:      path,
		New:       func() transport.Transport { return NewAt(path, opts) },
	}}
}

func dialPort(ctx context.Context, path string) (transport.Port, error) {
	if path == "" {
		path = server.Path()
	}
	dctx, cancel := context.WithTimeout(ctx, DialTimeout)
	defer cancel()
	c, err := server.DialPath(dctx, path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", device.ErrSimulatorNotFound, err)
	}
	return &connPort{Conn: c}, nil
}

// connPort is a transport.Port on a local stream (OD-45: the same bytes
// as a serial line).
type connPort struct {
	net.Conn
}

func (p *connPort) Read(b []byte, timeout time.Duration) (int, error) {
	if timeout > 0 {
		_ = p.Conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		_ = p.Conn.SetReadDeadline(time.Time{})
	}
	n, err := p.Conn.Read(b)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return 0, nil
		}
		if errors.Is(err, io.EOF) && n > 0 {
			return n, nil
		}
		return n, err
	}
	return n, nil
}

func (p *connPort) Write(b []byte) (int, error) {
	_ = p.Conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return p.Conn.Write(b)
}

func (p *connPort) Close() error { return p.Conn.Close() }
