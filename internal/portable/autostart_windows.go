// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package portable

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func shortcutName() string { return "VoltioVolt.lnk" }

func startupDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Startup, 0)
}

var (
	clsidShellLink  = windows.GUID{Data1: 0x00021401, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIShellLinkW  = windows.GUID{Data1: 0x000214F9, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIPersistFile = windows.GUID{Data1: 0x0000010B, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

	modole32             = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = modole32.NewProc("CoCreateInstance")
)

type comObject struct{ lpVtbl *iUnknownVtbl }

type iUnknownVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

type iShellLinkVtbl struct {
	iUnknownVtbl
	GetPath             uintptr
	GetIDList           uintptr
	SetIDList           uintptr
	GetDescription      uintptr
	SetDescription      uintptr
	GetWorkingDirectory uintptr
	SetWorkingDirectory uintptr
	GetArguments        uintptr
	SetArguments        uintptr
	GetHotkey           uintptr
	SetHotkey           uintptr
	GetShowCmd          uintptr
	SetShowCmd          uintptr
	GetIconLocation     uintptr
	SetIconLocation     uintptr
	SetRelativePath     uintptr
	Resolve             uintptr
	SetPath             uintptr
}

type iPersistFileVtbl struct {
	iUnknownVtbl
	GetClassID    uintptr
	IsDirty       uintptr
	Load          uintptr
	Save          uintptr
	SaveCompleted uintptr
	GetCurFile    uintptr
}

func writeShortcut(path, target string) error {
	// S_FALSE (already initialized) and RPC_E_CHANGED_MODE (different
	// apartment) are not fatal: CoCreateInstance can still run.
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)

	var unk *comObject
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)),
		0,
		uintptr(windows.CLSCTX_INPROC_SERVER),
		uintptr(unsafe.Pointer(&iidIShellLinkW)),
		uintptr(unsafe.Pointer(&unk)),
	)
	if r != 0 {
		return fmt.Errorf("shell link: HRESULT 0x%08X", r)
	}
	defer syscall.SyscallN(unk.lpVtbl.Release, uintptr(unsafe.Pointer(unk)))

	v := (*iShellLinkVtbl)(unsafe.Pointer(unk.lpVtbl))
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	if hr, _, _ := syscall.SyscallN(v.SetPath, uintptr(unsafe.Pointer(unk)), uintptr(unsafe.Pointer(t))); hr != 0 {
		return fmt.Errorf("SetPath: HRESULT 0x%08X", hr)
	}

	var pf *comObject
	if hr, _, _ := syscall.SyscallN(unk.lpVtbl.QueryInterface, uintptr(unsafe.Pointer(unk)), uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&pf))); hr != 0 {
		return fmt.Errorf("IPersistFile: HRESULT 0x%08X", hr)
	}
	defer syscall.SyscallN(pf.lpVtbl.Release, uintptr(unsafe.Pointer(pf)))
	pv := (*iPersistFileVtbl)(unsafe.Pointer(pf.lpVtbl))
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if hr, _, _ := syscall.SyscallN(pv.Save, uintptr(unsafe.Pointer(pf)), uintptr(unsafe.Pointer(p)), 1); hr != 0 {
		return fmt.Errorf("Save: HRESULT 0x%08X", hr)
	}
	return nil
}
