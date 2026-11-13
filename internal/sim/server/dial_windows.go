// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package server

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// DialPath connects to a named pipe. It does not use TCP (OD-44).
func DialPath(ctx context.Context, path string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, path)
}
