// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// demoOpts are the test-only GUI capture flags. In production builds
// demoToolsEnabled is false and the flags are rejected.
type demoOpts struct {
	Demo      bool
	Screen    string
	Lang      string
	Mode      string
	ReadyFile string
}

func (o demoOpts) any() bool {
	return o.Demo || o.Screen != "" || o.Lang != "" || o.Mode != "" || o.ReadyFile != ""
}
