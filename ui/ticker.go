package ui

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"time"
)

// TickerH is the height of the now-playing strip.
const TickerH = 30

// tickerSep is what goes between the title and the rest of the ticker's
// text (MiniTape's form: "Title   ·   Artist   ·   Album").
const tickerSep = "   ·   "

// tickerGap is what goes between one turn of the text and the next.
const tickerGap = "        •        "

// SetTicker sets the now-playing text scrolled across the top of every
// screen ("" hides it). Screens then start TickerH lower. The text before
// the first "   ·   " is the title, drawn brighter than the rest.
func (s *Screen) SetTicker(text string) { s.ticker = text }

// SetNowPlaying sets the ticker to a title and, after it, a detail such as
// the artist and album, drawn dimmer. It is SetTicker with the two
// joined, for a title that might itself hold the separator.
func (s *Screen) SetNowPlaying(title, detail string) {
	text := title
	if detail != "" {
		text += tickerSep + detail
	}
	s.ticker = text
	s.tick.split(text, title, detail)
}

// tickerText is the ticker's text cut into its parts and measured, once
// for each text, so drawing a frame allocates nothing.
type tickerText struct {
	of    string    // the text these parts are for
	parts [3]string // the title, the rest (with its separator) and the gap, laid out
	w     [3]int    // their widths in Small
	unit  int       // the three together: one turn of the loop
	sub   *image.RGBA
	// the pill as last drawn, and what it was drawn on
	pill   *image.RGBA
	pillOn *image.RGBA
	pillBG color.RGBA
	clock  string // the time at the strip's end (tickerClock) ...
	min    int64  // ... and the minute it was made for
}

// split sets the parts for text, from title and detail.
func (t *tickerText) split(text, title, detail string) {
	t.of = text
	t.parts = [3]string{visual(title), "", visual(tickerGap)}
	if detail != "" {
		t.parts[1] = visual(tickerSep + detail)
	}
	t.unit = 0
	for i, p := range t.parts {
		t.w[i] = Measure(Small, p)
		t.unit += t.w[i]
	}
}

// parts returns the ticker's parts, working them out if the text changed.
func (s *Screen) tickerParts() *tickerText {
	if s.tick.of != s.ticker || s.tick.unit == 0 {
		title, detail, _ := strings.Cut(s.ticker, tickerSep)
		s.tick.split(s.ticker, title, detail)
	}
	return &s.tick
}

func (s *Screen) inset() int {
	if s.ticker == "" {
		return 0
	}
	return TickerH
}

// The strip scrolls one art pixel (2 px) every TickerFrame: 40 px a second,
// the same distance every frame, so it glides instead of stepping 2 px one
// frame and 4 px the next. Its position comes from the clock, not a frame
// count, so it doesn't jump when the whole screen is redrawn in between,
// and a frame drawn up to half a frame late still lands on its own step.
// Apps time their ticker redraws with TickerClock.
const TickerFrame = 50 * time.Millisecond

var tickerStart = time.Now()

// tickerStep is the strip's position at t, in art pixels.
func tickerStep(t time.Time) int {
	return int((t.Sub(tickerStart) + TickerFrame/2) / TickerFrame)
}

// TickerWait is how long until the strip's next step is due.
func TickerWait() time.Duration {
	return TickerFrame - time.Since(tickerStart)%TickerFrame
}

// TickerClock returns a time.Ticker that fires as each step of the strip
// falls due (it waits for the first one, at most TickerFrame). Stop it when
// done.
func TickerClock() *time.Ticker {
	time.Sleep(TickerWait())
	return time.NewTicker(TickerFrame)
}

