// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/gui"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
)

// TestPINUnlockIdle is VLT-IS-009 TASK 03-00-00-001 step 1.2: a lock by
// the idle time, then Unlock with the same PIN. This test lives in the
// engine package so that it can move the IPC server clock (the default
// idle time is 10 minutes).
func TestPINUnlockIdle(t *testing.T) {
	d, ln := startPortableTest(t, dataDir(t))
	var mu sync.Mutex
	now := time.Now()
	d.srv.SetClock(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	})
	step := func(dt time.Duration) {
		mu.Lock()
		now = now.Add(dt)
		mu.Unlock()
	}

	m := gui.NewManager("", nil, nil)
	m.SetMode(gui.ModePortable)
	m.SetDial(func(ctx context.Context, addr string) (*client.Client, error) {
		c, err := ln.Dial()
		if err != nil {
			return nil, err
		}
		return client.New(c), nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); m.Close(); <-done })
	a := gui.NewAPI(m)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a.Connection().State == gui.ConnConnected {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if a.Connection().State != gui.ConnConnected {
		t.Fatal("not connected")
	}

	const pin = "k7Pq2Lm9"
	if err := a.SetPIN(pin); err != nil {
		t.Fatalf("SetPIN: %v", err)
	}
	if err := a.Login(pin); err != nil {
		t.Fatalf("first Unlock: %v", err)
	}
	st, err := a.AuthStatus()
	if err != nil || !st.Elevated {
		t.Fatalf("elevated after Unlock: %+v err=%v", st, err)
	}

	step(11 * time.Minute)
	st, err = a.AuthStatus()
	if err != nil || st.Elevated {
		t.Fatalf("after idle: %+v err=%v (want locked)", st, err)
	}
	if err := a.Login(pin); err != nil {
		t.Fatalf("Unlock after idle: %v", err)
	}
	st, err = a.AuthStatus()
	if err != nil || !st.Elevated {
		t.Fatalf("elevated after Unlock: %+v err=%v", st, err)
	}
}
