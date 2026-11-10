// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"context"
	"sync"
	"time"
)

// FakePort is a Port for tests. Respond gets each written request and
// returns the bytes that the fake UPS sends back. Pending bytes are old
// input data before the next request.
type FakePort struct {
	mu        sync.Mutex
	Respond   func(req []byte) []byte
	MaxWrite  int // if more than 0, Write writes only MaxWrite bytes
	Chunk     int // if more than 0, Read returns at most Chunk bytes
	pending   []byte
	Written   [][]byte
	Closed    bool
	WriteHits int
}

// Inject puts bytes in the input buffer (old input data).
func (f *FakePort) Inject(b []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = append(f.pending, b...)
}

// Write implements Port.
func (f *FakePort) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.WriteHits++
	n := len(p)
	if f.MaxWrite > 0 && n > f.MaxWrite {
		n = f.MaxWrite
	}
	f.Written = append(f.Written, append([]byte(nil), p[:n]...))
	if n == len(p) && f.Respond != nil {
		f.pending = append(f.pending, f.Respond(append([]byte(nil), p...))...)
	}
	return n, nil
}

// Read implements Port. It returns immediately with 0 bytes on no data.
func (f *FakePort) Read(p []byte, timeout time.Duration) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pending) == 0 {
		f.mu.Unlock()
		time.Sleep(min(timeout, time.Millisecond))
		f.mu.Lock()
		return 0, nil
	}
	n := len(f.pending)
	if f.Chunk > 0 && n > f.Chunk {
		n = f.Chunk
	}
	n = copy(p, f.pending[:n])
	f.pending = f.pending[n:]
	return n, nil
}

// Close implements Port.
func (f *FakePort) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Closed = true
	return nil
}

// FakeOpener returns an Opener for a FakePort.
func FakeOpener(p Port) Opener {
	return func(context.Context) (Port, error) { return p, nil }
}

// NewFake returns a Stream over a FakePort with short timing for tests.
func NewFake(p Port) *Stream {
	return NewStream(FakeOpener(p), Options{
		Timeout:     50 * time.Millisecond,
		MinInterval: time.Millisecond,
		FlushTime:   5 * time.Millisecond,
	})
}
