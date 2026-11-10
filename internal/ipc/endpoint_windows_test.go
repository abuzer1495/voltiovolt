// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package ipc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
)

func currentUserSID(t *testing.T) string {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return tu.User.Sid.String()
}

func TestWindowsPipe(t *testing.T) {
	b := make([]byte, 8)
	rand.Read(b)
	name := `\\.\pipe\vltd-test-` + hex.EncodeToString(b)
	ep := Endpoint{Pipe: name, ExtraSIDs: []string{currentUserSID(t)}}
	sddl := ep.SDDL()
	if !strings.HasPrefix(sddl, "D:P(A;;GA;;;SY)") {
		t.Fatalf("SDDL: %s", sddl)
	}
	t.Logf("SDDL: %s", sddl)
	// Windows accepts the SDDL, and the DACL has the ACEs of PipeSDDL.
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatalf("SecurityDescriptorFromString: %v", err)
	}
	ctl, _, err := sd.Control()
	if err != nil || ctl&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("DACL not protected: %v %v", ctl, err)
	}
	_, aces := parseDACL(t, sddl)
	for _, a := range aces {
		switch a.sid {
		case "SY":
			if a.rights != "GA" {
				t.Errorf("SYSTEM: %+v", a)
			}
		case "BA":
			if a.rights != "GRGW" {
				t.Errorf("Administrators: %+v", a)
			}
		}
	}
	ln, check, err := ep.Listen()
	if err != nil {
		t.Fatal(err)
	}
	// The peer identity is the SID of the client user.
	pc := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			pc <- "accept: " + err.Error()
			return
		}
		defer c.Close()
		p, err := check(c)
		if err != nil {
			pc <- "check: " + err.Error()
			return
		}
		pc <- p
	}()
	dc, err := client.Dial(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	if p := <-pc; p != "sid:"+currentUserSID(t) {
		t.Fatalf("peer %q", p)
	}
	dc.Close()

	e := newEnv(t)
	done := make(chan error, 1)
	go func() { done <- e.srv.Serve(ln, check) }()
	for i := 0; i < 3; i++ { // more than one pipe instance
		c, err := client.Dial(context.Background(), name)
		if err != nil {
			t.Fatal(err)
		}
		if h, err := c.Hello(context.Background()); err != nil || h.Protocol != 1 {
			t.Fatalf("hello: %+v %v", h, err)
		}
		c.Close()
	}
	e.srv.Close()
	if err := <-done; !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}
