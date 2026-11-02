package ui

import (
	"image"
	"image/color"
	"math"
	"sync"
	"sync/atomic"
)

// ---- background ----

var (
	ambientMu  sync.Mutex
	ambientImg atomic.Pointer[image.RGBA]
)

// bgLevels are the background's colours, darkest first (night blues in
// the Night theme); starColors are its fixed stars, dim and bright;
// wellColors the keyboard's recess. The theme sets them.
var (
	bgLevels   = nightPalette.levels
	starColors = nightPalette.stars
	wellColors = nightPalette.wells
)

// ambientAt is the background's level (0..3, fractional) at (x, y): dark,
// lit from the top left, with a faint second glow at the bottom right.
func ambientAt(x, y float64) float64 {
	v := 0.55
	for _, g := range [...]struct{ x, y, r, v float64 }{
		{70, 10, 620, 2.6},   // top left
		{620, 480, 360, 0.9}, // bottom right
	} {
		d := math.Hypot(x-g.x, y-g.y) / g.r
		if d < 1 {
			v += g.v * (1 - d) * (1 - d)
		}
	}
	return math.Min(v, float64(len(bgLevels)-1))
}

// starAt reports whether art pixel (ax, ay) holds a star, and how bright,
// for the Stars background.
func starAt(ax, ay int) int {
	switch h := hashAt(ax, ay, 0); {
	case h%2400 == 0:
		return 2
	case h%420 == 0:
		return 1
	}
	return 0
}

// ambient is the background, drawn once for each look.
func ambient() *image.RGBA {
	if img := ambientImg.Load(); img != nil {
		return img
	}
	ambientMu.Lock()
	defer ambientMu.Unlock()
	if img := ambientImg.Load(); img != nil {
		return img
	}
	// One colour per art pixel: the level, ordered-dithered between the
	// two nearest colours, and the background's fixed marks (stars,
	// for Stars).
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	star := [...]color.RGBA{{}, starColors[0], starColors[1]}
	mark := currentScene().mark
	for ay := 0; ay < H/art; ay++ {
		for ax := 0; ax < W/art; ax++ {
			v := ambientAt(float64(ax*art+1), float64(ay*art+1))
			lvl := int(v)
			if v-float64(lvl) > (float64(bayer4[ay&3][ax&3])+0.5)/16 {
				lvl++
			}
			c := bgLevels[min(lvl, len(bgLevels)-1)]
			if mark != nil {
				if k := mark(ax, ay); k > 0 {
					c = star[min(k, 2)]
				}
			}
			for dy := 0; dy < art; dy++ {
				i := img.PixOffset(ax*art, ay*art+dy)
				for dx := 0; dx < art; dx++ {
					img.Pix[i+dx*4], img.Pix[i+dx*4+1], img.Pix[i+dx*4+2], img.Pix[i+dx*4+3] = c.R, c.G, c.B, 255
				}
			}
		}
	}
	ambientImg.Store(img)
	return img
}

// resetAmbient drops the drawn background after the look changed.
func resetAmbient() { ambientImg.Store(nil) }

// Background draws the default background.
func (s *Screen) Background() {
	copy(s.Pix, ambient().Pix)
	s.bgID = ambient()
}

// ---- panels ----

// Panel draws a card in r: a flat night-blue panel with an ink border, a
// light top and left edge, a dark bottom and right edge, stepped corners
// and a hard drop shadow.
func (s *Screen) Panel(r image.Rectangle, rad int) { s.PanelTint(r, rad, 0.10) }

// PanelTint is Panel with a chosen lightness: 0.10 is a normal card, higher
// is lighter (a selected card), lower is darker.
func (s *Screen) PanelTint(r image.Rectangle, rad int, tint float64) {
	if r.Intersect(s.Bounds()).Empty() {
		return
	}
	s.block(r, rad, panelRamp(tint), Ink, 2)
}

// panelRamp is the panel colours for a tint.
func panelRamp(tint float64) ramp {
	base := nightRamp.base
	if t := tint - 0.10; t > 0 {
		base = Mix(base, nightRamp.hi, math.Min(1, t/0.06))
	} else if t < 0 {
		base = Mix(base, nightRamp.lo, math.Min(1, -t/0.05))
	}
	return ramp{
		color.RGBA{sat(int(base.R) + 20), sat(int(base.G) + 21), sat(int(base.B) + 26), 255},
		base,
		Mix(base, color.RGBA{0, 0, 0, 255}, 0.28),
	}
}

func sat(v int) uint8 { return uint8(min(255, max(0, v))) }

// Focus draws the selection: a bright lavender block, bevelled, with an
// ink border and a small hard shadow.
func (s *Screen) Focus(r image.Rectangle, rad int) {
	s.block(r, rad, focusRamp, Ink, 1)
}

// Capsule draws a pill button: lavender when focused, a dark key otherwise.
func (s *Screen) Capsule(r image.Rectangle, label string, focused bool, fg color.RGBA) {
	rad := r.Dy() / 2
	if focused {
		s.block(r, rad, focusRamp, Ink, 1)
		if fg == Text {
			fg = OnFocus
		}
	} else {
		s.block(r, rad, keyRamp, Ink, 1)
	}
	s.TextCenter((r.Min.X+r.Max.X)/2, centerY(Heading, (r.Min.Y+r.Max.Y)/2-1), label, Heading, fg)
}
