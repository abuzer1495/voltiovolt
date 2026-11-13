// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"io"
	"testing"
)

func TestPipeListener(t *testing.T) {
	ln := NewPipeListener()
	t.Cleanup(func() { ln.Close() })
	errc := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			errc <- err
			return
		}
		_, err = c.Write([]byte("ok"))
		c.Close()
		errc <- err
	}()
	c, err := ln.Dial()
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(c)
	c.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "ok" {
		t.Fatalf("got %q", b)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if ln.Addr().Network() != "pipe" || ln.Addr().String() != "in-process" {
		t.Fatalf("addr %s %s", ln.Addr().Network(), ln.Addr().String())
	}
	peer, err := InProcessCheck(nil)
	if err != nil || peer != "in-process" {
		t.Fatalf("check %q %v", peer, err)
	}
}

func TestPipeListenerClose(t *testing.T) {
	ln := NewPipeListener()
	ln.Close()
	if _, err := ln.Dial(); err == nil {
		t.Fatal("dial after close")
	}
}
