package ui

import (
	"fmt"
	"os"
	"runtime/pprof"
	"sync/atomic"
	"time"
)

// With MINIDECK_UI_TRACE=FILE set, every frame shown after a button press
// appends a line to FILE: how long the press took to reach the screen, and
// how much of that was the copy to the framebuffer and the vsync wait.
// Measurement aid; off by default.

var (
	started    = time.Now() // package init: close enough to process start
	firstShown bool
	traceFile  *os.File
	lastPress  atomic.Int64 // unix nanos of the newest press not yet shown
	tracePress int
	rowsCopied int           // rows the last Present copied (of 480)
	panDur     time.Duration // how long the page flip call took
)

// With MINIDECK_PPROF=FILE set, a CPU profile is written to FILE until the
// screen closes.
var profFile *os.File

func init() {
	if p := os.Getenv("MINIDECK_UI_TRACE"); p != "" {
		traceFile, _ = os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}
	if p := os.Getenv("MINIDECK_PPROF"); p != "" {
		if f, err := os.Create(p); err == nil && pprof.StartCPUProfile(f) == nil {
			profFile = f
		}
	}
}

func stopProfile() {
	if profFile != nil {
		pprof.StopCPUProfile()
		profFile.Close()
		profFile = nil
	}
}

func notePress() {
	if traceFile != nil {
		lastPress.CompareAndSwap(0, time.Now().UnixNano())
	}
}

// traceFrame is called by Present with the time the copy started and when
// the vsync wait began.
func traceFrame(copyStart, vsyncStart time.Time) {
	if traceFile == nil {
		return
	}
	if !firstShown {
		firstShown = true
		fmt.Fprintf(traceFile, "start: first frame %d ms after launch\n", time.Since(started).Milliseconds())
	}
	p := lastPress.Swap(0)
	if p == 0 {
		return
	}
	tracePress++
	end := time.Now()
	fmt.Fprintf(traceFile, "press %d: %d ms to screen (draw %d, copy %d [%d rows, flip %d], vsync %d)\n", tracePress,
		end.Sub(time.Unix(0, p)).Milliseconds(), copyStart.Sub(time.Unix(0, p)).Milliseconds(),
		vsyncStart.Sub(copyStart).Milliseconds(), rowsCopied, panDur.Milliseconds(), end.Sub(vsyncStart).Milliseconds())
}

// TraceMark notes a named moment (for start-up timing) when tracing is on.
func TraceMark(name string) {
	if traceFile != nil {
		fmt.Fprintf(traceFile, "mark %s: %d ms after launch\n", name, time.Since(started).Milliseconds())
	}
}

// While tracing, the now-playing strip's motion is summed up every 10 s:
// how many frames moved it exactly one step (smooth), how many redrew it in
// place, and how many skipped a step (a visible jump), with the longest gap
// between frames.
var tickerTrace struct {
	same, one, jumped int
	maxGap            time.Duration
	last, since       time.Time
}

func traceTicker(s *Screen, step int) {
	t := &tickerTrace
	if traceFile == nil {
		return
	}
	now := time.Now()
	switch d := step - s.tickerLast; {
	case s.tickerLast == 0:
		t.since = now
	case d == 0:
		t.same++
	case d == 1:
		t.one++
	default:
		t.jumped++
	}
	if !t.last.IsZero() {
		t.maxGap = max(t.maxGap, now.Sub(t.last))
	}
	s.tickerLast, t.last = step, now
	if now.Sub(t.since) >= 10*time.Second {
		fmt.Fprintf(traceFile, "ticker: %d smooth, %d in place, %d jumped; longest gap %d ms\n",
			t.one, t.same, t.jumped, t.maxGap.Milliseconds())
		t.same, t.one, t.jumped, t.maxGap, t.since = 0, 0, 0, 0, now
	}
}
