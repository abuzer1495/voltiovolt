// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package vault

import (
	"testing"
)

func TestNewPortableProviderIsDPAPI(t *testing.T) {
	dir := t.TempDir()
	id := make([]byte, 16)
	p, ok := NewPortableProvider(dir, id).(*DPAPIProvider)
	if !ok {
		t.Fatalf("type %T", NewPortableProvider(dir, id))
	}
	h := NewHostProvider(dir, id).(*DPAPIProvider)
	if p.Path != h.Path {
		t.Fatalf("path %q vs %q", p.Path, h.Path)
	}
	if string(p.entropy()) != string(h.entropy()) {
		t.Fatal("entropy is not the same as NewHostProvider")
	}
	if string(p.entropy())[:len(entropyPrefix)] != entropyPrefix {
		t.Fatalf("entropy prefix %q", p.entropy())
	}
}
