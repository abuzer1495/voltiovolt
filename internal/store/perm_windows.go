// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func defaultDataDir() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "VoltioVolt")
}

// ServiceAccount is the account that gets "modify" on the data directory.
const ServiceAccount = `NT SERVICE\vltd`

// Access masks (icacls F and M).
const (
	accessFull   = 0x001F01FF
	accessModify = 0x001301BF
)

// ACE flag of an inherited entry.
const inheritedACE = 0x10

// CheckDataDir checks the ACL of the data directory on Windows: the DACL is
// protected (no inheritance from the parent) and has only allow entries for
// SYSTEM (full control), Administrators (full control) and NT SERVICE\vltd
// (modify or less). No inherited entries. Deny entries are permitted (they
// are stricter).
func CheckDataDir(dir string) error {
	sid, err := lookupSID(ServiceAccount)
	if err != nil {
		return fmt.Errorf("%w: cannot find %s: %v", ErrPermissions, ServiceAccount, err)
	}
	return CheckDataDirFor(dir, sid)
}

func lookupSID(account string) (*windows.SID, error) {
	sid, _, _, err := windows.LookupSID("", account)
	return sid, err
}

// CheckDataDirFor is CheckDataDir with a given service account SID.
func CheckDataDirFor(dir string, service *windows.SID) error {
	st, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("store: data directory: %w", err)
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is not a directory", ErrPermissions, dir)
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("store: read ACL of %s: %w", dir, err)
	}
	ctl, _, err := sd.Control()
	if err != nil {
		return err
	}
	if ctl&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("%w: %s inherits entries from the parent", ErrPermissions, dir)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("%w: %s: no DACL: %v", ErrPermissions, dir, err)
	}
	if dacl == nil {
		return fmt.Errorf("%w: %s has a NULL DACL", ErrPermissions, dir)
	}
	system, _ := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	admins, _ := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return fmt.Errorf("store: read ACE %d: %w", i, err)
		}
		if ace.Header.AceFlags&inheritedACE != 0 {
			return fmt.Errorf("%w: %s has an inherited entry", ErrPermissions, dir)
		}
		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
			continue
		case windows.ACCESS_ALLOWED_ACE_TYPE:
		default:
			return fmt.Errorf("%w: %s has an entry of type %d", ErrPermissions, dir, ace.Header.AceType)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		mask := uint32(ace.Mask)
		switch {
		case sid.Equals(system), sid.Equals(admins):
			if mask&^accessFull != 0 && mask != windows.GENERIC_ALL {
				return fmt.Errorf("%w: %s: unknown rights 0x%x", ErrPermissions, dir, mask)
			}
		case service != nil && sid.Equals(service):
			if mask&^accessModify != 0 {
				return fmt.Errorf("%w: %s: %s has more than modify (0x%x)", ErrPermissions, dir, ServiceAccount, mask)
			}
		default:
			return fmt.Errorf("%w: %s gives access to %s", ErrPermissions, dir, sid.String())
		}
	}
	return nil
}
