// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/abuzer1495/voltiovolt/internal/store"
)

var (
	modnetapi32 = windows.NewLazySystemDLL("netapi32.dll")
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")

	procNetLocalGroupAdd        = modnetapi32.NewProc("NetLocalGroupAdd")
	procNetLocalGroupDel        = modnetapi32.NewProc("NetLocalGroupDel")
	procNetLocalGroupGetMembers = modnetapi32.NewProc("NetLocalGroupGetMembers")
	procLsaOpenPolicy           = modadvapi32.NewProc("LsaOpenPolicy")
	procLsaClose                = modadvapi32.NewProc("LsaClose")
	procLsaAddAccountRights     = modadvapi32.NewProc("LsaAddAccountRights")
	procLsaRemoveAccountRights  = modadvapi32.NewProc("LsaRemoveAccountRights")
	procLsaNtStatusToWinError   = modadvapi32.NewProc("LsaNtStatusToWinError")
)

// Error codes of the NetLocalGroup functions.
const (
	nerrGroupExists   = 2223 // NERR_GroupExists
	errorAliasExists  = 1379 // ERROR_ALIAS_EXISTS
	nerrGroupNotFound = 2220 // NERR_GroupNotFound
	errorNoSuchAlias  = 1376 // ERROR_NO_SUCH_ALIAS
	maxPreferredLen   = 0xFFFFFFFF
)

// LSA policy access rights (ntsecapi.h).
const (
	policyCreateAccount = 0x00000010
	policyLookupNames   = 0x00000800
)

// Win32 error of LsaRemoveAccountRights when the account has no rights.
const errorFileNotFound = 2

type localGroupInfo1 struct {
	name    *uint16
	comment *uint16
}

type lsaUnicodeString struct {
	length        uint16
	maximumLength uint16
	buffer        *uint16
}

type lsaObjectAttributes struct {
	length                   uint32
	rootDirectory            uintptr
	objectName               *lsaUnicodeString
	attributes               uint32
	securityDescriptor       uintptr
	securityQualityOfService uintptr
}

type serviceRequiredPrivilegesInfo struct {
	requiredPrivileges *uint16
}

func lsaString(s string) (lsaUnicodeString, error) {
	p, err := windows.UTF16FromString(s)
	if err != nil {
		return lsaUnicodeString{}, err
	}
	n := uint16((len(p) - 1) * 2)
	return lsaUnicodeString{length: n, maximumLength: n + 2, buffer: &p[0]}, nil
}

func lsaErr(status uintptr) error {
	if status == 0 {
		return nil
	}
	r, _, _ := procLsaNtStatusToWinError.Call(status)
	return windows.Errno(r)
}

// lsaPolicy opens the local LSA policy.
func lsaPolicy() (uintptr, error) {
	var attr lsaObjectAttributes
	attr.length = uint32(unsafe.Sizeof(attr))
	var h uintptr
	st, _, _ := procLsaOpenPolicy.Call(0, uintptr(unsafe.Pointer(&attr)),
		policyCreateAccount|policyLookupNames, uintptr(unsafe.Pointer(&h)))
	if err := lsaErr(st); err != nil {
		return 0, fmt.Errorf("LsaOpenPolicy: %w", err)
	}
	return h, nil
}

func setRight(sid *windows.SID, right string, add bool) error {
	h, err := lsaPolicy()
	if err != nil {
		return err
	}
	defer procLsaClose.Call(h)
	us, err := lsaString(right)
	if err != nil {
		return err
	}
	var st uintptr
	if add {
		st, _, _ = procLsaAddAccountRights.Call(h, uintptr(unsafe.Pointer(sid)), uintptr(unsafe.Pointer(&us)), 1)
	} else {
		st, _, _ = procLsaRemoveAccountRights.Call(h, uintptr(unsafe.Pointer(sid)), 0, uintptr(unsafe.Pointer(&us)), 1)
	}
	if err := lsaErr(st); err != nil {
		if !add && errors.Is(err, windows.Errno(errorFileNotFound)) {
			return nil
		}
		if add {
			return fmt.Errorf("LsaAddAccountRights %s: %w", right, err)
		}
		return fmt.Errorf("LsaRemoveAccountRights %s: %w", right, err)
	}
	return nil
}

