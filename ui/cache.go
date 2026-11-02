package ui

import (
	"image"
	"image/color"
	"image/draw"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
)

// glyph is one character drawn by a pixel face, kept for reuse.
type glyph struct {
	mask    *image.Alpha
	rect    image.Rectangle // relative to the dot
	advance fixed.Int26_6
	adv     int  // advance in whole pixels
	ok      bool // there is something to draw
	missing bool // drawn as a stand-in: the face doesn't have this character
	// mark: a combining mark, drawn over the character before it, its
	// cell (cell pixels wide) centred on that character's advance.
	mark bool
	cell int
}

// pictures caches images scaled to a size at art-pixel resolution, with
// stepped corners (clear in the alpha channel), so cover art isn't rescaled
// every frame.
var pictures = struct {
	sync.Mutex
	m map[picKey]*image.RGBA
}{m: map[picKey]*image.RGBA{}}

type picKey struct {
	img    image.Image
	w, h   int
	radius int
}

// ScaleCrop scales img to fill w x h with a Catmull-Rom filter, cutting
// the longer side equally so the picture keeps its proportions. Cover art
// and screenshots are photos: a sharp, smooth scale matters more for them
// than speed, and callers cache the result.
func ScaleCrop(img image.Image, w, h int) *image.RGBA {
	b := img.Bounds()
	src := b
	switch {
	case b.Dx()*h > b.Dy()*w: // wider than the target: crop the sides
		cw := b.Dy() * w / h
		src.Min.X = b.Min.X + (b.Dx()-cw)/2
		src.Max.X = src.Min.X + cw
	case b.Dx()*h < b.Dy()*w: // taller: crop top and bottom
		ch := b.Dx() * h / w
		src.Min.Y = b.Min.Y + (b.Dy()-ch)/2
		src.Max.Y = src.Min.Y + ch
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	if src.Dx() == w && src.Dy() == h {
		draw.Draw(out, out.Bounds(), img, src.Min, draw.Src)
		return out
	}
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, src, draw.Src, nil)
	return out
}

// scaledRound scales img to w x h at full resolution (pictures are the one
// thing not drawn in 2x2 blocks: cover art in blocks looked blurred), with
// corners stepped in art pixels to match the frames around them.
func scaledRound(img image.Image, w, h, rad int) *image.RGBA {
	k := picKey{img, w, h, rad}
	pictures.Lock()
	defer pictures.Unlock()
	if p, ok := pictures.m[k]; ok {
		return p
	}
	if len(pictures.m) > 96 {
		pictures.m = map[picKey]*image.RGBA{}
	}
	p := ScaleCrop(img, w, h)
	sh := rectShape(p.Bounds(), rad)
	for y := 0; y < h; y++ {
		sp := sh.rows[y/art]
		row := p.Pix[y*p.Stride : y*p.Stride+w*4]
		for x := 0; x < w; x++ {
			if x/art < sp.l || x/art >= sp.r {
				row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = 0, 0, 0, 0 // outside the corner
			} else {
				row[x*4+3] = 255
			}
		}
	}
	pictures.m[k] = p
	return p
}

// drawCached blends a cached picture (alpha = coverage) at r.Min.
func (s *Screen) drawCached(p *image.RGBA, at image.Point) {
	b := p.Bounds()
	dst := image.Rectangle{Min: at, Max: at.Add(b.Size())}.Intersect(s.Rect)
	for y := dst.Min.Y; y < dst.Max.Y; y++ {
		src := p.Pix[(y-at.Y)*p.Stride:]
		for x := dst.Min.X; x < dst.Max.X; {
			si := (x - at.X) * 4
			a := src[si+3]
			switch a {
			case 0:
				x++
			case 255: // opaque: copy the whole run at once
				e := x + 1
				for e < dst.Max.X && src[(e-at.X)*4+3] == 255 {
					e++
				}
				di := s.PixOffset(x, y)
				copy(s.Pix[di:di+(e-x)*4], src[si:si+(e-x)*4])
				x = e
			default:
				s.blend(x, y, color.RGBA{src[si], src[si+1], src[si+2], 0xff}, float64(a)/255)
				x++
			}
		}
	}
}
