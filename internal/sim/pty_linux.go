// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package sim

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// PTY is a pty endpoint of the simulator for integration tests of the
// serial Transport. Linux only.
type PTY struct {
	Path   string // path of the pty that the serial Transport opens
	master *os.File
	slave  *os.File
	done   chan struct{}
}

// makeRaw sets raw mode on the pty: no echo, no CR translation.
func makeRaw(f *os.File) error {
	fd := int(f.Fd())
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB
	t.Cflag |= unix.CS8
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
	return unix.IoctlSetTermios(fd, unix.TCSETS, t)
}

// StartPTY starts a pty endpoint for the simulator s. It stops when ctx is
// done or when Close is called.
func StartPTY(ctx context.Context, s *Sim) (*PTY, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return nil, err
	}
	if err := makeRaw(slave); err != nil {
		_ = master.Close()
		_ = slave.Close()
		return nil, err
	}
	p := &PTY{Path: slave.Name(), master: master, slave: slave, done: make(chan struct{})}
	go p.serve(ctx, s)
	return p, nil
}

func (p *PTY) serve(ctx context.Context, s *Sim) {
	defer close(p.done)
	buf := make([]byte, 256)
	go func() {
		<-ctx.Done()
		_ = p.master.SetReadDeadline(time.Now())
	}()
	for {
		_ = p.master.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
		n, err := p.master.Read(buf)
		if n > 0 {
			s.Feed(buf[:n])
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			return
		}
		for {
			out := s.take(256)
			if len(out) == 0 {
				break
			}
			if _, err := p.master.Write(out); err != nil {
				return
			}
		}
	}
}

// Close stops the endpoint.
func (p *PTY) Close() error {
	err := p.master.Close()
	_ = p.slave.Close()
	<-p.done
	return err
}
