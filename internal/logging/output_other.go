// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package logging

import (
	"io"
	"os"
)

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// platformWriter returns standard error. On Linux, systemd sends it to the
// journal.
func platformWriter(string) (io.WriteCloser, error) {
	return nopCloser{os.Stderr}, nil
}
