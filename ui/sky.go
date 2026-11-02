package ui

import (
	"image/color"
	"math"
	"math/rand/v2"
	"time"
	"unsafe"
)

// The background behind every screen moves a little. With Stars, the
// default, stars twinkle, new ones pop into view and fade, and now and
// then a comet crosses; the other scenes (scene.go) have snow, rain,
// fireflies and so on. Each comet, flake and sparkle is made up at random
// as it starts: its colour, size, path, speed and length.
//
// It costs almost nothing, and apps don't draw it: the sky never touches
// their frame (s.Pix). Fifteen times a second it works out which art
// pixels (2x2 blocks) should look different from the plain background,
// and writes just those, straight into each framebuffer page, where that
// page still shows the plain background (never over a panel or text).
// Present puts the sky back onto a page after copying a new frame to it.
// A few dozen pixels a tick: no full redraws, no extra frames.
//
// Appearance › Motion sets how much: Calm has fewer sprites, moving
// slower; Lively more and faster (motionPace). It stays still with Motion
// off, with the Plain backdrop, or with MINIDECK_SKY=0.

const (
	skyTick  = 66 * time.Millisecond // ~15 frames a second
	skyCols  = W / art
	skyRows  = H / art
	maxPops  = 3
	maxTwink = 6
)

type sprite interface {
	// step advances by dt ticks of Normal motion (more at Lively, less
	// at Calm); false when it has finished.
	step(r *rand.Rand, dt float64) bool
	// paint adds its cells: put(ax, ay, colour, strength 0..1).
	paint(put func(ax, ay int, c color.RGBA, a float64))
}

// ender is a sprite that leaves another behind when it finishes (a
// raindrop's splash, a bubble's pop).
type ender interface{ end() sprite }

type sky struct {
	s       *Screen
	r       *rand.Rand
	stars   [][2]int // art cells holding the background's stars
	spawnFn func(k *sky, now time.Time)
	density float64 // how many sprites, against Normal motion
	dt      float64 // how far each tick moves them, against Normal
	sprites []sprite
	drawn   map[int]color.RGBA // what is on the pages now, by cell
	next    time.Time          // when the next comet may come
	stop    chan struct{}
	done    chan struct{}
}

func (s *Screen) startSky() {
	b := currentScene()
	a := CurrentAppearance()
	if a.Motion == "off" || b.spawn == nil || s.mem == nil {
		return
	}
	k := newSky(s, b, rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), rand.Uint64())))
	k.density, k.dt = motionPace(a.Motion)
	s.sky = k
	go k.run()
}

// motionPace is the sprites' density and speed for a Motion setting,
// against Normal's.
func motionPace(motion string) (density, speed float64) {
	switch motion {
	case "calm":
		return 0.5, 0.6
	case "lively":
		return 1.6, 1.4
	}
	return 1, 1
}

// newSky makes the moving layer for backdrop b at Normal motion; run
// starts it.
func newSky(s *Screen, b Scene, r *rand.Rand) *sky {
	k := &sky{s: s, r: r, spawnFn: b.spawn, density: 1, dt: 1, drawn: map[int]color.RGBA{},
		stop: make(chan struct{}), done: make(chan struct{})}
	if b.mark != nil {
		for ay := range skyRows {
			for ax := range skyCols {
				if b.mark(ax, ay) > 0 {
					k.stars = append(k.stars, [2]int{ax, ay})
				}
			}
		}
	}
	k.next = time.Now().Add(time.Duration(4+k.r.IntN(8)) * time.Second)
	return k
}

func (s *Screen) stopSky() {
	if s.sky == nil {
		return
	}
	close(s.sky.stop)
	<-s.sky.done
	s.sky = nil
}

func (k *sky) run() {
	defer close(k.done)
	t := time.NewTicker(skyTick)
	defer t.Stop()
	for {
		select {
		case <-k.stop:
			return
		case now := <-t.C:
			k.spawn(now)
			want := k.frame()
			k.s.skyMu.Lock()
			k.apply(want)
			k.s.skyMu.Unlock()
		}
	}
}

