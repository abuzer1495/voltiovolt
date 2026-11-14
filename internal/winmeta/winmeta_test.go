// SPDX-License-Identifier: AGPL-3.0-or-later

package winmeta

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestNumericVersion(t *testing.T) {
	cases := map[string]string{
		"0.8.0":                    "0.8.0.0",
		"v1.2.3":                   "1.2.3.0",
		"0.8.0-test.20261114.1200": "0.8.0.0",
		"1.2.3.4":                  "1.2.3.4",
		"ci-abc123":                "0.0.0.0",
		"0.0.0-dev":                "0.0.0.0",
		"70000.1":                  "0.0.0.0",
	}
	for in, want := range cases {
		if got := DottedVersion(in); got != want {
			t.Errorf("%q: %s", in, got)
		}
	}
}

// TestManifest is step 3.3: valid XML, per-monitor v2, Common Controls 6,
// asInvoker.
func TestManifest(t *testing.T) {
	m := Manifest("VoltioVolt", "0.8.0-test.1")
	var v struct{}
	if err := xml.Unmarshal([]byte(m), &v); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`permonitorv2,permonitor</dpiAwareness>`,
		`true/pm</dpiAware>`,
		`name="Microsoft.Windows.Common-Controls" version="6.0.0.0"`,
		`level="asInvoker"`,
		`version="0.8.0.0"`,
	} {
		if !strings.Contains(m, s) {
			t.Errorf("no %s", s)
		}
	}
}

func TestTexts(t *testing.T) {
	if ProductName != "VoltioVolt" || FileDescription != "VoltioVolt UPS Management" || License != "AGPL-3.0-or-later" ||
		!strings.Contains(Copyright, "VoltioVolt contributors") {
		t.Fatal("texts of step 3.2")
	}
}
