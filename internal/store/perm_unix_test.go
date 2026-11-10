// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDataDirLinux(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CheckDataDir(dir); err != nil {
		t.Fatalf("0700 directory: %v", err)
	}
	s := openTest(t, dir, Options{})
	if err := CheckDataDir(dir); err != nil {
		t.Fatalf("after Open: %v", err)
	}
	s.Close()
	st, _ := os.Stat(filepath.Join(dir, DBFileName))
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("database mode %04o, want 0600", st.Mode().Perm())
	}

	for _, m := range []os.FileMode{0o750, 0o755, 0o701, 0o777} {
		os.Chmod(dir, m)
		if err := CheckDataDir(dir); !errors.Is(err, ErrPermissions) {
			t.Errorf("directory mode %04o: err = %v", m, err)
		}
	}
	os.Chmod(dir, 0o700)
	for _, m := range []os.FileMode{0o640, 0o644, 0o660, 0o700} {
		os.Chmod(filepath.Join(dir, DBFileName), m)
		if err := CheckDataDir(dir); !errors.Is(err, ErrPermissions) {
			t.Errorf("file mode %04o: err = %v", m, err)
		}
	}
	os.Chmod(filepath.Join(dir, DBFileName), 0o400)
	if err := CheckDataDir(dir); err != nil {
		t.Errorf("file mode 0400 (stricter): %v", err)
	}
	// A symbolic link is not permitted.
	link := filepath.Join(t.TempDir(), "link")
	os.Symlink(dir, link)
	if err := CheckDataDir(link); !errors.Is(err, ErrPermissions) {
		t.Errorf("symlink: err = %v", err)
	}
	if err := CheckDataDir(filepath.Join(dir, "none")); err == nil {
		t.Error("missing directory accepted")
	}
}
