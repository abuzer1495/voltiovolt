// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/appicon"
)

type recEmitter struct{ names, data []string }

func (r *recEmitter) Emit(name string, data any) {
	r.names = append(r.names, name)
	if s, ok := data.(string); ok {
		r.data = append(r.data, s)
	}
}

// TestBuildMenu is VLT-IS-011 TASK 03-00-00-005 step 2.1.
func TestBuildMenu(t *testing.T) {
	st := MenuState{Mode: "expert", Scale: "125", Scales: []MenuChoice{{"auto", "Auto"}, {"100", "100 %"}, {"125", "125 %"}},
		Texts: map[string]string{"menu.file": "&Dosya", "menu.view": "&Görünüm"}}
	m := BuildMenu(st, "windows")
	var tops []string
	for _, g := range m {
		tops = append(tops, g.Label)
	}
	if got := strings.Join(tops, "|"); got != "&Dosya|&Görünüm|&Tools|&Help" {
		t.Fatal(got)
	}
	if m[0].Items[0].Action != ActionExit {
		t.Fatal("File has no Exit")
	}
	view := m[1].Items
	if view[0].Action != ActionSimple || view[0].Checked || view[1].Action != ActionExpert || !view[1].Checked {
		t.Fatalf("mode items %+v", view[:2])
	}
	sizes := view[3].Items
	if len(sizes) != 3 || sizes[2].Action != "scale:125" || !sizes[2].Checked || sizes[0].Checked {
		t.Fatalf("sizes %+v", sizes)
	}
	if m[2].Items[0].Action != ActionWizard || m[2].Items[1].Action != ActionSimHelp {
		t.Fatal("Tools")
	}
	if m[3].Items[0].Action != ActionDocs || m[3].Items[0].Accel != "F1" || m[3].Items[2].Action != ActionAbout {
		t.Fatal("Help")
	}
}

// TestMnemonic is step 4: "&" is the access key on Windows, "_" in GTK.
func TestMnemonic(t *testing.T) {
	cases := []struct{ in, goos, want string }{
		{"&Görünüm", "windows", "&Görünüm"},
		{"&Görünüm", "linux", "_Görünüm"},
		{"a_b &c", "linux", "a__b _c"},
		{"&File", "darwin", "File"},
	}
	for _, c := range cases {
		if got := Mnemonic(c.in, c.goos); got != c.want {
			t.Errorf("%q %s: %q", c.in, c.goos, got)
		}
	}
}

func TestNormalizeColor(t *testing.T) {
	cases := map[string]string{
		"rgb(0,120,212)":        "#0078d4",
		"rgba(0, 120, 212, 1)":  "#0078d4",
		"#0078D4":               "#0078d4",
		"rgb(300,0,0)":          "",
		"red":                   "",
		"rgb(0,0,0); color:red": "",
	}
	for in, want := range cases {
		if got := NormalizeColor(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

// TestConfirmNative is step 2.2: Yes, No and Cancel.
func TestConfirmNative(t *testing.T) {
	m := NewManager("", nil, nil)
	a := NewAPI(m)
	req := ConfirmRequest{Title: "T", Text: "Sure?", Yes: "Evet", No: "Hayır", Cancel: "İptal"}
	if _, err := a.ConfirmNative(req); err == nil || err.(*Error).Code != "unavailable" {
		t.Fatalf("no native dialog: %v", err)
	}
	if a.Theme().Native {
		t.Fatal("Native without a hook")
	}
	var got ConfirmRequest
	answer := "no"
	m.SetNativeHooks(NativeHooks{Confirm: func(r ConfirmRequest) string { got = r; return answer }})
	if !a.Theme().Native {
		t.Fatal("Native")
	}
	for _, c := range []struct{ answer, want string }{{"yes", "yes"}, {"no", "no"}, {"cancel", "cancel"}, {"", "cancel"}} {
		answer = c.answer
		r, err := a.ConfirmNative(req)
		if err != nil || r != c.want {
			t.Fatalf("%q: %q %v", c.answer, r, err)
		}
	}
	if got != req {
		t.Fatalf("texts %+v", got)
	}
	if _, err := a.ConfirmNative(ConfirmRequest{Text: " ", Yes: "a", No: "b", Cancel: "c"}); err == nil {
		t.Fatal("empty text")
	}
}

// TestMenuActions: Exit, the help links and About in Go; the others go
// to the front end.
func TestMenuActions(t *testing.T) {
	m := NewManager("", nil, nil)
	a := NewAPI(m)
	e := &recEmitter{}
	m.SetEmitter(e)
	now := time.Unix(1000, 0)
	m.now = func() time.Time { return now }
	var opened []string
	m.SetOpenURL(func(u string) error { opened = append(opened, u); return nil })
	quit := 0
	var info string
	var menus [][]MenuItem
	m.SetNativeHooks(NativeHooks{Quit: func() { quit++ }, Info: func(_, s string) { info = s }, Menu: func(mi []MenuItem) { menus = append(menus, mi) }})
	for _, act := range []string{ActionSimple, ActionExpert, ActionWizard, "scale:150", "evil"} {
		m.MenuAction(act)
	}
	if strings.Join(e.data, ",") != "mode:simple,mode:expert,wizard,scale:150" {
		t.Fatal(e.data)
	}
	for _, n := range e.names {
		if n != EventMenu {
			t.Fatal(n)
		}
	}
	m.MenuAction(ActionExit)
	if quit != 1 {
		t.Fatal("quit")
	}
	m.MenuAction(ActionDocs)
	_ = a.OpenDocs() // the same F1 press: no second window
	now = now.Add(2 * time.Second)
	m.MenuAction(ActionSimHelp)
	if strings.Join(opened, ",") != DocsURL+","+SimulatorHelpURL {
		t.Fatal(opened)
	}
	a.SetMenu(MenuState{Texts: map[string]string{"menu.about.text": "VoltioVolt {version} sürüm"}})
	if len(menus) != 1 || len(menus[0]) != 4 {
		t.Fatal("menu not set")
	}
	m.MenuAction(ActionAbout)
	if info != "VoltioVolt "+Version+" sürüm" {
		t.Fatal(info)
	}
}

func TestHelpURLs(t *testing.T) {
	for _, u := range []string{DocsURL, SimulatorHelpURL} {
		if !strings.HasPrefix(u, "https://github.com/abuzer1495/voltiovolt/") {
			t.Fatal(u)
		}
	}
}

// TestTrayIconIsAppIcon is step 3.1: the tray icon is the application icon
// (top left part) with the state mark (bottom right).
func TestTrayIconIsAppIcon(t *testing.T) {
	want := appicon.Image(iconSize)
	for _, s := range []string{TrayMains, TrayBattery, TrayCritical, TrayOffline} {
		img, err := png.Decode(bytes.NewReader(TrayIcon(s)))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range [][2]int{{16, 2}, {8, 10}, {10, 6}} {
			r1, g1, b1, a1 := img.At(p[0], p[1]).RGBA()
			r2, g2, b2, a2 := want.At(p[0], p[1]).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 || a2 == 0 {
				t.Fatalf("%s: pixel %v is not the application icon", s, p)
			}
		}
	}
}
