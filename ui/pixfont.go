package ui

import (
	_ "embed"
	"image"
	"image/draw"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// The interface font is Pixel Operator by Jayvee Enaguas (CC0 1.0, see
// licenses/pixel-operator-CC0.txt): its outlines sit exactly on a pixel grid
// at the size it was drawn for (16 px, and 8 px for the "8" cut). Glyphs are
// rasterised at that size, where every pixel is fully on or off, and scaled
// up by a whole number with no smoothing, so each font pixel is a crisp
// block on screen.
//
// Characters Pixel Operator lacks come from Inter (Greek, Cyrillic and
// extended Latin, rasterised at the same cap height and thresholded, so
// they keep the size and blockiness of the letters around them), then from
// GNU Unifont (unifont.go), a bitmap font with the rest of the Basic
// Multilingual Plane. Unifont glyphs are 16 pixels tall, so they are drawn
// one scale step down (1x in the 2x faces, 2x in the 3x ones), centred on
// the capitals, and thickened by a pixel in the bold designs.

//go:embed fonts/PixelOperator.ttf
var poRegular []byte

//go:embed fonts/PixelOperator-Bold.ttf
var poBold []byte

//go:embed fonts/PixelOperator8.ttf
var po8Regular []byte

//go:embed fonts/PixelOperator8-Bold.ttf
var po8Bold []byte

// pixelDesign is one pixel font at its native size.
type pixelDesign struct {
	mu       sync.Mutex // the faces below aren't safe for concurrent use
	font     *sfnt.Font
	face     font.Face // at the native size, unhinted
	fallback font.Face // Inter, for characters the pixel font lacks
	bold     bool      // Unifont glyphs are thickened to match
	buf      sfnt.Buffer
	// line metrics in font pixels
	asc, desc, height, capH, xH int
}

func newDesign(ttf []byte, size float64, fallback []byte, asc, desc, height, capH, xH int) *pixelDesign {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(err)
	}
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	// Inter at the same cap height (Inter's caps are 0.727 em).
	fb := face(fallback, float64(capH)/0.727)
	return &pixelDesign{font: f, face: fc, fallback: fb, asc: asc, desc: desc, height: height, capH: capH, xH: xH}
}

// thick marks a bold design: Unifont, which has one weight, is thickened
// by a pixel to stand beside it.
func (d *pixelDesign) thick() *pixelDesign {
	d.bold = true
	return d
}

// pixelFace is a pixelDesign at a whole-number scale. It keeps the glyphs
// it has drawn: ASCII for good, without locking, as nearly every string
// is mostly ASCII; the rest in a cache of recent ones, so a library of
// Chinese titles doesn't keep thousands of them.
type pixelFace struct {
	d      *pixelDesign
	scale  int
	ascii  [128]atomic.Pointer[glyph]
	glyphs lru[rune, *glyph]
}

func newPixelFace(d *pixelDesign, scale int) *pixelFace {
	return &pixelFace{d: d, scale: scale, glyphs: lru[rune, *glyph]{size: 512}}
}

func (f *pixelFace) Close() error { return nil }

func (f *pixelFace) Metrics() font.Metrics {
	s := f.scale
	return font.Metrics{
		Height:     fixed.I(f.d.height * s),
		Ascent:     fixed.I(f.d.asc * s),
		Descent:    fixed.I(f.d.desc * s),
		XHeight:    fixed.I(f.d.xH * s),
		CapHeight:  fixed.I(f.d.capH * s),
		CaretSlope: image.Point{0, 1},
	}
}

func (f *pixelFace) Kern(r0, r1 rune) fixed.Int26_6 { return 0 }

func (f *pixelFace) glyphOf(r rune) *glyph {
	if r >= 0 && r < 128 {
		if g := f.ascii[r].Load(); g != nil {
			return g
		}
		g := f.d.render(r, f.scale)
		f.ascii[r].Store(g)
		return g
	}
	if g, ok := f.glyphs.get(r); ok {
		return g
	}
	g := f.d.render(r, f.scale)
	f.glyphs.put(r, g)
	return g
}

func (f *pixelFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	g := f.glyphOf(r)
	if !g.ok {
		return image.Rectangle{}, nil, image.Point{}, g.advance, g.advance > 0
	}
	p := image.Pt(dot.X.Round(), dot.Y.Round())
	return g.rect.Add(p), g.mask, image.Point{}, g.advance, true
}

