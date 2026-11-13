// SPDX-License-Identifier: AGPL-3.0-or-later

package policy

import (
	"context"
	"sync"
)

// Executor turns off the computer. internal/shutdown has the OS executor;
// only cmd/vltd and cmd/voltiovolt give it to the Policy Engine (SEC-092).
type Executor interface {
	Shutdown(ctx context.Context, reason string) error
	Hibernate(ctx context.Context, reason string) error
}

// ShutdownChecker reports whether this process can shut down the computer
// (VLT-IS-008 TASK 03-00-00-004). The OS executor implements it. DryRun
// and the test fakes do not: a missing check does not fail the Simulation.
type ShutdownChecker interface {
	CanShutdown(ctx context.Context) error
}

// OSShutdownName is the Simulation step name of the right check.
const OSShutdownName = "OS shutdown"

// OSShutdownStep is the Simulation step "OS shutdown". If Pass is false,
// the policy cannot be activated.
type OSShutdownStep struct {
	Name  string `json:"name"`
	Pass  bool   `json:"pass"`
	Error string `json:"error,omitempty"`
}

// Call is one executor call.
type Call struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// DryRun is an executor that only records the calls. The Simulation uses
// it. It does not call the OS and does not send a command to the UPS.
type DryRun struct {
	mu    sync.Mutex
	calls []Call
}

// Shutdown records a shutdown request.
func (d *DryRun) Shutdown(_ context.Context, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, Call{Action: ActionShutdown, Reason: reason})
	return nil
}

// Hibernate records a hibernate request.
func (d *DryRun) Hibernate(_ context.Context, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, Call{Action: ActionHibernate, Reason: reason})
	return nil
}

// Calls returns a copy of the recorded calls.
func (d *DryRun) Calls() []Call {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Call(nil), d.calls...)
}
