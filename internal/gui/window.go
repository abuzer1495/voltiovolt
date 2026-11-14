// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

// Minimum window size at the interface size 100 % (VLT-IS-009 TASK
// 03-00-00-003). VLT-IS-011 TASK 03-00-00-004: it scales with the
// interface size.
const (
	MinWindowWidth  = 1024
	MinWindowHeight = 680
)

// ScaledMinSize is the minimum window size for an interface factor in
// percent of the OS display scale (100: no change). The factor is limited
// to 50 to 400.
func ScaledMinSize(percent int) (int, int) {
	if percent < 50 {
		percent = 50
	}
	if percent > 400 {
		percent = 400
	}
	return MinWindowWidth * percent / 100, MinWindowHeight * percent / 100
}

// SetMinSizeFunc sets the function that changes the minimum window size.
func (m *Manager) SetMinSizeFunc(f func(w, h int)) {
	m.mu.Lock()
	m.minSize = f
	m.mu.Unlock()
}

// SetInterfaceScale is called by the front end after a change of the
// interface size. percent is the CSS factor in percent of the OS display
// scale; the minimum window size scales with it.
func (a *API) SetInterfaceScale(percent int) {
	a.m.mu.Lock()
	f := a.m.minSize
	a.m.mu.Unlock()
	if f == nil {
		return
	}
	f(ScaledMinSize(percent))
}
