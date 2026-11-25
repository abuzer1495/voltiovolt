// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

// DemoNavResult is the test-build navigation target for CI screenshots.
// Screen is empty when screenshot mode is off (production and normal use).
type DemoNavResult struct {
	Active bool   `json:"active"`
	Screen string `json:"screen"`
}

// SetDemoNav records the --demo-screen value for the front end.
func (m *Manager) SetDemoNav(screen string) {
	m.mu.Lock()
	m.demoScreen = screen
	m.mu.Unlock()
}

// SetDemoShotReady registers the callback that writes the ready file when
// the front end has applied the demo hash.
func (m *Manager) SetDemoShotReady(f func()) {
	m.mu.Lock()
	m.demoShotReady = f
	m.mu.Unlock()
}

// DemoNav reports whether the window was started for CI screenshots and
// which screen to open. It never enables extra navigation in a normal
// session (Active is false when SetDemoNav was not called).
func (a *API) DemoNav() DemoNavResult {
	a.m.mu.Lock()
	s := a.m.demoScreen
	a.m.mu.Unlock()
	if s == "" {
		return DemoNavResult{}
	}
	return DemoNavResult{Active: true, Screen: s}
}

// DemoShotReady records that the front end applied the demo hash. It is a
// no-op when screenshot mode is off.
func (a *API) DemoShotReady() {
	a.m.mu.Lock()
	f := a.m.demoShotReady
	a.m.mu.Unlock()
	if f != nil {
		f()
	}
}
