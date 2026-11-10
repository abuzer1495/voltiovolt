// SPDX-License-Identifier: AGPL-3.0-or-later

package vault

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const vaultImport = "github.com/abuzer1495/voltiovolt/internal/vault"

// useAllowList is the list of packages (directories relative to the module
// root) that can call Vault.Use. It is empty in VLT-IS-003.
var useAllowList = map[string]bool{}

// findUseCalls returns the positions of selector expressions ".Use" in a
// file that imports internal/vault.
func findUseCalls(fset *token.FileSet, f *ast.File) []string {
	imported := false
	for _, im := range f.Imports {
		if p, _ := strconv.Unquote(im.Path.Value); p == vaultImport {
			imported = true
		}
	}
	if !imported {
		return nil
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Use" {
			out = append(out, fset.Position(sel.Pos()).String())
		}
		return true
	})
	return out
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod")
		}
		dir = parent
	}
}

// TestNoUseOutsideVault is VLT-IS-003 TASK 03-00-00-004 step 8.
func TestNoUseOutsideVault(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	files := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && rel != "." || rel == "internal/vault" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		files++
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		if useAllowList[filepath.ToSlash(filepath.Dir(rel))] {
			return nil
		}
		for _, pos := range findUseCalls(fset, f) {
			t.Errorf("Vault.Use outside internal/vault: %s", pos)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 10 {
		t.Fatalf("only %d Go files found; the walk is not correct", files)
	}
}

// TestFindUseCalls proves that the detector finds a call.
func TestFindUseCalls(t *testing.T) {
	src := `package x
import "` + vaultImport + `"
func f(v *vault.Vault) { _ = v.Use(nil, "a", nil); g := v.Use; _ = g }`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(findUseCalls(fset, f)); n != 2 {
		t.Fatalf("found %d, want 2", n)
	}
	src2 := strings.Replace(src, vaultImport, "example.invalid/other", 1)
	f2, _ := parser.ParseFile(fset, "y.go", src2, 0)
	if n := len(findUseCalls(fset, f2)); n != 0 {
		t.Fatalf("found %d in a file without the import", n)
	}
}
