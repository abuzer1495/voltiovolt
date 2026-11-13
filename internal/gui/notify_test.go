// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"sync"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// TestDesktopNotification: a "notification" push message goes to the
// OnNotify function (VLT-IS-007 TASK 03-00-00-004) and not to the front end
// state.
func TestDesktopNotification(t *testing.T) {
	f := newFake(t)
	m, a, _ := start(t, f)
	var mu sync.Mutex
	var got []OSNotification
	m.OnNotify(func(n OSNotification) { mu.Lock(); got = append(got, n); mu.Unlock() })
	f.push(wire.PushStatus, map[string]any{"device": map[string]any{"state": "connected"}, "ups": map[string]any{}})
	waitFor(t, "status", func() bool { _, ok := a.Last()["status"]; return ok })
	f.push(wire.PushNotification, map[string]any{"subject": "[VLT][CRITICAL] On battery", "text": "Mode: battery", "severity": "CRITICAL", "type": "ON_BATTERY", "time": "2026-11-10T10:00:00Z"})
	waitFor(t, "notification", func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 1 })
	mu.Lock()
	n := got[0]
	mu.Unlock()
	if n.Title != "[VLT][CRITICAL] On battery" || n.Body != "Mode: battery" || n.Severity != "CRITICAL" || n.Type != "ON_BATTERY" {
		t.Fatalf("notification %+v", n)
	}
	if _, ok := a.Last()[wire.PushNotification]; ok {
		t.Fatal("notification kept in the last values")
	}
}
