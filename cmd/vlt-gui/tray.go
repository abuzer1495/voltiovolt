// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/abuzer1495/voltiovolt/internal/gui"
)

// setupTray makes the tray icon (TASK 03-00-00-004 step 2): the icon shape
// and color of the state, the tooltip with the state and the runtime, and
// the menu Open, Lock, Quit. Quit closes the VLT GUI only; vltd continues.
func setupTray(app *application.App, win *application.WebviewWindow, m *gui.Manager, api *gui.API) {
	tray := app.SystemTray.New()
	menu := application.NewMenu()
	tx := m.TrayTexts()
	open := menu.Add(tx.Open)
	lock := menu.Add(tx.Lock)
	quit := menu.Add(tx.Quit)
	open.OnClick(func(*application.Context) { win.Show(); win.Focus() })
	lock.OnClick(func(*application.Context) { _ = api.Logout() })
	quit.OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	tray.OnClick(func() { win.Show(); win.Focus() })
	var mu sync.Mutex
	var lastState, lastTip string
	var lastTexts gui.TrayTexts
	apply := func(ts gui.TrayState) {
		mu.Lock()
		defer mu.Unlock()
		tx := m.TrayTexts()
		tip := gui.Tooltip(ts, tx)
		if ts.State == lastState && tip == lastTip && tx == lastTexts {
			return
		}
		if ts.State != lastState {
			tray.SetIcon(gui.TrayIcon(ts.State))
		}
		if tx != lastTexts {
			open.SetLabel(tx.Open)
			lock.SetLabel(tx.Lock)
			quit.SetLabel(tx.Quit)
			menu.Update()
		}
		tray.SetTooltip(tip)
		lastState, lastTip, lastTexts = ts.State, tip, tx
	}
	apply(gui.TrayState{State: gui.TrayOffline})
	m.OnTray(apply)
}
