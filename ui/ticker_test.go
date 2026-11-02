package ui

import (
	"image"
	"testing"
	"time"
)

// Frames that come late by less than half a frame still move the strip
// exactly one step each: the same distance every frame.
func TestTickerStepsEvenly(t *testing.T) {
	for _, late := range []time.Duration{0, time.Millisecond, 10 * time.Millisecond, 24 * time.Millisecond} {
		prev := tickerStep(tickerStart)
		for k := 1; k <= 400; k++ {
			jitter := time.Duration(k * 7919 % int(late+1)) // varies from frame to frame
			step := tickerStep(tickerStart.Add(time.Duration(k)*TickerFrame + jitter))
			if step != prev+1 {
				t.Fatalf("%v late: frame %d moved %d steps", late, k, step-prev)
			}
			prev = step
		}
	}
}

func TestTickerWait(t *testing.T) {
	for range 20 {
		w := TickerWait()
		if w <= 0 || w > TickerFrame {
			t.Fatalf("wait %v", w)
		}
		// the step due after the wait is a new one
		if tickerStep(time.Now().Add(w)) <= tickerStep(time.Now().Add(w-TickerFrame/2-time.Millisecond)) {
			t.Fatal("waiting doesn't reach a new step")
		}
	}
}

// The ticker's text splits into a title and the rest, with either setter.
func TestTickerParts(t *testing.T) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	s.SetTicker("Northbound   ·   Paper Satellites   ·   Low Orbit")
	p := s.tickerParts()
	if p.parts[0] != "Northbound" || p.parts[1] != "   ·   Paper Satellites   ·   Low Orbit" {
		t.Errorf("parts %q", p.parts)
	}
	s.SetTicker("Just a title")
	if p := s.tickerParts(); p.parts[0] != "Just a title" || p.parts[1] != "" {
		t.Errorf("parts %q", p.parts)
	}
	s.SetNowPlaying("A   ·   B", "C")
	if p := s.tickerParts(); p.parts[0] != "A   ·   B" || p.parts[1] != "   ·   C" || s.ticker != "A   ·   B   ·   C" {
		t.Errorf("parts %q, text %q", p.parts, s.ticker)
	}
	if p.unit != Measure(Small, p.parts[0])+Measure(Small, p.parts[1])+Measure(Small, tickerGap) {
		t.Errorf("unit %d", p.unit)
	}
}

// Redrawing the strip, as apps do 20 times a second, allocates nothing,
// full width or slim.
func TestDrawTickerAllocates(t *testing.T) {
	for _, compact := range []bool{false, true} {
		s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
		s.Background()
		s.SetTicker("Northbound   ·   Paper Satellites   ·   Low Orbit")
		s.tickerCompact = compact
		s.DrawTicker()
		if n := testing.AllocsPerRun(50, s.DrawTicker); n != 0 {
			t.Errorf("compact %v: %.1f allocations a frame", compact, n)
		}
	}
}

func BenchmarkDrawTicker(b *testing.B) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	s.Background()
	s.SetTicker("Northbound   ·   Paper Satellites   ·   Low Orbit")
	b.ReportAllocs()
	for b.Loop() {
		s.DrawTicker()
	}
}
