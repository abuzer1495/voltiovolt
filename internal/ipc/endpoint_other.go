// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux && !windows

package ipc

import (
	"errors"
	"net"
)

// Endpoint is not available on this platform.
type Endpoint struct{}

// DefaultEndpoint returns an Endpoint that cannot listen.
func DefaultEndpoint() Endpoint { return Endpoint{} }

// Listen returns an error: only Linux and Windows are supported.
func (Endpoint) Listen() (net.Listener, PeerCheck, error) {
	return nil, nil, errors.New("ipc: only Linux and Windows are supported")
}
