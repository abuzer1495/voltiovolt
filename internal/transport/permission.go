// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ErrPermissionDenied is returned when the open of a Linux USB serial port
// (/dev/ttyUSB* or /dev/ttyACM*) or a hidraw device fails because the user
// has no permission (VLT-IS-008 TASK 03-00-00-005).
var ErrPermissionDenied = errors.New("transport: permission denied")

// IsPermission reports whether err is a permission-denied error of an open.
func IsPermission(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrPermissionDenied) || errors.Is(err, os.ErrPermission) || errors.Is(err, fs.ErrPermission) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && (errno == syscall.EACCES || errno == syscall.EPERM) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "permission denied")
}

// IsLinuxUSBSerial reports whether name is a Linux USB serial device
// (/dev/ttyUSB* or /dev/ttyACM*).
func IsLinuxUSBSerial(name string) bool {
	base := filepath.Base(name)
	return strings.HasPrefix(base, "ttyUSB") || strings.HasPrefix(base, "ttyACM")
}

// IsLinuxHID reports whether name is a Linux hidraw device.
func IsLinuxHID(name string) bool {
	return strings.HasPrefix(filepath.Base(name), "hidraw")
}
