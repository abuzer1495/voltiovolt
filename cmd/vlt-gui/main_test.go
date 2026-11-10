// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abuzer1495/voltiovolt/internal/gui"
)

// TestCSPHeader is SEC-095: each response has the CSP header.
func TestCSPHeader(t *testing.T) {
	h := cspHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("x")) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Content-Security-Policy"); got != gui.CSP {
		t.Fatalf("CSP header %q", got)
	}
}

// TestEmbeddedFrontEnd is SEC-095: the built front end is in the binary
// and its index.html has the CSP.
func TestEmbeddedFrontEnd(t *testing.T) {
	b, err := fs.ReadFile(assets, "frontend/dist/index.html")
	if err != nil {
		t.Fatalf("front end not built: %v", err)
	}
	if !strings.Contains(string(b), gui.CSP) {
		t.Fatal("index.html has no CSP")
	}
}
