// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"errors"
	"os"
	"testing"
)

func TestListenExclusiveSecondFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	ln, err := ListenExclusive()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, err = ListenExclusive()
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second listen: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	ln2, err := ListenExclusive()
	if err != nil {
		t.Fatalf("after close: %v", err)
	}
	_ = ln2.Close()
}

func TestListenExclusiveNoTCP(t *testing.T) {
	p := Path()
	if len(p) >= 4 && (p[:4] == "tcp:" || p[:6] == "http:") {
		t.Fatalf("Path looks like TCP: %s", p)
	}
	_ = os.PathSeparator
}
