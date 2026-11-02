package ui

import (
	"image/color"
	"math"
	"math/bits"
	"math/rand/v2"
	"time"
)

// Scene is a background: the fixed marks drawn into it (stars, flakes)
// and what moves over it. The moving parts are sprites on the sky (sky.go),
// so every backdrop gets the same rules: drawn only on plain background,
// a few cells away from anything an app drew, at 15 frames a second.
type Scene struct {
	ID, Name string
	About    string // a few words for Settings
	// mark is the fixed mark at art pixel (ax, ay): 0 none, 1 dim, 2 bright.
	mark func(ax, ay int) int
	// spawn starts new sprites; nil for a backdrop that never moves.
	spawn func(k *sky, now time.Time)
}

// Scenes is every background, the default first.
var Scenes = []Scene{
	{ID: "stars", Name: "Stars", About: "Twinkling stars, flares and comets", mark: starAt, spawn: spawnStars},
	{ID: "snow", Name: "Snow", About: "Flakes drifting down", mark: flakeAt, spawn: spawnSnow},
	{ID: "rain", Name: "Rain", About: "Rain streaks and splashes", spawn: spawnRain},
	{ID: "fireflies", Name: "Fireflies", About: "Glowing lights wandering", mark: dustAt, spawn: spawnFireflies},
	{ID: "bubbles", Name: "Bubbles", About: "Bubbles rising and popping", spawn: spawnBubbles},
	{ID: "petals", Name: "Petals", About: "Blossom petals tumbling", spawn: spawnPetals},
	{ID: "code", Name: "Code", About: "Columns of falling light", spawn: spawnCode},
	{ID: "embers", Name: "Embers", About: "Sparks rising and fading", spawn: spawnEmbers},
	{ID: "aurora", Name: "Aurora", About: "Slow bands of light near the top", mark: dustAt, spawn: spawnAurora},
	{ID: "clouds", Name: "Clouds", About: "Soft clouds drifting by", spawn: spawnClouds},
	{ID: "leaves", Name: "Leaves", About: "Autumn leaves swaying down", spawn: spawnLeaves},
	{ID: "meteors", Name: "Meteor shower", About: "Shooting stars", mark: starAt, spawn: spawnMeteors},
	{ID: "warp", Name: "Warp", About: "Stars rushing outwards", spawn: spawnWarp},
	{ID: "plain", Name: "Plain", About: "Just the colour, nothing moving"},
}

func sceneByID(id string) Scene {
	for _, b := range Scenes {
		if b.ID == id {
			return b
		}
	}
	return Scenes[0]
}

func currentScene() Scene { return sceneByID(CurrentAppearance().Background) }

// hashAt is a fixed pseudo-random number for art pixel (ax, ay).
func hashAt(ax, ay int, seed uint32) uint32 {
	h := uint32(ax)*73856093 ^ uint32(ay)*19349663 ^ seed*83492791
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return h
}

// flakeAt: a light scatter of still flakes, for Snow.
func flakeAt(ax, ay int) int {
	switch h := hashAt(ax, ay, 7); {
	case h%1900 == 0:
		return 2
	case h%600 == 0:
		return 1
	}
	return 0
}

// dustAt: a few faint specks, for Fireflies.
func dustAt(ax, ay int) int {
	if hashAt(ax, ay, 11)%900 == 0 {
		return 1
	}
	return 0
}

// count reports how many of the sky's sprites are of type T.
func count[T sprite](k *sky) int {
	n := 0
	for _, sp := range k.sprites {
		if _, ok := sp.(T); ok {
			n++
		}
	}
	return n
}

// maxSprites caps every backdrop, whatever its spawn rates.
const maxSprites = 48

func (k *sky) add(sp sprite) {
	if len(k.sprites) < maxSprites {
		k.sprites = append(k.sprites, sp)
	}
}

// ---- snow ----

type flake struct {
	x, y   float64
	vy     float64
	phase  float64 // sway
	sway   float64
	big    bool
	c      color.RGBA
	bright float64
}

