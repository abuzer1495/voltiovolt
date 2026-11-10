// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package ipc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
)

func TestLinuxEndpoint(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vltd.sock")
	ep := Endpoint{Path: path, GID: os.Getgid(), SetDirMode: true}
	ln, check, err := ep.Listen()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o660 || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket mode: %v %v", fi.Mode(), err)
	}
	if di, _ := os.Stat(dir); di.Mode().Perm() != 0o750 {
		t.Fatalf("dir mode: %v", di.Mode())
	}
	e := newEnv(t)
	done := make(chan error, 1)
	go func() { done <- e.srv.Serve(ln, check) }()
	c, err := client.Dial(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if h, err := c.Hello(context.Background()); err != nil || h.Protocol != 1 {
		t.Fatalf("hello: %+v %v", h, err)
	}
	e.srv.Close()
	if err := <-done; !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("socket file not removed at close")
	}
}

func TestPeerCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for i := 0; i < 3; i++ {
			c, err := net.Dial("unix", path)
			if err == nil {
				defer c.Close()
			}
		}
		time.Sleep(time.Second)
	}()
	accept := func() net.Conn {
		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	uid := uint32(os.Getuid())
	other := uint32(os.Getgid()) + 12345
	noGroups := func(uint32) ([]string, error) { return nil, nil }

	// Own primary group: accepted.
	if err := peerCheck(uint32(os.Getgid()), noGroups)(accept()); err != nil {
		t.Fatalf("own gid: %v", err)
	}
	// Other group, not a member: refused (unless root).
	err = peerCheck(other, noGroups)(accept())
	if uid != 0 && err == nil {
		t.Fatal("non-member accepted")
	}
	if uid == 0 && err != nil {
		t.Fatalf("root refused: %v", err)
	}
	// Supplementary group member: accepted.
	member := func(uint32) ([]string, error) { return []string{"1", "12", itoa(other)}, nil }
	if err := peerCheck(other, member)(accept()); err != nil {
		t.Fatalf("member: %v", err)
	}
}

func itoa(v uint32) string {
	b := []byte{}
	for {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
		if v == 0 {
			return string(b)
		}
	}
}
