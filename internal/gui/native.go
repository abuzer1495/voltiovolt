// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/ipc/wire"
)

// VLT-IS-011 TASK 03-00-00-005 (R-12, OD-55): native parts of the
// window. The Wails code is in cmd/voltiovolt and cmd/vlt-gui; this file
// has the parts that can be tested without a window: the menu model, the
// confirmation dialog result, the accent color and the help links.

// Help links of the menu (step 2.1) and of F1 (step 4). They open in the
// web browser of the OS; the window never loads remote content (SEC-095).
const (
	DocsURL          = SourceURL + "/tree/main/docs/user"
	SimulatorHelpURL = SourceURL + "/blob/main/docs/user/simulator.md"
)

// EventMenu is the Wails event of a menu command for the front end. The
// data is the action: "mode:simple", "mode:expert", "scale:<value>" or
// "wizard".
const EventMenu = "vlt:menu"

// Menu actions.
const (
	ActionExit    = "exit"
	ActionSimple  = "mode:simple"
	ActionExpert  = "mode:expert"
	ActionWizard  = "wizard"
	ActionSimHelp = "simhelp"
	ActionDocs    = "docs"
	ActionAbout   = "about"
	ActionScale   = "scale:" // + value
)

// NativeHooks are the functions of the Wails application.
type NativeHooks struct {
	// Confirm shows a native question dialog and returns "yes", "no" or
	// "cancel".
	Confirm func(ConfirmRequest) string
	// Menu sets the menu bar of the window.
	Menu func([]MenuItem)
	// Accent returns the accent color of the OS ("" if there is none).
	Accent func() string
	// Info shows a native information dialog.
	Info func(title, text string)
	// Quit asks to quit the application (with its own confirmation).
	Quit func()
}

type nativeState struct {
	hooks    NativeHooks
	lastDocs time.Time
	about    string
}

// SetNativeHooks sets the native functions (after the Wails window is made).
func (m *Manager) SetNativeHooks(h NativeHooks) {
	m.mu.Lock()
	m.native.hooks = h
	m.mu.Unlock()
}

func (m *Manager) hooks() NativeHooks {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.native.hooks
}

// ThemeInfo tells the front end about the OS (step 1.2, step 5).
type ThemeInfo struct {
	OS     string `json:"os"`
	Accent string `json:"accent"` // "#rrggbb" or ""
	Native bool   `json:"native"` // native confirmation dialogs
	Menu   bool   `json:"menu"`   // native menu bar
}

// Theme returns the OS, the accent color and the native parts.
func (a *API) Theme() ThemeInfo {
	h := a.m.hooks()
	ti := ThemeInfo{OS: runtime.GOOS, Native: h.Confirm != nil, Menu: h.Menu != nil}
	if h.Accent != nil {
		ti.Accent = NormalizeColor(h.Accent())
	}
	return ti
}

var rgbRe = regexp.MustCompile(`^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*(?:,\s*[\d.]+\s*)?\)$`)
var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// NormalizeColor returns "#rrggbb" for "rgb(r,g,b)", "rgba(...)" or
// "#rrggbb", else "". The front end uses only this form.
func NormalizeColor(s string) string {
	s = strings.TrimSpace(s)
	if hexRe.MatchString(s) {
		return strings.ToLower(s)
	}
	m := rgbRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	var v [3]int
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil || n > 255 {
			return ""
		}
		v[i] = n
	}
	return fmt.Sprintf("#%02x%02x%02x", v[0], v[1], v[2])
}

// ConfirmRequest is a native confirmation (step 2.2) with the texts of
// the front end (en/tr).
type ConfirmRequest struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	Yes    string `json:"yes"`
	No     string `json:"no"`
	Cancel string `json:"cancel"`
}

// ConfirmNative shows a native question dialog with Yes, No and Cancel.
// The result is "yes", "no" or "cancel". Without a native dialog the
// error is "unavailable"; the front end then shows its own dialog.
func (a *API) ConfirmNative(r ConfirmRequest) (string, error) {
	h := a.m.hooks()
	if h.Confirm == nil {
		return "", &Error{Code: wire.CodeUnavailable}
	}
	if strings.TrimSpace(r.Text) == "" || r.Yes == "" || r.No == "" || r.Cancel == "" {
		return "", &Error{Code: wire.CodeInvalid}
	}
	switch h.Confirm(r) {
	case "yes":
		return "yes", nil
	case "no":
		return "no", nil
	}
	return "cancel", nil
}

// MenuItem is one item of the menu model. A group has Items.
type MenuItem struct {
	Label     string
	Action    string
	Accel     string
	Radio     bool
	Checked   bool
	Separator bool
	Items     []MenuItem
}

// MenuChoice is one interface size in the View menu.
type MenuChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// MenuState is sent by the front end after a change of the language, the
// mode or the interface size. Texts have the menu.* keys of en.json and
// tr.json; "&" marks the access key (step 4).
type MenuState struct {
	Texts  map[string]string `json:"texts"`
	Mode   string            `json:"mode"`
	Scale  string            `json:"scale"`
	Scales []MenuChoice      `json:"scales"`
}

// Mnemonic converts the "&" access key mark for an OS: Windows uses "&",
// GTK uses "_", the others have no access keys.
func Mnemonic(label, goos string) string {
	switch goos {
	case "windows":
		return label
	case "linux":
		label = strings.ReplaceAll(label, "_", "__")
		return strings.ReplaceAll(label, "&", "_")
	}
	return strings.ReplaceAll(label, "&", "")
}