func spawnSnow(k *sky, _ time.Time) {
	r := k.r
	if count[*flake](k) < k.room(26) && k.chance(0.5) {
		f := &flake{x: r.Float64() * skyCols, y: -1, vy: 0.25 + r.Float64()*0.55, phase: r.Float64() * 2 * math.Pi,
			sway: 0.6 + r.Float64()*1.6, big: r.IntN(5) == 0, bright: 0.45 + r.Float64()*0.5}
		f.c = Mix(rgb3(255, 255, 255), Accent, 0.15*r.Float64())
		if f.big {
			f.vy *= 1.3 // big flakes are nearer: faster
			f.bright = math.Min(1, f.bright+0.2)
		}
		k.add(f)
	}
}

func (f *flake) step(_ *rand.Rand, dt float64) bool {
	f.y += f.vy * dt
	f.phase += 0.09 * dt
	return f.y < skyRows+2
}

func (f *flake) paint(put func(int, int, color.RGBA, float64)) {
	x := int(math.Round(f.x + f.sway*math.Sin(f.phase)))
	y := int(math.Round(f.y))
	put(x, y, f.c, f.bright)
	if f.big {
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			put(x+d[0], y+d[1], f.c, f.bright*0.45)
		}
	}
}

// ---- rain ----

type drop struct {
	x, y   float64
	dx, dy float64
	length int
	ground float64 // where it lands
	c      color.RGBA
}

type splash struct {
	ax, ay int
	t      float64 // ticks so far
	c      color.RGBA
}

func spawnRain(k *sky, _ time.Time) {
	r := k.r
	for range 2 {
		if count[*drop](k) < k.room(16) && k.chance(0.55) {
			k.add(&drop{x: r.Float64() * (skyCols + 30), y: -float64(r.IntN(8)), dx: -0.9, dy: 3.5 + r.Float64()*2,
				length: 3 + r.IntN(4), ground: skyRows*0.45 + r.Float64()*skyRows*0.6,
				c: Mix(rgb3(168, 228, 255), Accent, 0.35)})
		}
	}
}

func (d *drop) step(_ *rand.Rand, dt float64) bool {
	d.x += d.dx * dt
	d.y += d.dy * dt
	return d.y < d.ground
}

func (d *drop) paint(put func(int, int, color.RGBA, float64)) {
	for i := range d.length {
		f := 1 - float64(i)/float64(d.length)
		put(int(math.Round(d.x-d.dx*float64(i)/d.dy)), int(math.Round(d.y))-i, d.c, 0.75*f)
	}
}

// landed reports a drop that has reached the ground, for its splash.
func (d *drop) landed() bool { return d.y >= d.ground }

func (s *splash) step(_ *rand.Rand, dt float64) bool { s.t += dt; return s.t <= 5 }

func (s *splash) paint(put func(int, int, color.RGBA, float64)) {
	a := 0.7 * (1 - s.t/6)
	t := int(s.t)
	put(s.ax-t, s.ay-1+t/3, s.c, a)
	put(s.ax+t, s.ay-1+t/3, s.c, a)
	if t <= 2 {
		put(s.ax, s.ay, s.c, a)
	}
}

// ---- fireflies ----

type firefly struct {
	x, y   float64
	vx, vy float64
	t      float64 // ticks so far
	n      int
	pulse  float64
	c      color.RGBA
}

func spawnFireflies(k *sky, _ time.Time) {
	r := k.r
	if count[*firefly](k) < k.room(8) && k.chance(0.04) {
		c := rgb3(214, 245, 120)
		if r.IntN(3) == 0 {
			c = rgb3(255, 214, 110)
		}
		k.add(&firefly{x: 4 + r.Float64()*(skyCols-8), y: 4 + r.Float64()*(skyRows-8), n: 90 + r.IntN(160),
			pulse: 0.08 + r.Float64()*0.1, c: c})
	}
}

func (f *firefly) step(r *rand.Rand, dt float64) bool {
	f.t += dt
	// a slow wander: the heading drifts a little each tick
	f.vx = math.Max(-0.5, math.Min(0.5, f.vx+(r.Float64()-0.5)*0.12))
	f.vy = math.Max(-0.4, math.Min(0.4, f.vy+(r.Float64()-0.5)*0.12))
	f.x += f.vx * dt
	f.y += f.vy * dt
	return f.t <= float64(f.n)
}

