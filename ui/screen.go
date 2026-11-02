// Package ui draws full-screen apps on the Miyoo Mini framebuffer and reads
// its buttons. It needs no SDL: apps draw into an image.RGBA, and Present
// copies it to /dev/fb0 (rotated 180 degrees, BGRA, double-buffered by
// panning).
//
// With MINIDECK_UI_PNG=path set, Present writes the frame to that PNG instead,
// so screens can be checked on any computer.
package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/zakijss/minideck/fb"
)

const (
	W = fb.W
	H = fb.H
)

type varScreenInfo = fb.VarScreenInfo

const (
	fbioGetVScreenInfo = fb.IOGetVScreenInfo
	fbioPanDisplay     = fb.IOPanDisplay
	fbioWaitForVsync   = fb.IOWaitForVsync
)

// Screen is a frame being drawn (the embedded image) and the framebuffer
// it is shown on.
type Screen struct {
	*image.RGBA
	f             *os.File
	mem           []byte
	vinfo         varScreenInfo
	origY         uint32
	page          int
	pages         int
	pngPath       string
	frame         int
	bgID          *image.RGBA // the background last drawn whole, for redrawing the ticker strip
	ticker        string      // now-playing text across the top ("" = none)
	tickerClock   bool        // overlay: show the time at the right of the strip
	tickerCompact bool        // overlay: a slim 20 px strip
	tickerW       int         // strip width (0 = the whole screen)
	noVsync       bool        // the driver can't wait for vsync
	shadow        [][]byte    // per framebuffer page, the frame last written to it
	fullAt        []time.Time // per page, the last full copy
	sky           *sky        // the moving background (sky.go)
	skyMu         sync.Mutex  // guards the pages and shadow between Present and the sky
	static        *image.RGBA // StaticFrame's kept frame
	staticKey     string
	staticBg      *image.RGBA
	staticTop     int
	tickerClean   *image.RGBA // DrawTicker's copy of the strip's background
	tick          tickerText  // the ticker's text in parts (ticker.go)
	tickerLast    int         // the step last drawn, for the trace
}

// Open opens the framebuffer, blacks it out, takes on the saved look
// (theme and background, see theme.go) and starts the moving sky.
// With MINIDECK_UI_PNG set, it opens no device and Present writes PNGs;
// with MINIDECK_UI_NULL set, Present does nothing.
func Open() (*Screen, error) {
	applyAppearance(LoadAppearance())
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	if os.Getenv("MINIDECK_UI_NULL") != "" { // draw without showing (benchmarks)
		s.pngPath = os.DevNull
		return s, nil
	}
	if p := os.Getenv("MINIDECK_UI_PNG"); p != "" {
		s.pngPath = p
		return s, nil
	}
	f, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err := ioctl(f, fbioGetVScreenInfo, unsafe.Pointer(&s.vinfo)); err != nil {
		f.Close()
		return nil, fmt.Errorf("FBIOGET_VSCREENINFO: %w", err)
	}
	if s.vinfo.Xres != W || s.vinfo.Yres != H || s.vinfo.BitsPerPixel != 32 {
		f.Close()
		return nil, fmt.Errorf("unsupported framebuffer %dx%d@%d", s.vinfo.Xres, s.vinfo.Yres, s.vinfo.BitsPerPixel)
	}
	s.pages = int(s.vinfo.YresVirtual / H)
	if s.pages < 1 {
		s.pages = 1
	}
	mem, err := syscall.Mmap(int(f.Fd()), 0, W*H*4*s.pages, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap: %w", err)
	}
	s.f, s.mem, s.origY = f, mem, s.vinfo.Yoffset
	s.page = int(s.vinfo.Yoffset / H)
	// Black out every page at once: until the app's first frame (fonts,
	// settings and secrets take a moment to load) the screen would
	// otherwise keep the menu's last, often half-drawn, frame.
	for i := 0; i < len(mem); i += 4 {
		mem[i], mem[i+1], mem[i+2], mem[i+3] = 0, 0, 0, 0xff
	}
	s.startSky()
	return s, nil
}

