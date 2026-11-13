// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"testing"
)

func TestAboutModeAndAutostart(t *testing.T) {
	f := allFake(t)
	m, a, _ := start(t, f)
	info := a.About()
	if info.Mode != ModeService {
		t.Fatalf("default mode %q", info.Mode)
	}
	if _, err := a.StartAtLogon(); err == nil {
		t.Fatal("want unavailable")
	}
	m.SetMode(ModePortable)
	if a.About().Mode != ModePortable {
		t.Fatalf("mode %q", a.About().Mode)
	}
	s := &fakeAutostart{}
	m.SetAutostart(s)
	on, err := a.StartAtLogon()
	if err != nil || on {
		t.Fatalf("enabled %v %v", on, err)
	}
	if err := a.SetStartAtLogon(true); err != nil {
		t.Fatal(err)
	}
	on, err = a.StartAtLogon()
	if err != nil || !on {
		t.Fatalf("after set %v %v", on, err)
	}
	if err := a.SetStartAtLogon(false); err != nil {
		t.Fatal(err)
	}
	if s.on {
		t.Fatal("still on")
	}
	m.SetDataDir(`/tmp/VoltioVolt-data`)
	if a.About().DataDir != `/tmp/VoltioVolt-data` {
		t.Fatalf("data dir %q", a.About().DataDir)
	}
}

type fakeFirstRun struct{ done bool }

func (f *fakeFirstRun) Completed() (bool, error) { return f.done, nil }
func (f *fakeFirstRun) SetCompleted() error      { f.done = true; return nil }

func TestWizardCompleted(t *testing.T) {
	m, a, _ := start(t, allFake(t))
	if _, err := a.WizardCompleted(); err == nil {
		t.Fatal("want unavailable")
	}
	f := &fakeFirstRun{}
	m.SetFirstRun(f)
	on, err := a.WizardCompleted()
	if err != nil || on {
		t.Fatalf("completed %v %v", on, err)
	}
	if err := a.SetWizardCompleted(); err != nil {
		t.Fatal(err)
	}
	on, err = a.WizardCompleted()
	if err != nil || !on {
		t.Fatalf("after set %v %v", on, err)
	}
}
