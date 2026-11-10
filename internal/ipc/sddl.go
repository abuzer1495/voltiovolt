// SPDX-License-Identifier: AGPL-3.0-or-later

package ipc

import (
	"fmt"
	"strings"
)

// PipeSDDL returns the security descriptor of the Windows pipe (VLT-IS-004
// QI-4). D:P is a protected DACL (no inherited entries):
//
//	SYSTEM               GA (full access)
//	NT SERVICE\vltd      GA (full access)
//	Administrators       GRGW (read and write)
//	VLT Operators        GRGW (read and write), if the group exists
//
// extra SIDs get GRGW (tests: the current user). GRGW contains
// FILE_APPEND_DATA, which is FILE_CREATE_PIPE_INSTANCE for a pipe, so a
// server with GRGW can make new pipe instances. An empty SID is left out.
func PipeSDDL(serviceSID, operatorsSID string, extra []string) string {
	var b strings.Builder
	b.WriteString("D:P(A;;GA;;;SY)")
	if serviceSID != "" {
		fmt.Fprintf(&b, "(A;;GA;;;%s)", serviceSID)
	}
	b.WriteString("(A;;GRGW;;;BA)")
	if operatorsSID != "" {
		fmt.Fprintf(&b, "(A;;GRGW;;;%s)", operatorsSID)
	}
	for _, s := range extra {
		if s != "" {
			fmt.Fprintf(&b, "(A;;GRGW;;;%s)", s)
		}
	}
	return b.String()
}
