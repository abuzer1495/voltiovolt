// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// VLT-IS-011 TASK 03-00-00-005 (R-12, OD-55): the native parts of the
// window: the menu bar, the native confirmation dialogs and the accent
// color. The same file is in cmd/vlt-gui (the Wails code of each binary
// is in its own module).

import (
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/abuzer1495/voltiovolt/internal/gui"
)

// confirmTimeout ends a native question that has no answer.
const confirmTimeout = 10 * time.Minute

// setupNative sets the native hooks and the first menu bar.
func setupNative(app *application.App, win *application.WebviewWindow, m *gui.Manager, quit func()) {
	setMenu := func(items []gui.MenuItem) {
		win.SetMenu(wailsMenu(items, m.MenuAction))
	}
	h := gui.NativeHooks{
		Confirm: func(r gui.ConfirmRequest) string { return nativeConfirm(app, win, r) },
		Menu:    setMenu,
		Info: func(title, text string) {
			app.Dialog.Info().SetTitle(title).SetMessage(text).AttachToWindow(win).Show()
		},
		Quit: quit,
	}
	// Wails returns a fixed color on Linux; there the front end uses the
	// GTK theme colors (step 5).
	if runtime.GOOS == "windows" {
		h.Accent = app.Env.GetAccentColor
	}
	m.SetNativeHooks(h)
	setMenu(gui.DefaultMenu())
}

// wailsMenu makes the Wails menu from the model of internal/gui.
func wailsMenu(items []gui.MenuItem, action func(string)) *application.Menu {
	menu := application.NewMenu()
	for _, g := range items {
		addItems(menu.AddSubmenu(g.Label), g.Items, action)
	}
	return menu
}

func addItems(menu *application.Menu, items []gui.MenuItem, action func(string)) {
	for _, it := range items {
		switch {
		case it.Separator:
			menu.AddSeparator()
		case it.Items != nil:
			addItems(menu.AddSubmenu(it.Label), it.Items, action)
		default:
			var mi *application.MenuItem
			if it.Radio {
				mi = menu.AddRadio(it.Label, it.Checked)
			} else {
				mi = menu.Add(it.Label)
			}
			if it.Accel != "" {
				mi.SetAccelerator(it.Accel)
			}
			a := it.Action
			// Not on the UI thread: an action can show a dialog.
			mi.OnClick(func(*application.Context) { go action(a) })
		}
	}
}

// nativeConfirm shows a question dialog with Yes, No and Cancel. Cancel
// is the default button and the Escape key (no action without a choice).
func nativeConfirm(app *application.App, win *application.WebviewWindow, r gui.ConfirmRequest) string {
	res := make(chan string, 1)
	answer := func(s string) func() {
		return func() {
			select {
			case res <- s:
			default:
			}
		}
	}
	d := app.Dialog.Question()
	d.SetTitle(r.Title)
	d.SetMessage(r.Text)
	d.AttachToWindow(win)
	d.AddButton(r.Yes).OnClick(answer("yes"))
	d.AddButton(r.No).OnClick(answer("no"))
	c := d.AddButton(r.Cancel).OnClick(answer("cancel"))
	d.SetDefaultButton(c)
	d.SetCancelButton(c)
	d.Show()
	select {
	case s := <-res:
		return s
	case <-time.After(confirmTimeout):
		return "cancel"
	}
}
