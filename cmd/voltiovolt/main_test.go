// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/gui"
)

func TestCSPHeader(t *testing.T) {
	h := cspHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("x")) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Content-Security-Policy"); got != gui.CSP {
		t.Fatalf("CSP header %q", got)
	}
}

func TestEmbeddedFrontEnd(t *testing.T) {
	b, err := fs.ReadFile(assets, "frontend/dist/index.html")
	if err != nil {
		t.Fatalf("front end not built: %v", err)
	}
	if !strings.Contains(string(b), gui.CSP) {
		t.Fatal("index.html has no CSP")
	}
}

func TestSelfTest(t *testing.T) {
	dir := t.TempDir()
	if err := SelfTest(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "self-test-result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.HasPrefix(got, "ok ") {
		t.Fatalf("result %q", got)
	}
	if !strings.Contains(got, "network=pipe") {
		t.Fatalf("not in-process: %q", got)
	}
}

func TestExactTexts(t *testing.T) {
	if TextStillMonitoring != "VoltioVolt continues to monitor the UPS in the tray." {
		t.Fatal(TextStillMonitoring)
	}
	if TextQuitConfirm != "If you quit, VoltioVolt stops the monitoring and the automatic shutdown." {
		t.Fatal(TextQuitConfirm)
	}
}
