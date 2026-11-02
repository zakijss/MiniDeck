package ui

import (
	"image"
	"image/color"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Text goes through two steps before it is drawn or measured: Arabic is
// shaped (shape.go) and mixed-direction text is put in visual order
// (bidi.go). Strings with no right-to-left characters skip both: for them
// the visual string is the string itself, found by one pass over the
// bytes. The others are laid out once and kept in a small cache, as the
// same labels are drawn frame after frame.

// needsLayout reports whether s holds right-to-left characters or
// directional controls, which need shaping and reordering.
func needsLayout(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x80 {
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if rtlRune(r) {
			return true
		}
		i += n - 1
	}
	return false
}

// rtlRune covers the right-to-left scripts (Hebrew to the Arabic
// supplements, the Hebrew and Arabic presentation forms, the historic
// scripts and Arabic mathematics in plane 1) and the directional controls.
func rtlRune(r rune) bool {
	switch {
	case r < 0x590:
		return false
	case r <= 0x8ff, r >= 0xfb1d && r <= 0xfdff, r >= 0xfe70 && r <= 0xfefe:
		return true
	case r == 0x200e || r == 0x200f, r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
		return true
	case r >= 0x10800 && r <= 0x10fff, r >= 0x1e800 && r <= 0x1efff:
		return true
	}
	return false
}

// lru is a small cache that drops what hasn't been used lately: entries
// live in cur, and when it fills it becomes old and a new cur starts; a hit
// in old moves back to cur. So anything used since the last swap stays.
type lru[K comparable, V any] struct {
	mu       sync.Mutex
	size     int
	cur, old map[K]V
}

func (c *lru[K, V]) get(k K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.cur[k]; ok {
		return v, true
	}
	v, ok := c.old[k]
	if ok {
		c.putLocked(k, v)
	}
	return v, ok
}

func (c *lru[K, V]) put(k K, v V) {
	c.mu.Lock()
	c.putLocked(k, v)
	c.mu.Unlock()
}

func (c *lru[K, V]) putLocked(k K, v V) {
	if c.cur == nil || len(c.cur) >= c.size {
		c.old, c.cur = c.cur, map[K]V{}
	}
	c.cur[k] = v
}

// visuals caches the visual form of right-to-left strings.
var visuals = lru[string, string]{size: 128}

// visual returns s shaped and in visual order, ready to draw left to right.
func visual(s string) string {
	if !needsLayout(s) {
		return s
	}
	if v, ok := visuals.get(s); ok {
		return v
	}
	v := layout(s)
	visuals.put(s, v)
	return v
}

func layout(s string) string { return string(bidiReorder(shapeArabic([]rune(s)))) }

// glyphSource is a face of this package, which hands out its glyphs
// directly so text can be drawn without going through font.Drawer.
type glyphSource interface {
	glyphOf(r rune) *glyph
}

// runWidth is the width of s in f, s already in visual order. A combining
// mark takes no room after another character (it is drawn over it, see
// drawRun).
func runWidth(f glyphSource, s string) int {
	w, prev := 0, 0
	for _, r := range s {
		g := f.glyphOf(r)
		if g.mark && prev > 0 {
			continue
		}
		adv := g.adv
		if g.mark {
			adv = g.cell
		}
		w += adv
		if adv > 0 { // zero-width characters don't come between a mark and its base
			prev = adv
		}
	}
	return w
}

// drawRun draws s (in visual order) on dst with its baseline at y,
// starting at x, and returns the width. Marks are centred on the
// character before them, cell on cell.
func drawRun(dst *image.RGBA, x, y int, s string, f glyphSource, c color.RGBA) int {
	dot, prevX, prev := x, x, 0
	for _, r := range s {
		g := f.glyphOf(r)
		gx := dot
		if g.mark && prev > 0 {
			gx = prevX + (prev-g.cell)/2
		}
		if g.ok {
			blitMask(dst, g.mask, g.rect.Add(image.Pt(gx, y)), c)
		}
		if g.mark && prev > 0 {
			continue
		}
		adv := g.adv
		if g.mark {
			adv = g.cell
		}
		if adv > 0 {
			prevX, prev = dot, adv
		}
		dot += adv
	}
	return dot - x
}

// blitMask paints c through mask (placed at r) onto dst. Pixel glyphs are
// all on or off; anything between is blended.
func blitMask(dst *image.RGBA, mask *image.Alpha, r image.Rectangle, c color.RGBA) {
	cl := r.Intersect(dst.Rect)
	if cl.Empty() {
		return
	}
	for y := cl.Min.Y; y < cl.Max.Y; y++ {
		mrow := mask.Pix[(y-r.Min.Y)*mask.Stride:]
		d := dst.Pix[dst.PixOffset(cl.Min.X, y):]
		for x := cl.Min.X; x < cl.Max.X; x, d = x+1, d[4:] {
			switch a := mrow[x-r.Min.X]; a {
			case 0:
			case 0xff:
				d[0], d[1], d[2], d[3] = c.R, c.G, c.B, 0xff
			default:
				ia := int32(a) + 1
				d[0] = uint8(int32(d[0]) + (int32(c.R)-int32(d[0]))*ia>>8)
				d[1] = uint8(int32(d[1]) + (int32(c.G)-int32(d[1]))*ia>>8)
				d[2] = uint8(int32(d[2]) + (int32(c.B)-int32(d[2]))*ia>>8)
				d[3] = 0xff
			}
		}
	}
}

// drawText draws str on dst with its baseline at y. str may be in logical
// order: it is laid out here.
func drawText(dst *image.RGBA, x, y int, str string, f font.Face, c color.RGBA) int {
	str = visual(str)
	if gs, ok := f.(glyphSource); ok {
		return drawRun(dst, x, y, str, gs, c)
	}
	d := &font.Drawer{Dst: dst, Src: &image.Uniform{c}, Face: f, Dot: fixed.P(x, y)}
	d.DrawString(str)
	return d.Dot.X.Ceil() - x
}

// Text draws str with its top-left corner at (x, y) and returns its width.
// Right-to-left text is drawn in visual order from x as well (see bidi.go).
func (s *Screen) Text(x, y int, str string, f font.Face, c color.Color) int {
	return drawText(s.RGBA, x, y+f.Metrics().Ascent.Ceil(), str, f, toRGBA(c))
}

// TextRight draws str ending at x.
func (s *Screen) TextRight(x, y int, str string, f font.Face, c color.Color) {
	s.Text(x-Measure(f, str), y, str, f, c)
}

// TextCenter draws str centred on cx.
func (s *Screen) TextCenter(cx, y int, str string, f font.Face, c color.Color) {
	s.Text(cx-Measure(f, str)/2, y, str, f, c)
}

// centerY is the y at which to draw text in f so its capitals are centred
// on cy.
func centerY(f font.Face, cy int) int {
	m := f.Metrics()
	return cy - m.Ascent.Ceil() + m.CapHeight.Ceil()/2
}

// Measure returns the width of str in f, in pixels.
func Measure(f font.Face, str string) int {
	if gs, ok := f.(glyphSource); ok {
		return runWidth(gs, visual(str))
	}
	return font.MeasureString(f, visual(str)).Ceil()
}

// LineHeight returns the distance between lines of text in f.
func LineHeight(f font.Face) int { return f.Metrics().Height.Ceil() }

// width is Measure without caching the layout: Fit and Wrap try many
// prefixes, which are not worth keeping.
func width(f font.Face, str string) int {
	if needsLayout(str) {
		str = layout(str)
	}
	if gs, ok := f.(glyphSource); ok {
		return runWidth(gs, str)
	}
	return font.MeasureString(f, str).Ceil()
}

type fitKey struct {
	f font.Face
	s string
	w int
}

// fits caches Fit's answers for right-to-left strings, which take a few
// shaping passes each.
var fits = lru[fitKey, string]{size: 64}

// Fit shortens str with an ellipsis until it fits in w pixels. It cuts in
// logical order, between characters (never between a letter and its
// marks), so the end of the text is what goes: on the right for
// left-to-right text, on the left for right-to-left text, where the
// ellipsis follows it.
func Fit(f font.Face, str string, w int) string {
	if Measure(f, str) <= w {
		return str
	}
	complex := needsLayout(str)
	if complex {
		if v, ok := fits.get(fitKey{f, str, w}); ok {
			return v
		}
	}
	// the places it may be cut: after each character not followed by a mark
	var cuts []int
	for i, r := range str {
		if i > 0 && !isMark(r) {
			cuts = append(cuts, i)
		}
	}
	// the longest prefix that fits with the ellipsis (widths only grow
	// with the prefix, give or take a ligature)
	lo, hi := 0, len(cuts)
	for lo < hi {
		m := (lo + hi + 1) / 2
		if width(f, str[:cuts[m-1]]+"…") <= w {
			lo = m
		} else {
			hi = m - 1
		}
	}
	out := "…"
	if lo > 0 {
		out = str[:cuts[lo-1]] + "…"
	}
	if complex {
		fits.put(fitKey{f, str, w}, out)
	}
	return out
}

// Wrap breaks text into lines no wider than w, in logical order; each line
// is put in visual order when it is drawn. A line of a right-to-left
// paragraph that starts with a left-to-right word gets a right-to-left
// mark (and the other way round), so it is ordered in its paragraph's
// direction rather than its own.
func Wrap(f font.Face, text string, w int) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		first := len(lines)
		line := ""
		for _, word := range strings.Split(para, " ") {
			if word == "" {
				continue
			}
			try := word
			if line != "" {
				try = line + " " + word
			}
			if width(f, try) > w && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				line = try
			}
		}
		lines = append(lines, line)
		if !needsLayout(para) || len(lines)-first < 2 {
			continue
		}
		rtl := rtlText(para)
		for i := first; i < len(lines); i++ {
			if l := lines[i]; l != "" && rtlText(l) != rtl {
				if rtl {
					lines[i] = "\u200f" + l
				} else {
					lines[i] = "\u200e" + l
				}
			}
		}
	}
	return lines
}
