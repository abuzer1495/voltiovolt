// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package client

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// DefaultAddress is the vltd pipe.
const DefaultAddress = `\\.\pipe\vltd`

func dial(ctx context.Context, addr string) (net.Conn, error) {
	if addr == "" {
		addr = DefaultAddress
	}
	return winio.DialPipeContext(ctx, addr)
}
