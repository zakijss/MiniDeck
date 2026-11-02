package ui

import (
	"encoding/json"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/zakijss/minideck/appdir"
	"github.com/zakijss/minideck/atomicfile"
)

// Themes recolour the whole interface. Every theme is the default Night
// palette with its hues turned: the background family (backdrop, panels,
// keys, text) to one hue and the accent family (selection, focus, the
// title's shadow) to another, keeping each colour's lightness. The layout
// of light and dark is the same in every theme, so text keeps its contrast
// (theme_test.go checks every pair the interface draws).
//
// Good, Warn and Bad keep their colours in every theme: they mean
// something. App icons keep theirs too.

// palette is every colour a theme sets.
type palette struct {
	bg, surface, raised, line, text, muted, faint, ink color.RGBA
	onFocus2, accent, titleShadow                      color.RGBA
	night, focus, key, key2, accentR                   ramp
	levels                                             [4]color.RGBA // the backdrop, darkest first
	stars                                              [2]color.RGBA // dim and bright fixed stars
	wells                                              [2]color.RGBA // the keyboard's recess
}

// nightPalette is the default look: night blue with a lavender selection.
var nightPalette = palette{
	bg: rgb(0x111226), surface: rgb(0x282b4e), raised: rgb(0x3c4068), line: rgb(0x464a74),
	text: rgb(0xf0f1fa), muted: rgb(0xacb0d2), faint: rgb(0x8489ae), ink: rgb3(27, 29, 58),
	onFocus2: rgb(0x3e4280), accent: rgb(0xbec2f8), titleShadow: rgb3(58, 61, 122),
	night:   ramp{rgb3(60, 64, 104), rgb3(40, 43, 78), rgb3(29, 31, 60)},
	focus:   ramp{rgb3(232, 234, 255), rgb3(190, 194, 248), rgb3(126, 131, 214)},
	key:     ramp{rgb3(96, 101, 148), rgb3(68, 72, 114), rgb3(44, 47, 84)},
	key2:    ramp{rgb3(74, 78, 120), rgb3(52, 55, 92), rgb3(36, 38, 70)},
	accentR: ramp{rgb3(190, 194, 248), rgb3(140, 145, 222), rgb3(98, 102, 184)},
	levels:  [4]color.RGBA{rgb3(12, 13, 28), rgb3(17, 18, 38), rgb3(22, 23, 48), rgb3(27, 29, 58)},
	stars:   [2]color.RGBA{rgb3(52, 55, 92), rgb3(110, 115, 160)},
	wells:   [2]color.RGBA{rgb3(14, 15, 32), rgb3(20, 21, 44)},
}

// Theme is a colour scheme, by name.
type Theme struct {
	ID, Name string
	About    string // a few words for Settings
	accent   string // its accent's name, for Settings
	// hues in degrees; sat scales saturation; light scales the
	// background family's lightness (below 1: darker)
	bgHue, bgSat, bgLight float64
	acHue, acSat          float64
	night                 bool // the palette as drawn, unchanged
}