// GlyphBounds and GlyphAdvance report ok = false, as the opentype faces
// do, for characters the face only stands in for, so callers can choose
// their own substitute.
func (f *pixelFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.glyphOf(r).bounds()
}

func (f *pixelFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	g := f.glyphOf(r)
	return g.advance, !g.missing
}

func (g *glyph) bounds() (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	b := fixed.Rectangle26_6{
		Min: fixed.Point26_6{X: fixed.I(g.rect.Min.X), Y: fixed.I(g.rect.Min.Y)},
		Max: fixed.Point26_6{X: fixed.I(g.rect.Max.X), Y: fixed.I(g.rect.Max.Y)},
	}
	return b, g.advance, !g.missing
}

// render draws one character at the native size, as a 1-bit mask, and
// scales it up. Characters come from Pixel Operator, then the glyphs drawn
// here, then a stand-in, then Inter, then Unifont.
func (d *pixelDesign) render(r rune, scale int) *glyph {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r < 0x20 || r == 0x7f {
		return &glyph{missing: true}
	}
	if invisible(r) {
		return &glyph{}
	}
	var mask *image.Alpha
	var adv int
	missing := false
	if gi, err := d.font.GlyphIndex(&d.buf, r); err == nil && gi != 0 {
		mask, adv = threshold(d.face, r)
	} else if bm, ok := extraGlyphs[r]; ok {
		mask, adv = bitmapGlyph(bm, d.capH)
	} else if m, ok := substitutes[r]; ok {
		d.mu.Unlock()
		g := *d.render(m, scale)
		d.mu.Lock()
		g.missing = true
		return &g
	} else if _, ok := d.fallback.GlyphAdvance(r); ok {
		mask, adv = threshold(d.fallback, r)
	} else if rows, w, mark, ok := unifontGlyph(r); ok {
		k := d.uniScale(scale)
		// raise the cell so its middle meets the middle of the capitals,
		// in whole art pixels
		raise := (d.capH*scale/2 - (unifontAscent-unifontRows/2)*k) / art * art
		return unifontGlyphAt(rows, w, mark, k, d.bold, max(0, raise))
	} else {
		mask, adv = threshold(d.face, r) // the pixel font's empty box
		missing = true
	}
	g := &glyph{advance: fixed.I(adv * scale), adv: adv * scale, missing: missing}
	if mask == nil || mask.Rect.Empty() {
		return g
	}
	// scale up, hard edges
	b := mask.Rect
	out := image.NewAlpha(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if mask.Pix[y*mask.Stride+x] == 0 {
				continue
			}
			for yy := 0; yy < scale; yy++ {
				row := out.Pix[(y*scale+yy)*out.Stride+x*scale:]
				for xx := 0; xx < scale; xx++ {
					row[xx] = 0xff
				}
			}
		}
	}
	g.mask = out
	g.rect = image.Rect(b.Min.X*scale, b.Min.Y*scale, b.Max.X*scale, b.Max.Y*scale)
	g.ok = true
	return g
}

// uniScale is the scale Unifont's 16-pixel glyphs are drawn at in a face
// at scale: one step smaller, so they fit the line (16 px in the 26-px
// lines of the 2x faces, 32 in the 39-px lines of the 3x ones).
func (d *pixelDesign) uniScale(scale int) int { return max(1, scale-1) }

// unifontGlyphAt makes a glyph of Unifont rows at scale k, standing on the
// baseline (raised by raise pixels), cropped to its ink. A combining mark takes no advance and
// keeps its cell width, to be centred on the character before it.
func unifontGlyphAt(rows []byte, w int, mark bool, k int, bold bool, raise int) *glyph {
	bw := w / 8
	bit := func(x, y int) bool { return rows[y*bw+x/8]&(0x80>>(x%8)) != 0 }
	on := func(x, y int) bool {
		return bit(x, y) || (bold && x > 0 && bit(x-1, y))
	}
	minX, minY, maxX, maxY := w, unifontRows, -1, -1
	for y := 0; y < unifontRows; y++ {
		for x := 0; x < w; x++ {
			if on(x, y) {
				minX, maxX = min(minX, x), max(maxX, x)
				minY, maxY = min(minY, y), max(maxY, y)
			}
		}
	}
	g := &glyph{advance: fixed.I(w * k), adv: w * k, mark: mark, cell: w * k}
	if mark {
		g.advance, g.adv = 0, 0
	}
	if maxX < 0 {
		return g
	}
	m := image.NewAlpha(image.Rect(0, 0, (maxX-minX+1)*k, (maxY-minY+1)*k))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if !on(x, y) {
				continue
			}
			for yy := 0; yy < k; yy++ {
				row := m.Pix[((y-minY)*k+yy)*m.Stride+(x-minX)*k:]
				for xx := 0; xx < k; xx++ {
					row[xx] = 0xff
				}
			}
		}
	}
	g.mask = m
	g.rect = image.Rect(minX*k, (minY-unifontAscent)*k-raise, (maxX+1)*k, (maxY+1-unifontAscent)*k-raise)
	g.ok = true
	return g
}

