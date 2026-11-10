// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"
	"unicode/utf16"
)

// Subcommands. There are no other subcommands (TASK 03-00-00-002 step 1).
const (
	cmdInstall   = "dev-install"
	cmdUninstall = "dev-uninstall"
)

// Names of the development install.
const (
	serviceName    = "vltd"
	serviceAccount = `NT SERVICE\vltd` // virtual account (VLT-IS-002 QA-03)
	displayName    = "VoltioVolt service (development)"
	description    = "VoltioVolt UPS service, development install"
	operatorsGroup = "VLT Operators"
	groupComment   = "VoltioVolt operators: read and write access to the vltd IPC pipe"
	installSubdir  = "VoltioVolt" // in %ProgramFiles%
	dataSubdir     = "VoltioVolt" // in %ProgramData%
	exeName        = "vltd.exe"
	deleteAnswer   = "DELETE"
)

// shutdownRight is the user right that LsaAddAccountRights gives to the SID
// of NT SERVICE\vltd (step 2.3).
const shutdownRight = "SeShutdownPrivilege"

// requiredPrivileges is the list for SERVICE_CONFIG_REQUIRED_PRIVILEGES_INFO
// (step 2.4). Source: Microsoft Learn, SERVICE_REQUIRED_PRIVILEGES_INFOW
// structure: "If you specify privileges for a service, the SCM will remove
// the privileges that are not required from the process token when the
// process starts." and "the SeChangeNotifyPrivilege privilege is never
// removed from a process token ... a service need not explicitly specify
// this privilege." We list it to make the list explicit. SeShutdownPrivilege
// is necessary for InitiateSystemShutdownEx (Microsoft Learn,
// InitiateSystemShutdownExW: "the calling thread must have the
// SE_SHUTDOWN_NAME privilege"). No other privilege is added.
var requiredPrivileges = []string{"SeShutdownPrivilege", "SeChangeNotifyPrivilege"}

// recoveryAction is one entry of the service failure actions (step 2.6).
type recoveryAction struct {
	restart bool
	delay   time.Duration
}

// recoveryPlan: restart after the first and the second failure, after 5
// seconds. The SCM repeats the last entry for later failures, so the third
// entry is "no action".
var recoveryPlan = []recoveryAction{
	{restart: true, delay: 5 * time.Second},
	{restart: true, delay: 5 * time.Second},
	{restart: false, delay: 0},
}

// recoveryResetSeconds is the time without failures after which the SCM
// sets the failure count to 0 (one day).
const recoveryResetSeconds = 24 * 60 * 60

// dataDirSDDL is the ACL of VLT-IS-003 TASK 03-00-00-003: protected (no
// inheritance from the parent), SYSTEM full control, Administrators full
// control, NT SERVICE\vltd modify (0x1301bf), for the directory, the
// subdirectories and the files.
func dataDirSDDL(serviceSID string) string {
	return fmt.Sprintf("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;%s)", serviceSID)
}

// multiString encodes a REG_MULTI_SZ style list: each string ends with a
// 0, and the list ends with one more 0.
func multiString(list []string) []uint16 {
	var out []uint16
	for _, s := range list {
		out = append(out, utf16.Encode([]rune(s))...)
		out = append(out, 0)
	}
	return append(out, 0)
}
