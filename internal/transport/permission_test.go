// SPDX-License-Identifier: AGPL-3.0-or-later

package transport

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestIsLinuxUSBSerial(t *testing.T) {
	for _, n := range []string{"/dev/ttyUSB0", "/dev/ttyUSB12", "/dev/ttyACM0", "/dev/ttyACM3", "ttyUSB0"} {
		if !IsLinuxUSBSerial(n) {
			t.Fatalf("%q: want true", n)
		}
	}
	for _, n := range []string{"COM3", "COM1", "/dev/ttyS0", "/dev/hidraw0", "", "tty"} {
		if IsLinuxUSBSerial(n) {
			t.Fatalf("%q: want false", n)
		}
	}
}

func TestIsLinuxHID(t *testing.T) {
	if !IsLinuxHID("/dev/hidraw0") || !IsLinuxHID("hidraw3") {
		t.Fatal("hidraw")
	}
	if IsLinuxHID("/dev/ttyUSB0") || IsLinuxHID("COM3") || IsLinuxHID("") {
		t.Fatal("not hidraw")
	}
}

func TestIsPermission(t *testing.T) {
	if IsPermission(nil) || IsPermission(errors.New("other")) || IsPermission(ErrPortBusy) {
		t.Fatal("false positive")
	}
	for _, err := range []error{
		ErrPermissionDenied,
		os.ErrPermission,
		syscall.EACCES,
		syscall.EPERM,
		&os.PathError{Op: "open", Path: "/dev/ttyUSB0", Err: syscall.EACCES},
		fmt.Errorf("%w: /dev/ttyUSB0", ErrPermissionDenied),
		errors.New("open /dev/ttyUSB0: permission denied"),
	} {
		if !IsPermission(err) {
			t.Fatalf("not permission: %v", err)
		}
	}
}

func TestWrapSerialOpenError(t *testing.T) {
	err := wrapSerialOpenError("/dev/ttyUSB0", syscall.EACCES)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("ttyUSB: %v", err)
	}
	err = wrapSerialOpenError("/dev/ttyACM1", &os.PathError{Op: "open", Path: "/dev/ttyACM1", Err: syscall.EACCES})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("ttyACM: %v", err)
	}
	err = wrapSerialOpenError("COM3", syscall.EACCES)
	if errors.Is(err, ErrPermissionDenied) {
		t.Fatal("COM3 must not map to the Linux permission error")
	}
	err = wrapSerialOpenError("/dev/ttyS0", syscall.EACCES)
	if errors.Is(err, ErrPermissionDenied) {
		t.Fatal("ttyS0 is not a USB serial port of the task")
	}
	other := errors.New("no such file")
	if wrapSerialOpenError("/dev/ttyUSB0", other) != other {
		t.Fatal("other error changed")
	}
}