// invisible reports the characters drawn as nothing at all: format
// characters (joiners, directional marks, soft hyphens) and variation
// selectors, which Unifont would show as dashed boxes.
func invisible(r rune) bool {
	switch {
	case r < 0xad:
		return false
	case r >= 0xfe00 && r <= 0xfe0f, r >= 0xe0100 && r <= 0xe01ef, r == 0x34f:
		return true
	}
	return unicode.Is(unicode.Cf, r)
}

// isMark reports combining marks, which belong to the character before
// them: text is never cut between the two.
func isMark(r rune) bool {
	return r >= 0x300 && unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc)
}

// threshold rasterises r with f and keeps the pixels at least half covered.
// The mask's rectangle is relative to the dot (the baseline at y = 0).
func threshold(f font.Face, r rune) (*image.Alpha, int) {
	dr, m, mp, adv, ok := f.Glyph(fixed.P(0, 0), r)
	if !ok {
		return nil, adv.Round()
	}
	a := image.NewAlpha(dr)
	if m != nil && !dr.Empty() {
		draw.Draw(a, dr, m, mp, draw.Src)
		for i, v := range a.Pix {
			if v >= 0x80 {
				a.Pix[i] = 0xff
			} else {
				a.Pix[i] = 0
			}
		}
	}
	return a, adv.Round()
}

// bitmapGlyph turns rows of '#' into a mask standing on the baseline, its
// top at the cap height (taller art hangs below the baseline).
func bitmapGlyph(rows []string, capH int) (*image.Alpha, int) {
	h := len(rows)
	w := 0
	for _, r := range rows {
		w = max(w, len(r))
	}
	top := -min(capH, h)
	a := image.NewAlpha(image.Rect(1, top, 1+w, top+h))
	for y, row := range rows {
		for x, c := range row {
			if c == '#' {
				a.Pix[y*a.Stride+x] = 0xff
			}
		}
	}
	return a, w + 2
}

// substitutes stand in for characters neither font has.
var substitutes = map[rune]rune{
	'✔': '✓', '☑': '✓', '✅': '✓', '✖': '✗', '❌': '✗', '❎': '✗',
	'❤': '♥', '💖': '♥', '⬅': '←', '➡': '→', '⬆': '↑', '⬇': '↓',
	'❗': '!', '❕': '!', '⭐': '★', '🌟': '★', '👉': '→',
	' ': ' ', ' ': ' ', ' ': ' ',
}

// extraGlyphs are drawn here for characters Pixel Operator lacks, on the
// same pixel grid. Seven rows tall: the cap height of the small cut.
var extraGlyphs = map[rune][]string{
	'✓': rowsOf(`
.....#
....##
#..##.
##.#..
.###..
..#...
......`),
	'✗': rowsOf(`
#...#
##.##
.###.
..#..
.###.
##.##
#...#`),
	'←': rowsOf(`
..#....
.##....
#######
.##....
..#....
.......
.......`),
	'→': rowsOf(`
....#..
....##.
#######
....##.
....#..
.......
.......`),
	'↑': rowsOf(`
..#..
.###.
#####
..#..
..#..
..#..
..#..`),
	'↓': rowsOf(`
..#..
..#..
..#..
..#..
#####
.###.
..#..`),
	'♥': rowsOf(`
.##.##.
#######
#######
.#####.
..###..
...#...
.......`),
	'★': rowsOf(`
...#...
..###..
#######
.#####.
..###..
.##.##.
.#...#.`),
	'⚠': rowsOf(`
...#...
..#.#..
..#.#..
.#.#.#.
.#...#.
#..#..#
#######`),
}

