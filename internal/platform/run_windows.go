// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"golang.org/x/sys/windows/svc"
)

// Run runs s. If the Service Control Manager started the process, the
// service state is "start pending", then "running" after Start, and
// "stop pending" during Stop. Otherwise s runs in the console until Ctrl+C.
func Run(name string, s Service, log *slog.Logger) error {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isSvc {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		if err := s.Start(ctx); err != nil {
			return fmt.Errorf("%s: start: %w", name, err)
		}
		<-ctx.Done()
		sctx, scancel := context.WithTimeout(context.Background(), StopTimeout)
		defer scancel()
		return s.Stop(sctx)
	}
	h := &handler{s: s, log: log}
	if err := svc.Run(name, h); err != nil {
		return err
	}
	return h.err
}

// IsService reports whether the Service Control Manager started the process.
func IsService() bool {
	ok, _ := svc.IsWindowsService()
	return ok
}

type handler struct {
	s   Service
	log *slog.Logger
	err error
}

func (h *handler) Execute(_ []string, r <-chan svc.ChangeRequest, st chan<- svc.Status) (bool, uint32) {
	st <- svc.Status{State: svc.StartPending, WaitHint: 30000}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := h.s.Start(ctx); err != nil {
		h.err = err
		h.log.Error("start failed", "err", err)
		return true, 1
	}
	st <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	h.log.Info("service state running")
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			st <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			st <- svc.Status{State: svc.StopPending, WaitHint: uint32(StopTimeout.Milliseconds())}
			sctx, scancel := context.WithTimeout(context.Background(), StopTimeout)
			h.err = h.s.Stop(sctx)
			scancel()
			return false, 0
		}
	}
	return false, 0
}
