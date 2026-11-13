// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package main

import (
	"syscall"
)

func defaultLangOS() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	proc := k32.NewProc("GetUserDefaultUILanguage")
	r, _, _ := proc.Call()
	lang := uint32(r)
	// 0x041F is Turkish (Turkey). The primary language is the low 10 bits:
	// LANG_TURKISH = 0x1F.
	if lang&0x3FF == 0x1F {
		return "tr"
	}
	return "en"
}