// addGroup makes the local group. It returns false if the group exists.
func addGroup(name, comment string) (bool, error) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return false, err
	}
	c, err := windows.UTF16PtrFromString(comment)
	if err != nil {
		return false, err
	}
	info := localGroupInfo1{name: n, comment: c}
	var parmErr uint32
	r, _, _ := procNetLocalGroupAdd.Call(0, 1, uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Pointer(&parmErr)))
	switch r {
	case 0:
		return true, nil
	case nerrGroupExists, errorAliasExists:
		return false, nil
	}
	return false, fmt.Errorf("NetLocalGroupAdd: %w", windows.Errno(r))
}

// groupMembers returns the number of members, or -1 if the group does not
// exist.
func groupMembers(name string) (int, error) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	var buf *byte
	var read, total uint32
	var resume uintptr
	r, _, _ := procNetLocalGroupGetMembers.Call(0, uintptr(unsafe.Pointer(n)), 0,
		uintptr(unsafe.Pointer(&buf)), maxPreferredLen,
		uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&resume)))
	if buf != nil {
		windows.NetApiBufferFree(buf)
	}
	switch r {
	case 0:
		return int(total), nil
	case nerrGroupNotFound, errorNoSuchAlias:
		return -1, nil
	}
	return 0, fmt.Errorf("NetLocalGroupGetMembers: %w", windows.Errno(r))
}

func deleteGroup(name string) error {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	if r, _, _ := procNetLocalGroupDel.Call(0, uintptr(unsafe.Pointer(n))); r != 0 {
		return fmt.Errorf("NetLocalGroupDel: %w", windows.Errno(r))
	}
	return nil
}

func setRequiredPrivileges(s *mgr.Service, privs []string) error {
	ms := multiString(privs)
	info := serviceRequiredPrivilegesInfo{requiredPrivileges: &ms[0]}
	if err := windows.ChangeServiceConfig2(s.Handle, windows.SERVICE_CONFIG_REQUIRED_PRIVILEGES_INFO, (*byte)(unsafe.Pointer(&info))); err != nil {
		return fmt.Errorf("ChangeServiceConfig2 required privileges: %w", err)
	}
	return nil
}

func setRecovery(s *mgr.Service) error {
	var acts []mgr.RecoveryAction
	for _, a := range recoveryPlan {
		t := mgr.NoAction
		if a.restart {
			t = mgr.ServiceRestart
		}
		acts = append(acts, mgr.RecoveryAction{Type: t, Delay: a.delay})
	}
	if err := s.SetRecoveryActions(acts, recoveryResetSeconds); err != nil {
		return fmt.Errorf("recovery actions: %w", err)
	}
	return nil
}

func setDataDirACL(dir string, svcSID *windows.SID) error {
	sd, err := windows.SecurityDescriptorFromString(dataDirSDDL(svcSID.String()))
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, info, nil, nil, dacl, nil); err != nil {
		return fmt.Errorf("set ACL of %s: %w", dir, err)
	}
	// Self-check with the same code that vltd uses at start.
	if err := store.CheckDataDirFor(dir, svcSID); err != nil {
		return fmt.Errorf("ACL check of %s: %w", dir, err)
	}
	return nil
}

func checkAdmin() error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("run vlt-setup as Administrator")
	}
	return nil
}

