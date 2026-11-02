package ui

import (
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

func rgb(v uint32) color.RGBA {
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

// Colours, from the pixel icons' palette (see pixel.go): a night-blue
// interface, near-white text, bright lavender for the selection, and colour
// only where it means something (good, warning, problem). Text colours keep
// at least 4.5:1 contrast against the surfaces they sit on.
//
// These are the Night theme's; a theme replaces all but Good, Warn and Bad
// when the screen opens (see theme.go). Read them when drawing, not into
// variables of your own: they change when the user picks another theme.
var (
	BG       = nightPalette.bg      // behind everything
	Surface  = nightPalette.surface // cards and lists (night)
	Raised   = nightPalette.raised  // a step lighter
	Line     = nightPalette.line    // separators, empty tracks
	Text     = nightPalette.text
	Muted    = nightPalette.muted
	Faint    = nightPalette.faint    // placeholders
	OnFocus  = nightPalette.ink      // text on the selection
	OnFocus2 = nightPalette.onFocus2 // secondary text on the selection
	Good     = rgb(0x5ccf86)
	Warn     = rgb(0xf5a742)
	Bad      = rgb(0xf05c54)
	// Accent is the focus and emphasis colour: the selection's lavender.
	Accent = nightPalette.accent
)

// Mix blends a toward b by t (0..1).
func Mix(a, b color.RGBA, t float64) color.RGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 0xff}
}

// Inter draws the characters the pixel font lacks that it has, such as
// Greek and Cyrillic in song titles, at the pixel font's size; GNU Unifont
// draws the rest (see pixfont.go and unifont.go).

//go:embed fonts/Inter-Regular.ttf
var interRegular []byte

//go:embed fonts/Inter-Bold.ttf
var interBold []byte

// Font faces: Pixel Operator at whole-number scales (see pixfont.go).
var (
	Display font.Face // bold, 3x: big titles and codes
	Title   font.Face // bold, 3x: screen titles
	Heading font.Face // bold, 2x: card titles, list labels
	Body    font.Face // 2x
	Small   font.Face // the 8 px cut, 2x: secondary lines
	Caps    font.Face // pixel small capitals 3x5, 2x: short labels
	Big     font.Face // bold, 3x: numbers
)

func face(ttf []byte, size float64) font.Face {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(err)
	}
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return fc
}

// Body text is the 16 px design at 2x (capitals 18 px tall), small text the
// 8 px design at 2x (14 px), titles bold at 3x (27 px).
func init() {
	Display = newPixelFace(poBoldDesign, 3)
	Title = newPixelFace(poBoldDesign, 3)
	Heading = newPixelFace(poBoldDesign, 2)
	Body = newPixelFace(poDesign, 2)
	Small = newPixelFace(po8Design, 2)
	Caps = newMicroFace(2, newPixelFace(po8BoldDes, 2))
	Big = newPixelFace(poBoldDesign, 3)
}

// ---- pixels ----