func (f *firefly) paint(put func(int, int, color.RGBA, float64)) {
	life := math.Min(1, math.Min(f.t/12, (float64(f.n)-f.t)/12)) // fade in and out
	a := life * (0.55 + 0.45*math.Sin(f.t*f.pulse))
	x, y := int(math.Round(f.x)), int(math.Round(f.y))
	put(x, y, f.c, a)
	if a > 0.5 {
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			put(x+d[0], y+d[1], f.c, (a-0.5)*0.8)
		}
	}
}

// ---- bubbles ----

type bubble struct {
	x, y  float64
	vy    float64
	phase float64
	rad   int
	top   float64 // where it pops
	c     color.RGBA
}

func spawnBubbles(k *sky, _ time.Time) {
	r := k.r
	if count[*bubble](k) < k.room(9) && k.chance(0.12) {
		rad := 1 + r.IntN(3)
		k.add(&bubble{x: 4 + r.Float64()*(skyCols-8), y: skyRows + float64(rad), vy: 0.35 + r.Float64()*0.5,
			phase: r.Float64() * 6, rad: rad, top: r.Float64() * skyRows * 0.6,
			c: Mix(rgb3(168, 228, 255), Accent, 0.5)})
	}
}

func (b *bubble) step(_ *rand.Rand, dt float64) bool {
	b.y -= b.vy * dt
	b.phase += 0.12 * dt
	return b.y > b.top
}

// popped reports a bubble that reached its height, for its burst.
func (b *bubble) popped() bool { return b.y <= b.top }

