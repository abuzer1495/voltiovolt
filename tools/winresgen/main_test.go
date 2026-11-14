// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func utf16le(s string) []byte {
	var b bytes.Buffer
	for _, r := range utf16.Encode([]rune(s)) {
		_ = binary.Write(&b, binary.LittleEndian, r)
	}
	return b.Bytes()
}

// TestSyso writes a .syso file and checks the COFF machine and the texts
// of the version resource and the manifest in it.
func TestSyso(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rsrc_windows_amd64.syso")
	if err := run("0.8.0-test.7", "VoltioVolt", "VoltioVolt.exe", out, "amd64"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(b) != 0x8664 {
		t.Fatalf("machine %#x", binary.LittleEndian.Uint16(b))
	}
	for _, s := range []string{"VoltioVolt UPS Management", "Copyright VoltioVolt contributors", "License: AGPL-3.0-or-later", "0.8.0-test.7", "VoltioVolt.exe"} {
		if !bytes.Contains(b, utf16le(s)) {
			t.Errorf("no %q", s)
		}
	}
	if !bytes.Contains(b, []byte("permonitorv2")) {
		t.Error("no manifest")
	}
	if !bytes.Contains(b, []byte("\x89PNG")) {
		t.Error("no icon")
	}
}
