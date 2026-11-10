// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package transport

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SetupAPI and hid.dll through golang.org/x/sys/windows. golang.org/x/sys
// has no wrapper for these hid.dll functions and for
// SetupDiEnumDeviceInterfaces and SetupDiGetDeviceInterfaceDetailW.
var (
	modHID                       = windows.NewLazySystemDLL("hid.dll")
	procGetHidGuid               = modHID.NewProc("HidD_GetHidGuid")
	procGetAttributes            = modHID.NewProc("HidD_GetAttributes")
	procGetPreparsedData         = modHID.NewProc("HidD_GetPreparsedData")
	procFreePreparsedData        = modHID.NewProc("HidD_FreePreparsedData")
	procHidPGetCaps              = modHID.NewProc("HidP_GetCaps")
	procGetProductString         = modHID.NewProc("HidD_GetProductString")
	procGetSerialNumberStr       = modHID.NewProc("HidD_GetSerialNumberString")
	modSetupAPI                  = windows.NewLazySystemDLL("setupapi.dll")
	procEnumDeviceIfaces         = modSetupAPI.NewProc("SetupDiEnumDeviceInterfaces")
	procGetIfaceDetailW          = modSetupAPI.NewProc("SetupDiGetDeviceInterfaceDetailW")
	errHIDNoMoreItems      error = windows.ERROR_NO_MORE_ITEMS
)

type hiddAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

type hidpCaps struct {
	Usage                   uint16
	UsagePage               uint16
	InputReportByteLength   uint16
	OutputReportByteLength  uint16
	FeatureReportByteLength uint16
	Reserved                [17]uint16
	Counts                  [10]uint16
}

type spDeviceInterfaceData struct {
	CbSize             uint32
	InterfaceClassGUID windows.GUID
	Flags              uint32
	Reserved           uintptr
}

const hidpStatusSuccess = 0x00110000

func hidGUID() windows.GUID {
	var g windows.GUID
	_, _, _ = procGetHidGuid.Call(uintptr(unsafe.Pointer(&g)))
	return g
}

// ListHID lists the HID devices with VID 0665 and PID 5161. It sends no
// command.
func ListHID() ([]HIDDevice, error) {
	g := hidGUID()
	set, err := windows.SetupDiGetClassDevsEx(&g, "", 0, windows.DIGCF_PRESENT|windows.DIGCF_DEVICEINTERFACE, 0, "")
	if err != nil {
		return nil, fmt.Errorf("transport: SetupDiGetClassDevsEx: %w", err)
	}
	defer set.Close()
	var out []HIDDevice
	for i := 0; ; i++ {
		var ifd spDeviceInterfaceData
		ifd.CbSize = uint32(unsafe.Sizeof(ifd))
		r, _, e := procEnumDeviceIfaces.Call(uintptr(set), 0, uintptr(unsafe.Pointer(&g)), uintptr(i), uintptr(unsafe.Pointer(&ifd)))
		if r == 0 {
			if errors.Is(e, errHIDNoMoreItems) {
				break
			}
			return out, fmt.Errorf("transport: SetupDiEnumDeviceInterfaces: %w", e)
		}
		path, err := interfacePath(set, &ifd)
		if err != nil {
			continue
		}
		d, ok := queryHID(path)
		if ok && d.VID == VendorID && d.PID == ProductID {
			out = append(out, d)
		}
	}
	return out, nil
}

func interfacePath(set windows.DevInfo, ifd *spDeviceInterfaceData) (string, error) {
	var need uint32
	_, _, _ = procGetIfaceDetailW.Call(uintptr(set), uintptr(unsafe.Pointer(ifd)), 0, 0, uintptr(unsafe.Pointer(&need)), 0)
	if need < 8 {
		return "", errors.New("transport: no interface detail size")
	}
	buf := make([]uint64, (need+7)/8)
	cb := uint32(8) // SP_DEVICE_INTERFACE_DETAIL_DATA_W on 64-bit
	if unsafe.Sizeof(uintptr(0)) == 4 {
		cb = 6
	}
	*(*uint32)(unsafe.Pointer(&buf[0])) = cb
	r, _, e := procGetIfaceDetailW.Call(uintptr(set), uintptr(unsafe.Pointer(ifd)), uintptr(unsafe.Pointer(&buf[0])), uintptr(need), 0, 0)
	if r == 0 {
		return "", e
	}
	p := (*uint16)(unsafe.Add(unsafe.Pointer(&buf[0]), 4))
	return windows.UTF16PtrToString(p), nil
}

func createFile(path string, access uint32, flags uint32) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, flags, 0)
}

