// SPDX-License-Identifier: AGPL-3.0-or-later

// Package winmeta has the data of the Windows resources of VoltioVolt.exe
// and vlt-gui.exe (VLT-IS-011 TASK 03-00-00-005 steps 3.2 and 3.3): the
// version resource texts and the application manifest. tools/winresgen
// writes them with the icon of internal/appicon into a .syso file before
// the Windows build.
package winmeta

import (
	"fmt"
	"strconv"
	"strings"
)

// Texts of the version resource (step 3.2).
const (
	ProductName     = "VoltioVolt"
	FileDescription = "VoltioVolt UPS Management"
	Copyright       = "Copyright VoltioVolt contributors"
	License         = "AGPL-3.0-or-later"
	// Comments carries the license: the version resource has no license
	// field.
	Comments = "License: " + License
)

// NumericVersion returns the four numbers of a build version for the
// fixed part of the version resource: "0.8.0-test.3" is 0.8.0.0,
// "ci-abc" is 0.0.0.0. The full text stays in the ProductVersion and
// FileVersion strings.
func NumericVersion(v string) [4]uint16 {
	var out [4]uint16
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	for i, p := range strings.SplitN(v, ".", 4) {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return [4]uint16{}
		}
		out[i] = uint16(n)
	}
	return out
}

// DottedVersion is NumericVersion as "a.b.c.d".
func DottedVersion(v string) string {
	n := NumericVersion(v)
	return fmt.Sprintf("%d.%d.%d.%d", n[0], n[1], n[2], n[3])
}

// Manifest is the application manifest (step 3.3):
//   - per-monitor DPI awareness v2 (with per-monitor v1 as the fallback
//     of old Windows 10 builds). Wails v3 sets the same at run time when
//     the process has no DPI setting (pkg/application/application_windows.go,
//     SetProcessDpiAwarenessContext); the manifest sets it before the
//     first window.
//   - Common Controls version 6: the native question dialog of Wails uses
//     a TaskDialog for the button texts (Yes, No, Cancel in the language
//     of the user); without version 6 it falls back to a MessageBox.
//   - Windows 10 and 11, asInvoker (no administrator rights).
func Manifest(name, version string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0" xmlns:asmv3="urn:schemas-microsoft-com:asm.v3">
  <assemblyIdentity type="win32" name="` + name + `" version="` + DottedVersion(version) + `" processorArchitecture="*"/>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <asmv3:application>
    <asmv3:windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">permonitorv2,permonitor</dpiAwareness>
    </asmv3:windowsSettings>
  </asmv3:application>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>
</assembly>
`
}
