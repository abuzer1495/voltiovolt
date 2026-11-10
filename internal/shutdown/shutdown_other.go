// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows && !linux

package shutdown

import "context"

type otherOS struct{}

func defaultOS() osAPI { return otherOS{} }

func (otherOS) shutdown(ctx context.Context, e *Executor, _ string) error {
	e.record(ctx, "os_shutdown_request", "failure", map[string]any{"error": ErrNotSupported.Error()})
	return ErrNotSupported
}

func (otherOS) hibernate(ctx context.Context, e *Executor, _ string) error {
	e.record(ctx, "os_hibernate_request", "failure", map[string]any{"error": ErrNotSupported.Error()})
	return ErrNotSupported
}