func hidString(proc *windows.LazyProc, h windows.Handle) string {
	buf := make([]uint16, 128)
	r, _, _ := proc.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)*2))
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// queryHID reads the attributes, strings and report lengths of a device.
// It opens the device with no read and no write access.
func queryHID(path string) (HIDDevice, bool) {
	h, err := createFile(path, 0, 0)
	if err != nil {
		return HIDDevice{}, false
	}
	defer windows.CloseHandle(h)
	var a hiddAttributes
	a.Size = uint32(unsafe.Sizeof(a))
	if r, _, _ := procGetAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&a))); r == 0 {
		return HIDDevice{}, false
	}
	d := HIDDevice{Path: path, VID: a.VendorID, PID: a.ProductID}
	if d.VID != VendorID || d.PID != ProductID {
		return d, true
	}
	d.Product = hidString(procGetProductString, h)
	d.Serial = hidString(procGetSerialNumberStr, h)
	var pp uintptr
	if r, _, _ := procGetPreparsedData.Call(uintptr(h), uintptr(unsafe.Pointer(&pp))); r != 0 {
		var c hidpCaps
		st, _, _ := procHidPGetCaps.Call(pp, uintptr(unsafe.Pointer(&c)))
		_, _, _ = procFreePreparsedData.Call(pp)
		if uint32(st) == hidpStatusSuccess {
			// HidP_GetCaps lengths include the report ID byte.
			d.Lengths = ReportLengths{
				Input:   max(int(c.InputReportByteLength)-1, 0),
				Output:  max(int(c.OutputReportByteLength)-1, 0),
				Feature: max(int(c.FeatureReportByteLength)-1, 0),
			}
		}
	}
	return d, true
}

type winHID struct {
	h   windows.Handle
	in  []byte
	out int
}

func openHID(path string) (ReportDevice, HIDDevice, error) {
	d, ok := queryHID(path)
	if !ok || d.VID != VendorID || d.PID != ProductID {
		return nil, HIDDevice{}, fmt.Errorf("transport: %s is not VID 0665 PID 5161", path)
	}
	h, err := createFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_FLAG_OVERLAPPED)
	if err != nil {
		return nil, HIDDevice{}, fmt.Errorf("transport: open HID device: %w", err)
	}
	return &winHID{h: h, in: make([]byte, d.Lengths.Input+1), out: d.Lengths.Output + 1}, d, nil
}

func (w *winHID) overlapped(fn func(o *windows.Overlapped) error, timeout time.Duration) (uint32, bool, error) {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, false, err
	}
	defer windows.CloseHandle(ev)
	o := windows.Overlapped{HEvent: ev}
	err = fn(&o)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return 0, false, err
	}
	ms := uint32(timeout.Milliseconds())
	if ms == 0 {
		ms = 1
	}
	ev2, err := windows.WaitForSingleObject(ev, ms)
	if err != nil {
		return 0, false, err
	}
	var n uint32
	if ev2 == uint32(windows.WAIT_TIMEOUT) {
		_ = windows.CancelIoEx(w.h, &o)
		_ = windows.GetOverlappedResult(w.h, &o, &n, true)
		return 0, true, nil
	}
	if err := windows.GetOverlappedResult(w.h, &o, &n, false); err != nil {
		return 0, false, err
	}
	return n, false, nil
}

func (w *winHID) WriteReport(r []byte) (int, error) {
	if w.h == windows.InvalidHandle {
		return 0, ErrClosed
	}
	if len(r) != w.out {
		return 0, fmt.Errorf("transport: report length %d, expected %d", len(r), w.out)
	}
	n, timedOut, err := w.overlapped(func(o *windows.Overlapped) error {
		return windows.WriteFile(w.h, r, nil, o)
	}, DefaultTimeout)
	if err != nil {
		return int(n), err
	}
	if timedOut {
		return 0, errors.New("transport: HID write timeout")
	}
	return int(n), nil
}

func (w *winHID) ReadReport(p []byte, timeout time.Duration) (int, error) {
	if w.h == windows.InvalidHandle {
		return 0, ErrClosed
	}
	n, timedOut, err := w.overlapped(func(o *windows.Overlapped) error {
		return windows.ReadFile(w.h, w.in, nil, o)
	}, timeout)
	if err != nil || timedOut || n == 0 {
		return 0, err
	}
	// The first byte is the report ID.
	return copy(p, w.in[1:n]), nil
}

func (w *winHID) Close() error {
	if w.h == windows.InvalidHandle {
		return nil
	}
	err := windows.CloseHandle(w.h)
	w.h = windows.InvalidHandle
	return err
}
