// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/engine"
	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/transport"
)

// SelfTest starts the portable engine with the UPS simulator, connects
// through net.Pipe, runs Hello, writes a result file and stops. It is the
// CI start test of VLT-IS-008 04-00-00. No real hardware, no service.
func SelfTest(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ln := ipc.NewPipeListener()
	defer ln.Close()
	model := sim.NewModel(sim.ProfileQ1)
	s := sim.New(model)
	opts := transport.Options{Timeout: 30 * time.Millisecond, MinInterval: time.Nanosecond, FlushTime: time.Millisecond}
	d, err := startEngine(dir, log, nil, func() (net.Listener, ipc.PeerCheck, error) {
		return ln, ipc.InProcessCheck, nil
	}, func(c *engine.Config) {
		c.Lister = func(context.Context, device.Config) ([]device.Candidate, error) {
			return []device.Candidate{{Transport: device.TransportSerial, Port: "sim0",
				New: func() transport.Transport { return s.StreamTransport(opts) }}}, nil
		}
		c.MakeExecutor = func(func() bool, *audit.Log) policy.Executor { return &policy.DryRun{} }
	})
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = d.Stop(ctx)
	}()
	conn, err := ln.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	cl := client.New(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := cl.Hello(ctx)
	if err != nil {
		return fmt.Errorf("hello: %w", err)
	}
	result := filepath.Join(dir, "self-test-result.txt")
	body := fmt.Sprintf("ok version=%s protocol=%d network=%s\n", h.Version, h.Protocol, ln.Addr().Network())
	if err := os.WriteFile(result, []byte(body), 0o600); err != nil {
		return err
	}
	fmt.Print(body)
	return nil
}
