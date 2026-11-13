// SPDX-License-Identifier: AGPL-3.0-or-later

// Command voltiovolt is the portable VLT application (VLT-IS-008): the
// VLT GUI and the monitoring engine in one process. The binary is
// VoltioVolt.exe on Windows and voltiovolt on Linux. The GUI connects to
// the engine through net.Pipe (no named pipe, no Unix socket).
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/abuzer1495/voltiovolt/internal/audit"
	"github.com/abuzer1495/voltiovolt/internal/device"
	"github.com/abuzer1495/voltiovolt/internal/engine"
	"github.com/abuzer1495/voltiovolt/internal/gui"
	"github.com/abuzer1495/voltiovolt/internal/ipc"
	"github.com/abuzer1495/voltiovolt/internal/ipc/client"
	"github.com/abuzer1495/voltiovolt/internal/logging"
	"github.com/abuzer1495/voltiovolt/internal/policy"
	"github.com/abuzer1495/voltiovolt/internal/portable"
	"github.com/abuzer1495/voltiovolt/internal/shutdown"
	"github.com/abuzer1495/voltiovolt/internal/transport"
	"github.com/abuzer1495/voltiovolt/internal/vault"
)

//go:embed all:frontend
var assets embed.FS

// version is set at build time with -ldflags "-X main.version=...".
var version = "0.0.0-dev"

// Exact English texts of VLT-IS-008 TASK 03-00-00-001 steps 5.1 and 5.2.
const (
	TextStillMonitoring = "VoltioVolt continues to monitor the UPS in the tray."
	TextQuitConfirm     = "If you quit, VoltioVolt stops the monitoring and the automatic shutdown."
)

func main() {
	fs := flag.NewFlagSet("VoltioVolt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	selfTest := fs.Bool("self-test", false, "start with the simulator, run a short check and stop")
	dataDirFlag := fs.String("data-dir", "", "data folder (default: the portable data folder)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showVersion {
		fmt.Println("VoltioVolt", version)
		return
	}
	dir := *dataDirFlag
	if dir == "" {
		var err error
		dir, err = portable.DataDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "VoltioVolt:", err)
			os.Exit(1)
		}
	}
	if *selfTest {
		if err := SelfTest(dir); err != nil {
			fmt.Fprintln(os.Stderr, "VoltioVolt self-test:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "VoltioVolt:", err)
		os.Exit(1)
	}
}

type portableAutostart struct{}

func (portableAutostart) Enabled() (bool, error) { return portable.AutostartEnabled() }
func (portableAutostart) Set(on bool) error      { return portable.SetAutostart(on) }

// startEngine starts the monitoring engine with an in-process pipe.
// CheckPerms is false: portable mode does not check or change the ACL
// (VLT-IS-008 01-20-00, SEC-102).
func startEngine(dir string, log *slog.Logger, closer io.Closer, listen func() (net.Listener, ipc.PeerCheck, error), opts ...func(*engine.Config)) (*engine.Daemon, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	cfg := engine.Config{
		DataDir:    dir,
		CheckPerms: false,
		Provider:   vault.NewPortableProvider,
		Listen:     listen,
		Log:        log,
		LogCloser:  closer,
		Version:    version,
		Lister:     device.OSLister(transport.Options{Timeout: device.ResponseTimeout}),
		Scanner:    device.RealScanner(),
		MakeExecutor: func(forceClose func() bool, au *audit.Log) policy.Executor {
			return shutdown.New(shutdown.Options{ForceClose: forceClose, Audit: au, Log: log})
		},
	}
	for _, o := range opts {
		o(&cfg)
	}
	d := engine.New(cfg)
	if err := d.Start(context.Background()); err != nil {
		return nil, err
	}
	return d, nil
}

func run(dir string) error {
	log, closer, err := logging.Open(dir)
	if err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	ln := ipc.NewPipeListener()
	d, err := startEngine(dir, log, closer, func() (net.Listener, ipc.PeerCheck, error) {
		return ln, ipc.InProcessCheck, nil
	})
	if err != nil {
		ln.Close()
		return err
	}

	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		d.Stop(context.Background())
		return err
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		d.Stop(context.Background())
		return fmt.Errorf("the front end is not built (run npm run build in web/): %w", err)
	}

	m := gui.NewManager("", nil, log)
	m.SetMode(gui.ModePortable)
	m.SetDial(func(ctx context.Context, addr string) (*client.Client, error) {
		c, err := ln.Dial()
		if err != nil {
			return nil, err
		}
		return client.New(c), nil
	})
	m.SetAutostart(portableAutostart{})
	m.SetDataDir(dir)
	m.SetFirstRun(gui.FileFirstRun{Dir: dir})

	api := gui.NewAPI(m)
	osn := newOSNotifier(log)
	m.OnNotify(osn.show)
	ctx, cancel := context.WithCancel(context.Background())

	var stopping sync.Once
	stopAll := func() {
		stopping.Do(func() {
			cancel()
			m.Close()
			sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer scancel()
			_ = d.Stop(sctx)
			_ = ln.Close()
		})
	}

	app := application.New(application.Options{
		Name:        "VoltioVolt",
		Description: "VLT UPS monitor",
		Services:    []application.Service{application.NewService(api), application.NewService(osn)},
		Assets: application.AssetOptions{
			Handler: cspHandler(application.BundledAssetFileServer(dist)),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.voltiovolt.app",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if w := application.Get().Window.Current(); w != nil {
					w.Show()
					w.Focus()
				}
			},
		},
		OnShutdown: func() { stopAll() },
	})
	m.SetEmitter(emitter{app})
	m.SetOpenURL(app.Browser.OpenURL)
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "VoltioVolt",
		Width:     1100,
		Height:    760,
		MinWidth:  gui.MinWindowWidth,
		MinHeight: gui.MinWindowHeight,
		URL:       "/",
	})
	var hinted sync.Once
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
		hinted.Do(func() {
			showStillMonitoring(app, dir)
		})
	})
	setupTray(app, win, m, api, func() { confirmAndQuit(app, stopAll) })
	go m.Run(ctx)
	return app.Run()
}

func showStillMonitoring(app *application.App, dir string) {
	marker := filepath.Join(dir, ".tray-hint-shown")
	if _, err := os.Stat(marker); err == nil {
		return
	}
	_ = os.WriteFile(marker, []byte("1\n"), 0o600)
	app.Dialog.Info().SetTitle("VoltioVolt").SetMessage(TextStillMonitoring).Show()
}

func confirmAndQuit(app *application.App, stop func()) {
	ok := false
	d := app.Dialog.Question()
	d.SetTitle("VoltioVolt")
	d.SetMessage(TextQuitConfirm)
	yes := d.AddButton("Quit")
	no := d.AddButton("Cancel")
	yes.OnClick(func() { ok = true })
	d.SetDefaultButton(no)
	d.SetCancelButton(no)
	d.Show()
	if ok {
		stop()
		app.Quit()
	}
}

// emitter sends Wails events.
type emitter struct{ app *application.App }

func (e emitter) Emit(name string, data any) { e.app.Event.Emit(name, data) }

func cspHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", gui.CSP)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}