// BuildMenu returns the menu bar of step 2.1: File (Exit), View (Simple
// mode, Expert mode, Interface size), Tools (Setup wizard, Simulator
// help), Help (Documentation, About).
func BuildMenu(st MenuState, goos string) []MenuItem {
	tx := func(k string) string {
		if v, ok := st.Texts[k]; ok && v != "" {
			return Mnemonic(v, goos)
		}
		return Mnemonic(defaultMenuTexts[k], goos)
	}
	var sizes []MenuItem
	for _, c := range st.Scales {
		sizes = append(sizes, MenuItem{Label: c.Label, Action: ActionScale + c.Value, Radio: true, Checked: c.Value == st.Scale})
	}
	return []MenuItem{
		{Label: tx("menu.file"), Items: []MenuItem{
			{Label: tx("menu.file.exit"), Action: ActionExit},
		}},
		{Label: tx("menu.view"), Items: []MenuItem{
			{Label: tx("menu.view.simple"), Action: ActionSimple, Radio: true, Checked: st.Mode != "expert"},
			{Label: tx("menu.view.expert"), Action: ActionExpert, Radio: true, Checked: st.Mode == "expert"},
			{Separator: true},
			{Label: tx("menu.view.scale"), Items: sizes},
		}},
		{Label: tx("menu.tools"), Items: []MenuItem{
			{Label: tx("menu.tools.wizard"), Action: ActionWizard},
			{Label: tx("menu.tools.simhelp"), Action: ActionSimHelp},
		}},
		{Label: tx("menu.help"), Items: []MenuItem{
			{Label: tx("menu.help.docs"), Action: ActionDocs, Accel: "F1"},
			{Separator: true},
			{Label: tx("menu.help.about"), Action: ActionAbout},
		}},
	}
}

// defaultMenuTexts are the English texts (the same as web/i18n/en.json)
// for the time before the front end starts.
var defaultMenuTexts = map[string]string{
	"menu.file":          "&File",
	"menu.file.exit":     "E&xit",
	"menu.view":          "&View",
	"menu.view.simple":   "&Simple mode",
	"menu.view.expert":   "&Expert mode",
	"menu.view.scale":    "&Interface size",
	"menu.tools":         "&Tools",
	"menu.tools.wizard":  "&Setup wizard",
	"menu.tools.simhelp": "Simulator &help",
	"menu.help":          "&Help",
	"menu.help.docs":     "&Documentation",
	"menu.help.about":    "&About VoltioVolt",
	"menu.about.text":    "VoltioVolt {version}\nVoltioVolt UPS Management\nCopyright VoltioVolt contributors\nLicense: AGPL-3.0-or-later",
}

// SetMenu sets the menu bar from the front end state.
func (a *API) SetMenu(st MenuState) {
	h := a.m.hooks()
	a.m.mu.Lock()
	about := st.Texts["menu.about.text"]
	if about == "" {
		about = defaultMenuTexts["menu.about.text"]
	}
	a.m.native.about = strings.ReplaceAll(about, "{version}", Version)
	a.m.mu.Unlock()
	if h.Menu != nil {
		h.Menu(BuildMenu(st, runtime.GOOS))
	}
}

// DefaultMenu is the menu before the front end sends its texts.
func DefaultMenu() []MenuItem {
	return BuildMenu(MenuState{Mode: "simple", Scale: "auto", Scales: []MenuChoice{{Value: "auto", Label: "Auto"}}}, runtime.GOOS)
}

// MenuAction runs a menu command. Exit, the help links and About are done
// in Go; the others go to the front end as EventMenu.
func (m *Manager) MenuAction(action string) {
	h := m.hooks()
	switch action {
	case ActionExit:
		if h.Quit != nil {
			h.Quit()
		}
	case ActionDocs:
		_ = m.openDocs(DocsURL)
	case ActionSimHelp:
		_ = m.openDocs(SimulatorHelpURL)
	case ActionAbout:
		m.mu.Lock()
		text := m.native.about
		m.mu.Unlock()
		if text == "" {
			text = strings.ReplaceAll(defaultMenuTexts["menu.about.text"], "{version}", Version)
		}
		if h.Info != nil {
			h.Info("VoltioVolt", text)
		}
	default:
		if action == ActionSimple || action == ActionExpert || action == ActionWizard || strings.HasPrefix(action, ActionScale) {
			m.mu.Lock()
			e := m.emit
			m.mu.Unlock()
			if e != nil {
				e.Emit(EventMenu, action)
			}
		}
	}
}

// docsDebounce: F1 in the page and the F1 accelerator of the menu can come
// for the same key press; the link opens once.
const docsDebounce = time.Second

func (m *Manager) openDocs(u string) error {
	m.mu.Lock()
	now := m.now()
	if !m.native.lastDocs.IsZero() && now.Sub(m.native.lastDocs) < docsDebounce {
		m.mu.Unlock()
		return nil
	}
	m.native.lastDocs = now
	f := m.openURL
	m.mu.Unlock()
	if f == nil {
		return &Error{Code: wire.CodeUnavailable}
	}
	if err := f(u); err != nil {
		return &Error{Code: "ipc_error"}
	}
	return nil
}

// OpenDocs opens the documentation (F1, step 4).
func (a *API) OpenDocs() error { return a.m.openDocs(DocsURL) }
