// SPDX-License-Identifier: AGPL-3.0-or-later

// Package appicon draws the VoltioVolt application icon (VLT-IS-011 TASK
// 03-00-00-005 step 3.1): an original, simple design of a battery with a
// lightning symbol. The icon is drawn by code, so the repository has no
// binary image files, and every size is sharp. The same icon is used for
// the exe (ICO in the Windows resource), the window, the tray (with a
// state mark, see internal/gui) and the Start menu shortcut (the shortcut
// takes the icon of the exe).
package appicon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// Colors of the icon.
var (
	Edge  = color.NRGBA{0x0e, 0x2a, 0x47, 0xff} // dark blue edge
	Body  = color.NRGBA{0x00, 0x67, 0xc0, 0xff} // blue battery body
	Bolt  = color.NRGBA{0xff, 0xd3, 0x3d, 0xff} // yellow lightning
	Light = color.NRGBA{0x4c, 0xa3, 0xf0, 0xff} // top light of the body
)

// ICOSizes are the image sizes in the ICO file.
var ICOSizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

// Shape coordinates: x and y in [0, 1], y down.
const (
	bodyX0, bodyX1 = 0.20, 0.80
	bodyY0, bodyY1 = 0.17, 0.95
	bodyR          = 0.10
	nubX0, nubX1   = 0.37, 0.63
	nubY0          = 0.05
	edgeW          = 0.055
)

// bolt is the lightning polygon.
var bolt = [][2]float64{
	{0.585, 0.26}, {0.33, 0.60}, {0.48, 0.60}, {0.41, 0.88},
	{0.68, 0.51}, {0.52, 0.51}, {0.60, 0.26},
}

func inRoundRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func inPolygon(x, y float64, p [][2]float64) bool {
	in := false
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		xi, yi, xj, yj := p[i][0], p[i][1], p[j][0], p[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// sample is the color at a point; ok is false outside the icon.
func sample(x, y float64) (color.NRGBA, bool) {
	outer := inRoundRect(x, y, bodyX0, bodyY0, bodyX1, bodyY1, bodyR) ||
		inRoundRect(x, y, nubX0, nubY0, nubX1, bodyY0+bodyR, 0.03)
	if !outer {
		return color.NRGBA{}, false
	}
	inner := inRoundRect(x, y, bodyX0+edgeW, bodyY0+edgeW, bodyX1-edgeW, bodyY1-edgeW, bodyR-edgeW/2)
	if !inner {
		return Edge, true
	}
	if inPolygon(x, y, bolt) {
		return Bolt, true
	}
	if y < bodyY0+edgeW+0.08 {
		return Light, true
	}
	return Body, true
}

// Image draws the icon at size x size pixels with 4 x 4 samples per pixel.
func Image(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	for j := 0; j < size; j++ {
		for i := 0; i < size; i++ {
			var r, g, b, a float64
			for sj := 0; sj < ss; sj++ {
				for si := 0; si < ss; si++ {
					x := (float64(i) + (float64(si)+0.5)/ss) / float64(size)
					y := (float64(j) + (float64(sj)+0.5)/ss) / float64(size)
					if c, ok := sample(x, y); ok {
						r += float64(c.R)
						g += float64(c.G)
						b += float64(c.B)
						a++
					}
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(i, j, color.NRGBA{
				R: uint8(r/a + 0.5), G: uint8(g/a + 0.5), B: uint8(b/a + 0.5),
				A: uint8(a/(ss*ss)*255 + 0.5),
			})
		}
	}
	return img
}

// PNG returns the icon as a PNG file.
func PNG(size int) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, Image(size))
	return b.Bytes()
}

// ICO returns an ICO file with PNG images of ICOSizes (Windows Vista and
// later read PNG images in ICO files).
func ICO() []byte {
	imgs := make([][]byte, len(ICOSizes))
	for i, s := range ICOSizes {
		imgs[i] = PNG(s)
	}
	var b bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&b, le, [3]uint16{0, 1, uint16(len(imgs))})
	off := 6 + 16*len(imgs)
	for i, s := range ICOSizes {
		w := uint8(s)
		if s >= 256 {
			w = 0 // 0 means 256
		}
		b.Write([]byte{w, w, 0, 0})
		_ = binary.Write(&b, le, [2]uint16{1, 32})
		_ = binary.Write(&b, le, [2]uint32{uint32(len(imgs[i])), uint32(off)})
		off += len(imgs[i])
	}
	for _, p := range imgs {
		b.Write(p)
	}
	return b.Bytes()
}
