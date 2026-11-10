// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package client

import (
	"context"
	"net"
)

// DefaultAddress is the vltd socket.
const DefaultAddress = "/run/vltd/vltd.sock"

func dial(ctx context.Context, addr string) (net.Conn, error) {
	if addr == "" {
		addr = DefaultAddress
	}
	var d net.Dialer
	return d.DialContext(ctx, "unix", addr)
}