// Themes is every theme, the default first.
var Themes = []Theme{
	{ID: "night", Name: "Night", About: "Night blue and lavender", accent: "lavender", night: true},
	{ID: "forest", Name: "Forest", About: "Pine green and mint", accent: "mint", bgHue: 152, bgSat: 0.85, bgLight: 0.85, acHue: 140, acSat: 0.75},
	{ID: "ocean", Name: "Ocean", About: "Deep sea and aqua", accent: "aqua", bgHue: 205, bgSat: 1, bgLight: 1, acHue: 186, acSat: 0.85},
	{ID: "ember", Name: "Ember", About: "Charcoal brown and amber", accent: "amber", bgHue: 16, bgSat: 0.7, bgLight: 1, acHue: 36, acSat: 0.95},
	{ID: "grape", Name: "Grape", About: "Dark violet and orchid", accent: "orchid", bgHue: 272, bgSat: 1, bgLight: 1, acHue: 296, acSat: 0.8},
	{ID: "rose", Name: "Rose", About: "Wine red and pink", accent: "pink", bgHue: 338, bgSat: 0.8, bgLight: 1, acHue: 344, acSat: 0.85},
	{ID: "graphite", Name: "Graphite", About: "Greys and silver", accent: "silver", bgHue: 235, bgSat: 0.12, bgLight: 1, acHue: 235, acSat: 0.1},
	{ID: "contrast", Name: "High contrast", About: "Near black with a yellow selection", accent: "yellow", bgHue: 235, bgSat: 0, bgLight: 0.7, acHue: 50, acSat: 1},
	{ID: "dusk", Name: "Dusk", About: "Indigo and peach", accent: "peach", bgHue: 250, bgSat: 0.75, bgLight: 1, acHue: 22, acSat: 1},
	{ID: "mint", Name: "Mint", About: "Dark teal and mint", accent: "mint", bgHue: 178, bgSat: 0.55, bgLight: 0.9, acHue: 154, acSat: 0.9},
	{ID: "sand", Name: "Sand", About: "Olive brown and sand", accent: "sand", bgHue: 45, bgSat: 0.35, bgLight: 0.95, acHue: 44, acSat: 0.6},
	{ID: "slate", Name: "Slate", About: "Blue grey and sky blue", accent: "sky blue", bgHue: 212, bgSat: 0.3, bgLight: 1, acHue: 200, acSat: 1},
	{ID: "coffee", Name: "Coffee", About: "Dark roast and cream", accent: "cream", bgHue: 24, bgSat: 0.4, bgLight: 0.85, acHue: 32, acSat: 0.45},
	{ID: "sakura", Name: "Sakura", About: "Plum and cherry blossom", accent: "blossom pink", bgHue: 300, bgSat: 0.45, bgLight: 1, acHue: 336, acSat: 0.75},
	{ID: "deepsea", Name: "Deep sea", About: "Darkest navy and sea green", accent: "sea green", bgHue: 222, bgSat: 1, bgLight: 0.7, acHue: 165, acSat: 0.85},
	{ID: "neon", Name: "Neon", About: "Black violet and lime", accent: "lime", bgHue: 268, bgSat: 0.6, bgLight: 0.7, acHue: 92, acSat: 1},
}

func themeByID(id string) Theme {
	for _, t := range Themes {
		if t.ID == id {
			return t
		}
	}
	return Themes[0]
}

// AccentHue is a colour for the selection, highlights and the title's
// shadow, in place of the theme's own.
type AccentHue struct {
	ID, Name string
	own      bool    // the theme's own accent
	hue, sat float64 // as Theme's acHue and acSat
}

// AccentHues is every accent, the theme's own first.
var AccentHues = []AccentHue{
	{ID: "theme", Name: "Theme", own: true},
	{ID: "lavender", Name: "Lavender", hue: 236, sat: 1},
	{ID: "sky", Name: "Sky", hue: 200, sat: 1},
	{ID: "aqua", Name: "Aqua", hue: 178, sat: 1},
	{ID: "mint", Name: "Mint", hue: 148, sat: 0.9},
	{ID: "lime", Name: "Lime", hue: 88, sat: 1},
	{ID: "gold", Name: "Gold", hue: 46, sat: 1},
	{ID: "orange", Name: "Orange", hue: 26, sat: 1},
	{ID: "coral", Name: "Coral", hue: 6, sat: 1},
	{ID: "pink", Name: "Pink", hue: 332, sat: 1},
	{ID: "violet", Name: "Violet", hue: 276, sat: 1},
	{ID: "silver", Name: "Silver", hue: 230, sat: 0.08},
}

func accentByID(id string) AccentHue {
	for _, a := range AccentHues {
		if a.ID == id {
			return a
		}
	}
	return AccentHues[0]
}

// option is one choice of a setting that has a fixed set of them.
type option struct{ ID, Name, About string }

func optionByID(opts []option, id string, def int) option {
	for _, o := range opts {
		if o.ID == id {
			return o
		}
	}
	return opts[def]
}

func optionIndex(opts []option, id string) int {
	for i, o := range opts {
		if o.ID == id {
			return i
		}
	}
	return 0
}

// brightnesses are the backdrop's lightness settings; panels, keys and
// text stay as the theme has them. brightScale is each one's lightness
// factor for the backdrop.
var (
	brightnesses = []option{
		{"dim", "Dim", "A darker backdrop"},
		{"normal", "Normal", "The backdrop as the theme has it"},
		{"bright", "Bright", "A lighter backdrop"},
	}
	brightScale = map[string]float64{"dim": 0.6, "normal": 1, "bright": 1.5}
)

