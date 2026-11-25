// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build production

package main

import (
	"fmt"

	"github.com/abuzer1495/voltiovolt/internal/engine"
)

const demoToolsEnabled = false

func prepareDemo(string, demoOpts) error {
	return fmt.Errorf("demo capture flags are not available in a production build")
}

func demoNavJS(string) string { return "" }

func demoReadyTouch(string) {}

func withDemoStore(demoOpts) func(*engine.Config) { return func(*engine.Config) {} }
