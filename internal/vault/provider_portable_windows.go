// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package vault

// NewPortableProvider returns the portable Windows host binding
// (VLT-IS-008 TASK 03-00-00-002 step 2): DPAPI in the scope of the
// current user (no CRYPTPROTECT_LOCAL_MACHINE), entropy
// "VoltioVolt-DEK-v1" and the install ID. This is the same algorithm as
// NewHostProvider. The service mode keeps NewHostProvider without
// changes.
func NewPortableProvider(dataDir string, installID []byte) KeyProvider {
	return NewHostProvider(dataDir, installID)
}
