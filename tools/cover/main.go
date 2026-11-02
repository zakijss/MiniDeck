// cover draws a README cover (1280x640, GitHub's social preview size): a
// title and a line under it, an optional app icon beside the title, and one
// to three app screens under them, each with a label, on the apps'
// night-sky background, in the same pixel style. The screens come from the
// apps' screen tests (make screens), at half size, so every 2x2 art pixel
// stays one sharp pixel. Run it from the repository root:
//
//	go run ./tools/cover -title MiniTape -sub "Music for the Miyoo Mini Plus" \
//	    -icon icons/MiniTape.png -o cover.png now-playing.png:"Now playing" ...
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/zakijss/minideck/ui"
)

func main() {
	log.SetFlags(0)
	title := flag.String("title", "MiniDeck", "the title")
	sub := flag.String("sub", "Apps for the Miyoo Mini Plus on Onion OS", "the line under the title")
	icon := flag.String("icon", "", "an app icon (PNG) to draw beside the title")
	out := flag.String("o", "cover.png", "the PNG to write")
	flag.Parse()
	if n := flag.NArg(); n < 1 || n > 3 {
		log.Fatal("usage: cover [flags] PATH:LABEL [PATH:LABEL [PATH:LABEL]]")
	}
	save(*out, drawCover(*title, *sub, *icon, flag.Args()))
}

func save(path string, img image.Image) {
	out, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(out, img); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
}

const coverW, coverH = 1280, 640

// drawCover draws the title (with the icon left of it), the line under
// it, then the screenshots at half size in bevelled frames, with their
// labels under them.
func drawCover(title, sub, icon string, shots []string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, coverW, coverH))
	background(img)
	bold := loadFont("ui/fonts/PixelOperator-Bold.ttf")
	regular := loadFont("ui/fonts/PixelOperator.ttf")
	ts, ss := 7, 3
	shadow := color.RGBA{58, 61, 122, 255}
	tw, sw := font.MeasureString(bold, title).Ceil()*ts, font.MeasureString(regular, sub).Ceil()*ss
	top := 64 + 9*ts
	tx := (coverW - tw) / 2
	if icon != "" {
		ic := loadPNG(icon)
		const iconGap = 28
		tx = (coverW - tw - ic.Bounds().Dx() - iconGap) / 2
		iy := top - 9*ts + (9*ts-ic.Bounds().Dy())/2
		draw.Draw(img, ic.Bounds().Add(image.Pt(tx, iy)), ic, ic.Bounds().Min, draw.Over)
		tx += ic.Bounds().Dx() + iconGap
	}
	text(img, tx, top+ts, title, bold, ts, shadow)
	text(img, tx, top, title, bold, ts, ui.Text)
	text(img, (coverW-sw)/2, top+28+9*ss, sub, regular, ss, ui.Muted)

	const shotW, shotH, gap = ui.W / 2, ui.H / 2, 48
	x := (coverW - len(shots)*shotW - (len(shots)-1)*gap) / 2
	y := 300
	for _, a := range shots {
		path, label, _ := strings.Cut(a, ":")
		frame(img, image.Rect(x, y, x+shotW, y+shotH))
		half(img, loadPNG(path), x, y)
		lw := font.MeasureString(regular, label).Ceil() * 2
		text(img, x+(shotW-lw)/2, y+shotH+44, label, regular, 2, ui.Muted)
		x += shotW + gap
	}
	return img
}

