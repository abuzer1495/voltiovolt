// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package server

import (
	"context"
	"net"
)

// DialPath connects to a Unix socket. It does not use TCP (OD-44).
func DialPath(ctx context.Context, path string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", path)
}
