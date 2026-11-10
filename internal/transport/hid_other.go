// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !linux && !windows

package transport

import "errors"

var errHIDPlatform = errors.New("transport: HID is supported on Linux and Windows only")

// ListHID is not supported on this platform.
func ListHID() ([]HIDDevice, error) { return nil, errHIDPlatform }

func openHID(string) (ReportDevice, HIDDevice, error) {
	return nil, HIDDevice{}, errHIDPlatform
}