// spawn starts new sprites for the backdrop.
func (k *sky) spawn(now time.Time) {
	if k.spawnFn != nil {
		k.spawnFn(k, now)
	}
}

// chance reports, at random, whether to start a sprite this tick: p is the
// chance at Normal motion. It scales with the density and the speed, so
// faster sprites (which finish sooner) are started more often.
func (k *sky) chance(p float64) bool { return k.r.Float64() < p*k.density*k.dt }

// room is a cap of n sprites at Normal motion, scaled by the density.
func (k *sky) room(n int) int { return max(1, int(math.Round(float64(n)*k.density))) }

// spawnStars starts new twinkles, pops and comets at random.
func spawnStars(k *sky, now time.Time) {
	var twinkles, pops, comets int
	for _, sp := range k.sprites {
		switch sp.(type) {
		case *twinkle:
			twinkles++
		case *pop:
			pops++
		case *comet:
			comets++
		}
	}
	r := k.r
	if twinkles < k.room(maxTwink) && len(k.stars) > 0 && k.chance(0.35) {
		st := k.stars[r.IntN(len(k.stars))]
		k.add(newTwinkle(r, st[0], st[1]))
	}
	if pops < k.room(maxPops) && k.chance(0.06) {
		k.add(newPop(r))
	}
	if comets == 0 && now.After(k.next) {
		k.add(newComet(r))
		// mostly 10-30 s apart; once in a while two close together
		gap := 10 + r.IntN(21)
		if r.IntN(6) == 0 {
			gap = 2 + r.IntN(3)
		}
		k.next = now.Add(time.Duration(float64(gap) / k.density * float64(time.Second)))
	}
}

// frame steps every sprite and returns the cells they cover, blended over
// the background.
func (k *sky) frame() map[int]color.RGBA {
	want := make(map[int]color.RGBA, len(k.drawn)+16)
	alive := k.sprites[:0]
	put := func(ax, ay int, c color.RGBA, a float64) {
		if ax < 0 || ay < 0 || ax >= skyCols || ay >= skyRows || a <= 0.02 {
			return
		}
		key := ay*skyCols + ax
		base, ok := want[key]
		if !ok {
			base = bgCell(ax, ay)
		}
		want[key] = Mix(base, c, math.Min(a, 1))
	}
	var next []sprite
	for _, sp := range k.sprites {
		if sp.step(k.r, k.dt) {
			sp.paint(put)
			alive = append(alive, sp)
		} else if e, ok := sp.(ender); ok {
			if n := e.end(); n != nil {
				next = append(next, n)
			}
		}
	}
	clear(k.sprites[len(alive):])
	k.sprites = alive
	for _, n := range next {
		k.add(n)
	}
	return want
}

// apply writes what changed since the last tick to every page.
func (k *sky) apply(want map[int]color.RGBA) {
	for key, c := range want {
		if old, ok := k.drawn[key]; ok && old == c {
			continue
		}
		k.s.skyCell(key%skyCols, key/skyCols, c, -1)
	}
	for key := range k.drawn {
		if _, ok := want[key]; !ok {
			ax, ay := key%skyCols, key/skyCols
			k.s.skyCell(ax, ay, bgCell(ax, ay), -1)
		}
	}
	k.drawn = want
}

// reapply puts the sky back on page p after Present copied a frame to it.
// The caller holds skyMu.
func (k *sky) reapply(p int) {
	for key, c := range k.drawn {
		k.s.skyCell(key%skyCols, key/skyCols, c, p)
	}
}

func bgCell(ax, ay int) color.RGBA {
	bg := ambient()
	i := bg.PixOffset(ax*art, ay*art)
	return color.RGBA{bg.Pix[i], bg.Pix[i+1], bg.Pix[i+2], 255}
}

