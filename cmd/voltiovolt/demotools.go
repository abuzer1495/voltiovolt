// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !production

package main

import (
	"context"

	"github.com/abuzer1495/voltiovolt/internal/engine"
	"github.com/abuzer1495/voltiovolt/internal/shotnav"
	"github.com/abuzer1495/voltiovolt/internal/store"
)

// demoToolsEnabled is true only in test and development builds.
const demoToolsEnabled = true

func prepareDemo(dir string, _ demoOpts) error {
	return shotnav.PrepareDir(dir)
}

func demoNavJS(screen string) string { return shotnav.NavJS(screen) }

func demoReadyTouch(path string) { shotnav.ReadyTouch(path) }

func withDemoStore(o demoOpts) func(*engine.Config) {
	return func(cfg *engine.Config) {
		cfg.PrepareStore = func(ctx context.Context, st *store.Store) error {
			return shotnav.ApplySettings(ctx, st, shotnav.Options{
				Demo: o.Demo, Screen: o.Screen, Lang: o.Lang, Mode: o.Mode,
			})
		}
	}
}
