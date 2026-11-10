// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const controlImport = "github.com/abuzer1495/voltiovolt/internal/control"

// SEC-098 allow lists (directories relative to the module root). The
// package internal/control itself (its tests included) is always allowed.
var (
	// grantAllow: the packages that can make each Grant.
	grantAllow = map[string]map[string]bool{
		"IPCGrant":    {"internal/ipc": true},
		"PolicyGrant": {"internal/policy": true},
	}
)

type finding struct{ kind, name, pos string }

// inspectControl returns the SEC-098 findings in one file.
func inspectControl(fset *token.FileSet, f *ast.File, checkLiterals bool) []finding {
	names := map[string]bool{} // local names of the control import
	for _, im := range f.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		if p != controlImport {
			continue
		}
		n := "control"
		if im.Name != nil {
			n = im.Name.Name
		}
		if n == "." || n == "_" {
			names["."] = true
			continue
		}
		names[n] = true
	}
	var out []finding
	isControl := func(e ast.Expr) bool {
		id, ok := e.(*ast.Ident)
		return ok && names[id.Name]
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if !isControl(x.X) {
				return true
			}
			name := x.Sel.Name
			switch {
			case name == "IPCGrant" || name == "PolicyGrant":
				out = append(out, finding{"grant", name, fset.Position(x.Pos()).String()})
			case strings.HasPrefix(name, "Encode"):
				out = append(out, finding{"encoder", name, fset.Position(x.Pos()).String()})
			}
		case *ast.CompositeLit:
			if sel, ok := x.Type.(*ast.SelectorExpr); ok && isControl(sel.X) && (sel.Sel.Name == "Grant" || sel.Sel.Name == "Command") {
				out = append(out, finding{"literal", sel.Sel.Name, fset.Position(x.Pos()).String()})
			}
		case *ast.BasicLit:
			if checkLiterals && x.Kind == token.STRING {
				if s, err := strconv.Unquote(x.Value); err == nil && Check([]byte(s)) == nil {
					out = append(out, finding{"bytes", s, fset.Position(x.Pos()).String()})
				}
			}
		}
		return true
	})
	if names["."] {
		out = append(out, finding{"dotimport", "", fset.Position(f.Pos()).String()})
	}
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

// TestOnlyControlMakesControlBytes is VLT-IS-006 TASK 03-00-00-005 step 3.3
// (SEC-098). It fails if a package other than internal/ipc makes an IPC
// Grant, a package other than internal/policy makes a Policy Grant, a
// package other than internal/control calls an encoder or makes a Grant or
// Command literal, or a non-test file outside internal/control has the
// bytes of a CONTROL command. The pattern is the internal/shutdown test
// TestOnlyPolicyCallsExecutor.
func TestOnlyControlMakesControlBytes(t *testing.T) {
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
			if strings.HasPrefix(d.Name(), ".") && rel != "." || d.Name() == "testdata" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		files++
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "internal/control" {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		isTest := strings.HasSuffix(p, "_test.go")
		for _, x := range inspectControl(fset, f, !isTest) {
			switch {
			case x.kind == "grant" && !grantAllow[x.name][dir]:
				t.Errorf("control.%s outside its package: %s", x.name, x.pos)
			case x.kind == "encoder":
				t.Errorf("control.%s outside internal/control: %s", x.name, x.pos)
			case x.kind == "literal":
				t.Errorf("control.%s literal outside internal/control: %s", x.name, x.pos)
			case x.kind == "bytes":
				t.Errorf("CONTROL bytes %q outside internal/control: %s", x.name, x.pos)
			case x.kind == "dotimport":
				t.Errorf("dot import of internal/control: %s", x.pos)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 50 {
		t.Fatalf("only %d Go files found; the walk is not correct", files)
	}
}

// TestInspectControlFinds proves that the detector finds each kind.
func TestInspectControlFinds(t *testing.T) {
	src := `package x
import (
	ctl "` + controlImport + `"
)
func f() {
	g, _ := ctl.IPCGrant(true, ctl.FuncBeeper, "")
	p := ctl.PolicyGrant()
	_, _ = ctl.EncodeBeeperToggle(g)
	_, _ = ctl.EncodeShutdownRestore(p, 0, 1)
	_ = ctl.Grant{}
	_ = &ctl.Command{}
	_ = "S.5R0001\r"
	_ = "Q1\r" // a READ command
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	n := map[string]int{}
	for _, x := range inspectControl(fset, f, true) {
		n[x.kind]++
	}
	if n["grant"] != 2 || n["encoder"] != 2 || n["literal"] != 2 || n["bytes"] != 1 {
		t.Fatalf("findings %v, want grant 2, encoder 2, literal 2, bytes 1", n)
	}
}

// exportedFuncs returns the exported top-level functions of this package
// (non-test files) with the prefix.
func exportedFuncs(t *testing.T, prefix string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.IsExported() && strings.HasPrefix(fd.Name.Name, prefix) {
					out = append(out, fd.Name.Name)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestNoIPCShutdown (SEC-098, TASK 03-00-00-007 step 2): no non-test file
// of internal/ipc or internal/gui uses the shutdown and restore function
// or the cancel shutdown function, and no non-test file outside
// internal/control uses FuncCancelShutdown ("C" is never sent). Test files
// can use the names to check that a result does not contain them.
func TestNoIPCShutdown(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && rel != "." || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "internal/control" {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ipcSide := strings.HasPrefix(dir, "internal/ipc") || strings.HasPrefix(dir, "internal/gui")
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "FuncCancelShutdown", "EncodeCancelShutdown":
				t.Errorf("cancel shutdown used outside internal/control: %s", fset.Position(sel.Pos()))
			case "ShutdownRestore", "FuncShutdownRestore", "EncodeShutdownRestore":
				if ipcSide {
					t.Errorf("shutdown and restore used in %s: %s", dir, fset.Position(sel.Pos()))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
