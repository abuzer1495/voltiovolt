// SPDX-License-Identifier: AGPL-3.0-or-later

package appicon

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
)

// TestIconShape: transparent corners, an opaque body, the yellow lightning
// in the middle, the battery nub at the top center.
func TestIconShape(t *testing.T) {
	img := Image(64)
	if _, _, _, a := img.At(1, 1).RGBA(); a != 0 {
		t.Fatal("corner is not transparent")
	}
	if c := img.NRGBAAt(32, 40); c != Bolt {
		t.Fatalf("center is %v, want the lightning color", c)
	}
	if c := img.NRGBAAt(20, 75*64/100); c != Body {
		t.Fatalf("body is %v", c)
	}
	if _, _, _, a := img.At(32, 5).RGBA(); a == 0 {
		t.Fatal("no battery nub")
	}
	if _, _, _, a := img.At(8, 5).RGBA(); a != 0 {
		t.Fatal("the nub is too wide")
	}
}

func TestPNGSizes(t *testing.T) {
	for _, s := range []int{16, 32, 256} {
		img, err := png.Decode(bytes.NewReader(PNG(s)))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != s || img.Bounds().Dy() != s {
			t.Fatalf("size %d: %v", s, img.Bounds())
		}
	}
}

// TestICO checks the ICO header and that each entry is a PNG of its size.
func TestICO(t *testing.T) {
	ico := ICO()
	le := binary.LittleEndian
	if le.Uint16(ico[0:]) != 0 || le.Uint16(ico[2:]) != 1 || int(le.Uint16(ico[4:])) != len(ICOSizes) {
		t.Fatal("header")
	}
	for i, s := range ICOSizes {
		e := ico[6+16*i:]
		w := int(e[0])
		if w == 0 {
			w = 256
		}
		if w != s {
			t.Fatalf("entry %d width %d", i, w)
		}
		n, off := le.Uint32(e[8:]), le.Uint32(e[12:])
		img, err := png.Decode(bytes.NewReader(ico[off : off+n]))
		if err != nil || img.Bounds().Dx() != s {
			t.Fatalf("entry %d: %v", i, err)
		}
	}
}
