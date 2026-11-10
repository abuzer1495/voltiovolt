// SPDX-License-Identifier: AGPL-3.0-or-later

// Package shutdown is the OS shutdown executor of vltd (VLT-IS-004 TASK
// 03-00-00-007). It turns off (or hibernates) the local computer.
//
// SEC-092: only the Policy Engine (internal/policy) calls Shutdown and
// Hibernate, and only cmd/vltd imports this package (to give the executor
// to the Policy Engine). The test TestOnlyPolicyCallsExecutor checks this
// with go/ast. No IPC method calls the executor.
package shutdown

import (
	"context"
	"errors"
	"log/slog"
)

// Actions in the audit entries.
const (
	ActionShutdown  = "shutdown"
	ActionHibernate = "hibernate"
)

// ErrNotSupported is returned on an OS without an executor.
var ErrNotSupported = errors.New("shutdown: not supported on this OS")

// Auditor writes audit entries (internal/audit.Log).
type Auditor interface {
	Record(ctx context.Context, actor, action, target, result string, details map[string]any) error
}

// Options are the executor options.
type Options struct {
	// ForceClose returns the Setting policy.force_close_applications
	// (Windows). nil means on.
	ForceClose func() bool
	Audit      Auditor
	Log        *slog.Logger
}

// Executor is the OS executor. It implements policy.Executor.
type Executor struct {
	opts Options
	os   osAPI
}

// New returns the OS executor.
func New(o Options) *Executor {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	return &Executor{opts: o, os: defaultOS()}
}

func (e *Executor) forceClose() bool {
	if e.opts.ForceClose == nil {
		return true
	}
	return e.opts.ForceClose()
}

func (e *Executor) record(ctx context.Context, action, result string, d map[string]any) {
	if e.opts.Audit == nil {
		return
	}
	if err := e.opts.Audit.Record(ctx, "system", action, "os", result, d); err != nil {
		e.opts.Log.Error("shutdown: audit write failed", "action", action, "err", err)
	}
}

// Shutdown turns off the computer.
func (e *Executor) Shutdown(ctx context.Context, reason string) error {
	return e.os.shutdown(ctx, e, reason)
}

// Hibernate hibernates the computer. If hibernation is not available, it
// turns off the computer and writes an audit entry.
func (e *Executor) Hibernate(ctx context.Context, reason string) error {
	return e.os.hibernate(ctx, e, reason)
}

// osAPI is the OS part. Tests replace it with fakes; the fakes never call
// the OS.
type osAPI interface {
	shutdown(ctx context.Context, e *Executor, reason string) error
	hibernate(ctx context.Context, e *Executor, reason string) error
}
