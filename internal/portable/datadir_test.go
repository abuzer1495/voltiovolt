// SPDX-License-Identifier: AGPL-3.0-or-later

package portable

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWritableDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	if !writable(dir) {
		t.Fatal("temp dir is not writable")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestProfileDir(t *testing.T) {
	d, err := ProfileDir()
	if err != nil {
		t.Fatal(err)
	}
	if d == "" {
		t.Fatal("empty")
	}
	if runtime.GOOS == "windows" {
		if filepath.Base(d) != "VoltioVolt" {
			t.Fatalf("windows profile %q", d)
		}
	} else if runtime.GOOS == "linux" {
		if filepath.Base(d) != "voltiovolt" {
			t.Fatalf("linux profile %q", d)
		}
	}
}

func TestDataDirFallback(t *testing.T) {
	// DataDir uses the binary folder when it is writable, else the
	// profile folder. Both must be a non-empty path. The function does
	// not call CheckDataDir (no ACL).
	d, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if d == "" {
		t.Fatal("empty")
	}
}
