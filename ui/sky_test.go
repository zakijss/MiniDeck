package ui

import (
	"bytes"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
	"time"
	"unsafe"
)

// fakeScreen is a Screen with in-memory framebuffer pages showing the
// plain background.
func fakeScreen(pages int) *Screen {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H)), pages: pages, mem: make([]byte, W*H*4*pages)}
	s.shadow = make([][]byte, pages)
	for p := range pages {
		s.shadow[p] = append([]byte(nil), ambient().Pix...)
		copyRotated(s.mem[p*W*H*4:(p+1)*W*H*4], s.shadow[p])
	}
	return s
}

func newTestSky(s *Screen, seed uint64) *sky {
	return newSky(s, sceneByID("stars"), rand.New(rand.NewPCG(seed, 7)))
}

// fbAt reads screen pixel (x, y) of page p back as RGB.
func fbAt(s *Screen, p, x, y int) [3]byte {
	i := p*W*H*4 + ((H-1-y)*W+(W-1-x))*4 // BGRA, upside down
	m := s.mem
	return [3]byte{m[i+2], m[i+1], m[i]}
}

func TestSkyAnimatesOnlyTheBackground(t *testing.T) {
	s := fakeScreen(2)
	// a panel covers the middle of page 0 only
	panel := image.Rect(100, 100, 400, 300)
	for y := panel.Min.Y; y < panel.Max.Y; y++ {
		for x := panel.Min.X; x < panel.Max.X; x++ {
			i := (y*W + x) * 4
			copy(s.shadow[0][i:i+4], []byte{40, 43, 78, 255})
		}
	}
	copyRotated(s.mem[:W*H*4], s.shadow[0])

	k := newTestSky(s, 1)
	now := time.Now()
	k.next = now // a comet at once
	changed := false
	for i := range 400 {
		k.spawn(now.Add(time.Duration(i) * skyTick))
		k.apply(k.frame())
		// the panel on page 0 is never touched
		for _, pt := range []image.Point{panel.Min, {250, 200}, panel.Max.Sub(image.Pt(1, 1))} {
			if got := fbAt(s, 0, pt.X, pt.Y); got != [3]byte{40, 43, 78} {
				t.Fatalf("tick %d: panel pixel %v became %v", i, pt, got)
			}
		}
		if len(k.drawn) > 0 {
			changed = true
		}
	}
	if !changed {
		t.Fatal("the sky never moved")
	}
	// once every sprite has finished, both pages show the plain background
	// again, byte for byte
	k.sprites = nil
	k.apply(k.frame())
	for p := range 2 {
		want := make([]byte, W*H*4)
		copyRotated(want, s.shadow[p])
		if !bytes.Equal(s.mem[p*W*H*4:(p+1)*W*H*4], want) {
			t.Errorf("page %d not restored", p)
		}
	}
}

func TestSkySpritesFinish(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for range 200 {
		for _, sp := range []sprite{newComet(r), newPop(r), newTwinkle(r, 10, 10)} {
			n := 0
			for sp.step(r, 1) {
				sp.paint(func(int, int, color.RGBA, float64) {})
				if n++; n > 2000 {
					t.Fatalf("%T never finishes", sp)
				}
			}
		}
	}
}

func TestReapplyAfterPresent(t *testing.T) {
	s := fakeScreen(2)
	k := newTestSky(s, 9)
	k.sprites = []sprite{&pop{ax: 20, ay: 20, n: 10, c: rgb3(255, 255, 255), size: 2}}
	k.apply(k.frame())
	want := fbAt(s, 1, 40, 40)
	if want == fbAt(s, 1, 0, 0) {
		t.Fatal("the pop didn't draw")
	}
	// Present copies a fresh frame (the plain background) to page 1 ...
	copyRotated(s.mem[W*H*4:], s.shadow[1])
	// ... and puts the sky back
	k.reapply(1)
	if got := fbAt(s, 1, 40, 40); got != want {
		t.Errorf("after reapply %v, want %v", got, want)
	}
}

func BenchmarkSkyTick(b *testing.B) {
	s := fakeScreen(2)
	k := newTestSky(s, 5)
	now := time.Now()
	for i := range b.N {
		k.spawn(now.Add(time.Duration(i) * skyTick))
		k.apply(k.frame())
	}
}

func TestSkyKeepsAwayFromText(t *testing.T) {
	s := fakeScreen(1)
	// a 2x2 block of text at an odd position (101-102, 100-101), the
	// smallest thing the UI draws, with background all around it
	for _, p := range [][2]int{{101, 100}, {102, 100}, {101, 101}, {102, 101}} {
		i := (p[1]*W + p[0]) * 4
		copy(s.shadow[0][i:i+4], []byte{240, 241, 250, 255})
	}
	bg := unsafe.Slice((*uint32)(unsafe.Pointer(&ambient().Pix[0])), W*H)
	fr := unsafe.Slice((*uint32)(unsafe.Pointer(&s.shadow[0][0])), W*H)
	// it covers the top-left pixel of cell (51, 50)
	for _, c := range [][2]int{{51, 50}, {51 + skyMargin, 50}, {51, 50 - skyMargin}, {51 - skyMargin, 50 + skyMargin}} {
		if clearAround(fr, bg, c[0], c[1]) {
			t.Errorf("cell %v counted as clear, within %d cells of the text", c, skyMargin)
		}
	}
	for _, c := range [][2]int{{51 + skyMargin + 1, 50}, {51, 50 + skyMargin + 1}} {
		if !clearAround(fr, bg, c[0], c[1]) {
			t.Errorf("cell %v, beyond the margin, isn't clear", c)
		}
	}
}

// BenchmarkSceneTick is one tick of each moving background once it has
// filled up (after 30 s of ticks), on a screen with nothing drawn on it,
// so every sprite cell is written.
func BenchmarkSceneTick(b *testing.B) {
	for _, sc := range Scenes {
		if sc.spawn == nil {
			continue
		}
		b.Run(sc.ID, func(b *testing.B) {
			withAppearance(b, Appearance{Background: sc.ID})
			s := fakeScreen(2)
			k := newSky(s, sc, rand.New(rand.NewPCG(5, 5)))
			now := time.Now()
			k.next = now
			tick := 0
			for ; tick < 450; tick++ {
				k.spawn(now.Add(time.Duration(tick) * skyTick))
				k.apply(k.frame())
			}
			b.ResetTimer()
			for range b.N {
				k.spawn(now.Add(time.Duration(tick) * skyTick))
				k.apply(k.frame())
				tick++
			}
		})
	}
}
