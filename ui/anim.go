package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"golang.org/x/image/font"
)

// Animation support. Drawing code calls wantFrame while something is
// moving; loops call NeedsFrame after drawing and, if true, draw again
// after FrameDelay instead of waiting for input.

// FrameDelay is the time between animation frames.
const FrameDelay = 30 * time.Millisecond

var needFrame bool

func wantFrame() { needFrame = true }

// NeedsFrame reports (and clears) whether the last draw is mid-animation.
func NeedsFrame() bool {
	n := needFrame
	needFrame = false
	return n
}

// Next waits for input, or for the next animation frame if the last draw
// asked for one. ok is false when input has closed; frame is true when it
// returned for a frame rather than an event. While the now-playing ticker
// shows, it also keeps the ticker scrolling (redrawing only its strip), so
// modal screens (forms, keyboard, lists) never freeze it.
func (s *Screen) Next(input <-chan Event) (ev Event, ok, frame bool) {
	anim := NeedsFrame()
	for {
		var frameC, tickC <-chan time.Time
		if anim {
			frameC = time.After(FrameDelay)
		}
		if s.ticker != "" {
			tickC = time.After(TickerWait())
		}
		select {
		case ev, ok = <-input:
			return ev, ok, false
		case <-frameC:
			return Event{}, true, true
		case <-tickC:
			s.DrawTicker()
			s.Present()
			if anim {
				return Event{}, true, true
			}
		}
	}
}

// Marquee draws text in a box of width w at (x, y); if it doesn't fit it
// scrolls back and forth, pausing at each end, so a long value (a URL) can
// be read in full. Right-to-left text starts at its right end, where it
// begins.
func (s *Screen) Marquee(x, y, w int, text string, f font.Face, c color.Color) {
	tw := Measure(f, text)
	if tw <= w {
		s.Text(x, y, text, f, c)
		return
	}
	over := float64(tw - w + 12)
	period := over/40 + 3 // 40 px/s each way, 1.5 s pause at each end
	t := math.Mod(time.Since(marqueeStart).Seconds(), 2*period)
	var off float64
	switch {
	case t < 1.5:
		off = 0
	case t < period:
		off = (t - 1.5) / (period - 1.5) * over
	case t < period+1.5:
		off = over
	default:
		off = over - (t-period-1.5)/(period-1.5)*over
	}
	if rtlText(text) {
		off = over - off
	}
	clip := image.Rect(x, y-2, x+w, y+LineHeight(f)+4).Intersect(s.Rect)
	sub := &Screen{RGBA: s.RGBA.SubImage(clip).(*image.RGBA)}
	sub.Text(x-int(off), y, text, f, c)
	wantFrame()
}

var marqueeStart = time.Now()
