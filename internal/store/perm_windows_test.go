// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package store

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func setDACL(t *testing.T, dir, sddl string, protected bool) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION)
	if protected {
		info |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	} else {
		info |= windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, info, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCheckDataDirWindows(t *testing.T) {
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	me := tu.User.Sid
	good := "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;" + me.String() + ")"

	dir := t.TempDir()
	setDACL(t, dir, good, true)
	if err := CheckDataDirFor(dir, me); err != nil {
		t.Fatalf("good ACL: %v", err)
	}
	cases := map[string]struct {
		sddl      string
		protected bool
	}{
		"Everyone":      {good + "(A;OICI;FA;;;WD)", true},
		"Users":         {good + "(A;OICI;FR;;;BU)", true},
		"full for vltd": {"D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + me.String() + ")", true},
		"not protected": {good, false},
	}
	for name, c := range cases {
		d := t.TempDir()
		setDACL(t, d, c.sddl, c.protected)
		if err := CheckDataDirFor(d, me); !errors.Is(err, ErrPermissions) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// Stricter: no entry for the service account.
	d := t.TempDir()
	setDACL(t, d, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", true)
	if err := CheckDataDirFor(d, me); err != nil {
		t.Errorf("stricter ACL: %v", err)
	}
}