func rowsOf(s string) []string { return strings.Split(strings.TrimSpace(s), "\n") }

// The designs and their line metrics (font pixels): the 16 px cut has caps
// 9 px tall and descenders 2 px deep; the 8 px cut, 7 and 1.
var (
	poDesign     = newDesign(poRegular, 16, interRegular, 10, 3, 13, 9, 7)
	poBoldDesign = newDesign(poBold, 16, interBold, 10, 3, 13, 9, 7).thick()
	po8Design    = newDesign(po8Regular, 8, interRegular, 8, 2, 10, 7, 5)
	po8BoldDes   = newDesign(po8Bold, 8, interBold, 8, 2, 10, 7, 5).thick()
)

// ---- small capitals ----

// microFace is a 3x5 pixel font of capitals, digits and punctuation (drawn
// here, part of minideck) at a whole-number scale: the Caps face, for short
// labels. Lowercase letters are drawn as capitals; characters it lacks come
// from fallback.
type microFace struct {
	scale    int
	fallback font.Face
	mu       sync.Mutex
	glyphs   map[rune]*glyph // only the micro font's own
}

func newMicroFace(scale int, fallback font.Face) *microFace {
	return &microFace{scale: scale, fallback: fallback, glyphs: map[rune]*glyph{}}
}

func (f *microFace) Close() error                   { return nil }
func (f *microFace) Kern(r0, r1 rune) fixed.Int26_6 { return 0 }

// Metrics: capitals 5 px tall, with 2 px above them and 2 below.
func (f *microFace) Metrics() font.Metrics {
	s := f.scale
	return font.Metrics{Height: fixed.I(9 * s), Ascent: fixed.I(7 * s), Descent: fixed.I(2 * s),
		XHeight: fixed.I(5 * s), CapHeight: fixed.I(5 * s), CaretSlope: image.Point{0, 1}}
}

