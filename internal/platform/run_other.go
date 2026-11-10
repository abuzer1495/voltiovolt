// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-systemd/v22/daemon"
)

// Run starts s, sends READY=1 to systemd, waits for SIGTERM or SIGINT, sends
// STOPPING=1 and stops s in StopTimeout or less.
func Run(name string, s Service, log *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("%s: start: %w", name, err)
	}
	if sent, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		log.Warn("sd_notify READY=1 failed", "err", err)
	} else if sent {
		log.Info("sd_notify READY=1 sent")
	}
	<-ctx.Done()
	log.Info("stop signal received")
	_, _ = daemon.SdNotify(false, daemon.SdNotifyStopping)
	sctx, scancel := context.WithTimeout(context.Background(), StopTimeout)
	defer scancel()
	return s.Stop(sctx)
}

// IsService reports whether systemd started the process (NOTIFY_SOCKET).
func IsService() bool { return os.Getenv("NOTIFY_SOCKET") != "" }
