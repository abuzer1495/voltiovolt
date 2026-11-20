// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"io"
	"testing"
)

// VLT-IS-012 TASK 03-00-00-006 step 2: "version" and "--version".
func TestVersionArgs(t *testing.T) {
	for _, a := range []string{"version", "--version", "-version"} {
		c, err := parseArgs([]string{a}, io.Discard)
		if err != nil || c.mode != "version" {
			t.Errorf("%s: %+v %v", a, c, err)
		}
	}
	if _, err := parseArgs([]string{"--version", "x"}, io.Discard); err == nil {
		t.Error("--version with an argument accepted")
	}
}
