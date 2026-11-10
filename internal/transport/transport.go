// SPDX-License-Identifier: AGPL-3.0-or-later

// Package transport contains the physical links to the UPS: USB HID and
// RS-232 serial. A Transport sends one request and returns one response.
//
// Exchange accepts only the request bytes of a READ command in the command
// registry (SEC-037). It never sends a CR alone and never sends the remaining
// bytes of a partial request (SEC-038).
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/control"
	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

// Default values.
const (
	DefaultTimeout     = 3 * time.Second
	DefaultMinInterval = 500 * time.Millisecond
	FlushTime          = 100 * time.Millisecond
)

// Errors.
var (
	ErrClosed        = errors.New("transport: closed")
	ErrPartialWrite  = errors.New("transport: partial write; transport closed (SEC-038)")
	ErrTimeout       = errors.New("transport: timeout before CR")
	ErrTooLong       = errors.New("transport: response is longer than the maximum")
	ErrNotPermitted  = errors.New("transport: request is not a READ command in the registry (SEC-037)")
	ErrAlreadyOpened = errors.New("transport: already open")
)

// Response is one raw response.
type Response struct {
	Raw    []byte // raw bytes, CR included when received
	Millis int64  // time from the start of the write to the CR, in milliseconds
}

// Transport is the interface of a physical link.
type Transport interface {
	Open(ctx context.Context) error
	Close() error
	// Exchange sends one request and returns one response. Only one
	// Exchange runs at a time.
	Exchange(ctx context.Context, req []byte) (Response, error)
}

// Port is a byte stream to the UPS.
type Port interface {
	// Write writes p. It returns the number of bytes written.
	Write(p []byte) (int, error)
	// Read reads the bytes that are available. It waits for a maximum of
	// timeout. On timeout it returns 0 and a nil error.
	Read(p []byte, timeout time.Duration) (int, error)
	Close() error
}

// Opener opens a Port.
type Opener func(ctx context.Context) (Port, error)

// Options are the timing options of a Stream. Zero values select defaults.
type Options struct {
	Timeout     time.Duration
	MinInterval time.Duration
	FlushTime   time.Duration
	Now         func() time.Time
	// Sleep waits for d or until ctx is done.
	Sleep func(ctx context.Context, d time.Duration) error
}

func (o Options) withDefaults() Options {
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.MinInterval <= 0 {
		o.MinInterval = DefaultMinInterval
	}
	if o.FlushTime <= 0 {
		o.FlushTime = FlushTime
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Sleep == nil {
		o.Sleep = sleepCtx
	}
	return o
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Stream is a Transport over a Port.
type Stream struct {
	mu   sync.Mutex
	open Opener
	opts Options
	port Port
	last time.Time
}

// NewStream returns a Transport that uses open to get its Port.
func NewStream(open Opener, opts Options) *Stream {
	return &Stream{open: open, opts: opts.withDefaults()}
}

// Open opens the Port.
func (s *Stream) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.port != nil {
		return ErrAlreadyOpened
	}
	p, err := s.open(ctx)
	if err != nil {
		return err
	}
	s.port = p
	return nil
}

// Close closes the Port.
func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked()
}

func (s *Stream) closeLocked() error {
	if s.port == nil {
		return nil
	}
	err := s.port.Close()
	s.port = nil
	return err
}

// CheckRequest returns an error if req is not the exact request of a READ
// command in the registry.
func CheckRequest(req []byte) error {
	if len(req) < 2 || req[len(req)-1] != '\r' || bytes.IndexByte(req[:len(req)-1], '\r') >= 0 {
		return ErrNotPermitted
	}
	want, err := dialect.EncodeRead(string(req[:len(req)-1]))
	if err != nil || !bytes.Equal(want, req) {
		return ErrNotPermitted
	}
	return nil
}

