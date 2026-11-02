package ui

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/font"
)

func TestCopyRotated(t *testing.T) {
	src := make([]byte, W*H*4)
	for i := range src {
		src[i] = byte(i*7 + i/4)
	}
	got := make([]byte, W*H*4)
	copyRotated(got, src)
	n := W * H
	for i := 0; i < n; i++ { // checked pixel by pixel
		si, di := i*4, (n-1-i)*4
		if got[di] != src[si+2] || got[di+1] != src[si+1] || got[di+2] != src[si] || got[di+3] != 0xff {
			t.Fatalf("pixel %d: got %v, want %v", i, got[di:di+4], []byte{src[si+2], src[si+1], src[si], 0xff})
		}
	}
}

func BenchmarkCopyRotated(b *testing.B) {
	src, dst := make([]byte, W*H*4), make([]byte, W*H*4)
	for i := 0; i < b.N; i++ {
		copyRotated(dst, src)
	}
}

func newTestScreen() *Screen {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	for i := range s.Pix {
		s.Pix[i] = byte(i * 13)
	}
	return s
}

// Outline must paint exactly the stepped shape minus the shape shrunk by
// the outline's width, and leave everything else alone.
func TestOutlineIsShapeMinusInside(t *testing.T) {
	for _, tc := range []struct {
		r      image.Rectangle
		rad, w int
	}{{image.Rect(20, 60, 314, 232), 18, 3}, {image.Rect(0, 0, 40, 30), 12, 2}, {image.Rect(5, 5, 200, 50), 22, 1}, {image.Rect(600, 400, 700, 500), 8, 4}, {image.Rect(11, 13, 60, 41), 30, 2}} {
		a, b := newTestScreen(), newTestScreen()
		a.Outline(tc.r, tc.rad, tc.w, Accent)
		outer := rectShape(tc.r, tc.rad)
		inner := outer
		for i := 0; i < outlineArt(tc.w); i++ {
			inner = inner.erode()
		}
		keep := newTestScreen()
		b.fillShape(outer, Accent)
		// put back what lies inside the inner shape
		for i, sp := range inner.rows {
			if sp.empty() {
				continue
			}
			y0, y1 := inner.ys(i)
			x0, x1 := inner.xs(sp)
			for y := max(0, y0); y < min(H, y1); y++ {
				for x := max(0, x0); x < min(W, x1); x++ {
					o := keep.PixOffset(x, y)
					copy(b.Pix[o:o+4], keep.Pix[o:o+4])
				}
			}
		}
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Errorf("Outline(%v, %d, %d) differs from the shape minus its inside", tc.r, tc.rad, tc.w)
		}
	}
}

// Stepped shapes are symmetric: a corner cut at the top left is cut the
// same at the other three corners.
func TestRectShapeSymmetric(t *testing.T) {
	for _, rad := range []int{0, 6, 12, 18, 26, 40} {
		sh := rectShape(image.Rect(0, 0, 120, 60), rad)
		cols := 60
		for i, sp := range sh.rows {
			j := len(sh.rows) - 1 - i
			if sp != sh.rows[j] || sp.l != cols-sp.r {
				t.Fatalf("rad %d: row %d %v, mirrored row %v", rad, i, sp, sh.rows[j])
			}
		}
	}
}

func TestDrawCachedMatchesBlend(t *testing.T) {
	p := image.NewRGBA(image.Rect(0, 0, 120, 60))
	for i := range p.Pix {
		p.Pix[i] = byte(i * 31)
	}
	for y := 0; y < 60; y++ { // mostly opaque, some clear, some partial
		for x := 0; x < 120; x++ {
			a := byte(255)
			switch {
			case x < 3 || y < 2:
				a = byte(x*40 + y*20)
			case x > 115:
				a = 0
			}
			p.Pix[p.PixOffset(x, y)+3] = a
		}
	}
	for _, at := range []image.Point{{10, 20}, {-30, -10}, {580, 450}} {
		a, b := newTestScreen(), newTestScreen()
		a.drawCached(p, at)
		for y := 0; y < 60; y++ { // the same picture blended pixel by pixel
			for x := 0; x < 120; x++ {
				i := p.PixOffset(x, y)
				if p.Pix[i+3] != 0 {
					b.blend(at.X+x, at.Y+y, color.RGBA{p.Pix[i], p.Pix[i+1], p.Pix[i+2], 0xff}, float64(p.Pix[i+3])/255)
				}
			}
		}
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Errorf("drawCached at %v differs from blending", at)
		}
	}
}

