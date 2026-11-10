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
	if !strings.HasPrefix(sddl, "D:P(A;;GA;;;SY)(A;;GA;;;BA)") {
		t.Fatalf("SDDL: %s", sddl)
	}
	t.Logf("SDDL: %s", sddl)
	ln, check, err := ep.Listen()
	if err != nil {
		t.Fatal(err)
	}
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
