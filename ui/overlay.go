package ui

import (
	"encoding/json"
	"image"
	"os"
	"time"

	"github.com/zakijss/minideck/fb"
)

// Now is what's playing, as MiniTape's background player publishes it in
// /tmp/minideck-now.json for other apps' tickers.
type Now struct {
	Text   string
	Paused bool
}

// ReadNow returns the ticker text for what's playing, or "".
func ReadNow() string {
	b, err := os.ReadFile("/tmp/minideck-now.json")
	if err != nil {
		return ""
	}
	var n Now
	if json.Unmarshal(b, &n) != nil || n.Paused {
		return ""
	}
	return n.Text
}

// OverlayH is the height of the slim strip drawn over the Onion menu: it
// sits above the theme's title instead of cutting through it. OverlayW ends
// it before the status icons at the top right (MiniScale's badge, then the
// theme's Wi-Fi and battery), so the text fades out before reaching them.
const (
	OverlayH = 20
	OverlayW = 460
)

// The strip waits for the status icons, then fades in.
const (
	overlayDelay = 400 * time.Millisecond
	overlayFade  = 500 * time.Millisecond
)

// Overlay draws the now-playing strip over the Onion menu (see Patch).
type Overlay struct {
	p     *fb.Patch
	s     *Screen
	bg    *image.RGBA
	start time.Time
}

// OpenOverlay maps the framebuffer for the strip.
func OpenOverlay() (*Overlay, error) {
	p, err := fb.OpenPatch(image.Rect(0, 0, OverlayW, OverlayH))
	if err != nil {
		return nil, err
	}
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, OverlayH))}
	bg := image.NewRGBA(image.Rect(0, 0, W, H))
	for i := 3; i < len(bg.Pix); i += 4 {
		bg.Pix[i] = 255
	}
	return &Overlay{p: p, s: s, bg: bg}, nil
}

// Begin is called when the strip is about to appear over the menu: it
// remembers what's under it and restarts the fade-in.
func (o *Overlay) Begin() {
	// Draw on the theme's top-bar colour, read from the top right corner,
	// which nothing of ours covers. (Reading what's under the strip itself
	// can pick up a strip left over from last time.)
	c := o.p.ScreenPixel(W-4, 3)
	o.p.SnapshotColor(c)
	for y := 0; y < OverlayH; y++ {
		for x := 0; x < OverlayW; x++ {
			i := o.bg.PixOffset(x, y)
			o.bg.Pix[i], o.bg.Pix[i+1], o.bg.Pix[i+2] = c.R, c.G, c.B
		}
	}
	o.start = time.Now()
}

// Draw renders the strip with text and writes it to every framebuffer page.
func (o *Overlay) Draw(text string) {
	t := time.Since(o.start) - overlayDelay
	if t < 0 {
		return
	}
	s := o.s
	for i := range s.Pix {
		s.Pix[i] = 0
		if i%4 == 3 {
			s.Pix[i] = 255
		}
	}
	s.bgID = o.bg
	s.ticker = text // DrawTicker splits it again only when it changes
	s.tickerCompact = true
	s.tickerW = OverlayW
	s.DrawTicker()
	o.p.Set(s.RGBA, float64(t)/float64(overlayFade))
}

// Guard puts the strip back where the menu has drawn over it.
func (o *Overlay) Guard() {
	if time.Since(o.start) >= overlayDelay {
		o.p.Guard()
	}
}