func BenchmarkSelectedCard(b *testing.B) {
	s := newTestScreen()
	r := image.Rect(20, 60, 314, 232)
	for i := 0; i < b.N; i++ {
		s.Outline(r, 18, 3, Accent)
	}
}

// A panel is ink on its outer ring, light on its top edge, dark on its
// bottom edge, and leaves the screen outside it (above and left of the
// shadow) alone.
func TestPanelLayers(t *testing.T) {
	s := newTestScreen()
	before := newTestScreen()
	r := image.Rect(20, 60, 314, 232)
	s.Panel(r, 18)
	c := panelRamp(0.10)
	at := func(x, y int) color.RGBA {
		i := s.PixOffset(x, y)
		return color.RGBA{s.Pix[i], s.Pix[i+1], s.Pix[i+2], 255}
	}
	if got := at(100, 60); got != Ink {
		t.Errorf("top border %v, want ink", got)
	}
	if got := at(100, 62); got != c.hi {
		t.Errorf("top edge %v, want %v", got, c.hi)
	}
	if got := at(100, 228); got != c.lo {
		t.Errorf("bottom edge %v, want %v", got, c.lo)
	}
	if got := at(100, 150); got != c.base {
		t.Errorf("inside %v, want %v", got, c.base)
	}
	for _, p := range []image.Point{{20, 60}, {21, 61}, {19, 100}, {100, 59}} { // cut corner, outside
		i := s.PixOffset(p.X, p.Y)
		if !bytes.Equal(s.Pix[i:i+4], before.Pix[i:i+4]) {
			t.Errorf("pixel %v outside the panel changed", p)
		}
	}
}

// The background is made of 2x2 blocks of its own few colours.
func TestBackgroundBlocks(t *testing.T) {
	img := ambient()
	allowed := map[color.RGBA]bool{starColors[0]: true, starColors[1]: true}
	for _, c := range bgLevels {
		allowed[c] = true
	}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			i := img.PixOffset(x, y)
			c := color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
			if !allowed[c] {
				t.Fatalf("(%d,%d) is %v, not a background colour", x, y, c)
			}
			j := img.PixOffset(x&^1, y&^1)
			if !bytes.Equal(img.Pix[i:i+4], img.Pix[j:j+4]) {
				t.Fatalf("(%d,%d) differs from its 2x2 block", x, y)
			}
		}
	}
}

func BenchmarkAmbient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		resetAmbient()
		ambient()
	}
}

// Text is drawn without smoothing: only the text colour or what was there.
func TestTextCrisp(t *testing.T) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	s.Fill(s.Rect, BG)
	for i, f := range []font.Face{Body, Heading, Small, Caps, Title} {
		s.Text(10, 10+i*40, "Agj Wm 1@ · é — … ✓ Привет", f, Text)
	}
	for i := 0; i < len(s.Pix); i += 4 {
		c := color.RGBA{s.Pix[i], s.Pix[i+1], s.Pix[i+2], 255}
		if c != BG && c != Text {
			t.Fatalf("pixel %d is %v: smoothed text", i/4, c)
		}
	}
	if Measure(Body, "Hello") != 2*Measure(newPixelFace(poDesign, 1), "Hello") {
		t.Error("2x text isn't twice as wide as 1x")
	}
}

func TestCopyChangedRows(t *testing.T) {
	a := make([]byte, W*H*4)
	for i := range a {
		a[i] = byte(i * 7)
	}
	full := make([]byte, W*H*4)
	copyRotated(full, a)
	// a second frame with a few changed rows
	b := append([]byte(nil), a...)
	for _, y := range []int{0, 17, 240, 479} {
		for x := 0; x < W*4; x++ {
			b[y*W*4+x] ^= 0x5a
		}
	}
	want := make([]byte, W*H*4)
	copyRotated(want, b)
	got := append([]byte(nil), full...)
	shadow := append([]byte(nil), a...)
	if n := copyChangedRows(got, b, shadow); n != 4 {
		t.Fatalf("copied %d rows, want 4", n)
	}
	if !bytes.Equal(got, want) || !bytes.Equal(shadow, b) {
		t.Fatal("changed-rows copy differs from a full copy")
	}
}

func BenchmarkPanel(b *testing.B) {
	s := newTestScreen()
	for i := 0; i < b.N; i++ {
		s.Panel(image.Rect(20, 78, 614, 422), 18)
	}
}
