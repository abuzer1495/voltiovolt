// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// TestConnectHelloSubscribe is TASK 03-00-00-003 step 1.1: system.hello at
// start, then status.subscribe; the push messages go to the front end as
// Wails events.
func TestConnectHelloSubscribe(t *testing.T) {
	f := newFake(t)
	_, a, rec := start(t, f)
	waitFor(t, "subscribe", func() bool { return strings.Contains(strings.Join(f.methods(), ","), wire.MethodSubscribe) })
	ms := f.methods()
	if ms[0] != wire.MethodHello {
		t.Fatalf("first request %v", ms)
	}
	if c := a.Connection(); c.Version != "0.5.0-test" || c.Protocol != wire.Version {
		t.Fatalf("%+v", c)
	}
	f.push(wire.PushStatus, map[string]any{"device": map[string]any{"state": "connected"}, "ups": map[string]any{"on_battery": true}})
	waitFor(t, "status event", func() bool { return rec.find(EventStatus, nil) })
	waitFor(t, "tray battery", func() bool {
		return rec.find(EventTray, func(d any) bool { return d.(TrayState).State == TrayBattery })
	})
	if _, ok := a.Last()["status"]; !ok {
		t.Fatal("no last status")
	}
}

// TestReconnect is step 1.3: a new connection after the connection ends.
func TestReconnect(t *testing.T) {
	f := newFake(t)
	_, a, rec := start(t, f)
	f.drop()
	waitFor(t, "lost", func() bool {
		return rec.find(EventConn, func(d any) bool { return d.(ConnInfo).Reason == ReasonLost })
	})
	waitFor(t, "connected again", func() bool {
		n := 0
		for _, m := range f.methods() {
			if m == wire.MethodHello {
				n++
			}
		}
		return n >= 2 && a.Connection().State == ConnConnected
	})
}

// TestRetryInterval is step 1.3: 5 seconds.
func TestRetryInterval(t *testing.T) {
	if RetryInterval.Seconds() != 5 {
		t.Fatal(RetryInterval)
	}
}

// TestReasons is step 1.2: the failure reasons.
func TestReasons(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{&fs.PathError{Op: "dial", Path: "/run/vltd/vltd.sock", Err: syscall.EACCES}, ReasonAccessDenied},
		{fmt.Errorf("dial: %w", fs.ErrPermission), ReasonAccessDenied},
		{&fs.PathError{Op: "dial", Path: "x", Err: syscall.ENOENT}, ReasonNotRunning},
		{fmt.Errorf("x: %w", syscall.ECONNREFUSED), ReasonNotRunning},
		{&wire.Error{Code: "bad_request"}, ReasonProtocol},
		{client.ErrProtocol, ReasonProtocol},
		{errors.New("open \\\\.\\pipe\\vltd: Access is denied."), ReasonAccessDenied},
		{errors.New("other"), ReasonOther},
	} {
		if got := Reason(c.err); got != c.want {
			t.Errorf("%v: %s", c.err, got)
		}
	}
}

// TestDialFailure: the reason goes to the front end; the Manager tries
// again.
func TestDialFailure(t *testing.T) {
	rec := &recorder{}
	m := NewManager("", rec, nil)
	var n atomic.Int32
	m.dial = func(context.Context, string) (*client.Client, error) {
		n.Add(1)
		return nil, &fs.PathError{Op: "dial", Path: "/run/vltd/vltd.sock", Err: syscall.EACCES}
	}
	m.retry = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	waitFor(t, "denied", func() bool {
		return rec.find(EventConn, func(d any) bool { return d.(ConnInfo).Reason == ReasonAccessDenied && !d.(ConnInfo).NextTry.IsZero() })
	})
	waitFor(t, "retry", func() bool { return n.Load() >= 2 })
	cancel()
	<-done
	if NewAPI(m).Connection().State != ConnDisconnected {
		t.Fatal("state")
	}
}

// TestProtocolMismatch: a vltd with another protocol version.
func TestProtocolMismatch(t *testing.T) {
	f := newFake(t)
	f.handle(wire.MethodHello, func(json.RawMessage) (any, *wire.Error) {
		return wire.HelloResult{Version: "9", Protocol: 99}, nil
	})
	rec := &recorder{}
	m := NewManager("", rec, nil)
	m.dial = f.dial
	if _, err := m.connect(context.Background()); err == nil {
		t.Fatal("no error")
	}
	if c := NewAPI(m).Connection(); c.State != ConnDisconnected || c.Reason != ReasonProtocol {
		t.Fatalf("%+v", c)
	}
}

// TestClose: Close closes the IPC connection only. The GUI sends no request
// to stop vltd (there is no such method).
func TestClose(t *testing.T) {
	f := newFake(t)
	m, _, _ := start(t, f)
	m.Close()
	for _, x := range f.methods() {
		if !strings.HasPrefix(x, "system.hello") && !strings.HasPrefix(x, "auth.") && x != wire.MethodSubscribe {
			t.Fatalf("request %s at close", x)
		}
	}
}

// TestCSPText is SEC-095: the CSP constant and the meta element of
// web/index.html have the same text.
func TestCSPText(t *testing.T) {
	want := "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"
	if CSP != want {
		t.Fatalf("CSP %q", CSP)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `content="`+CSP+`"`) {
		t.Fatal("web/index.html has no meta element with the CSP")
	}
}

// TestNoStoreImports is TASK 03-00-00-002 step 4.1: the Go side of the VLT
// GUI does not import (also not indirectly) the Store, the Vault or SQLite.
func TestNoStoreImports(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	for _, goos := range []string{"linux", "windows"} {
		cmd := exec.Command("go", "list", "-deps", ".")
		cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v %s", goos, err, out)
		}
		for _, p := range strings.Fields(string(out)) {
			if strings.HasSuffix(p, "/internal/store") || strings.HasSuffix(p, "/internal/vault") ||
				strings.Contains(p, "sqlite") || strings.HasSuffix(p, "/internal/policy") || strings.HasSuffix(p, "/internal/ipc/client/typed") {
				t.Errorf("%s: internal/gui depends on %s", goos, p)
			}
		}
	}
}
