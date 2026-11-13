// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"testing"
)

func TestFileFirstRunLater(t *testing.T) {
	dir := t.TempDir()
	f := FileFirstRun{Dir: dir}
	on, err := f.Completed()
	if err != nil || on {
		t.Fatalf("completed: %v %v", on, err)
	}
	n, err := f.LaterCount()
	if err != nil || n != 0 {
		t.Fatalf("later 0: %d %v", n, err)
	}
	n, err = f.AddLater()
	if err != nil || n != 1 {
		t.Fatalf("later 1: %d %v", n, err)
	}
	n, err = f.AddLater()
	if err != nil || n != 2 {
		t.Fatalf("later 2: %d %v", n, err)
	}
	n, err = f.LaterCount()
	if err != nil || n != 2 {
		t.Fatalf("read 2: %d %v", n, err)
	}
	if n < LaterLimit {
		t.Fatal("two Laters must reach the limit")
	}
	if err := f.SetCompleted(); err != nil {
		t.Fatal(err)
	}
	on, err = f.Completed()
	if err != nil || !on {
		t.Fatalf("after set: %v %v", on, err)
	}
}
