// SPDX-License-Identifier: AGPL-3.0-or-later

package shutdown

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/policy"
)

// The OS executor implements the Policy Engine interface.
var _ policy.Executor = (*Executor)(nil)

// fakeExecutor is the fake executor for tests. It records the calls and
// never calls the OS.
type fakeExecutor struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeExecutor) Shutdown(_ context.Context, r string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "shutdown:"+r)
	return f.err
}

func (f *fakeExecutor) Hibernate(_ context.Context, r string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "hibernate:"+r)
	return f.err
}

var _ policy.Executor = (*fakeExecutor)(nil)

// fakeOS replaces the OS part of Executor.
type fakeOS struct{ calls []string }

func (f *fakeOS) shutdown(_ context.Context, _ *Executor, r string) error {
	f.calls = append(f.calls, "shutdown:"+r)
	return nil
}

func (f *fakeOS) hibernate(_ context.Context, _ *Executor, r string) error {
	f.calls = append(f.calls, "hibernate:"+r)
	return nil
}

type auditEntry struct {
	action, result string
	details        map[string]any
}

type fakeAudit struct{ entries []auditEntry }

func (a *fakeAudit) Record(_ context.Context, _, action, _, result string, d map[string]any) error {
	a.entries = append(a.entries, auditEntry{action, result, d})
	return nil
}

func TestExecutorUsesOSPart(t *testing.T) {
	f := &fakeOS{}
	e := New(Options{})
	e.os = f
	if err := e.Shutdown(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if err := e.Hibernate(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || f.calls[0] != "shutdown:a" || f.calls[1] != "hibernate:b" {
		t.Fatalf("calls %v", f.calls)
	}
	if !e.forceClose() {
		t.Fatal("force_close_applications default must be on")
	}
}

// TestFakeExecutor checks the fake executor for tests.
func TestFakeExecutor(t *testing.T) {
	f := &fakeExecutor{}
	if err := f.Shutdown(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	f.err = errors.New("fail")
	if err := f.Hibernate(context.Background(), "y"); err == nil {
		t.Fatal("no error")
	}
	if len(f.calls) != 2 {
		t.Fatalf("calls %v", f.calls)
	}
}
