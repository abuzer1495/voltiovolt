// SPDX-License-Identifier: AGPL-3.0-or-later

package shutdown

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

const shutdownImport = "github.com/abuzer1495/voltiovolt/internal/shutdown"

// SEC-092 allow lists (directories relative to the module root).
var (
	// importAllow: packages that can import internal/shutdown. cmd/vltd
	// and cmd/voltiovolt give the executor to the Policy Engine.
	importAllow = map[string]bool{"cmd/vltd": true, "cmd/voltiovolt": true, "internal/shutdown": true}
	// callAllow: packages that can call Shutdown(ctx, reason) or
	// Hibernate(ctx, reason) on an executor.
	callAllow = map[string]bool{"internal/policy": true, "internal/shutdown": true}
	// osAllow: packages that can use the OS shutdown functions.
	osAllow = map[string]bool{"internal/shutdown": true}
)

// osNames are imports and functions that turn off a computer.
// godbus is not an OS-shutdown import by itself: the portable Vault uses
// it for org.freedesktop.secrets (VLT-IS-008 TASK 03-00-00-002). The
// login1 method names stay in the string detector below.
var osImports = map[string]bool{}
var osFuncs = map[string]bool{
	"InitiateSystemShutdownEx": true, "InitiateSystemShutdown": true, "ExitWindowsEx": true,
	"SetSuspendState": true, "PowerOff": true, "Reboot": true,
}

type finding struct{ kind, pos string }

// inspect returns the SEC-092 findings in one file.
func inspect(fset *token.FileSet, f *ast.File) []finding {
	var out []finding
	for _, im := range f.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		if p == shutdownImport {
			out = append(out, finding{"import", fset.Position(im.Pos()).String()})
		}
		if osImports[p] {
			out = append(out, finding{"os", fset.Position(im.Pos()).String()})
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Shutdown" || sel.Sel.Name == "Hibernate") && len(x.Args) == 2 {
				out = append(out, finding{"call", fset.Position(x.Pos()).String()})
			}
		case *ast.SelectorExpr:
			if osFuncs[x.Sel.Name] {
				out = append(out, finding{"os", fset.Position(x.Pos()).String()})
			}
		case *ast.BasicLit:
			// A D-Bus method name or a DLL procedure name in a string.
			if x.Kind == token.STRING {
				s, _ := strconv.Unquote(x.Value)
				if strings.Contains(s, "login1") || osFuncs[s] || strings.HasPrefix(s, "InitiateSystemShutdown") {
					out = append(out, finding{"os", fset.Position(x.Pos()).String()})
				}
			}
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

// TestOnlyPolicyCallsExecutor is VLT-IS-004 TASK 03-00-00-007 step 4
// (SEC-092). Test files are not checked for calls: they use fakes.
func TestOnlyPolicyCallsExecutor(t *testing.T) {
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
			if strings.HasPrefix(d.Name(), ".") && rel != "." || d.Name() == "testdata" {
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
		dir := filepath.ToSlash(filepath.Dir(rel))
		isTest := strings.HasSuffix(p, "_test.go")
		for _, x := range inspect(fset, f) {
			switch {
			case x.kind == "import" && !importAllow[dir]:
				t.Errorf("internal/shutdown imported outside cmd/vltd: %s", x.pos)
			case x.kind == "call" && !isTest && !callAllow[dir]:
				t.Errorf("executor call outside internal/policy: %s", x.pos)
			case x.kind == "os" && !isTest && !osAllow[dir]:
				t.Errorf("OS shutdown function outside internal/shutdown: %s", x.pos)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 20 {
		t.Fatalf("only %d Go files found; the walk is not correct", files)
	}
}

// TestInspectFinds proves that the detector finds each kind.
func TestInspectFinds(t *testing.T) {
	src := `package x
import (
	"` + shutdownImport + `"
	"github.com/godbus/dbus/v5"
)
func f(e *shutdown.Executor) {
	_ = e.Shutdown(nil, "a")
	_ = e.Hibernate(nil, "b")
	_ = srv.Shutdown(nil) // one argument: not an executor call
	_ = windows.InitiateSystemShutdownEx(nil, nil, 0, false, false, 0)
	_ = "org.freedesktop.login1.Manager.PowerOff"
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	n := map[string]int{}
	for _, x := range inspect(fset, f) {
		n[x.kind]++
	}
	if n["import"] != 1 || n["call"] != 2 || n["os"] != 2 {
		t.Fatalf("findings %v, want import 1, call 2, os 2 (InitiateSystemShutdownEx and login1)", n)
	}
}
