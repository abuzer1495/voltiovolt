// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package main

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// These tests only read. They do not run dev-install or dev-uninstall: the
// tool changes the system (service, user right, group).

func TestProcsExist(t *testing.T) {
	for _, p := range []*windows.LazyProc{procNetLocalGroupAdd, procNetLocalGroupDel, procNetLocalGroupGetMembers, procNetLocalGroupAddMembers,
		procLsaOpenPolicy, procLsaClose, procLsaAddAccountRights, procLsaRemoveAccountRights, procLsaNtStatusToWinError} {
		if err := p.Find(); err != nil {
			t.Errorf("%s: %v", p.Name, err)
		}
	}
}

func TestLsaString(t *testing.T) {
	us, err := lsaString(shutdownRight)
	if err != nil {
		t.Fatal(err)
	}
	if int(us.length) != 2*len(shutdownRight) || us.maximumLength != us.length+2 {
		t.Fatalf("%+v", us)
	}
	if got := windows.UTF16PtrToString(us.buffer); got != shutdownRight {
		t.Fatalf("%q", got)
	}
	// LSA_OBJECT_ATTRIBUTES has 6 members (ntsecapi.h).
	var a lsaObjectAttributes
	if want := uintptr(4 + 4*unsafe.Sizeof(uintptr(0)) + 4); unsafe.Sizeof(a) < want {
		t.Fatalf("size %d", unsafe.Sizeof(a))
	}
}

func TestGroupMembersUnknownGroup(t *testing.T) {
	n, err := groupMembers("vlt-test-no-such-group-7f3a")
	if err != nil {
		t.Fatal(err)
	}
	if n != -1 {
		t.Fatalf("members %d", n)
	}
}

func TestDataDirSDDLParses(t *testing.T) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := windows.SecurityDescriptorFromString(dataDirSDDL(tu.User.Sid.String())); err != nil {
		t.Fatal(err)
	}
}

// TestDataDirACL sets the ACL on a temporary directory with the SID of the
// test user in place of NT SERVICE\vltd, and runs the vltd check.
func TestDataDirACL(t *testing.T) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := setDataDirACL(dir, tu.User.Sid); err != nil {
		t.Fatal(err)
	}
}

// TestCallingUser reads the user of the process token (no change to the
// system).
func TestCallingUser(t *testing.T) {
	sid, name, err := callingUser()
	if err != nil {
		t.Fatal(err)
	}
	if !sid.IsValid() || name == "" {
		t.Fatalf("sid %v name %q", sid, name)
	}
}
