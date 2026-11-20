// SPDX-License-Identifier: AGPL-3.0-or-later

package version

import "testing"

func TestFromTag(t *testing.T) {
	for tag, want := range map[string]string{"v0.9.0-rc.1": "0.9.0-rc.1", "v1.0.0": "1.0.0", "v10.20.30": "10.20.30", "v1.2.3-rc.12": "1.2.3-rc.12"} {
		if got, ok := FromTag(tag); !ok || got != want {
			t.Errorf("FromTag(%q) = %q %v", tag, got, ok)
		}
	}
	for _, tag := range []string{"", "0.9.0", "v0.9", "v0.9.0-rc", "v0.9.0-rc.0", "v0.9.0-beta.1", "v01.0.0", "test-latest", "v1.0.0+abc", "v1.0.0-rc.1 ", "v1.0.0\n"} {
		if got, ok := FromTag(tag); ok {
			t.Errorf("FromTag(%q) = %q, want no release tag", tag, got)
		}
	}
}

func TestTest(t *testing.T) {
	if v, err := Test("77d1134"); err != nil || v != "0.0.0-test+77d1134" {
		t.Fatalf("%q %v", v, err)
	}
	if v, err := Test("77d11342e4591ec47af44d7ec91c793142cddd02"); err != nil || v != "0.0.0-test+77d1134" {
		t.Fatalf("%q %v", v, err)
	}
	for _, s := range []string{"", "77d11", "XYZ1234", "77d1134 "} {
		if _, err := Test(s); err == nil {
			t.Errorf("Test(%q) accepted", s)
		}
	}
	if IsRelease("0.0.0-test+77d1134") || IsRC("0.0.0-test+77d1134") {
		t.Fatal("a test build is not a release")
	}
}

func TestWindowTitle(t *testing.T) {
	for v, want := range map[string]string{
		"0.9.0-rc.1": "VoltioVolt 0.9.0-rc.1", "1.0.0": "VoltioVolt", "0.0.0-test+77d1134": "VoltioVolt", "0.0.0-dev": "VoltioVolt", "": "VoltioVolt",
	} {
		if got := WindowTitle(v); got != want {
			t.Errorf("WindowTitle(%q) = %q, want %q", v, got, want)
		}
	}
	if !IsRC("0.9.0-rc.1") || IsRC("0.9.0") || !IsRelease("0.9.0") || !IsRelease("0.9.0-rc.1") {
		t.Fatal("IsRC or IsRelease")
	}
	if Line("vltd", "0.9.0-rc.1") != "vltd 0.9.0-rc.1" {
		t.Fatal("Line")
	}
}
