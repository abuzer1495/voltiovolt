// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// fakeServer is a fake vltd IPC server for the tests (no vltd, no Store).
// It answers each method with a handler and records the requests.
type fakeServer struct {
	t        *testing.T
	mu       sync.Mutex
	handlers map[string]func(params json.RawMessage) (any, *wire.Error)
	reqs     []wire.Request
	conns    []net.Conn
}

func newFake(t *testing.T) *fakeServer {
	f := &fakeServer{t: t, handlers: map[string]func(json.RawMessage) (any, *wire.Error){}}
	f.handle(wire.MethodHello, func(json.RawMessage) (any, *wire.Error) {
		return wire.HelloResult{Version: "0.5.0-test", Protocol: wire.Version}, nil
	})
	f.handle(wire.MethodAuthStatus, func(json.RawMessage) (any, *wire.Error) {
		return wire.AuthStatusResult{PINSet: true}, nil
	})
	f.handle(wire.MethodSubscribe, ok)
	return f
}

func ok(json.RawMessage) (any, *wire.Error) { return wire.OKResult{OK: true}, nil }

func (f *fakeServer) handle(method string, h func(json.RawMessage) (any, *wire.Error)) {
	f.mu.Lock()
	f.handlers[method] = h
	f.mu.Unlock()
}

func (f *fakeServer) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		out = append(out, r.Method)
	}
	return out
}

func (f *fakeServer) last(method string) json.RawMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.reqs) - 1; i >= 0; i-- {
		if f.reqs[i].Method == method {
			return f.reqs[i].Params
		}
	}
	return nil
}

// dial is the Dialer of the Manager: a new in-memory connection.
func (f *fakeServer) dial(context.Context, string) (*client.Client, error) {
	a, b := net.Pipe()
	f.mu.Lock()
	f.conns = append(f.conns, b)
	f.mu.Unlock()
	go f.serve(b)
	return client.New(a), nil
}

var writeMu sync.Mutex

func (f *fakeServer) serve(c net.Conn) {
	for {
		b, err := wire.ReadFrame(c)
		if err != nil {
			return
		}
		r, err := wire.DecodeRequest(b)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.reqs = append(f.reqs, r)
		h := f.handlers[r.Method]
		f.mu.Unlock()
		resp := wire.Response{V: wire.Version, ID: *r.ID}
		if h == nil {
			resp.Error = &wire.Error{Code: wire.CodeUnknownMethod, Message: "unknown method"}
		} else if res, werr := h(r.Params); werr != nil {
			resp.Error = werr
		} else {
			resp.Result, _ = json.Marshal(res)
		}
		writeMu.Lock()
		_ = wire.WriteFrame(c, resp)
		writeMu.Unlock()
	}
}

// push sends a push message on all connections.
func (f *fakeServer) push(name string, data any) {
	raw, _ := json.Marshal(data)
	f.mu.Lock()
	cs := append([]net.Conn(nil), f.conns...)
	f.mu.Unlock()
	for _, c := range cs {
		writeMu.Lock()
		_ = wire.WriteFrame(c, wire.Push{V: wire.Version, Push: name, Data: raw})
		writeMu.Unlock()
	}
}

// drop closes all connections (vltd stops).
func (f *fakeServer) drop() {
	f.mu.Lock()
	cs := f.conns
	f.conns = nil
	f.mu.Unlock()
	for _, c := range cs {
		c.Close()
	}
}

// recorder is a fake Wails event emitter.
type recorder struct {
	mu  sync.Mutex
	evs []struct {
		name string
		data any
	}
}

func (r *recorder) Emit(name string, data any) {
	r.mu.Lock()
	r.evs = append(r.evs, struct {
		name string
		data any
	}{name, data})
	r.mu.Unlock()
}

func (r *recorder) find(name string, match func(any) bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.evs {
		if e.name == name && (match == nil || match(e.data)) {
			return true
		}
	}
	return false
}

func waitFor(t *testing.T, what string, f func() bool) {
	t.Helper()
	dl := time.Now().Add(5 * time.Second)
	for time.Now().Before(dl) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", what)
}

// start runs a Manager on the fake server.
func start(t *testing.T, f *fakeServer) (*Manager, *API, *recorder) {
	t.Helper()
	rec := &recorder{}
	m := NewManager("", rec, nil)
	m.dial = f.dial
	m.retry = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); m.Close(); <-done })
	a := NewAPI(m)
	waitFor(t, "connected", func() bool { return a.Connection().State == ConnConnected })
	return m, a, rec
}
