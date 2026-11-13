// SPDX-License-Identifier: AGPL-3.0-or-later

// Command vlt-simulator is the VoltioVolt UPS simulator console (RQ-01,
// VLT-IS-010 TASK 03-00-00-003). It listens on the local stream of OD-44
// (a Windows named pipe or a Unix socket). It does not use TCP and it
// does not change the operating system (SEC-102).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/abuzer1495/voltiovolt/internal/sim"
	"github.com/abuzer1495/voltiovolt/internal/sim/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, nil))
}

// newScreen, when not nil, is used instead of tcell.NewScreen (tests).
func run(ctx context.Context, args []string, stdout, stderr io.Writer, newScreen func() (tcell.Screen, error)) int {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	ln, err := server.ListenExclusive()
	if err != nil {
		if errors.Is(err, server.ErrAlreadyRunning) {
			fmt.Fprintln(stderr, cfg.lang.AlreadyRuns)
			return 1
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer ln.Close()

	m := sim.NewConsoleModel(cfg.dialect)
	srv := server.New(m)
	r := NewRunner()
	ss := newSession(m, srv, r, cfg.lang, server.Path())
	if cfg.scenario != 0 {
		if cfg.headless && cfg.scenarioDelay > 0 {
			go func() {
				timer := time.NewTimer(cfg.scenarioDelay)
				defer timer.Stop()
				select {
				case <-timer.C:
					ss.applyScenario(cfg.scenario)
				case <-ctx.Done():
				}
			}()
		} else {
			ss.applyScenario(cfg.scenario)
		}
	}
	go func() { _ = srv.Serve(ctx, ln) }()

	if cfg.headless {
		fmt.Fprintf(stdout, "vlt-simulator: %s %s\n", server.Path(), cfg.dialect.DisplayName())
		<-ctx.Done()
		return 0
	}

	if newScreen == nil {
		newScreen = tcell.NewScreen
	}
	sc, err := newScreen()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return ss.loop(ctx, sc)
}
