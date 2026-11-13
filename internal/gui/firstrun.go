// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	wizardDoneFile  = "wizard-completed"
	wizardLaterFile = "wizard-later"
	// LaterLimit is the number of "Later" answers after which the first-start
	// dialog is not shown again (VLT-IS-009 OD-41).
	LaterLimit = 2
)

// FileFirstRun stores the wizard-completed marker and the "Later" count
// in a data directory (portable mode).
type FileFirstRun struct{ Dir string }

// Completed reports whether the first-run wizard was finished.
func (f FileFirstRun) Completed() (bool, error) {
	_, err := os.Stat(filepath.Join(f.Dir, wizardDoneFile))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// SetCompleted records that the first-run wizard was finished.
func (f FileFirstRun) SetCompleted() error {
	if err := os.MkdirAll(f.Dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.Dir, wizardDoneFile), []byte("1\n"), 0o600)
}

// LaterCount returns how many times the user chose "Later".
func (f FileFirstRun) LaterCount() (int, error) {
	b, err := os.ReadFile(filepath.Join(f.Dir, wizardLaterFile))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 0 {
		return 0, nil
	}
	return n, nil
}

// AddLater adds one "Later" and returns the new count.
func (f FileFirstRun) AddLater() (int, error) {
	n, err := f.LaterCount()
	if err != nil {
		return 0, err
	}
	n++
	if err := os.MkdirAll(f.Dir, 0o700); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(f.Dir, wizardLaterFile), []byte(fmt.Sprintf("%d\n", n)), 0o600); err != nil {
		return 0, err
	}
	return n, nil
}
