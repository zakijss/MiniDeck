package ui

import (
	"image"
	"image/color"
	"testing"
)

func TestPixelArt(t *testing.T) {
	// a gradient, 11x9 so the last cells are cut by the edge
	src := image.NewRGBA(image.Rect(0, 0, 11, 9))
	for y := range 9 {
		for x := range 11 {
			src.Set(x, y, color.RGBA{uint8(x * 23), uint8(y * 28), 128, 255})
		}
	}
	out := PixelArt(src, 6)
	if out.Bounds() != src.Bounds() {
		t.Fatalf("size %v, want %v", out.Bounds(), src.Bounds())
	}
	levels := map[uint8]bool{0: true, 51: true, 102: true, 153: true, 204: true, 255: true}
	for y := range 9 {
		for x := range 11 {
			c := out.RGBAAt(x, y)
			if !levels[c.R] || !levels[c.G] || !levels[c.B] || c.A != 255 {
				t.Fatalf("(%d,%d) = %v: not on the 6-level palette", x, y, c)
			}
			// every art pixel is one colour
			if x%art == 1 && out.RGBAAt(x-1, y) != c {
				t.Fatalf("(%d,%d) differs from its cell", x, y)
			}
			if y%art == 1 && out.RGBAAt(x, y-1) != c {
				t.Fatalf("(%d,%d) differs from its cell", x, y)
			}
		}
	}
	// a colour already on the palette stays exactly as it is
	flat := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(flat.Pix); i += 4 {
		flat.Pix[i], flat.Pix[i+1], flat.Pix[i+2], flat.Pix[i+3] = 204, 51, 0, 255
	}
	for _, c := range PixelArt(flat, 6).Pix {
		if c != 204 && c != 51 && c != 0 && c != 255 {
			t.Fatalf("a palette colour was dithered: %d", c)
		}
	}
}
