// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

// TestShutdownMessage is the dry-run OS-shutdown text of the test
// connection (VLT-IS-010 OD-46).
const TestShutdownMessage = "Test: the computer would shut down now."

// testMode is the active flag and dry-run message of the test connection.
type testMode struct {
	mu      sync.Mutex
	active  bool
	message string
}

func (t *testMode) SetActive(v bool) {
	t.mu.Lock()
	t.active = v
	if !v {
		t.message = ""
	}
	t.mu.Unlock()
}

func (t *testMode) Active() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active
}

func (t *testMode) SetMessage(s string) {
	t.mu.Lock()
	t.message = s
	t.mu.Unlock()
}

func (t *testMode) Message() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.message
}

// realShutdownOn reports Setting test.real_shutdown. The default is off
// (OD-46). A test must never turn this on.
func realShutdownOn(st *store.Store) bool {
	if st == nil {
		return false
	}
	s, err := st.GetSetting(context.Background(), store.SettingTestRealShutdown)
	if err != nil {
		return false
	}
	var v bool
	if err := json.Unmarshal(s.Value, &v); err != nil {
		return false
	}
	return v
}
