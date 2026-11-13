// SPDX-License-Identifier: AGPL-3.0-or-later

package demo

import (
	"context"

	"github.com/abuzer1495/voltiovolt/internal/policy"
)

// Executor is a policy.Executor that is a dry run while the Session is
// active. It never calls the real executor in demo mode.
type Executor struct {
	Real policy.Executor
	Sess *Session
}

func (e *Executor) Shutdown(ctx context.Context, reason string) error {
	if e.Sess != nil && e.Sess.Active() {
		e.Sess.SetShutdownMessage(ShutdownMessageEN)
		return nil
	}
	if e.Real == nil {
		return nil
	}
	return e.Real.Shutdown(ctx, reason)
}

func (e *Executor) Hibernate(ctx context.Context, reason string) error {
	if e.Sess != nil && e.Sess.Active() {
		e.Sess.SetShutdownMessage(ShutdownMessageEN)
		return nil
	}
	if e.Real == nil {
		return nil
	}
	return e.Real.Hibernate(ctx, reason)
}

func (e *Executor) CanShutdown(ctx context.Context) error {
	if e.Sess != nil && e.Sess.Active() {
		return nil
	}
	if c, ok := e.Real.(policy.ShutdownChecker); ok {
		return c.CanShutdown(ctx)
	}
	return nil
}
