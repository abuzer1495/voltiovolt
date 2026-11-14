// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"os"
	"sync/atomic"
)

// The test-only endpoint (VLT-IS-011 TASK 03-00-00-006, Q-1). Go runs the
// test packages at the same time, and on Windows all of them used the one
// named pipe of OD-44: a test could see "The simulator already runs".
// Each test now uses its own name. Product code never calls SetTestPath:
// the product endpoint (defaultPath) does not change.
var (
	overridePath atomic.Pointer[string]
	testSeq      atomic.Uint64
)

// Path is the endpoint of OD-44, or the test-only endpoint of SetTestPath.
func Path() string {
	if p := overridePath.Load(); p != nil {
		return *p
	}
	return defaultPath()
}

// ProductPath is the endpoint of OD-44 without a test-only override.
func ProductPath() string { return defaultPath() }

// SetTestPath makes Path return p until restore is called. Only tests
// call it, for example t.Cleanup(server.SetTestPath(server.NewTestPath())).
func SetTestPath(p string) (restore func()) {
	old := overridePath.Swap(&p)
	return func() { overridePath.Store(old) }
}

// NewTestPath returns a new endpoint name for one test: the process ID
// and a counter make it unique between the test packages.
func NewTestPath() string {
	return newTestPath(os.Getpid(), testSeq.Add(1))
}
