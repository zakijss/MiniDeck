package ui

import (
	"image"
	"image/color"
)

// The look: pixel art drawn on a grid of 2x2 screen pixels ("art pixels"),
// matching the app icons (scripts/make-icons.py): an ink outline, light
// from the top left (a light edge on top and left, a dark one on the bottom
// and right), a hard drop shadow, and stepped corners instead of arcs.

// art is the size of one art pixel in screen pixels.
const art = 2

// ramp is a colour in three tones: light, base and dark.
type ramp struct{ hi, base, lo color.RGBA }

func rgb3(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 0xff} }

// The icons' palette.
var (
	Ink = nightPalette.ink // outlines, and text on light fills (themed)

	lavRamp    = ramp{rgb3(190, 194, 248), rgb3(140, 145, 222), rgb3(98, 102, 184)}
	skyRamp    = ramp{rgb3(168, 228, 255), rgb3(95, 184, 240), rgb3(58, 130, 196)}
	orangeRamp = ramp{rgb3(255, 210, 140), rgb3(245, 167, 66), rgb3(201, 112, 42)}
	greenRamp  = ramp{rgb3(170, 240, 186), rgb3(92, 207, 134), rgb3(47, 154, 99)}
	redRamp    = ramp{rgb3(255, 156, 142), rgb3(229, 83, 75), rgb3(168, 50, 58)}
	yellowRamp = ramp{rgb3(255, 242, 160), rgb3(247, 210, 74), rgb3(201, 154, 42)}
	whiteRamp  = ramp{rgb3(255, 255, 255), rgb3(233, 234, 242), rgb3(185, 187, 208)}
	steelRamp  = ramp{rgb3(122, 127, 160), rgb3(78, 82, 116), rgb3(50, 53, 84)}
	nightRamp  = nightPalette.night // themed
	pinkRamp   = ramp{rgb3(255, 182, 212), rgb3(240, 107, 168), rgb3(184, 65, 122)}

	// focusRamp fills the selection: bright lavender, read with ink text.
	focusRamp = nightPalette.focus
	// keys and buttons that aren't selected
	keyRamp  = nightPalette.key
	key2Ramp = nightPalette.key2
	// accentRamp is the theme's accent as a ramp: icon tiles without a
	// colour of their own, the scroll bar. (lavRamp stays lavender for
	// the app icons.)
	accentRamp = nightPalette.accentR
)

// rampOf makes a ramp around any colour.
func rampOf(c color.RGBA) ramp {
	return ramp{Mix(c, rgb3(255, 255, 255), 0.38), c, Mix(c, Ink, 0.38)}
}

// light reports whether c is light enough to carry ink on it.
func light(c color.RGBA) bool {
	return 299*int(c.R)+587*int(c.G)+114*int(c.B) > 120*1000
}

// ---- spans: shapes on the art grid ----

// shape is a set of art pixels, one horizontal run per art row, laid on the
// screen from (x0, y0). Rows and runs are in art pixels.
type shape struct {
	x0, y0 int
	w, h   int // screen size, so a last odd row or column is one pixel
	rows   []span
}

type span struct{ l, r int } // [l, r) in art pixels; empty when l >= r

func (s span) empty() bool { return s.l >= s.r }

// cornerCut is how far a stepped corner cuts into each art row from the
// edge, for a corner radius given in screen pixels (as for RoundRect).
func cornerCut(rad int) []int {
	switch {
	case rad <= 3:
		return nil
	case rad <= 9:
		return []int{1}
	case rad <= 15:
		return []int{2, 1}
	case rad <= 22:
		return []int{3, 1, 1}
	default:
		return []int{4, 2, 1, 1}
	}
}

// rectShape is r with stepped corners.
func rectShape(r image.Rectangle, rad int) shape {
	cols, rows := (r.Dx()+art-1)/art, (r.Dy()+art-1)/art
	sh := shape{x0: r.Min.X, y0: r.Min.Y, w: r.Dx(), h: r.Dy(), rows: make([]span, max(0, rows))}
	cut := cornerCut(rad)
	// the corners can take at most half the shape each way
	for len(cut) > 0 && (2*len(cut) > rows || 2*cut[0] > cols) {
		cut = cut[1:]
	}
	for i := range sh.rows {
		k := 0
		if j := min(i, rows-1-i); j < len(cut) {
			k = cut[j]
		}
		sh.rows[i] = span{k, cols - k}
	}
	return sh
}

