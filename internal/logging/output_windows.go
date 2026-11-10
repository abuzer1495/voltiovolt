// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package logging

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// platformWriter returns the rotating log file in dataDir\logs.
func platformWriter(dataDir string) (io.WriteCloser, error) {
	if dataDir == "" {
		return nil, errors.New("logging: no data directory")
	}
	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return NewRotatingFile(dir, FileName, MaxFileBytes, MaxFiles)
}
