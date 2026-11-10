// SPDX-License-Identifier: AGPL-3.0-or-later

package gui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"
)

// iconSize is the size of the tray icon in pixels.
const iconSize = 32

// Icon colors. The shape changes with the state too (step 2.2): color is
// not the only signal.
var (
	iconGreen  = color.NRGBA{0x1e, 0x8e, 0x3e, 0xff}
	iconYellow = color.NRGBA{0xf2, 0xb2, 0x00, 0xff}
	iconRed    = color.NRGBA{0xc6, 0x28, 0x28, 0xff}
	iconGrey   = color.NRGBA{0x80, 0x80, 0x80, 0xff}
	iconEdge   = color.NRGBA{0x10, 0x10, 0x10, 0xff}
)

// inside functions: x, y in [-1, 1].
func inCircle(x, y float64) bool { return x*x+y*y <= 0.85*0.85 }

func inTriangle(x, y float64) bool {
	// point up: apex (0, -0.9), base y = 0.8
	if y < -0.9 || y > 0.8 {
		return false
	}
	half := (y + 0.9) / 1.7 * 0.95
	return math.Abs(x) <= half
}

func inSquare(x, y float64) bool { return math.Abs(x) <= 0.8 && math.Abs(y) <= 0.8 }

func inRing(x, y float64) bool {
	r := math.Sqrt(x*x + y*y)
	return r <= 0.85 && r >= 0.5
}

// TrayShape is the shape of a tray state: circle (mains), triangle
// (battery), square (critical), ring (offline).
func TrayShape(state string) string {
	switch state {
	case TrayMains:
		return "circle"
	case TrayBattery:
		return "triangle"
	case TrayCritical:
		return "square"
	}
	return "ring"
}

// TrayIcon returns the PNG icon of a tray state.
func TrayIcon(state string) []byte {
	var in func(x, y float64) bool
	var c color.NRGBA
	switch state {
	case TrayMains:
		in, c = inCircle, iconGreen
	case TrayBattery:
		in, c = inTriangle, iconYellow
	case TrayCritical:
		in, c = inSquare, iconRed
	default:
		in, c = inRing, iconGrey
	}
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	px := func(i int) float64 { return (float64(i)+0.5)/iconSize*2 - 1 }
	for j := 0; j < iconSize; j++ {
		for i := 0; i < iconSize; i++ {
			x, y := px(i), px(j)
			if !in(x, y) {
				continue
			}
			// A dark edge: a pixel with a neighbour outside the shape.
			d := 2.0 / iconSize
			edge := !in(x-d, y) || !in(x+d, y) || !in(x, y-d) || !in(x, y+d)
			if edge {
				img.SetNRGBA(i, j, iconEdge)
			} else {
				img.SetNRGBA(i, j, c)
			}
		}
	}
	// A mark in the critical square: "!" so that it differs in grey scale.
	if state == TrayCritical {
		for j := 8; j < 20; j++ {
			img.SetNRGBA(15, j, color.NRGBA{0xff, 0xff, 0xff, 0xff})
			img.SetNRGBA(16, j, color.NRGBA{0xff, 0xff, 0xff, 0xff})
		}
		for j := 22; j < 25; j++ {
			img.SetNRGBA(15, j, color.NRGBA{0xff, 0xff, 0xff, 0xff})
			img.SetNRGBA(16, j, color.NRGBA{0xff, 0xff, 0xff, 0xff})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// TrayTexts are the texts of the tray icon. The front end sets them from
// the i18n files (SetTrayTexts); the defaults are for the time before the
// front end starts.
type TrayTexts struct {
	Mains    string `json:"mains"`
	Battery  string `json:"battery"`
	Critical string `json:"critical"`
	Offline  string `json:"offline"`
	Runtime  string `json:"runtime"` // with {minutes}
	Open     string `json:"open"`
	Lock     string `json:"lock"`
	Quit     string `json:"quit"`
}

// DefaultTrayTexts are the English texts (the same as web/i18n/en.json).
var DefaultTrayTexts = TrayTexts{Mains: "VLT: mains power", Battery: "VLT: on battery", Critical: "VLT: CRITICAL",
	Offline: "VLT: no connection", Runtime: "runtime {minutes} min", Open: "Open", Lock: "Lock", Quit: "Quit"}

// Tooltip is the tray tooltip: state and runtime (step 2.3).
func Tooltip(ts TrayState, tx TrayTexts) string {
	var s string
	switch ts.State {
	case TrayMains:
		s = tx.Mains
	case TrayBattery:
		s = tx.Battery
	case TrayCritical:
		s = tx.Critical
	default:
		s = tx.Offline
	}
	if ts.Runtime != nil {
		s += ", " + strings.ReplaceAll(tx.Runtime, "{minutes}", strconv.Itoa(*ts.Runtime))
	}
	return s
}