// diskShape is a pixel circle of radius r (screen pixels) centred on (cx, cy),
// drawn like the icons' disks.
func diskShape(cx, cy, r float64) shape {
	ar := r / art
	n := int(2*ar+0.5) + 1
	if n < 1 {
		n = 1
	}
	c := float64(n) / 2
	sh := shape{x0: int(cx - c*art + 0.5), y0: int(cy - c*art + 0.5), w: n * art, h: n * art, rows: make([]span, n)}
	lim := ar*ar + ar*0.3
	for i := 0; i < n; i++ {
		dy := float64(i) + 0.5 - c
		sp := span{n, 0}
		for j := 0; j < n; j++ {
			dx := float64(j) + 0.5 - c
			if dx*dx+dy*dy <= lim {
				sp.l = min(sp.l, j)
				sp.r = max(sp.r, j+1)
			}
		}
		sh.rows[i] = sp
	}
	return sh
}

func (sh shape) row(i int) span {
	if i < 0 || i >= len(sh.rows) {
		return span{0, 0}
	}
	return sh.rows[i]
}

// erode shrinks the shape by one art pixel on every side.
func (sh shape) erode() shape {
	out := sh
	out.rows = make([]span, len(sh.rows))
	for i := range sh.rows {
		a, b, c := sh.row(i-1), sh.row(i), sh.row(i+1)
		sp := span{max(a.l, b.l+1, c.l), min(a.r, b.r-1, c.r)}
		if a.empty() || c.empty() || b.empty() {
			sp = span{0, 0}
		}
		out.rows[i] = sp
	}
	return out
}

// andShifted keeps the part of the shape that is also in the shape moved
// right and down by d art pixels.
func (sh shape) andShifted(d int) shape {
	out := sh
	out.rows = make([]span, len(sh.rows))
	for i := range sh.rows {
		a, b := sh.row(i-d), sh.row(i)
		if a.empty() || b.empty() {
			out.rows[i] = span{0, 0}
			continue
		}
		out.rows[i] = span{max(b.l, a.l+d), min(b.r, a.r+d)}
	}
	return out
}

// screen rows and columns of art row i and of the run [l, r)
func (sh shape) ys(i int) (int, int) {
	return sh.y0 + i*art, sh.y0 + min((i+1)*art, sh.h)
}

func (sh shape) xs(sp span) (int, int) {
	return sh.x0 + sp.l*art, sh.x0 + min(sp.r*art, sh.w)
}

// fillShape paints the shape in c.
func (s *Screen) fillShape(sh shape, c color.RGBA) {
	for i := 0; i < len(sh.rows); {
		sp := sh.rows[i]
		e := i + 1 // rows alike are one rectangle
		for e < len(sh.rows) && sh.rows[e] == sp {
			e++
		}
		if !sp.empty() {
			y0, _ := sh.ys(i)
			_, y1 := sh.ys(e - 1)
			x0, x1 := sh.xs(sp)
			s.fillRect(image.Rect(x0, y0, x1, y1), c)
		}
		i = e
	}
}

// blendShape paints the shape in c at opacity a (0..256).
func (s *Screen) blendShape(sh shape, c color.RGBA, a int32) {
	for i, sp := range sh.rows {
		if sp.empty() {
			continue
		}
		y0, y1 := sh.ys(i)
		x0, x1 := sh.xs(sp)
		s.blendRect(image.Rect(x0, y0, x1, y1), c, a)
	}
}

// ringShape paints what is in outer but not in inner (inner lies inside it).
func (s *Screen) ringShape(outer, inner shape, c color.RGBA, a int32) {
	paint := s.fillRect
	if a < 256 {
		paint = func(r image.Rectangle, c color.RGBA) { s.blendRect(r, c, a) }
	}
	for i, o := range outer.rows {
		if o.empty() {
			continue
		}
		y0, y1 := outer.ys(i)
		in := inner.row(i)
		if in.empty() {
			x0, x1 := outer.xs(o)
			paint(image.Rect(x0, y0, x1, y1), c)
			continue
		}
		x0, x1 := outer.x0+o.l*art, outer.x0+in.l*art
		paint(image.Rect(x0, y0, x1, y1), c)
		x2, x3 := outer.x0+in.r*art, min(outer.x0+o.r*art, outer.x0+outer.w)
		paint(image.Rect(x2, y0, x3, y1), c)
	}
}

// dropShadow darkens what lies right of and below the shape, d art pixels
// out: a hard shadow, as the icons have.
func (s *Screen) dropShadow(sh shape, d int, a int32) {
	off := d * art
	black := color.RGBA{0, 0, 0, 0xff}
	n := len(sh.rows)
	for i := 0; i < n+d; i++ {
		sp := sh.row(i - d) // the shadow's run in this row
		if sp.empty() {
			continue
		}
		y0 := sh.y0 + i*art
		y1 := y0 + art
		if i >= n {
			y1 = min(y1, sh.y0+sh.h+off)
		}
		l, r := sh.x0+sp.l*art+off, min(sh.x0+sp.r*art, sh.x0+sh.w)+off
		own := sh.row(i)
		if own.empty() {
			s.blendRect(image.Rect(l, y0, r, y1), black, a)
			continue
		}
		ol, or := sh.xs(own)
		// the shadow shows only outside the shape itself
		if l < ol {
			s.blendRect(image.Rect(l, y0, min(r, ol), y1), black, a)
		}
		if r > or {
			s.blendRect(image.Rect(max(l, or), y0, r, y1), black, a)
		}
	}
}

