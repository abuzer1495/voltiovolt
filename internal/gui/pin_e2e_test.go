// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/engine"
	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

// startPortableGUI starts the portable engine (in-process net.Pipe) and
// the GUI Go side. It is the end-to-end setup of VLT-IS-009 TASK
// 03-00-00-001 step 1.
func startPortableGUI(t *testing.T, dir string) (*engine.Daemon, *Manager, *API, func()) {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	ln := ipc.NewPipeListener()
	cfg := engine.Config{
		DataDir:    dir,
		CheckPerms: false,
		Provider:   vault.NewPortableProvider,
		Listen:     func() (net.Listener, ipc.PeerCheck, error) { return ln, ipc.InProcessCheck, nil },
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:    "pin-e2e",
	}
	d := engine.New(cfg)
	if err := d.Start(context.Background()); err != nil {
		ln.Close()
		t.Fatal(err)
	}
	rec := &recorder{}
	m := NewManager("", rec, nil)
	m.SetMode(ModePortable)
	m.SetDial(func(ctx context.Context, addr string) (*client.Client, error) {
		c, err := ln.Dial()
		if err != nil {
			return nil, err
		}
		return client.New(c), nil
	})
	m.retry = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	a := NewAPI(m)
	waitFor(t, "connected", func() bool { return a.Connection().State == ConnConnected })
	stop := func() {
		cancel()
		m.Close()
		<-done
		sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer scancel()
		_ = d.Stop(sctx)
		_ = ln.Close()
	}
	return d, m, a, stop
}

func mustSetAndLock(t *testing.T, a *API, pin string) {
	t.Helper()
	if err := a.SetPIN(pin); err != nil {
		t.Fatalf("SetPIN(%q): %v", pin, err)
	}
	st, err := a.AuthStatus()
	if err != nil || !st.PINSet {
		t.Fatalf("after SetPIN: %+v err=%v", st, err)
	}
	// SetPIN drops the session. A Lock after that must still be safe.
	if st.Elevated {
		if err := a.Logout(); err != nil {
			t.Fatalf("Lock: %v", err)
		}
	}
	st, err = a.AuthStatus()
	if err != nil || st.Elevated {
		t.Fatalf("after Lock: %+v err=%v", st, err)
	}
}

func mustUnlock(t *testing.T, a *API, pin string) {
	t.Helper()
	if err := a.Login(pin); err != nil {
		t.Fatalf("Unlock with PIN %q: %v", pin, err)
	}
	st, err := a.AuthStatus()
	if err != nil || !st.Elevated || !st.PINSet {
		t.Fatalf("after Unlock: %+v err=%v", st, err)
	}
}

// TestPINUnlockWizard is VLT-IS-009 TASK 03-00-00-001 step 1.1: set a
// PIN in the wizard, lock, unlock with the same PIN.
func TestPINUnlockWizard(t *testing.T) {
	_, _, a, stop := startPortableGUI(t, "")
	defer stop()
	const pin = "k7Pq2Lm9"
	// The wizard Set PIN step calls the same SetPIN method.
	mustSetAndLock(t, a, pin)
	mustUnlock(t, a, pin)
}

// TestPINUnlockCases is VLT-IS-009 TASK 03-00-00-001 step 1.2.
func TestPINUnlockCases(t *testing.T) {
	t.Run("digits only", func(t *testing.T) {
		_, _, a, stop := startPortableGUI(t, "")
		defer stop()
		const pin = "80274619"
		mustSetAndLock(t, a, pin)
		mustUnlock(t, a, pin)
	})
	t.Run("letters", func(t *testing.T) {
		_, _, a, stop := startPortableGUI(t, "")
		defer stop()
		const pin = "AbCdEf7x"
		mustSetAndLock(t, a, pin)
		mustUnlock(t, a, pin)
	})
	t.Run("set in General", func(t *testing.T) {
		_, _, a, stop := startPortableGUI(t, "")
		defer stop()
		// The General screen "Set PIN" calls the same SetPIN method.
		const pin = "9q8w7e6r"
		mustSetAndLock(t, a, pin)
		mustUnlock(t, a, pin)
	})
	t.Run("lock by Lock button", func(t *testing.T) {
		_, _, a, stop := startPortableGUI(t, "")
		defer stop()
		const pin = "k7Pq2Lm9"
		mustSetAndLock(t, a, pin)
		mustUnlock(t, a, pin)
		if err := a.Logout(); err != nil {
			t.Fatal(err)
		}
		st, err := a.AuthStatus()
		if err != nil || st.Elevated {
			t.Fatalf("after Lock button: %+v err=%v", st, err)
		}
		mustUnlock(t, a, pin)
	})
	// "lock by idle time" is TestPINUnlockIdle in the engine package:
	// the server clock is not accessible from this package.
	t.Run("restart between set and unlock", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "data")
		const pin = "k7Pq2Lm9"
		_, _, a, stop := startPortableGUI(t, dir)
		mustSetAndLock(t, a, pin)
		stop()
		_, _, a2, stop2 := startPortableGUI(t, dir)
		defer stop2()
		st, err := a2.AuthStatus()
		if err != nil || !st.PINSet {
			t.Fatalf("after restart: %+v err=%v", st, err)
		}
		if st.Elevated {
			t.Fatal("elevated after restart; Unlock would not be asked")
		}
		mustUnlock(t, a2, pin)
	})
}

// TestPINUnlockLeadingZero is a digits PIN that starts with 0. A number
// type would remove the leading zero and Unlock would fail.
func TestPINUnlockLeadingZero(t *testing.T) {
	_, _, a, stop := startPortableGUI(t, "")
	defer stop()
	const pin = "08174629"
	mustSetAndLock(t, a, pin)
	mustUnlock(t, a, pin)
	if err := a.Login("8174629"); err == nil {
		t.Fatal("a truncated PIN (leading zero removed) must not unlock")
	}
}