// skyCell writes art cell (ax, ay) in colour c on page p (-1: all pages),
// on each page only where that page's frame shows the plain background.
// The caller holds skyMu.
func (s *Screen) skyCell(ax, ay int, c color.RGBA, p int) {
	if s.shadow == nil {
		return
	}
	bg := unsafe.Slice((*uint32)(unsafe.Pointer(&ambient().Pix[0])), W*H)
	v := px(uint32(c.R) | uint32(c.G)<<8 | uint32(c.B)<<16)
	for page := range s.pages {
		if (p >= 0 && page != p) || s.shadow[page] == nil {
			continue
		}
		if !clearAround(unsafe.Slice((*uint32)(unsafe.Pointer(&s.shadow[page][0])), W*H), bg, ax, ay) {
			continue
		}
		mem := unsafe.Slice((*uint32)(unsafe.Pointer(&s.mem[page*W*H*4])), W*H)
		for dy := range art {
			y := ay*art + dy
			for dx := range art {
				x := ax*art + dx
				mem[(H-1-y)*W+(W-1-x)] = v // the panel is upside down
			}
		}
	}
}

// skyMargin is how far, in art cells, the sky keeps from anything drawn:
// without it a comet threads through the gaps between letters and reads
// as a scratch across the text.
const skyMargin = 4

// clearAround reports whether the frame shows the plain background in the
// cells within skyMargin of (ax, ay). One pixel per cell is enough: the UI
// draws in 2x2 blocks or larger, and any such block, wherever it lands,
// covers the top-left pixel of some cell.
func clearAround(frame, bg []uint32, ax, ay int) bool {
	for cy := max(0, ay-skyMargin); cy <= min(skyRows-1, ay+skyMargin); cy++ {
		row := cy * art * W
		for cx := max(0, ax-skyMargin); cx <= min(skyCols-1, ax+skyMargin); cx++ {
			if i := row + cx*art; frame[i] != bg[i] {
				return false
			}
		}
	}
	return true
}

// ---- the sprites ----

// Star colours: mostly white-blue, sometimes warm or tinted.
func starTint(r *rand.Rand) color.RGBA {
	switch r.IntN(10) {
	case 0:
		return rgb3(255, 226, 160) // gold
	case 1:
		return rgb3(255, 182, 212) // pink
	case 2:
		return rgb3(168, 228, 255) // sky
	case 3:
		return rgb3(190, 194, 248) // lavender
	}
	return rgb3(240, 241, 250)
}

// twinkle brightens one of the background's stars and lets it fade,
// sometimes with a little cross of light.
type twinkle struct {
	ax, ay   int
	t        float64 // ticks so far
	n        int
	c        color.RGBA
	cross    bool
	strength float64
}

func newTwinkle(r *rand.Rand, ax, ay int) *twinkle {
	return &twinkle{ax: ax, ay: ay, n: 8 + r.IntN(18), c: starTint(r), cross: r.IntN(3) == 0, strength: 0.6 + 0.4*r.Float64()}
}

func (tw *twinkle) step(_ *rand.Rand, dt float64) bool { tw.t += dt; return tw.t <= float64(tw.n) }

func (tw *twinkle) paint(put func(int, int, color.RGBA, float64)) {
	a := tw.strength * math.Sin(math.Pi*tw.t/float64(tw.n+1))
	put(tw.ax, tw.ay, tw.c, a)
	if tw.cross && a > 0.5 {
		arm := (a - 0.5) * 1.4
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			put(tw.ax+d[0], tw.ay+d[1], tw.c, arm)
		}
	}
}

// pop is a new star that appears somewhere, flares and goes out: a dot,
// then a cross, sometimes a bigger burst.
type pop struct {
	ax, ay int
	t      float64 // ticks so far
	n      int
	c      color.RGBA
	size   int // arm length at the peak: 1-3
}

func newPop(r *rand.Rand) *pop {
	return &pop{ax: 2 + r.IntN(skyCols-4), ay: 2 + r.IntN(skyRows-4), n: 10 + r.IntN(14), c: starTint(r), size: 1 + r.IntN(3)}
}

