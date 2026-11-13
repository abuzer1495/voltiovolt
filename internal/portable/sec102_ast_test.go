// SPDX-License-Identifier: AGPL-3.0-or-later

package portable

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPortableModeNoSystemChange is the acceptance test of VLT-IS-008
// 04-00-00: no code in portable mode calls a service, group, user right,
// registry or ACL function (SEC-102). It inspects the portable-only
// packages: this package and cmd/voltiovolt (non-test source).
func TestPortableModeNoSystemChange(t *testing.T) {
	root := moduleRoot(t)
	dirs := []string{
		filepath.Join(root, "internal", "portable"),
		filepath.Join(root, "cmd", "voltiovolt"),
	}
	forbiddenImports := map[string]string{
		"golang.org/x/sys/windows/svc":                       "service",
		"golang.org/x/sys/windows/svc/mgr":                   "service",
		"golang.org/x/sys/windows/registry":                  "registry",
		"github.com/abuzer1495/voltiovolt/internal/platform": "service",
		"github.com/abuzer1495/voltiovolt/cmd/vlt-setup":     "service",
	}
	// Function or selector names that change the operating system.
	// AdjustTokenPrivileges is not in this list: TASK 03-00-00-004 uses
	// it to enable a right that a normal user already has, and does not
	// change the user-right assignment (LsaAddAccountRights).
	forbiddenNames := map[string]string{
		"CreateService":           "service",
		"OpenService":             "service",
		"StartService":            "service",
		"ControlService":          "service",
		"ChangeServiceConfig":     "service",
		"DeleteService":           "service",
		"NetLocalGroupAdd":        "group",
		"NetLocalGroupDel":        "group",
		"NetLocalGroupAddMembers": "group",
		"NetLocalGroupDelMembers": "group",
		"NetUserAdd":              "group",
		"NetUserDel":              "group",
		"LsaAddAccountRights":     "user right",
		"LsaRemoveAccountRights":  "user right",
		"LsaOpenPolicy":           "user right",
		"CreateKey":               "registry",
		"SetStringValue":          "registry",
		"SetDWordValue":           "registry",
		"SetQWordValue":           "registry",
		"SetValue":                "registry",
		"DeleteValue":             "registry",
		"DeleteKey":               "registry",
		"SetNamedSecurityInfo":    "ACL",
		"SetNamedSecurityInfoW":   "ACL",
		"SetSecurityInfo":         "ACL",
		"SetEntriesInAcl":         "ACL",
		"SetEntriesInAclW":        "ACL",
		"SetFileSecurity":         "ACL",
		"SetKernelObjectSecurity": "ACL",
		"CheckDataDir":            "ACL",
		"CheckDataDirFor":         "ACL",
		"setDataDirACL":           "ACL",
	}
	var findings []string
	fset := token.NewFileSet()
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("portable source %s: %v", dir, err)
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				n := d.Name()
				if n == "frontend" || n == "dist" || n == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, im := range f.Imports {
				p, _ := strconv.Unquote(im.Path.Value)
				if kind, ok := forbiddenImports[p]; ok {
					findings = append(findings, fset.Position(im.Pos()).String()+": import "+p+" ("+kind+")")
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if kind, ok := forbiddenNames[id.Name]; ok {
					findings = append(findings, fset.Position(id.Pos()).String()+": "+id.Name+" ("+kind+")")
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(findings) > 0 {
		t.Fatalf("portable mode calls a forbidden function:\n  %s", strings.Join(findings, "\n  "))
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