// motions are how much the background moves. The speeds and densities
// are in sky.go (motionPace).
var motions = []option{
	{"off", "Off", "Still: a little less work for the battery"},
	{"calm", "Calm", "Fewer and slower"},
	{"normal", "Normal", "The background moves"},
	{"lively", "Lively", "More of it, and faster"},
}

// palette works out the theme's colours from the Night palette.
func (t Theme) palette() palette {
	if t.night {
		return nightPalette
	}
	p := nightPalette
	bg := func(c color.RGBA) color.RGBA { return turn(c, t.bgHue, t.bgSat, t.bgLight) }
	br := func(r ramp) ramp { return ramp{bg(r.hi), bg(r.base), bg(r.lo)} }
	for _, c := range []*color.RGBA{&p.bg, &p.surface, &p.raised, &p.line, &p.muted, &p.faint, &p.ink,
		&p.levels[0], &p.levels[1], &p.levels[2], &p.levels[3], &p.stars[0], &p.stars[1], &p.wells[0], &p.wells[1]} {
		*c = bg(*c)
	}
	// text stays near white: only a hint of the hue
	p.text = turn(p.text, t.bgHue, t.bgSat*0.5, 1)
	p.night, p.key, p.key2 = br(p.night), br(p.key), br(p.key2)
	p.setAccent(t.acHue, t.acSat)
	return p
}

// setAccent turns the Night palette's accent family (the selection,
// focus, the title's shadow) to hue h, with its saturation times sat.
func (p *palette) setAccent(h, sat float64) {
	n := nightPalette
	ac := func(c color.RGBA) color.RGBA { return turn(c, h, sat, 1) }
	ar := func(r ramp) ramp { return ramp{ac(r.hi), ac(r.base), ac(r.lo)} }
	p.onFocus2, p.accent, p.titleShadow = ac(n.onFocus2), ac(n.accent), ac(n.titleShadow)
	p.focus, p.accentR = ar(n.focus), ar(n.accentR)
}

// setBrightness scales the backdrop's lightness by f, and its fixed stars'
// by half as much, so they still show.
func (p *palette) setBrightness(f float64) {
	if f == 1 {
		return
	}
	scale := func(c color.RGBA, f float64) color.RGBA {
		h, s, l := toHSL(c)
		return fromHSL(h, s, math.Min(1, l*f))
	}
	for i := range p.levels {
		p.levels[i] = scale(p.levels[i], f)
	}
	for i := range p.stars {
		p.stars[i] = scale(p.stars[i], 1+(f-1)/2)
	}
}

// lookPalette is every colour of look a: its theme, with the accent and
// the backdrop's brightness applied.
func lookPalette(a Appearance) palette {
	p := themeByID(a.Theme).palette()
	if ac := accentByID(a.Accent); !ac.own {
		p.setAccent(ac.hue, ac.sat)
	}
	p.setBrightness(brightScale[optionByID(brightnesses, a.Brightness, 1).ID])
	return p
}

// turn gives c the hue h (degrees) and its saturation times sat. A dark
// colour (a surface) also gets its lightness times light; light ones (text)
// keep theirs.
func turn(c color.RGBA, h, sat, light float64) color.RGBA {
	_, s, l := toHSL(c)
	if l < 0.5 {
		l *= light
	}
	return fromHSL(h, math.Min(1, s*sat), l)
}

func toHSL(c color.RGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l
}

func fromHSL(h, s, l float64) color.RGBA {
	h = math.Mod(h, 360) / 360
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return rgb3(v, v, v)
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	f := func(t float64) uint8 {
		t = math.Mod(t+1, 1)
		var v float64
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 0.5:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		default:
			v = p
		}
		return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255))
	}
	return rgb3(f(h+1.0/3), f(h), f(h-1.0/3))
}

// ---- the saved choice ----