// bevel draws the shape as a pixel-art block: an ink outline one art pixel
// wide, a light edge on its top and left, a dark one on its bottom and
// right, and the base colour inside.
// Each pixel is painted once: every layer is its shape minus the ones
// inside it.
func (s *Screen) bevel(sh shape, c ramp, outline color.RGBA) {
	in := sh.erode()
	dark := in.andShifted(1) // all but the top and left edges
	core := in.erode()
	s.fillShape(core, c.base)
	s.paintDiff(sh, outline, in)
	s.paintDiff(in, c.hi, dark, core)
	s.paintDiff(dark, c.lo, core)
}

// paintDiff fills what is in a but in none of minus (all on a's grid).
func (s *Screen) paintDiff(a shape, c color.RGBA, minus ...shape) {
	var segs, next []span
	for i, sp := range a.rows {
		if sp.empty() {
			continue
		}
		segs = append(segs[:0], sp)
		for _, m := range minus {
			b := m.row(i)
			if b.empty() {
				continue
			}
			next = next[:0]
			for _, g := range segs {
				if l := (span{g.l, min(g.r, b.l)}); !l.empty() {
					next = append(next, l)
				}
				if r := (span{max(g.l, b.r), g.r}); !r.empty() {
					next = append(next, r)
				}
			}
			segs, next = next, segs
		}
		y0, y1 := a.ys(i)
		for _, g := range segs {
			x0, x1 := a.xs(g)
			s.fillRect(image.Rect(x0, y0, x1, y1), c)
		}
	}
}

// panel draws a card in r: a bevelled block with a hard drop shadow.
func (s *Screen) block(r image.Rectangle, rad int, c ramp, outline color.RGBA, shadow int) {
	sh := rectShape(r, rad)
	if shadow > 0 {
		s.dropShadow(sh, shadow, 110)
	}
	s.bevel(sh, c, outline)
}

// ---- fast rectangles ----

func (s *Screen) fillRect(r image.Rectangle, c color.RGBA) {
	r = r.Intersect(s.Rect)
	if r.Empty() {
		return
	}
	w := r.Dx() * 4
	row := s.Pix[s.PixOffset(r.Min.X, r.Min.Y):]
	first := row[:w]
	first[0], first[1], first[2], first[3] = c.R, c.G, c.B, 0xff
	for n := 4; n < w; n *= 2 { // the first row, by doubling
		copy(first[n:], first[:n])
	}
	for y := r.Min.Y + 1; y < r.Max.Y; y++ {
		o := (y - r.Min.Y) * s.Stride
		copy(row[o:o+w], first)
	}
}

// blendRect blends c over r at opacity a (0..256).
func (s *Screen) blendRect(r image.Rectangle, c color.RGBA, a int32) {
	if a >= 256 {
		s.fillRect(r, c)
		return
	}
	r = r.Intersect(s.Rect)
	if r.Empty() || a <= 0 {
		return
	}
	cr, cg, cb := int32(c.R), int32(c.G), int32(c.B)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		p := s.Pix[s.PixOffset(r.Min.X, y) : s.PixOffset(r.Min.X, y)+r.Dx()*4]
		for i := 0; i < len(p); i += 4 {
			p[i] = uint8(int32(p[i]) + (cr-int32(p[i]))*a>>8)
			p[i+1] = uint8(int32(p[i+1]) + (cg-int32(p[i+1]))*a>>8)
			p[i+2] = uint8(int32(p[i+2]) + (cb-int32(p[i+2]))*a>>8)
		}
	}
}

// alpha256 turns an opacity 0..1 into 0..256.
func alpha256(a float64) int32 {
	if a <= 0 {
		return 0
	}
	if a >= 1 {
		return 256
	}
	return int32(a*256 + 0.5)
}

// ---- small bitmaps ----

// bitmap draws rows of characters with each character's art pixel scaled
// by k: '#' in c, and any other letter in the colour pal gives it ('.' and
// ' ' are clear).
func (s *Screen) bitmap(x, y, k int, rows []string, c color.RGBA, pal map[byte]color.RGBA) {
	for j, row := range rows {
		for i := 0; i < len(row); {
			ch := row[i]
			if ch == '.' || ch == ' ' {
				i++
				continue
			}
			e := i + 1
			for e < len(row) && row[e] == ch {
				e++
			}
			col := c
			if ch != '#' {
				if pc, ok := pal[ch]; ok {
					col = pc
				}
			}
			s.fillRect(image.Rect(x+i*k, y+j*k, x+e*k, y+(j+1)*k), col)
			i = e
		}
	}
}