func dirs() (install, data string) {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pf, installSubdir), filepath.Join(pd, dataSubdir)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func devInstall(e env) error {
	if err := checkAdmin(); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	src := filepath.Join(filepath.Dir(self), exeName)
	if st, err := os.Stat(src); err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("%s not found next to vlt-setup.exe", src)
	}
	installDir, dataDir := dirs()

	// 1. Group.
	made, err := addGroup(operatorsGroup, groupComment)
	if err != nil {
		return err
	}
	if made {
		e.printf("1. Group %q made.\n", operatorsGroup)
	} else {
		e.printf("1. Group %q exists. Continue.\n", operatorsGroup)
	}

	// 2. Service with the virtual account.
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to the service manager: %w", err)
	}
	defer m.Disconnect()
	if s, err := m.OpenService(serviceName); err == nil {
		s.Close()
		return fmt.Errorf("the service %s exists; run vlt-setup dev-uninstall first", serviceName)
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	exe := filepath.Join(installDir, exeName)
	if err := copyFile(src, exe); err != nil {
		return fmt.Errorf("copy %s: %w", exeName, err)
	}
	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName:      displayName,
		Description:      description,
		StartType:        mgr.StartManual,
		ErrorControl:     mgr.ErrorNormal,
		ServiceStartName: serviceAccount,
		SidType:          windows.SERVICE_SID_TYPE_UNRESTRICTED,
	})
	if err != nil {
		return fmt.Errorf("CreateService: %w", err)
	}
	defer s.Close()
	e.printf("2. Service %s made (account %s, binary %s).\n", serviceName, serviceAccount, exe)

	sid, _, _, err := windows.LookupSID("", serviceAccount)
	if err != nil {
		return fmt.Errorf("find the SID of %s: %w", serviceAccount, err)
	}

	// 3. User right.
	if err := setRight(sid, shutdownRight, true); err != nil {
		return err
	}
	e.printf("3. User right %s given to %s (%s).\n", shutdownRight, serviceAccount, sid)

	// 4. Required privileges.
	if err := setRequiredPrivileges(s, requiredPrivileges); err != nil {
		return err
	}
	e.printf("4. Required privileges: %v.\n", requiredPrivileges)

	// 5. Data directory.
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	if err := setDataDirACL(dataDir, sid); err != nil {
		return err
	}
	e.printf("5. Data directory %s: ACL set and checked.\n", dataDir)

	// 6. Recovery.
	if err := setRecovery(s); err != nil {
		return err
	}
	e.printf("6. Recovery: restart after the first and the second failure, after 5 seconds.\n")

	// 7. Start.
	if err := s.Start(); err != nil {
		return fmt.Errorf("start the service: %w", err)
	}
	if err := waitState(s, svc.Running, 30*time.Second); err != nil {
		return err
	}
	e.printf("7. Service %s started. Log: %s\n", serviceName, filepath.Join(dataDir, "logs", "vltd.log"))
	return nil
}

func waitState(s *mgr.Service, want svc.State, max time.Duration) error {
	end := time.Now().Add(max)
	for {
		st, err := s.Query()
		if err != nil {
			return err
		}
		if st.State == want {
			return nil
		}
		if want == svc.Running && st.State == svc.Stopped {
			return fmt.Errorf("the service stopped (exit code %d); see the log", st.Win32ExitCode)
		}
		if time.Now().After(end) {
			return fmt.Errorf("the service state is %d after %v", st.State, max)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func devUninstall(e env) error {
	if err := checkAdmin(); err != nil {
		return err
	}
	installDir, dataDir := dirs()
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to the service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		e.printf("Service %s not found.\n", serviceName)
	} else {
		// The SID of the virtual account exists only while the service exists.
		sid, _, _, sidErr := windows.LookupSID("", serviceAccount)
		if st, err := s.Query(); err == nil && st.State != svc.Stopped {
			if _, err := s.Control(svc.Stop); err != nil {
				s.Close()
				return fmt.Errorf("stop the service: %w", err)
			}
			if err := waitState(s, svc.Stopped, 30*time.Second); err != nil {
				s.Close()
				return err
			}
		}
		e.printf("Service %s stopped.\n", serviceName)
		if sidErr == nil {
			if err := setRight(sid, shutdownRight, false); err != nil {
				s.Close()
				return err
			}
			e.printf("User right %s removed from %s.\n", shutdownRight, serviceAccount)
		} else {
			e.printf("Cannot find the SID of %s (%v). The user right is not changed.\n", serviceAccount, sidErr)
		}
		if err := s.Delete(); err != nil {
			s.Close()
			return fmt.Errorf("delete the service: %w", err)
		}
		s.Close()
		e.printf("Service %s removed.\n", serviceName)
	}

	exe := filepath.Join(installDir, exeName)
	if err := os.Remove(exe); err == nil {
		e.printf("%s removed.\n", exe)
	}
	_ = os.Remove(installDir) // only if it is empty

	switch n, err := groupMembers(operatorsGroup); {
	case err != nil:
		return err
	case n < 0:
		e.printf("Group %q not found.\n", operatorsGroup)
	case n > 0:
		e.printf("Group %q has %d members. It is not deleted.\n", operatorsGroup, n)
	default:
		if err := deleteGroup(operatorsGroup); err != nil {
			return err
		}
		e.printf("Group %q deleted (no members).\n", operatorsGroup)
	}

	if _, err := os.Stat(dataDir); err == nil {
		e.printf("The data directory %s contains the Store, the encrypted DEK and the logs.\n", dataDir)
		e.printf("If you delete it, all stored Secrets and Settings are lost.\n")
		if a := e.ask("Type DELETE to delete the data directory, or press Enter to keep it: "); a == deleteAnswer {
			if err := os.RemoveAll(dataDir); err != nil {
				return err
			}
			e.printf("Data directory deleted.\n")
		} else {
			e.printf("Data directory kept.\n")
		}
	}
	return nil
}
