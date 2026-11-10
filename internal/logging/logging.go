// SPDX-License-Identifier: AGPL-3.0-or-later

// Package logging makes the vltd logger. All output goes through the
// redaction handler of package redact (VLT-IS-003 TASK 03-00-00-002).
//
// Output:
//   - Linux: standard error, for the journal.
//   - Windows: a log file in the data directory. 5 files of 10 MiB each.
package logging

import (
	"io"
	"log/slog"

	"github.com/abuzer1495/voltiovolt/internal/redact"
)

// Rotation values for the Windows log file.
const (
	MaxFileBytes = 10 << 20 // 10 MiB
	MaxFiles     = 5
	FileName     = "vltd.log"
)

// NewLogger returns a logger that writes text records to w through the
// redaction handler.
func NewLogger(w io.Writer, level slog.Leveler) *slog.Logger {
	if level == nil {
		level = slog.LevelInfo
	}
	return slog.New(redact.NewHandler(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})))
}

// Open returns the vltd logger for this platform. dataDir is used on Windows
// only. The caller must call Close on the returned io.Closer at stop.
func Open(dataDir string) (*slog.Logger, io.Closer, error) {
	w, err := platformWriter(dataDir)
	if err != nil {
		return nil, nil, err
	}
	return NewLogger(w, slog.LevelInfo), w, nil
}
