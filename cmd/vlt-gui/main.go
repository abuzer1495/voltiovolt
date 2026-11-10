// SPDX-License-Identifier: AGPL-3.0-or-later

// Command vlt-gui is the VLT GUI (VLT-IS-005). It connects to vltd through
// internal/ipc/client (internal/gui). The front end (web/) is embedded and
// calls internal/gui.API through the Wails bindings. A close of the VLT GUI
// closes the IPC connection only; it does not stop vltd.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/abuzer1495/voltiovolt/internal/gui"
)

//go:embed all:frontend
var assets embed.FS

// emitter sends Wails events.
type emitter struct{ app *application.App }

func (e emitter) Emit(name string, data any) { e.app.Event.Emit(name, data) }

// cspHandler adds the Content Security Policy header (SEC-095). The built
// index.html has the same policy in a meta element.
func cspHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", gui.CSP)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}

func main() {
	addr := flag.String("addr", "", "vltd IPC address (default: the vltd endpoint)")
	flag.Parse()
	if err := run(*addr); err != nil {
		fmt.Fprintln(os.Stderr, "vlt-gui:", err)
		os.Exit(1)
	}
}

func run(addr string) error {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return err
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		return fmt.Errorf("the front end is not built (run npm run build in web/): %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	m := gui.NewManager(addr, nil, log)
	api := gui.NewAPI(m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := application.New(application.Options{
		Name:        "VLT",
		Description: "VLT UPS monitor",
		Services:    []application.Service{application.NewService(api)},
		Assets: application.AssetOptions{
			Handler: cspHandler(application.BundledAssetFileServer(dist)),
		},
		OnShutdown: func() {
			// Close the IPC connection only. vltd continues.
			cancel()
			m.Close()
		},
	})
	m.SetEmitter(emitter{app})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "main",
		Title:  "VLT",
		Width:  1100,
		Height: 760,
		URL:    "/",
	})
	go m.Run(ctx)
	return app.Run()
}