// DrawTicker draws (or redraws) only the strip, so an app can animate it
// without redrawing the rest of the screen. The text always scrolls, like
// a radio display, after a small equaliser that moves with it.
func (s *Screen) DrawTicker() {
	if s.ticker == "" {
		return
	}
	t := s.tickerParts()
	h, f := TickerH, Small
	if s.tickerCompact {
		h = 20
	}
	width := W
	if s.tickerW > 0 {
		width = s.tickerW
	}
	strip := image.Rect(0, 0, width, h)
	if s.bgID != nil {
		draw.Draw(s.RGBA, strip, s.bgID, image.Point{}, draw.Src)
	} else {
		s.Fill(strip, BG)
	}
	now := time.Now()
	step := tickerStep(now)
	pill := image.Rect(8, 3, width-8, h-1)
	if s.tickerCompact {
		pill = image.Rect(0, 0, width, h)
	}
	if s.tickerClock {
		_, _, loc := readStatus()
		if m := now.Unix() / 60; m != t.min || t.clock == "" {
			t.clock, t.min = now.In(loc).Format("15:04"), m
		}
		s.TextRight(width-12, (h-LineHeight(Caps))/2, t.clock, Caps, Text)
		pill.Max.X = width - 22 - Measure(Caps, t.clock)
	}
	title, rest, gap, eq := Text, Muted, Accent, accentRamp
	if s.tickerCompact {
		// a slim band with a line under it and at its end, on whatever
		// colour the menu's bar is
		s.blendRect(pill, Text, 26)
		s.blendRect(image.Rect(0, h-art, width, h), Text, 46)
		s.blendRect(image.Rect(width-art, 0, width, h-art), Text, 46)
		gap, eq = Muted, whiteRamp
	} else {
		s.tickerPill(pill)
	}
	ex := pill.Min.X + 6
	if !s.tickerCompact {
		ex = pill.Min.X + 12
	}
	s.equaliser(ex, (pill.Min.Y+pill.Max.Y)/2+5*art/2, step, eq)

	// The text, clipped, scrolling left and looping.
	clip := image.Rect(ex+eqWidth*art+8, pill.Min.Y+art, pill.Max.X-12, pill.Max.Y-art)
	if s.tickerClean == nil || s.tickerClean.Rect != clip {
		s.tickerClean = image.NewRGBA(clip)
		t.sub = nil
	}
	if t.sub == nil || t.sub.Rect != clip {
		t.sub = s.RGBA.SubImage(clip).(*image.RGBA)
	}
	clean := s.tickerClean
	draw.Draw(clean, clip, s.RGBA, clip.Min, draw.Src)
	traceTicker(s, step)
	off := step * art % max(1, t.unit)
	y := (h-LineHeight(f))/2 + f.Metrics().Ascent.Ceil()
	cols := [3]color.RGBA{title, rest, gap}
	for x := clip.Min.X - off; x < clip.Max.X; x += t.unit {
		px := x
		for i, p := range t.parts {
			if p != "" && px < clip.Max.X && px+t.w[i] > clip.Min.X {
				drawRun(t.sub, px, y, p, f.(glyphSource), cols[i])
			}
			px += t.w[i]
		}
	}
	// Fade the text out at both ends with an ordered dither: art pixels go
	// back to the clean background, more of them towards the edge.
	const fade = 9 // art pixels
	for i := 0; i < fade; i++ {
		lvl := int32(16 * (fade - i) / fade)
		for _, xx := range [2]int{clip.Min.X + i*art, clip.Max.X - (i+1)*art} {
			for yy := clip.Min.Y; yy < clip.Max.Y; yy += art {
				if bayer4[(yy/art)&3][(xx/art)&3] >= lvl {
					continue
				}
				for dy := 0; dy < art && yy+dy < clip.Max.Y; dy++ {
					k := clean.PixOffset(xx, yy+dy)
					o := s.PixOffset(xx, yy+dy)
					copy(s.Pix[o:o+art*4], clean.Pix[k:k+art*4])
				}
			}
		}
	}
}

// eqWidth is the equaliser's width in art pixels: four bars with gaps.
const eqWidth = 7

// eqSpeeds and eqPhases set each bar's rhythm.
var (
	eqSpeeds = [4]float64{0.55, 0.83, 0.47, 0.71}
	eqPhases = [4]float64{0, 2.1, 4.0, 1.2}
)

// equaliser draws four bars standing on baseline y (their bottom edge),
// rising and falling with the strip's step, so they move as smoothly as
// the text: from 1 to 5 art pixels tall, with a light top.
func (s *Screen) equaliser(x, y, step int, c ramp) {
	for i := range 4 {
		v := 0.5 + 0.5*math.Sin(float64(step)*eqSpeeds[i]*0.5+eqPhases[i])
		n := 1 + int(v*4.99)
		bx := x + 2*i*art
		s.fillRect(image.Rect(bx, y-n*art, bx+art, y), c.base)
		s.fillRect(image.Rect(bx, y-n*art, bx+art, y-(n-1)*art), c.hi)
	}
}

// tickerPill draws the strip's pill in r, from a copy kept while the
// background and the colours stay the same (drawing it allocates).
func (s *Screen) tickerPill(r image.Rectangle) {
	t := &s.tick
	if t.pill == nil || t.pill.Rect != r || t.pillOn != s.bgID || t.pillBG != nightRamp.base {
		s.block(r, 10, panelRamp(0.12), Ink, 0)
		if t.pill == nil || t.pill.Rect != r {
			t.pill = image.NewRGBA(r)
		}
		draw.Draw(t.pill, r, s.RGBA, r.Min, draw.Src)
		t.pillOn, t.pillBG = s.bgID, nightRamp.base
		return
	}
	draw.Draw(s.RGBA, r, t.pill, r.Min, draw.Src)
}
