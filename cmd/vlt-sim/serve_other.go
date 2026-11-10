// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func serve(ctx context.Context, profile string, mainsOff bool, faults []string, out io.Writer) error {
	if _, err := model(profile, mainsOff, faults); err != nil {
		return err
	}
	return errors.New("the pty endpoint is available on Linux only; on other platforms use the in-process simulator in tests")
}