func (p *pop) step(_ *rand.Rand, dt float64) bool { p.t += dt; return p.t <= float64(p.n) }

func (p *pop) paint(put func(int, int, color.RGBA, float64)) {
	x := p.t / float64(p.n+1)
	a := math.Sin(math.Pi * math.Pow(x, 0.6)) // quick to flare, slow to fade
	put(p.ax, p.ay, p.c, a)
	arms := int(math.Round(float64(p.size) * a))
	for i := 1; i <= arms; i++ {
		f := a * (1 - float64(i-1)/float64(p.size+1))
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			put(p.ax+d[0]*i, p.ay+d[1]*i, p.c, f)
		}
	}
	if p.size == 3 && a > 0.75 { // the biggest also flash their diagonals
		for _, d := range [4][2]int{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
			put(p.ax+d[0], p.ay+d[1], p.c, a*0.45)
		}
	}
}

// comet crosses part of the sky in a straight line with a fading tail.
type comet struct {
	x, y   float64 // head, in art cells
	dx, dy float64 // per tick
	trail  [][2]int
	length int
	head   color.RGBA
	tail   color.RGBA
	life   float64 // ticks left before it starts to fade out
	fade   float64
}

func newComet(r *rand.Rand) *comet {
	// start above the top edge or off one side, heading down and across
	angle := (20 + r.Float64()*45) * math.Pi / 180 // below the horizontal
	speed := 2.5 + r.Float64()*4                   // cells a tick
	dir := 1.0
	if r.IntN(2) == 0 {
		dir = -1
	}
	c := &comet{dx: dir * speed * math.Cos(angle), dy: speed * math.Sin(angle),
		length: 8 + r.IntN(16), life: float64(25 + r.IntN(35))}
	if r.IntN(2) == 0 { // from the top
		c.x, c.y = float64(r.IntN(skyCols)), -1
	} else if dir > 0 { // from the left
		c.x, c.y = -1, float64(r.IntN(skyRows/2))
	} else { // from the right
		c.x, c.y = skyCols, float64(r.IntN(skyRows/2))
	}
	tails := [...]color.RGBA{rgb3(140, 145, 222), rgb3(95, 184, 240), rgb3(240, 107, 168), rgb3(245, 167, 66), rgb3(92, 207, 134)}
	c.tail = tails[r.IntN(len(tails))]
	c.head = rgb3(255, 255, 255)
	if r.IntN(4) == 0 {
		c.head = Mix(c.head, c.tail, 0.3)
	}
	return c
}

func (c *comet) step(_ *rand.Rand, dt float64) bool {
	// every cell the head passes over joins the trail (so fast comets
	// leave no gaps)
	dx, dy := c.dx*dt, c.dy*dt
	steps := int(math.Ceil(math.Max(math.Abs(dx), math.Abs(dy))))
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		cell := [2]int{int(math.Round(c.x + dx*f)), int(math.Round(c.y + dy*f))}
		if n := len(c.trail); n == 0 || c.trail[n-1] != cell {
			c.trail = append(c.trail, cell)
		}
	}
	c.x += dx
	c.y += dy
	if len(c.trail) > c.length {
		c.trail = c.trail[len(c.trail)-c.length:]
	}
	c.life -= dt
	if c.life < 0 {
		c.fade += dt
	}
	off := c.x < -float64(c.length) || c.x > float64(skyCols+c.length) || c.y > float64(skyRows+c.length)
	return !off && c.fade < 8
}

func (c *comet) paint(put func(int, int, color.RGBA, float64)) {
	fade := 1 - c.fade/8
	n := len(c.trail)
	for i, p := range c.trail {
		f := float64(i+1) / float64(n) // 1 at the head
		col := Mix(c.tail, c.head, f*f)
		put(p[0], p[1], col, fade*math.Pow(f, 1.3))
	}
	if n > 0 && fade > 0.6 { // a soft glow beside the head
		h := c.trail[n-1]
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			put(h[0]+d[0], h[1]+d[1], c.head, 0.35*fade)
		}
	}
}
