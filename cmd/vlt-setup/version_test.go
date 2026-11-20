// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"
)

// VLT-IS-012 TASK 03-00-00-006 step 2: "version" and "--version".
func TestVersionArgs(t *testing.T) {
	for _, a := range []string{"version", "--version"} {
		var out, errOut bytes.Buffer
		if code := run([]string{a}, strings.NewReader(""), &out, &errOut); code != 0 || strings.TrimSpace(out.String()) != "vlt-setup "+version {
			t.Errorf("%s: %d %q %q", a, code, out.String(), errOut.String())
		}
	}
}
