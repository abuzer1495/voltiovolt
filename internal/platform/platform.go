// SPDX-License-Identifier: AGPL-3.0-or-later

// Package platform runs vltd as a service (VLT-IS-003 TASK 03-00-00-008).
// Linux: systemd Type=notify (sd_notify READY=1 and STOPPING=1). Windows:
// golang.org/x/sys/windows/svc. Outside a service manager, Run runs in the
// console and stops at SIGINT or SIGTERM (Ctrl+C on Windows).
package platform

import (
	"context"
	"time"
)

// StopTimeout is the maximum time for the stop sequence (step 2).
const StopTimeout = 10 * time.Second

// Service is what Run runs.
type Service interface {
	// Start runs the start sequence. When it returns nil, the service is
	// ready.
	Start(ctx context.Context) error
	// Stop runs the stop sequence. ctx ends after StopTimeout.
	Stop(ctx context.Context) error
}
