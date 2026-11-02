package fb

import (
	"image"
	"image/color"
	"os"
	"syscall"
	"unsafe"
)

// Patch draws a small region straight into the framebuffer, on top of
// another program (the Onion menu). It writes every page so it survives the
// menu's page flips, and never pans the display. It fades in over a solid
// colour, normally the theme's own background (see SnapshotColor).
type Patch struct {
	R     image.Rectangle // in screen coordinates
	f     *os.File
	mem   []byte
	pages int
	rows  [][]byte // what should be on screen, framebuffer byte order
	snap  [][]byte // what it fades in over (SnapshotColor)
}

// OpenPatch maps the framebuffer for drawing in r (clipped to the screen).
func OpenPatch(r image.Rectangle) (*Patch, error) {
	r = r.Intersect(image.Rect(0, 0, W, H))
	f, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	var vi VarScreenInfo
	if err := Ioctl(f, IOGetVScreenInfo, unsafe.Pointer(&vi)); err != nil {
		f.Close()
		return nil, err
	}
	pages := max(1, int(vi.YresVirtual/H))
	mem, err := syscall.Mmap(int(f.Fd()), 0, W*H*4*pages, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, err
	}
	p := &Patch{R: r, f: f, mem: mem, pages: pages}
	p.rows = make([][]byte, r.Dy())
	for y := range p.rows {
		p.rows[y] = make([]byte, r.Dx()*4)
	}
	return p, nil
}

// The panel is upside down: screen (x, y) is framebuffer (W-1-x, H-1-y), so
// screen row y of R is one run of framebuffer row H-1-y, reversed.
func (p *Patch) rowAt(page, y int) []byte {
	start := page*W*H*4 + (H-1-y)*W*4 + (W-p.R.Max.X)*4
	return p.mem[start : start+p.R.Dx()*4]
}

func (p *Patch) shownPage() int {
	var vi VarScreenInfo
	if Ioctl(p.f, IOGetVScreenInfo, unsafe.Pointer(&vi)) != nil {
		return 0
	}
	return min(p.pages-1, int(vi.Yoffset/H))
}

// SnapshotColor sets what the patch fades in over, and what Background
// returns, to a solid colour. Reading the screen under the patch instead
// could pick up the patch's own drawing, left there by an earlier run.
func (p *Patch) SnapshotColor(c color.RGBA) {
	p.snap = make([][]byte, p.R.Dy())
	for y := range p.snap {
		row := make([]byte, p.R.Dx()*4)
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2], row[i+3] = c.B, c.G, c.R, 0xff
		}
		p.snap[y] = row
	}
}

// ScreenPixel reads the colour at screen (x, y) from the page on screen.
func (p *Patch) ScreenPixel(x, y int) color.RGBA {
	i := p.shownPage()*W*H*4 + (H-1-y)*W*4 + (W-1-x)*4
	return color.RGBA{p.mem[i+2], p.mem[i+1], p.mem[i], 0xff}
}

// Background returns the snapshot as an image the size of R (black if there
// is none), to draw on.
func (p *Patch) Background() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, p.R.Dx(), p.R.Dy()))
	for y := 0; y < p.R.Dy(); y++ {
		for x := 0; x < p.R.Dx(); x++ {
			di := img.PixOffset(x, y)
			img.Pix[di+3] = 255
			if p.snap == nil {
				continue
			}
			si := (p.R.Dx() - 1 - x) * 4
			img.Pix[di], img.Pix[di+1], img.Pix[di+2] = p.snap[y][si+2], p.snap[y][si+1], p.snap[y][si]
		}
	}
	return img
}

// Set shows img (the size of R, origin 0,0) at opacity alpha (0..1) over
// the snapshot, on every page.
func (p *Patch) Set(img *image.RGBA, alpha float64) {
	a := uint32(max(0, min(1, alpha)) * 256)
	for y := 0; y < p.R.Dy(); y++ {
		row := p.rows[y]
		for x := 0; x < p.R.Dx(); x++ {
			si := img.PixOffset(x, y)
			di := (p.R.Dx() - 1 - x) * 4
			b, g, r := uint32(img.Pix[si+2]), uint32(img.Pix[si+1]), uint32(img.Pix[si])
			if a < 256 && p.snap != nil {
				s := p.snap[y]
				b = (b*a + uint32(s[di])*(256-a)) >> 8
				g = (g*a + uint32(s[di+1])*(256-a)) >> 8
				r = (r*a + uint32(s[di+2])*(256-a)) >> 8
			}
			row[di], row[di+1], row[di+2], row[di+3] = byte(b), byte(g), byte(r), 0xff
		}
	}
	for pg := 0; pg < p.pages; pg++ {
		p.put(pg)
	}
}

// put writes the patch to framebuffer page pg.
func (p *Patch) put(pg int) {
	for y := range p.rows {
		copy(p.rowAt(pg, p.R.Min.Y+y), p.rows[y])
	}
}

// Guard puts the patch back on any page where the menu has drawn over it.
// It only compares a few pixels, so it can run every few milliseconds.
func (p *Patch) Guard() {
	for pg := 0; pg < p.pages; pg++ {
		for _, y := range [...]int{1, p.R.Dy() / 2, p.R.Dy() - 2} {
			if y < 0 || y >= p.R.Dy() {
				continue
			}
			got, want := p.rowAt(pg, p.R.Min.Y+y), p.rows[y]
			for i := 0; i < len(want); i += 4 * 7 {
				if got[i] != want[i] || got[i+1] != want[i+1] || got[i+2] != want[i+2] {
					p.put(pg)
					goto next
				}
			}
		}
	next:
	}
}

// Close unmaps the framebuffer. What the patch drew stays on screen.
func (p *Patch) Close() {
	syscall.Munmap(p.mem)
	p.f.Close()
}
