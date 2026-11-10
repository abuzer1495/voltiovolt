// SPDX-License-Identifier: AGPL-3.0-or-later

package sim

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// ProbeList is the Probe List of VLT-IS-006 TASK 03-00-00-003 step 4, in order.
var ProbeList = []string{"Q1", "F", "I", "QS", "QGS", "QMOD", "QBV", "QWS", "QFLAG", "QRI", "QID", "QVFW", "QMD"}

// WatchList is the Watch List of VLT-IS-002 03-50-01.
var WatchList = []string{"Q1", "QS", "QGS", "QMOD", "QBV", "QWS"}

// Sim is a simulated UPS endpoint. It records each received command.
// It implements transport.Port (byte stream) and transport.ReportDevice
// (HID reports).
type Sim struct {
	Model *Model

	mu       sync.Mutex
	inbuf    []byte   // bytes received and not yet ended by CR
	pending  []byte   // bytes to send to the host
	received []string // each received command, without CR
	reportN  int      // HID report counter for FaultBrokenRpt
	closed   bool
}

// New returns a simulator for a model.
func New(m *Model) *Sim { return &Sim{Model: m} }

// InjectStale puts old bytes in the output buffer.
func (s *Sim) InjectStale(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = append(s.pending, b...)
}

// Received returns a copy of the received commands.
func (s *Sim) Received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.received...)
}

// NotIn returns each received command that is not in the list.
func (s *Sim) NotIn(list []string) []string {
	ok := map[string]bool{}
	for _, c := range list {
		ok[c] = true
	}
	var bad []string
	for _, c := range s.Received() {
		if !ok[c] {
			bad = append(bad, c)
		}
	}
	return bad
}

// Feed gives bytes from the host to the simulator. Each CR ends one
// command. A CR alone is recorded as an empty command.
func (s *Sim) Feed(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inbuf = append(s.inbuf, p...)
	for {
		i := bytes.IndexByte(s.inbuf, '\r')
		if i < 0 {
			return
		}
		cmd := string(s.inbuf[:i])
		s.inbuf = s.inbuf[i+1:]
		s.received = append(s.received, cmd)
		s.pending = append(s.pending, s.Model.Respond(cmd)...)
		if s.Model.HasFault(FaultStale) {
			s.pending = append(s.pending, []byte("(STALE")...)
		}
	}
}

// Write implements transport.Port.
func (s *Sim) Write(p []byte) (int, error) {
	if s.isClosed() {
		return 0, transport.ErrClosed
	}
	s.Feed(p)
	return len(p), nil
}

func (s *Sim) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// take removes at most n bytes from the output buffer.
func (s *Sim) take(n int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Model.HasFault(FaultSplit) && n > 3 {
		n = 3
	}
	n = min(n, len(s.pending))
	out := append([]byte(nil), s.pending[:n]...)
	s.pending = s.pending[n:]
	return out
}

// Read implements transport.Port.
func (s *Sim) Read(p []byte, timeout time.Duration) (int, error) {
	if s.isClosed() {
		return 0, transport.ErrClosed
	}
	b := s.take(len(p))
	if len(b) == 0 {
		time.Sleep(min(timeout, time.Millisecond))
		return 0, nil
	}
	return copy(p, b), nil
}

// Close implements transport.Port.
func (s *Sim) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// HIDLengths are the simulated report lengths: 8 bytes, no report IDs.
// These values are not the values of the Reference UPS (UNVERIFIED).
var HIDLengths = transport.ReportLengths{Input: 8, Output: 8}

// WriteReport implements transport.ReportDevice. The first byte is the
// report ID and is not data.
func (s *Sim) WriteReport(r []byte) (int, error) {
	if s.isClosed() {
		return 0, transport.ErrClosed
	}
	if len(r) != 1+HIDLengths.Output {
		return 0, transport.ErrClosed
	}
	data := r[1:]
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		data = data[:i+1] // zero fill after the CR is not data
	} else {
		data = bytes.TrimRight(data, "\x00")
	}
	s.Feed(data)
	return len(r), nil
}

// ReadReport implements transport.ReportDevice.
func (s *Sim) ReadReport(p []byte, timeout time.Duration) (int, error) {
	if s.isClosed() {
		return 0, transport.ErrClosed
	}
	b := s.take(HIDLengths.Input)
	if len(b) == 0 {
		time.Sleep(min(timeout, time.Millisecond))
		return 0, nil
	}
	s.mu.Lock()
	s.reportN++
	broken := s.Model.HasFault(FaultBrokenRpt) && s.reportN%2 == 1 && len(b) > 1
	s.mu.Unlock()
	if broken {
		b = b[:len(b)-1] // one byte is missing
	}
	r := make([]byte, HIDLengths.Input)
	copy(r, b)
	return copy(p, r), nil
}

// open marks the simulator as open. A simulator can be opened again after
// Close, like a real port.
func (s *Sim) open() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = false
}

// StreamTransport returns a byte-stream Transport over the simulator.
func (s *Sim) StreamTransport(opts transport.Options) *transport.Stream {
	return transport.NewStream(func(context.Context) (transport.Port, error) {
		s.open()
		return s, nil
	}, opts)
}

// HIDTransport returns a HID Transport over the simulator.
func (s *Sim) HIDTransport(opts transport.Options) (*transport.Stream, error) {
	port, err := transport.NewHIDPort(s, HIDLengths)
	if err != nil {
		return nil, err
	}
	return transport.NewStream(func(context.Context) (transport.Port, error) {
		s.open()
		return port, nil
	}, opts), nil
}