func (f *microFace) glyphOf(r rune) *glyph {
	rows, ok := microFont[unicode.ToUpper(r)]
	if !ok {
		rows, ok = microFont[r]
	}
	if !ok {
		if gs, isPixel := f.fallback.(glyphSource); isPixel {
			return gs.glyphOf(r)
		}
		return foreignGlyph(f.fallback, r)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if g, ok := f.glyphs[r]; ok {
		return g
	}
	w := len(rows[0])
	k := f.scale
	g := &glyph{advance: fixed.I((w + 1) * k), adv: (w + 1) * k}
	g.mask = image.NewAlpha(image.Rect(0, 0, w*k, 5*k))
	for y, row := range rows {
		for x := 0; x < len(row); x++ {
			if row[x] != '#' {
				continue
			}
			for yy := 0; yy < k; yy++ {
				for xx := 0; xx < k; xx++ {
					g.mask.Pix[(y*k+yy)*g.mask.Stride+x*k+xx] = 0xff
				}
			}
		}
	}
	g.rect = image.Rect(0, -5*k, w*k, 0)
	g.ok = true
	f.glyphs[r] = g
	return g
}

// foreignGlyph copies a glyph from a face of another kind.
func foreignGlyph(fc font.Face, r rune) *glyph {
	g := &glyph{missing: r < 0x20}
	if r < 0x20 {
		return g
	}
	dr, m, mp, adv, ok := fc.Glyph(fixed.P(0, 0), r)
	_, has := fc.GlyphAdvance(r)
	g.advance, g.adv, g.missing = adv, adv.Round(), !has
	if ok && m != nil && !dr.Empty() {
		a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
		draw.Draw(a, a.Bounds(), m, mp, draw.Src)
		g.mask, g.rect, g.ok = a, dr, true
	}
	return g
}

func (f *microFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	g := f.glyphOf(r)
	if !g.ok {
		return image.Rectangle{}, nil, image.Point{}, g.advance, g.advance > 0
	}
	p := image.Pt(dot.X.Round(), dot.Y.Round())
	return g.rect.Add(p), g.mask, image.Point{}, g.advance, true
}

func (f *microFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.glyphOf(r).bounds()
}

func (f *microFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	g := f.glyphOf(r)
	return g.advance, !g.missing
}

// microFont is the small capitals: 3x5, with M, N, W and a few signs
// wider. Also drawn directly on button pills (see micro).
var microFont = map[rune][]string{
	'A': {".#.", "#.#", "###", "#.#", "#.#"}, 'B': {"##.", "#.#", "##.", "#.#", "##."},
	'C': {".##", "#..", "#..", "#..", ".##"}, 'D': {"##.", "#.#", "#.#", "#.#", "##."},
	'E': {"###", "#..", "##.", "#..", "###"}, 'F': {"###", "#..", "##.", "#..", "#.."},
	'G': {".##", "#..", "#.#", "#.#", ".##"}, 'H': {"#.#", "#.#", "###", "#.#", "#.#"},
	'I': {"###", ".#.", ".#.", ".#.", "###"}, 'J': {"..#", "..#", "..#", "#.#", ".#."},
	'K': {"#.#", "#.#", "##.", "#.#", "#.#"}, 'L': {"#..", "#..", "#..", "#..", "###"},
	'M': {"#...#", "##.##", "#.#.#", "#...#", "#...#"}, 'N': {"#..#", "##.#", "#.##", "#..#", "#..#"},
	'O': {".#.", "#.#", "#.#", "#.#", ".#."}, 'P': {"##.", "#.#", "##.", "#..", "#.."},
	'Q': {".#.", "#.#", "#.#", "#.#", ".##"}, 'R': {"##.", "#.#", "##.", "#.#", "#.#"},
	'S': {".##", "#..", ".#.", "..#", "##."}, 'T': {"###", ".#.", ".#.", ".#.", ".#."},
	'U': {"#.#", "#.#", "#.#", "#.#", "###"}, 'V': {"#.#", "#.#", "#.#", "#.#", ".#."},
	'W': {"#...#", "#...#", "#.#.#", "##.##", "#...#"}, 'X': {"#.#", "#.#", ".#.", "#.#", "#.#"},
	'Y': {"#.#", "#.#", ".#.", ".#.", ".#."}, 'Z': {"###", "..#", ".#.", "#..", "###"},
	'0': {"###", "#.#", "#.#", "#.#", "###"}, '1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"##.", "..#", ".#.", "#..", "###"}, '3': {"##.", "..#", ".#.", "..#", "##."},
	'4': {"#.#", "#.#", "###", "..#", "..#"}, '5': {"###", "#..", "##.", "..#", "##."},
	'6': {".##", "#..", "###", "#.#", "###"}, '7': {"###", "..#", ".#.", ".#.", ".#."},
	'8': {"###", "#.#", "###", "#.#", "###"}, '9': {"###", "#.#", "###", "..#", "##."},
	'/': {"..#", "..#", ".#.", "#..", "#.."}, '-': {"...", "...", "###", "...", "..."},
	'+': {"...", ".#.", "###", ".#.", "..."}, ' ': {"..", "..", "..", "..", ".."},
	'.': {".", ".", ".", ".", "#"}, ',': {".", ".", ".", "#", "#"},
	':': {".", "#", ".", "#", "."}, ';': {".", "#", ".", "#", "#"},
	'%': {"#.#", "..#", ".#.", "#..", "#.#"}, '(': {".#", "#.", "#.", "#.", ".#"},
	')': {"#.", ".#", ".#", ".#", "#."}, '\'': {"#", "#", ".", ".", "."},
	'"': {"#.#", "#.#", "...", "...", "..."}, '!': {"#", "#", "#", ".", "#"},
	'?': {"##.", "..#", ".#.", "...", ".#."}, '#': {"#.#", "###", "#.#", "###", "#.#"},
	'_': {"...", "...", "...", "...", "###"}, '·': {".", ".", "#", ".", "."},
	'=': {"...", "###", "...", "###", "..."}, '<': {"..#", ".#.", "#..", ".#.", "..#"},
	'>': {"#..", ".#.", "..#", ".#.", "#.."}, '*': {"...", "#.#", ".#.", "#.#", "..."},
	'°': {"##", "##", "..", "..", ".."}, '…': {".....", ".....", ".....", ".....", "#.#.#"},
	'–': {"...", "...", "###", "...", "..."}, '—': {"....", "....", "####", "....", "...."},
	'→': {"....", "..#.", "####", "..#.", "...."}, '←': {"....", ".#..", "####", ".#..", "...."},
	'&': {".#.", "#.#", ".#.", "#.#", ".##"}, '•': {"..", "##", "##", "..", ".."},
	'[': {"##", "#.", "#.", "#.", "##"}, ']': {"##", ".#", ".#", ".#", "##"},
}