func (b *bubble) paint(put func(int, int, color.RGBA, float64)) {
	cx := b.x + 0.8*math.Sin(b.phase)
	for _, p := range ring(b.rad) {
		put(int(math.Round(cx))+p[0], int(math.Round(b.y))+p[1], b.c, 0.6)
	}
	// a highlight at the top left
	put(int(math.Round(cx))-b.rad/2, int(math.Round(b.y))-b.rad/2-boolInt(b.rad == 1), rgb3(255, 255, 255), 0.7)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// rings are the cells of circle outlines of radius 1-3.
var rings = [4][][2]int{
	1: {{0, -1}, {1, 0}, {0, 1}, {-1, 0}},
	2: {{-1, -2}, {0, -2}, {1, -2}, {2, -1}, {2, 0}, {2, 1}, {1, 2}, {0, 2}, {-1, 2}, {-2, 1}, {-2, 0}, {-2, -1}},
	3: {{-1, -3}, {0, -3}, {1, -3}, {2, -2}, {3, -1}, {3, 0}, {3, 1}, {2, 2}, {1, 3}, {0, 3}, {-1, 3},
		{-2, 2}, {-3, 1}, {-3, 0}, {-3, -1}, {-2, -2}},
}

// ring is the cells of a circle outline of radius r (1-3).
func ring(r int) [][2]int { return rings[max(1, min(r, 3))] }

// ---- petals ----

type petal struct {
	x, y   float64
	vx, vy float64
	spin   float64
	rate   float64
	c      color.RGBA
}

func spawnPetals(k *sky, _ time.Time) {
	r := k.r
	if count[*petal](k) < k.room(14) && k.chance(0.2) {
		cs := [...]color.RGBA{rgb3(255, 182, 212), rgb3(240, 107, 168), rgb3(255, 226, 236)}
		p := &petal{y: -2, vx: 0.3 + r.Float64()*0.5, vy: 0.3 + r.Float64()*0.4, spin: r.Float64() * 4,
			rate: 0.1 + r.Float64()*0.15, c: cs[r.IntN(len(cs))]}
		// from the top or the left edge, drifting right and down
		if r.IntN(3) == 0 {
			p.x, p.y = -2, r.Float64()*skyRows*0.6
		} else {
			p.x = r.Float64() * skyCols
		}
		k.add(p)
	}
}

func (p *petal) step(_ *rand.Rand, dt float64) bool {
	p.spin += p.rate * dt
	p.x += (p.vx + 0.25*math.Sin(p.spin*0.7)) * dt
	p.y += p.vy * dt
	return p.y < skyRows+2 && p.x < skyCols+2
}

// petalShapes are a petal tumbling: flat, tilted, edge on, tilted the
// other way.
var petalShapes = [4][2][2]int{{{0, 0}, {1, 0}}, {{0, 0}, {1, 1}}, {{0, 0}, {0, 1}}, {{1, 0}, {0, 1}}}

func (p *petal) paint(put func(int, int, color.RGBA, float64)) {
	x, y := int(math.Round(p.x)), int(math.Round(p.y))
	for i, d := range petalShapes[int(p.spin)&3] {
		put(x+d[0], y+d[1], p.c, 0.85-0.25*float64(i))
	}
}

// ---- code ----

type stream struct {
	ax     int
	y      float64
	speed  float64
	length int
	bits   uint64 // which cells of the column are lit
	c      color.RGBA
}

func spawnCode(k *sky, _ time.Time) {
	r := k.r
	if count[*stream](k) < k.room(10) && k.chance(0.15) {
		k.add(&stream{ax: 2 + 4*r.IntN(skyCols/4-1), y: -1, speed: 0.5 + r.Float64()*0.9, length: 8 + r.IntN(14),
			bits: r.Uint64(), c: accentRamp.base})
	}
}

func (s *stream) step(r *rand.Rand, dt float64) bool {
	s.y += s.speed * dt
	if r.IntN(4) == 0 {
		s.bits ^= 1 << uint(r.IntN(64)) // a cell flickers
	}
	return s.y-float64(s.length) < skyRows
}

func (s *stream) paint(put func(int, int, color.RGBA, float64)) {
	head := int(s.y)
	put(s.ax, head, rgb3(240, 255, 245), 0.9)
	for i := 1; i < s.length; i++ {
		if s.bits>>(uint(head-i)&63)&1 == 0 {
			continue
		}
		put(s.ax, head-i, s.c, 0.7*(1-float64(i)/float64(s.length)))
	}
}

func (d *drop) end() sprite {
	if !d.landed() {
		return nil
	}
	return &splash{ax: int(math.Round(d.x)), ay: int(math.Round(d.y)), c: d.c}
}

func (b *bubble) end() sprite {
	if !b.popped() {
		return nil
	}
	return &pop{ax: int(math.Round(b.x)), ay: int(math.Round(b.y)), n: 6, c: b.c, size: min(b.rad, 2)}
}

// ---- embers ----

// ember is a spark rising from the bottom edge, cooling from yellow to
// red as it goes.
type ember struct {
	x, y  float64
	vx    float64
	vy    float64 // upwards: negative
	phase float64 // flicker
	t     float64
	n     int
	big   bool
}

// emberHeat are an ember's colours, hottest first.
var emberHeat = [...]color.RGBA{rgb3(255, 244, 190), rgb3(255, 196, 92), rgb3(248, 128, 52), rgb3(214, 66, 40), rgb3(140, 36, 30)}

func spawnEmbers(k *sky, _ time.Time) {
	r := k.r
	if count[*ember](k) < k.room(24) && k.chance(0.35) {
		k.add(&ember{x: r.Float64() * skyCols, y: skyRows + 1, vx: (r.Float64() - 0.5) * 0.3, vy: -(0.5 + r.Float64()*0.7),
			phase: r.Float64() * 6, n: 70 + r.IntN(130), big: r.IntN(2) == 0})
	}
}

func (e *ember) step(r *rand.Rand, dt float64) bool {
	e.t += dt
	// rising air: a little sideways drift that changes now and then
	if r.IntN(8) == 0 {
		e.vx = math.Max(-0.4, math.Min(0.4, e.vx+(r.Float64()-0.5)*0.2))
	}
	e.x += e.vx * dt
	e.y += e.vy * dt
	e.phase += 0.7 * dt
	return e.t <= float64(e.n) && e.y > -2
}

func (e *ember) paint(put func(int, int, color.RGBA, float64)) {
	age := e.t / float64(e.n)
	c := emberHeat[min(len(emberHeat)-1, int(age*age*float64(len(emberHeat))))]
	a := (1 - age*age*age) * (0.75 + 0.25*math.Sin(e.phase))
	x, y := int(math.Round(e.x)), int(math.Round(e.y))
	put(x, y, c, a)
	if e.big {
		put(x, y+1, c, a*0.5) // a short glowing tail
	}
}

// ---- aurora ----

// aurora is a band of light hanging near the top: a wavy curtain that
// sways slowly, fades in, stays a while and fades out.
type aurora struct {
	x0, w     int     // the columns it spans
	y0        float64 // its middle row
	amp, freq float64 // the wave
	phase     float64
	rate      float64 // how fast the wave moves
	t         float64
	n         int
	c, edge   color.RGBA
}

func spawnAurora(k *sky, _ time.Time) {
	r := k.r
	if count[*aurora](k) < k.room(2) && k.chance(0.012) {
		cs := [...][2]color.RGBA{
			{rgb3(110, 245, 170), rgb3(80, 200, 230)},  // green, blue below
			{rgb3(120, 230, 210), rgb3(160, 120, 240)}, // teal, violet below
			{rgb3(170, 250, 140), rgb3(110, 245, 170)}, // yellow green
		}
		c := cs[r.IntN(len(cs))]
		w := 40 + r.IntN(60)
		k.add(&aurora{x0: r.IntN(skyCols - w/2), w: w, y0: 6 + r.Float64()*30, amp: 3 + r.Float64()*3,
			freq: 0.04 + r.Float64()*0.05, phase: r.Float64() * 6, rate: 0.008 + r.Float64()*0.012,
			n: 220 + r.IntN(260), c: c[0], edge: c[1]})
	}
}

func (au *aurora) step(_ *rand.Rand, dt float64) bool {
	au.t += dt
	au.phase += au.rate * dt
	return au.t <= float64(au.n)
}

func (au *aurora) paint(put func(int, int, color.RGBA, float64)) {
	// fade in and out over 40 ticks, in eight steps, so most cells keep
	// their colour from one tick to the next
	life := math.Min(1, math.Min(au.t, float64(au.n)-au.t)/40)
	life = math.Ceil(life*8) / 8
	for i := 0; i < au.w; i += 2 { // every other column: rays with gaps
		// soft ends: brightest in the middle of the band
		env := math.Sin(math.Pi * (float64(i) + 0.5) / float64(au.w))
		a := 0.6 * life * env
		if a < 0.05 {
			continue
		}
		x := au.x0 + i
		y := int(math.Round(au.y0 + au.amp*math.Sin(float64(i)*au.freq+au.phase)))
		// a curtain of rays hanging from the wave, longer and shorter
		// along the band, fading into the second colour
		rays := 5 + int(7.5*(0.5+0.5*math.Sin(float64(i)*0.45+au.phase*0.8)))
		for j := range rays {
			if j >= 3 && (j+i/2)&1 == 1 {
				continue // below the top, every other cell: a dithered fade
			}
			f := float64(j) / float64(rays)
			put(x, y+j, Mix(au.c, au.edge, f), a*(1-f*0.85))
		}
	}
}

// ---- clouds ----

// cloud is a soft blob of a few overlapping ellipses drifting sideways.
// It fades in where it starts and fades out after a while, or leaves.
// Its shape is worked out once, as a bit mask of cells.
type cloud struct {
	x, y float64 // the middle of its base, in cells
	vx   float64
	rx   int               // how far it reaches either side of x
	mask [cloudRows]uint64 // the cells it covers: bit dx+rx of row -dy
	t    float64
	n    int
	c    color.RGBA
}

// cloudRows is the most rows a cloud can be tall.
const cloudRows = 20

func spawnClouds(k *sky, _ time.Time) {
	r := k.r
	if count[*cloud](k) < k.room(3) && k.chance(0.02) {
		w := 10 + r.Float64()*7 // the base lobe's half width
		c := &cloud{x: r.Float64() * skyCols, y: 10 + r.Float64()*(skyRows*0.7), vx: 0.08 + r.Float64()*0.14,
			n: 300 + r.IntN(500), c: Mix(rgb3(255, 255, 255), Accent, 0.2)}
		// three ellipses wider than tall: centre (dx, dy) and half width;
		// the base is cut flat
		lobes := [3][3]float64{
			{0, 0, w},
			{-w * (0.2 + 0.4*r.Float64()), -w * 0.35, w * (0.45 + 0.2*r.Float64())},
			{w * (0.2 + 0.4*r.Float64()), -w * 0.3, w * (0.35 + 0.2*r.Float64())},
		}
		c.rx = int(math.Ceil(w*1.2)) + 1
		for row := range cloudRows {
			for dx := -c.rx; dx <= c.rx; dx++ {
				for _, l := range lobes {
					ex := (float64(dx) - l[0]) / l[2]
					ey := (float64(-row) - l[1]) / (l[2] * 0.6)
					if ex*ex+ey*ey <= 1 {
						c.mask[row] |= 1 << uint(dx+c.rx)
					}
				}
			}
		}
		k.add(c)
	}
}

func (c *cloud) step(_ *rand.Rand, dt float64) bool {
	c.t += dt
	c.x += c.vx * dt
	return c.t <= float64(c.n) && c.x-float64(c.rx) < skyCols
}

func (c *cloud) paint(put func(int, int, color.RGBA, float64)) {
	life := math.Min(1, math.Min(c.t, float64(c.n)-c.t)/40)
	life = math.Ceil(life*6) / 6 // in steps: see aurora
	cx, cy := int(math.Round(c.x)), int(math.Round(c.y))
	for row, m := range c.mask {
		var above uint64
		if row+1 < cloudRows {
			above = c.mask[row+1]
		}
		y := cy - row
		for m != 0 {
			i := bits.TrailingZeros64(m)
			m &^= 1 << uint(i)
			x := cx + i - c.rx
			a := 0.22 // the top edge, lit from above
			if above>>uint(i)&1 == 1 {
				// inside, every other cell, in a fixed checker so the
				// pattern doesn't crawl as the cloud moves
				if (x+y)&1 == 1 {
					continue
				}
				a = 0.16
			}
			put(x, y, c.c, a*life)
		}
	}
}

// ---- leaves ----

// leaf falls swaying from side to side, turning as it swings.
type leaf struct {
	x0, y  float64 // the middle of its swing
	vx, vy float64
	swing  float64 // half the swing, in cells
	phase  float64
	rate   float64
	c      color.RGBA
}

// leafColours are autumn's.
var leafColours = [...]color.RGBA{rgb3(232, 112, 40), rgb3(212, 64, 44), rgb3(244, 186, 52), rgb3(176, 108, 52), rgb3(198, 160, 60)}

// leafShapes are three of a 2x2 block's four cells, turning.
var leafShapes = [4][3][2]int{{{0, 0}, {1, 0}, {1, 1}}, {{0, 0}, {1, 0}, {0, 1}}, {{0, 0}, {0, 1}, {1, 1}}, {{1, 0}, {0, 1}, {1, 1}}}

func spawnLeaves(k *sky, _ time.Time) {
	r := k.r
	if count[*leaf](k) < k.room(12) && k.chance(0.12) {
		k.add(&leaf{x0: r.Float64() * skyCols, y: -2, vx: (r.Float64() - 0.3) * 0.2, vy: 0.25 + r.Float64()*0.3,
			swing: 2 + r.Float64()*4, phase: r.Float64() * 6, rate: 0.05 + r.Float64()*0.05,
			c: leafColours[r.IntN(len(leafColours))]})
	}
}

func (l *leaf) step(_ *rand.Rand, dt float64) bool {
	l.phase += l.rate * dt
	l.x0 += l.vx * dt
	// slower at the ends of the swing, as a leaf hangs before it turns
	l.y += l.vy * dt * (0.4 + 0.6*math.Abs(math.Cos(l.phase)))
	return l.y < skyRows+2 && l.x0 > -l.swing-2 && l.x0 < skyCols+l.swing+2
}

func (l *leaf) paint(put func(int, int, color.RGBA, float64)) {
	sw := math.Sin(l.phase)
	x, y := int(math.Round(l.x0+l.swing*sw)), int(math.Round(l.y))
	// the shape follows the swing: tilted one way, flat, tilted the other
	f := int((sw + 1) * 2)
	for i, d := range leafShapes[min(f, 3)] {
		put(x+d[0], y+d[1], l.c, 0.85-0.15*float64(i))
	}
}

// ---- meteor shower ----

// meteor is a short streak from the shower's corner: fast, straight,
// gone in a second or two.
type meteor struct {
	x, y   float64
	dx, dy float64 // per tick
	length int
	t      float64
	n      int // ticks before it burns out
	c      color.RGBA
}

func spawnMeteors(k *sky, _ time.Time) {
	r := k.r
	if count[*meteor](k) >= k.room(6) || !k.chance(0.08) {
		return
	}
	// sometimes two or three at once
	n := 1
	if r.IntN(5) == 0 {
		n = 2 + r.IntN(2)
	}
	for range n {
		// all from the top right, heading down and left
		angle := (148 + r.Float64()*14) * math.Pi / 180
		speed := 3 + r.Float64()*2.5
		m := &meteor{dx: speed * math.Cos(angle), dy: speed * math.Sin(angle), length: 3 + r.IntN(5),
			n: 6 + r.IntN(12), c: starTint(r)}
		if r.IntN(2) == 0 {
			m.x, m.y = skyCols*0.3+r.Float64()*skyCols*0.75, -1
		} else {
			m.x, m.y = skyCols+1, r.Float64()*skyRows*0.5
		}
		k.add(m)
	}
}

func (m *meteor) step(_ *rand.Rand, dt float64) bool {
	m.t += dt
	m.x += m.dx * dt
	m.y += m.dy * dt
	return m.t <= float64(m.n) && m.x > -float64(m.length) && m.y < skyRows+float64(m.length)
}

func (m *meteor) paint(put func(int, int, color.RGBA, float64)) {
	fade := math.Min(1, (float64(m.n)-m.t)/3+0.34) // dims over its last ticks
	speed := math.Hypot(m.dx, m.dy)
	ux, uy := m.dx/speed, m.dy/speed
	for i := range m.length {
		f := 1 - float64(i)/float64(m.length)
		c := m.c
		if i == 0 {
			c = rgb3(255, 255, 255)
		}
		put(int(math.Round(m.x-ux*float64(i))), int(math.Round(m.y-uy*float64(i))), c, fade*f)
	}
}

// ---- warp ----

// streak is a star rushing out from the middle of the screen, faster and
// longer as it nears the edge.
type streak struct {
	ux, uy float64 // its direction
	d      float64 // how far out, in cells
	v      float64 // cells a tick
	c      color.RGBA
}

func spawnWarp(k *sky, _ time.Time) {
	r := k.r
	for range 2 {
		if count[*streak](k) < k.room(16) && k.chance(0.3) {
			a := r.Float64() * 2 * math.Pi
			c := rgb3(240, 241, 250)
			if r.IntN(3) == 0 {
				c = Mix(c, Accent, 0.5)
			}
			k.add(&streak{ux: math.Cos(a), uy: math.Sin(a) * 0.8, d: 6 + r.Float64()*50, v: 0.2 + r.Float64()*0.3, c: c})
		}
	}
}

func (st *streak) step(_ *rand.Rand, dt float64) bool {
	st.d += st.v * dt
	st.v = math.Min(5, st.v*(1+0.06*dt)) // speeding up
	return st.d < skyCols*0.75
}

func (st *streak) paint(put func(int, int, color.RGBA, float64)) {
	cx, cy := skyCols/2.0, skyRows/2.0
	length := 1 + int(st.v*1.5)
	a := math.Min(1, st.d/60) // faint near the middle
	for i := range min(length, 10) {
		f := 1 - float64(i)/float64(length+1)
		dd := st.d - float64(i)
		put(int(math.Round(cx+st.ux*dd)), int(math.Round(cy+st.uy*dd)), st.c, a*f)
	}
}
