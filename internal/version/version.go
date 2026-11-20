// SPDX-License-Identifier: AGPL-3.0-or-later

// Package version has the rules of the VoltioVolt version (VLT-IS-012 TASK
// 03-00-00-006, OD-57). The version comes from the Git tag: vX.Y.Z or
// vX.Y.Z-rc.N. A build without a release tag has the version
// "0.0.0-test+<short SHA>". The build sets the version text of each
// binary with -ldflags "-X main.version=...".
package version

import (
	"fmt"
	"regexp"
)

// Product is the product name.
const Product = "VoltioVolt"

var (
	tagRe     = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.([1-9][0-9]*))?$`)
	releaseRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.([1-9][0-9]*))?$`)
	shaRe     = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

// FromTag returns the version of a release tag: "v0.9.0-rc.1" gives
// "0.9.0-rc.1". It returns false for a tag that is not a release tag.
func FromTag(tag string) (string, bool) {
	if !tagRe.MatchString(tag) {
		return "", false
	}
	return tag[1:], true
}

// Test returns the version of a build without a release tag.
func Test(shortSHA string) (string, error) {
	if !shaRe.MatchString(shortSHA) {
		return "", fmt.Errorf("version: %q is not a short commit SHA", shortSHA)
	}
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	return "0.0.0-test+" + shortSHA, nil
}

// IsRelease reports whether v is a release version (X.Y.Z or X.Y.Z-rc.N).
func IsRelease(v string) bool { return releaseRe.MatchString(v) }

// IsRC reports whether v is a release candidate (X.Y.Z-rc.N).
func IsRC(v string) bool {
	m := releaseRe.FindStringSubmatch(v)
	return m != nil && m[4] != ""
}

// WindowTitle is the title of the main window: "VoltioVolt 0.9.0-rc.1"
// for a release candidate, else "VoltioVolt" (step 2).
func WindowTitle(v string) string {
	if IsRC(v) {
		return Product + " " + v
	}
	return Product
}

// Line is the text of --version: "<name> <version>".
func Line(name, v string) string { return name + " " + v }