// frame draws a bevelled frame around r, like the apps' panels: an ink
// outline, a light top and left, a dark bottom and right, and a hard shadow.
func frame(dst *image.RGBA, r image.Rectangle) {
	fill := func(r image.Rectangle, c color.RGBA) { draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src) }
	ink := color.RGBA{27, 29, 58, 255}
	fill(r.Inset(-8).Add(image.Pt(8, 8)), color.RGBA{6, 7, 16, 255}) // shadow
	fill(r.Inset(-8), ink)
	fill(image.Rect(r.Min.X-6, r.Min.Y-6, r.Max.X+6, r.Min.Y-2), color.RGBA{96, 101, 148, 255})
	fill(image.Rect(r.Min.X-6, r.Min.Y-6, r.Min.X-2, r.Max.Y+6), color.RGBA{96, 101, 148, 255})
	fill(image.Rect(r.Min.X-6, r.Max.Y+2, r.Max.X+6, r.Max.Y+6), color.RGBA{44, 47, 84, 255})
	fill(image.Rect(r.Max.X+2, r.Min.Y-2, r.Max.X+6, r.Max.Y+6), color.RGBA{44, 47, 84, 255})
}

// half draws src at half size with its top-left corner at (x, y), each
// output pixel the average of a 2x2 block.
func half(dst *image.RGBA, src image.Image, x, y int) {
	b := src.Bounds()
	for sy := 0; sy+1 < b.Dy(); sy += 2 {
		for sx := 0; sx+1 < b.Dx(); sx += 2 {
			var sum [3]uint32
			for _, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				r, g, bl, _ := src.At(b.Min.X+sx+d[0], b.Min.Y+sy+d[1]).RGBA()
				sum[0], sum[1], sum[2] = sum[0]+r>>8, sum[1]+g>>8, sum[2]+bl>>8
			}
			dst.SetRGBA(x+sx/2, y+sy/2, color.RGBA{uint8(sum[0] / 4), uint8(sum[1] / 4), uint8(sum[2] / 4), 255})
		}
	}
}

// background fills img with the apps' screen background: the top left as
// the apps draw it, mirrored to the right (and down, past the screen's
// height), so the parts meet without a seam.
func background(img *image.RGBA) {
	os.Setenv("MINIDECK_UI_NULL", "1")
	os.Setenv("MINIDECK_SKY", "0")
	s, err := ui.Open()
	if err != nil {
		log.Fatal(err)
	}
	s.Background()
	b := img.Bounds()
	for y := 0; y < b.Dy(); y++ {
		sy := y
		if y >= ui.H { // taller than the screen: mirrored again
			sy = 2*ui.H - 1 - y
		}
		for x := 0; x < b.Dx(); x++ {
			sx := x
			if x >= ui.W {
				sx = 2*ui.W - 1 - x
			}
			img.Set(x, y, s.At(sx, sy))
		}
	}
}

func loadPNG(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Fatalf("%s: %v", path, err)
	}
	return img
}

// loadFont opens a Pixel Operator cut at its native 16 px, where every
// pixel of a glyph is fully on or off.
func loadFont(path string) font.Face {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	f, err := opentype.Parse(b)
	if err != nil {
		log.Fatalf("%s: %v", path, err)
	}
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 16, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		log.Fatal(err)
	}
	return fc
}

// text draws s with its baseline at y, each font pixel k x k, with no
// smoothing.
func text(dst *image.RGBA, x, y int, s string, f font.Face, k int, c color.Color) {
	m := f.Metrics()
	w := font.MeasureString(f, s).Ceil()
	asc, desc := m.Ascent.Ceil(), m.Descent.Ceil()
	mask := image.NewAlpha(image.Rect(0, 0, w, asc+desc))
	d := &font.Drawer{Dst: mask, Src: image.Opaque, Face: f, Dot: fixed.P(0, asc)}
	d.DrawString(s)
	src := image.NewUniform(c)
	for my := 0; my < mask.Rect.Dy(); my++ {
		for mx := 0; mx < w; mx++ {
			if mask.AlphaAt(mx, my).A < 0x80 {
				continue
			}
			r := image.Rect(x+mx*k, y+(my-asc)*k, x+(mx+1)*k, y+(my-asc+1)*k)
			draw.Draw(dst, r, src, image.Point{}, draw.Src)
		}
	}
}
