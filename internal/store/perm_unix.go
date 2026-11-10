// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func defaultDataDir() string { return "/var/lib/vltd" }

// CheckDataDir checks the data directory and the database files on Linux:
// directory mode 0700 or stricter, owner = the vltd user (the effective uid
// of this process), database files mode 0600 or stricter with the same
// owner. Symbolic links are not permitted.
func CheckDataDir(dir string) error {
	euid := os.Geteuid()
	st, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("store: data directory: %w", err)
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrPermissions, dir)
	}
	if err := checkOwnerMode(dir, st, euid, 0o700); err != nil {
		return err
	}
	for _, n := range []string{DBFileName, DBFileName + "-wal", DBFileName + "-shm"} {
		p := filepath.Join(dir, n)
		st, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("store: %s: %w", p, err)
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("%w: %s is not a regular file", ErrPermissions, p)
		}
		if err := checkOwnerMode(p, st, euid, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func checkOwnerMode(p string, st os.FileInfo, euid int, max os.FileMode) error {
	if perm := st.Mode().Perm(); perm&^max != 0 {
		return fmt.Errorf("%w: %s has mode %04o, maximum %04o", ErrPermissions, p, perm, max)
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%w: %s: no owner data", ErrPermissions, p)
	}
	if int(sys.Uid) != euid {
		return fmt.Errorf("%w: %s has owner uid %d, not %d (vltd)", ErrPermissions, p, sys.Uid, euid)
	}
	return nil
}