// Present shows the current image.
func (s *Screen) Present() {
	if s.pngPath == os.DevNull {
		return
	}
	if s.pngPath != "" {
		s.frame++
		path := s.pngPath
		if strings.Contains(path, "%d") {
			path = fmt.Sprintf(path, s.frame)
		}
		if f, err := os.Create(path); err == nil {
			png.Encode(f, s.RGBA)
			f.Close()
		}
		return
	}
	copyStart := time.Now()
	s.skyMu.Lock()
	next := (s.page + 1) % s.pages
	dst := s.mem[next*W*H*4 : (next+1)*W*H*4]
	src := s.Pix
	// Only rows that differ from what this page already shows are copied:
	// moving down a list changes a few rows, not the whole frame. shadow
	// keeps, per page, the frame last written to it.
	// Every 10 s each page still gets a full copy, in case something else
	// drew on the screen.
	if s.shadow == nil {
		s.shadow = make([][]byte, s.pages)
		s.fullAt = make([]time.Time, s.pages)
	}
	if s.shadow[next] == nil || time.Since(s.fullAt[next]) > 10*time.Second {
		if s.shadow[next] == nil {
			s.shadow[next] = make([]byte, len(src))
		}
		copyRotated(dst, src)
		copy(s.shadow[next], src)
		s.fullAt[next] = time.Now()
		rowsCopied = H
	} else {
		rowsCopied = copyChangedRows(dst, src, s.shadow[next])
	}
	if s.sky != nil {
		s.sky.reapply(next)
	}
	panStart := time.Now()
	if s.pages > 1 {
		s.vinfo.Yoffset = uint32(next * H)
		ioctl(s.f, fbioPanDisplay, unsafe.Pointer(&s.vinfo))
	}
	panDur = time.Since(panStart)
	s.page = next
	s.skyMu.Unlock()
	vsyncStart := time.Now()
	defer traceFrame(copyStart, vsyncStart)
	// Wait for the panel to switch pages before the next frame is written
	// into the page that was showing; without this a quick redraw can land
	// while it is still on screen, which flickers. The Miyoo's driver
	// already waits for the switch inside the flip (it takes 4-10 ms), so a
	// second wait would only add a whole frame (16 ms) to every press: wait
	// only when the flip came back at once.
	if !s.noVsync && panDur < time.Millisecond {
		var zero uint32
		if err := ioctl(s.f, fbioWaitForVsync, unsafe.Pointer(&zero)); err != nil {
			fmt.Fprintln(os.Stderr, "ui: can't wait for vsync:", err)
			s.noVsync = true
		}
	}
}

// Close blanks the screen and restores the original pan offset, so the
// menu redraws cleanly.
func (s *Screen) Close() {
	stopProfile()
	s.stopSky()
	if s.f == nil {
		return
	}
	for i := range s.mem {
		s.mem[i] = 0
	}
	s.vinfo.Yoffset = s.origY
	ioctl(s.f, fbioPanDisplay, unsafe.Pointer(&s.vinfo))
	syscall.Munmap(s.mem)
	s.f.Close()
	s.f = nil
}

func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error { return fb.Ioctl(f, req, arg) }

// The panel is mounted upside down: pixel (x, y) goes to (W-1-x, H-1-y).
// One 32-bit word per pixel: RGBA in memory is 0xAABBGGRR as a little-endian
// word, the framebuffer wants BGRA (0xAARRGGBB), so the R and B bytes swap
// and alpha is forced.
func px(v uint32) uint32 { return v&0x0000ff00 | v<<16&0x00ff0000 | v>>16&0x000000ff | 0xff000000 }

// copyRotated writes src (RGBA, W*H pixels) into dst (BGRA) turned 180°.
// It writes dst front to back: framebuffer memory is uncached, and its write
// buffer combines ascending writes far better than descending ones.
func copyRotated(dst, src []byte) {
	n := W * H
	d := unsafe.Slice((*uint32)(unsafe.Pointer(&dst[0])), n)
	sp := unsafe.Slice((*uint32)(unsafe.Pointer(&src[0])), n)
	for i := range d {
		d[i] = px(sp[n-1-i])
	}
}

// copyChangedRows copies to dst only the rows of src that differ from
// shadow (what dst shows), and updates shadow. It returns how many it copied.
func copyChangedRows(dst, src, shadow []byte) int {
	const row = W * 4
	n := 0
	for y := 0; y < H; y++ {
		sr := src[y*row : (y+1)*row]
		if bytes.Equal(sr, shadow[y*row:(y+1)*row]) {
			continue
		}
		n++
		copy(shadow[y*row:(y+1)*row], sr)
		// screen row y is framebuffer row H-1-y, mirrored
		dr := dst[(H-1-y)*row : (H-y)*row]
		d := unsafe.Slice((*uint32)(unsafe.Pointer(&dr[0])), W)
		sp := unsafe.Slice((*uint32)(unsafe.Pointer(&sr[0])), W)
		for x := range d {
			d[x] = px(sp[W-1-x])
		}
	}
	return n
}