// Appearance is the look every app uses: a theme, an accent, a background,
// how light the backdrop is and how much it moves. It is one file for all
// the apps, so a change in one shows in the others. Empty or unknown
// values mean the defaults.
type Appearance struct {
	Theme      string `json:"theme"`
	Accent     string `json:"accent,omitempty"` // an AccentHues ID; "theme": the theme's own
	Background string `json:"background"`
	// Brightness is the backdrop's: "dim", "normal" or "bright".
	Brightness string `json:"brightness,omitempty"`
	// Motion is "off", "calm", "normal" or "lively".
	Motion string `json:"motion,omitempty"`
	// Still is the older way to say Motion "off": it counts when Motion
	// is empty, and is kept in step with Motion, so files stay readable
	// by apps built before Motion.
	Still bool `json:"still,omitempty"`
}

// appearancePath is the shared file: App/.minideck/appearance.json (next
// to the apps' folders), or MINIDECK_APPEARANCE.
func appearancePath() string {
	if p := os.Getenv("MINIDECK_APPEARANCE"); p != "" {
		return p
	}
	return filepath.Join(filepath.Dir(appdir.Dir()), ".minideck", "appearance.json")
}

// LoadAppearance reads the saved look. A missing or damaged file, or a
// name this version doesn't know, gives the default for that part.
// MINIDECK_THEME, MINIDECK_ACCENT and MINIDECK_BACKGROUND override the
// file (for screenshots); MINIDECK_SKY=0 keeps the background still.
func LoadAppearance() Appearance {
	var a Appearance
	if b, err := os.ReadFile(appearancePath()); err == nil {
		json.Unmarshal(b, &a)
	}
	if v := os.Getenv("MINIDECK_THEME"); v != "" {
		a.Theme = v
	}
	if v := os.Getenv("MINIDECK_ACCENT"); v != "" {
		a.Accent = v
	}
	if v := os.Getenv("MINIDECK_BACKGROUND"); v != "" {
		a.Background = v
	}
	if os.Getenv("MINIDECK_SKY") == "0" {
		a.Motion, a.Still = "off", true
	}
	return a.normal()
}

// normal replaces unknown names with the defaults, and sets Still from
// Motion (or Motion from Still, for an older file or caller).
func (a Appearance) normal() Appearance {
	a.Theme = themeByID(a.Theme).ID
	a.Accent = accentByID(a.Accent).ID
	a.Background = sceneByID(a.Background).ID
	a.Brightness = optionByID(brightnesses, a.Brightness, 1).ID
	if a.Motion == "" && a.Still {
		a.Motion = "off"
	}
	a.Motion = optionByID(motions, a.Motion, 2).ID
	a.Still = a.Motion == "off"
	return a
}

// SaveAppearance writes the look for every app.
func SaveAppearance(a Appearance) error {
	a = a.normal()
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	p := appearancePath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return atomicfile.Write(p, append(b, '\n'), 0644)
}

// current is the look in use. Only the app's main goroutine changes it,
// with the sky stopped (see SetAppearance); the sky reads it.
var current struct {
	sync.Mutex
	a Appearance
}

// CurrentAppearance is the look in use.
func CurrentAppearance() Appearance {
	current.Lock()
	defer current.Unlock()
	return current.a
}

// applyAppearance sets the package colours and drops the cached
// background. The sky must not be running.
func applyAppearance(a Appearance) {
	a = a.normal()
	p := lookPalette(a)
	BG, Surface, Raised, Line = p.bg, p.surface, p.raised, p.line
	Text, Muted, Faint = p.text, p.muted, p.faint
	Ink, OnFocus, OnFocus2, Accent = p.ink, p.ink, p.onFocus2, p.accent
	titleShadow = p.titleShadow
	nightRamp, focusRamp, keyRamp, key2Ramp, accentRamp = p.night, p.focus, p.key, p.key2, p.accentR
	bgLevels, starColors, wellColors = p.levels, p.stars, p.wells
	current.Lock()
	current.a = a
	current.Unlock()
	resetAmbient()
}

// SetAppearance switches the look while the app runs: the colours, the
// background and its motion change at once. Nothing is saved; see
// SaveAppearance. The next frame must be drawn whole (StaticFrame and
// Background do that).
func (s *Screen) SetAppearance(a Appearance) {
	s.stopSky()
	s.skyMu.Lock()
	applyAppearance(a)
	s.static = nil
	// every page gets a full copy of the next frame
	for i := range s.shadow {
		s.shadow[i] = nil
	}
	s.skyMu.Unlock()
	s.startSky()
}