// Exchange sends one request and returns one response.
func (s *Stream) Exchange(ctx context.Context, req []byte) (Response, error) {
	if err := CheckRequest(req); err != nil {
		return Response{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.port == nil {
		return Response{}, ErrClosed
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.opts.Timeout)
		defer cancel()
	}
	if !s.last.IsZero() {
		if wait := s.opts.MinInterval - s.opts.Now().Sub(s.last); wait > 0 {
			if err := s.opts.Sleep(ctx, wait); err != nil {
				return Response{}, err
			}
		}
	}

	// a. Remove old input data. Read for a maximum of FlushTime.
	buf := make([]byte, 512)
	flushEnd := s.opts.Now().Add(s.opts.FlushTime)
	for {
		left := flushEnd.Sub(s.opts.Now())
		if left <= 0 {
			break
		}
		n, err := s.port.Read(buf, left)
		if err != nil {
			_ = s.closeLocked()
			return Response{}, fmt.Errorf("transport: flush: %w", err)
		}
		if n == 0 {
			break
		}
	}

	// b. and c. Write the full request. A partial write closes the Port.
	start := s.opts.Now()
	s.last = start
	n, err := s.port.Write(req)
	if err != nil || n != len(req) {
		_ = s.closeLocked()
		if err != nil {
			return Response{}, fmt.Errorf("%w: wrote %d of %d bytes: %v", ErrPartialWrite, n, len(req), err)
		}
		return Response{}, fmt.Errorf("%w: wrote %d of %d bytes", ErrPartialWrite, n, len(req))
	}

	// d. Read until CR or until the timeout.
	deadline, _ := ctx.Deadline()
	var out []byte
	for {
		left := deadline.Sub(s.opts.Now())
		if left <= 0 || ctx.Err() != nil {
			return Response{Raw: out, Millis: s.opts.Now().Sub(start).Milliseconds()}, ErrTimeout
		}
		n, err := s.port.Read(buf, left)
		if err != nil {
			_ = s.closeLocked()
			return Response{Raw: out}, fmt.Errorf("transport: read: %w", err)
		}
		if i := bytes.IndexByte(buf[:n], '\r'); i >= 0 {
			out = append(out, buf[:i+1]...)
			if len(out) > dialect.MaxResponseLen {
				return Response{Raw: out}, ErrTooLong
			}
			// e. Return the raw bytes and the time in milliseconds.
			return Response{Raw: out, Millis: s.opts.Now().Sub(start).Milliseconds()}, nil
		}
		out = append(out, buf[:n]...)
		if len(out) > dialect.MaxResponseLen {
			return Response{Raw: out}, ErrTooLong
		}
	}
}

// ControlWait is the maximum wait for a response to a CONTROL command.
// The response is not read (VLT-REF-001 section 5: "Not read").
const ControlWait = 300 * time.Millisecond

// ErrNotControl is returned when a CONTROL command did not come from
// internal/control or its bytes fail control.Check.
var ErrNotControl = errors.New("transport: not a CONTROL command of internal/control (SEC-098)")

// Controller is a Transport that can send a CONTROL command.
type Controller interface {
	Control(ctx context.Context, c control.Command) error
}

// Control sends one CONTROL command of internal/control (VLT-IS-006 TASK
// 03-00-00-005, SEC-098). It accepts only a control.Command that passes
// control.Check; Exchange still accepts only READ commands (SEC-037). It
// keeps the minimum time between requests, removes old input data, writes
// the full command (a partial write closes the Port, SEC-038) and reads a
// response until CR or for a maximum of ControlWait. A missing response is
// not an error.
func (s *Stream) Control(ctx context.Context, c control.Command) error {
	req := c.Bytes()
	if !c.Valid() || control.Check(req) != nil {
		return ErrNotControl
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.port == nil {
		return ErrClosed
	}
	if !s.last.IsZero() {
		if wait := s.opts.MinInterval - s.opts.Now().Sub(s.last); wait > 0 {
			if err := s.opts.Sleep(ctx, wait); err != nil {
				return err
			}
		}
	}
	buf := make([]byte, 512)
	flushEnd := s.opts.Now().Add(s.opts.FlushTime)
	for {
		left := flushEnd.Sub(s.opts.Now())
		if left <= 0 {
			break
		}
		n, err := s.port.Read(buf, left)
		if err != nil {
			_ = s.closeLocked()
			return fmt.Errorf("transport: flush: %w", err)
		}
		if n == 0 {
			break
		}
	}
	s.last = s.opts.Now()
	n, err := s.port.Write(req)
	if err != nil || n != len(req) {
		_ = s.closeLocked()
		return fmt.Errorf("%w: wrote %d of %d bytes", ErrPartialWrite, n, len(req))
	}
	end := s.opts.Now().Add(min(ControlWait, s.opts.Timeout))
	for {
		left := end.Sub(s.opts.Now())
		if left <= 0 || ctx.Err() != nil {
			return nil
		}
		n, err := s.port.Read(buf, left)
		if err != nil {
			_ = s.closeLocked()
			return fmt.Errorf("transport: read: %w", err)
		}
		// Read waited for the time left: no more response bytes.
		if n == 0 || bytes.IndexByte(buf[:n], '\r') >= 0 {
			return nil
		}
	}
}
