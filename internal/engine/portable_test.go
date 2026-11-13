// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client/typed"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

func startPortableTest(t *testing.T, dir string, opts ...func(*Config)) (*Daemon, *ipc.PipeListener) {
	t.Helper()
	ln := ipc.NewPipeListener()
	cfg := Config{
		DataDir:    dir,
		CheckPerms: false,
		Provider:   func(string, []byte) vault.KeyProvider { return &testOnlyDEKProvider{dek: testOnlyDEK()} },
		Listen:     func() (net.Listener, ipc.PeerCheck, error) { return ln, ipc.InProcessCheck, nil },
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:    "portable-test",
	}
	for _, o := range opts {
		o(&cfg)
	}
	d := New(cfg)
	if err := d.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopTestDaemon(t, d); ln.Close() })
	return d, ln
}

func TestPortableInProcessHello(t *testing.T) {
	s := sim.New(sim.NewModel(sim.ProfileQ1))
	ex := &fakeExecutor{ch: make(chan string, 4)}
	_, ln := startPortableTest(t, dataDir(t), withSim(s, ex))
	conn, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	c := typed.New(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := c.Hello(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h.Version != "portable-test" {
		t.Fatalf("version %q", h.Version)
	}
	if ln.Addr().Network() != "pipe" {
		t.Fatalf("network %s (named pipe or Unix socket is not permitted)", ln.Addr().Network())
	}
}

func TestPortableSkipsPermCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux mode check")
	}
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Service mode fails with 0755 (TestPermissionCheckStopsStart). Portable
	// mode does not check or change the ACL or the mode (SEC-102).
	_, _ = startPortableTest(t, dir)
}