func (s *Screen) blend(x, y int, c color.RGBA, a float64) {
	if a <= 0 || x < s.Rect.Min.X || y < s.Rect.Min.Y || x >= s.Rect.Max.X || y >= s.Rect.Max.Y {
		return
	}
	i := s.PixOffset(x, y)
	p := s.Pix[i : i+4 : i+4]
	if a >= 1 {
		p[0], p[1], p[2], p[3] = c.R, c.G, c.B, 0xff
		return
	}
	// integer maths: alpha in 1/256ths
	ia := int32(a*256 + 0.5)
	p[0] = uint8(int32(p[0]) + (int32(c.R)-int32(p[0]))*ia>>8)
	p[1] = uint8(int32(p[1]) + (int32(c.G)-int32(p[1]))*ia>>8)
	p[2] = uint8(int32(p[2]) + (int32(c.B)-int32(p[2]))*ia>>8)
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func toRGBA(c color.Color) color.RGBA {
	if v, ok := c.(color.RGBA); ok {
		return v
	}
	r, g, b, _ := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
}

// Clear fills the screen with c. Most screens use Background instead.
func (s *Screen) Clear(c color.Color) {
	s.Fill(s.Bounds(), c)
	s.bgID = nil
}

// Fill fills r with c.
func (s *Screen) Fill(r image.Rectangle, c color.Color) {
	s.fillRect(r, toRGBA(c))
}

// FillAlpha blends c over r with opacity a.
func (s *Screen) FillAlpha(r image.Rectangle, c color.Color, a float64) {
	s.blendRect(r, toRGBA(c), alpha256(a))
}

// RoundRect fills r with stepped pixel corners for a corner radius rad.
func (s *Screen) RoundRect(r image.Rectangle, rad int, c color.Color) {
	s.fillShape(rectShape(r, rad), toRGBA(c))
}

// RoundRectAlpha is RoundRect at opacity alpha.
func (s *Screen) RoundRectAlpha(r image.Rectangle, rad int, c color.Color, alpha float64) {
	s.blendShape(rectShape(r, rad), toRGBA(c), alpha256(alpha))
}

// outlineArt is how many art pixels an outline w screen pixels wide takes.
func outlineArt(w int) int { return max(1, (w+art-1)/art) }

// Outline draws the border of r, w pixels wide (rounded up to whole art
// pixels), with stepped corners.
func (s *Screen) Outline(r image.Rectangle, rad, w int, c color.Color) {
	outer := rectShape(r, rad)
	inner := outer
	for i := 0; i < outlineArt(w); i++ {
		inner = inner.erode()
	}
	s.ringShape(outer, inner, toRGBA(c), 256)
}

// Shadow draws a hard drop shadow for r, down and to the right (only
// around it: r itself is meant to be covered by whatever casts it).
func (s *Screen) Shadow(r image.Rectangle, rad int) {
	s.dropShadow(rectShape(r, rad), 2, 110)
}

// Circle fills a pixel circle.
func (s *Screen) Circle(cx, cy, r float64, c color.Color) {
	s.fillShape(diskShape(cx, cy, r), toRGBA(c))
}

// artCells calls fn for every art pixel (on the screen-wide grid of even
// coordinates) whose box overlaps [x0, x1) x [y0, y1), with its centre.
func artCells(x0, y0, x1, y1 float64, fn func(x, y int, cx, cy float64)) {
	gx0, gy0 := int(math.Floor(x0/art))*art, int(math.Floor(y0/art))*art
	for y := gy0; float64(y) < y1; y += art {
		for x := gx0; float64(x) < x1; x += art {
			fn(x, y, float64(x)+art/2.0, float64(y)+art/2.0)
		}
	}
}

// ringCells calls paint with the runs of art pixels of a ring (outer radius
// r, thickness th) from the top clockwise for frac of a turn.
func ringCells(cx, cy, r, th, frac float64, paint func(image.Rectangle)) {
	frac = clamp01(frac)
	if frac == 0 {
		return
	}
	outer := r*r + r*art*0.3
	in := math.Max(0, r-th)
	inner := in*in + in*art*0.3
	if in == 0 {
		inner = -1
	}
	limit := frac * 2 * math.Pi
	for y := int(math.Floor((cy-r-art)/art)) * art; float64(y) < cy+r+art; y += art {
		run := -1
		flush := func(x int) {
			if run >= 0 {
				paint(image.Rect(run, y, x, y+art))
				run = -1
			}
		}
		x := int(math.Floor((cx-r-art)/art)) * art
		for ; float64(x) < cx+r+art; x += art {
			dx, dy := float64(x)+art/2.0-cx, float64(y)+art/2.0-cy
			d2 := dx*dx + dy*dy
			ok := d2 <= outer && d2 > inner
			if ok && frac < 1 {
				a := math.Atan2(dx, -dy) // 0 at the top, clockwise
				if a < 0 {
					a += 2 * math.Pi
				}
				ok = a <= limit
			}
			if ok && run < 0 {
				run = x
			} else if !ok {
				flush(x)
			}
		}
		flush(x)
	}
}

// Ring draws an arc of a ring (thickness th, outer radius r) starting at the
// top and running clockwise for frac of a turn, in art pixels.
func (s *Screen) Ring(cx, cy, r, th, frac float64, c color.Color) {
	cc := toRGBA(c)
	ringCells(cx, cy, r, th, frac, func(b image.Rectangle) { s.fillRect(b, cc) })
}

// Pt is a point in screen pixels, for Stroke and Polygon.
type Pt struct{ X, Y float64 }

// Stroke draws a polyline of width w in art pixels.
func (s *Screen) Stroke(pts []Pt, w float64, c color.Color) {
	cc := toRGBA(c)
	if len(pts) == 0 {
		return
	}
	minX, minY, maxX, maxY := pts[0].X, pts[0].Y, pts[0].X, pts[0].Y
	for _, p := range pts {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	h := w/2 + art
	lim := math.Max(w/2, art*0.75)
	artCells(minX-h, minY-h, maxX+h, maxY+h, func(x, y int, px, py float64) {
		best := math.Hypot(px-pts[0].X, py-pts[0].Y)
		for i := 0; i+1 < len(pts); i++ {
			best = math.Min(best, segDist(px, py, pts[i], pts[i+1]))
		}
		if best <= lim {
			s.fillRect(image.Rect(x, y, x+art, y+art), cc)
		}
	})
}

func segDist(px, py float64, a, b Pt) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := dx*dx + dy*dy
	t := 0.0
	if l > 0 {
		t = clamp01(((px-a.X)*dx + (py-a.Y)*dy) / l)
	}
	return math.Hypot(px-(a.X+t*dx), py-(a.Y+t*dy))
}

// Polygon fills a polygon in art pixels (those whose centre is inside).
func (s *Screen) Polygon(pts []Pt, c color.Color) {
	cc := toRGBA(c)
	if len(pts) == 0 {
		return
	}
	minX, minY, maxX, maxY := pts[0].X, pts[0].Y, pts[0].X, pts[0].Y
	for _, p := range pts {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	artCells(minX, minY, maxX, maxY, func(x, y int, px, py float64) {
		if inPoly(px, py, pts) {
			s.fillRect(image.Rect(x, y, x+art, y+art), cc)
		}
	})
}

func inPoly(x, y float64, pts []Pt) bool {
	in := false
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		a, b := pts[i], pts[j]
		if (a.Y > y) != (b.Y > y) && x < (b.X-a.X)*(y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}

// ---- components ----

// Bar draws a horizontal progress bar filled to frac (0..1): a dark track
// and a flat fill with a light top edge, in whole art pixels. Bars 8 px or
// taller get an ink outline.
func (s *Screen) Bar(r image.Rectangle, frac float64, fg color.Color) {
	c := rampOf(toRGBA(fg))
	frac = clamp01(frac)
	in := r
	if r.Dy() >= 8 {
		s.fillShape(rectShape(r, 0), Ink)
		in = r.Inset(art)
		s.fillRect(in, nightRamp.lo)
	} else {
		s.blendRect(r, Text, 44)
	}
	w := int(float64(in.Dx())*frac) / art * art
	if frac > 0 && w < art {
		w = art
	}
	if w <= 0 {
		return
	}
	f := image.Rect(in.Min.X, in.Min.Y, in.Min.X+w, in.Max.Y)
	s.fillRect(f, c.base)
	if f.Dy() >= 3*art {
		s.fillRect(image.Rect(f.Min.X, f.Min.Y, f.Max.X, f.Min.Y+art), c.hi)
		s.fillRect(image.Rect(f.Min.X, f.Max.Y-art, f.Max.X, f.Max.Y), c.lo)
	}
}

// Gauge draws a ring gauge with the percentage in the middle and a label
// underneath. pct < 0 means unknown.
func (s *Screen) Gauge(cx, cy, r int, pct float64, label string) {
	th := math.Max(6, float64(r)*0.2)
	s.Ring(float64(cx), float64(cy), float64(r), th, 1, Line)
	txt := "–"
	if pct >= 0 {
		s.Ring(float64(cx), float64(cy), float64(r), th, pct/100, Level(pct))
		txt = itoa(int(pct + 0.5))
	}
	f := Heading
	if r >= 44 {
		f = Big
	}
	s.TextCenter(cx, cy-LineHeight(f)/2, txt, f, Text)
	if label != "" {
		s.TextCenter(cx, cy+r+6, label, Caps, Muted)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// Level picks a colour for a usage percentage: white normally, orange from
// 75 %, red from 90 % (the number is always shown too).
func Level(pct float64) color.Color {
	switch {
	case pct >= 90:
		return Bad
	case pct >= 75:
		return Warn
	}
	return Text
}

// RoundPicture draws img scaled into r at art-pixel resolution, with stepped
// corners and an ink frame. The scaled result is cached per image and size.
func (s *Screen) RoundPicture(img image.Image, r image.Rectangle, rad int) {
	s.drawCached(scaledRound(img, r.Dx(), r.Dy(), rad), r.Min)
	sh := rectShape(r, rad)
	s.ringShape(sh, sh.erode(), Ink, 256)
}

// bayer4 is a 4x4 ordered-dither matrix (0..15).
var bayer4 = [4][4]int32{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}

// Backdrop makes a full-screen image from img for drawing behind a
// now-playing screen: the cover's colours, blurred, darkened, and
// posterised with an ordered dither at art-pixel resolution. Compute it once
// per image.
func Backdrop(img image.Image) *image.RGBA {
	// Blur at 1/8 size, where a wide blur is cheap.
	const sw, sh = W / 8, H / 8
	small := image.NewRGBA(image.Rect(0, 0, sw, sh))
	xdraw.CatmullRom.Scale(small, small.Bounds(), img, img.Bounds(), draw.Src, nil)
	buf := make([]float32, sw*sh*3)
	for i := 0; i < sw*sh; i++ {
		buf[i*3], buf[i*3+1], buf[i*3+2] = float32(small.Pix[i*4]), float32(small.Pix[i*4+1]), float32(small.Pix[i*4+2])
	}
	for pass := 0; pass < 3; pass++ { // ~ a gaussian with sigma 6 (48 px full size)
		boxBlur(buf, sw, sh, 6)
	}
	for i := 0; i < sw*sh; i++ {
		r, g, b := float64(buf[i*3]), float64(buf[i*3+1]), float64(buf[i*3+2])
		// a little more colour, so it doesn't turn to grey mud
		l := 0.299*r + 0.587*g + 0.114*b
		r, g, b = l+(r-l)*1.35, l+(g-l)*1.35, l+(b-l)*1.35
		small.Pix[i*4], small.Pix[i*4+1], small.Pix[i*4+2] = clamp8(r), clamp8(g), clamp8(b)
	}
	const aw, ah = W / art, H / art
	mid := image.NewRGBA(image.Rect(0, 0, aw, ah))
	xdraw.BiLinear.Scale(mid, mid.Bounds(), small, small.Bounds(), draw.Src, nil)
	// Darken for contrast with white text (more towards the bottom, where
	// the footer is), then posterise to steps of 20 levels with the dither,
	// and blow each art pixel up to 2x2.
	const step = 20
	out := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < ah; y++ {
		k := int32(150 - 56*y/ah) // /256: 0.58 at the top, 0.37 at the bottom
		for x := 0; x < aw; x++ {
			i := mid.PixOffset(x, y)
			t := (bayer4[y&3][x&3]*2+1)*step/32 - step/2
			var px [3]uint8
			for c := 0; c < 3; c++ {
				v := int32(mid.Pix[i+c])*k>>8 + t + step/2
				v = v / step * step
				px[c] = uint8(min(255, max(0, v)))
			}
			for dy := 0; dy < art; dy++ {
				o := out.PixOffset(x*art, y*art+dy)
				for dx := 0; dx < art; dx++ {
					out.Pix[o+dx*4], out.Pix[o+dx*4+1], out.Pix[o+dx*4+2], out.Pix[o+dx*4+3] = px[0], px[1], px[2], 255
				}
			}
		}
	}
	return out
}

// boxBlur blurs an RGB float buffer in place, horizontally then vertically.
func boxBlur(buf []float32, w, h, rad int) {
	tmp := make([]float32, len(buf))
	n := float32(2*rad + 1)
	for y := 0; y < h; y++ {
		for c := 0; c < 3; c++ {
			var sum float32
			for x := -rad; x <= rad; x++ {
				sum += buf[(y*w+clampi(x, 0, w-1))*3+c]
			}
			for x := 0; x < w; x++ {
				tmp[(y*w+x)*3+c] = sum / n
				sum += buf[(y*w+clampi(x+rad+1, 0, w-1))*3+c] - buf[(y*w+clampi(x-rad, 0, w-1))*3+c]
			}
		}
	}
	for x := 0; x < w; x++ {
		for c := 0; c < 3; c++ {
			var sum float32
			for y := -rad; y <= rad; y++ {
				sum += tmp[(clampi(y, 0, h-1)*w+x)*3+c]
			}
			for y := 0; y < h; y++ {
				buf[(y*w+x)*3+c] = sum / n
				sum += tmp[(clampi(y+rad+1, 0, h-1)*w+x)*3+c] - tmp[(clampi(y-rad, 0, h-1)*w+x)*3+c]
			}
		}
	}
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// Blit copies a full-screen image (such as a Backdrop) onto the screen.
func (s *Screen) Blit(img *image.RGBA) {
	copy(s.Pix, img.Pix)
	s.bgID = img
}
