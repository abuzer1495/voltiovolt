// SPDX-License-Identifier: AGPL-3.0-or-later

// Command winresgen writes the Windows resource file (.syso) of
// VoltioVolt.exe or vlt-gui.exe before the Windows build (VLT-IS-011 TASK
// 03-00-00-005 step 3): the application icon (internal/appicon), the
// version resource and the application manifest (internal/winmeta). The
// Go linker adds a .syso file in the package folder to the exe.
//
//	go run . -version 0.8.0-test.1 -name VoltioVolt -file VoltioVolt.exe -out ../../cmd/voltiovolt/rsrc_windows_amd64.syso
//
// It is a separate module so that its dependency (goversioninfo, MIT, and
// rsrc, MIT) is not in the module of the binaries.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephspurrier/goversioninfo"

	"github.com/abuzer1495/voltiovolt/internal/appicon"
	"github.com/abuzer1495/voltiovolt/internal/winmeta"
)

func main() {
	version := flag.String("version", "0.0.0-dev", "build version")
	name := flag.String("name", winmeta.ProductName, "internal name")
	file := flag.String("file", "VoltioVolt.exe", "original file name")
	out := flag.String("out", "rsrc_windows_amd64.syso", "output .syso file")
	arch := flag.String("arch", "amd64", "amd64 or arm64")
	flag.Parse()
	if err := run(*version, *name, *file, *out, *arch); err != nil {
		fmt.Fprintln(os.Stderr, "winresgen:", err)
		os.Exit(1)
	}
}

// versionInfo returns the version resource of step 3.2.
func versionInfo(version, name, file string) goversioninfo.VersionInfo {
	n := winmeta.NumericVersion(version)
	fv := goversioninfo.FileVersion{Major: int(n[0]), Minor: int(n[1]), Patch: int(n[2]), Build: int(n[3])}
	var vi goversioninfo.VersionInfo
	vi.FixedFileInfo.FileVersion = fv
	vi.FixedFileInfo.ProductVersion = fv
	vi.FixedFileInfo.FileFlagsMask = "3f"
	vi.FixedFileInfo.FileOS = "040004" // VOS_NT_WINDOWS32
	vi.FixedFileInfo.FileType = "01"   // VFT_APP
	vi.StringFileInfo = goversioninfo.StringFileInfo{
		Comments:         winmeta.Comments,
		CompanyName:      "VoltioVolt contributors",
		FileDescription:  winmeta.FileDescription,
		FileVersion:      version,
		InternalName:     name,
		LegalCopyright:   winmeta.Copyright,
		OriginalFilename: file,
		ProductName:      winmeta.ProductName,
		ProductVersion:   version,
	}
	vi.VarFileInfo.Translation = goversioninfo.Translation{LangID: goversioninfo.LngUSEnglish, CharsetID: goversioninfo.CsUnicode}
	return vi
}

func run(version, name, file, out, arch string) error {
	tmp, err := os.MkdirTemp("", "winresgen")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	ico := filepath.Join(tmp, "app.ico")
	man := filepath.Join(tmp, "app.manifest")
	if err := os.WriteFile(ico, appicon.ICO(), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(man, []byte(winmeta.Manifest(name, version)), 0o600); err != nil {
		return err
	}
	vi := versionInfo(version, name, file)
	// One icon group with the ID IDI_APPLICATION (32512): Explorer, the
	// Start menu shortcut and the window class of Wails use it.
	vi.ApplicationIconPath = ico
	vi.ManifestPath = man
	vi.Build()
	vi.Walk()
	return vi.WriteSyso(out, arch)
}
