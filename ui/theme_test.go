package ui

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// luminance is the WCAG relative luminance of c.
func luminance(c color.RGBA) float64 {
	f := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c.R) + 0.7152*f(c.G) + 0.0722*f(c.B)
}

func contrast(a, b color.RGBA) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// withAppearance applies a for the test and puts the default back after.
func withAppearance(t testing.TB, a Appearance) {
	applyAppearance(a)
	t.Cleanup(func() { applyAppearance(Appearance{}) })
}

// pairs are the text and surface colours the interface draws together,
// with the least contrast each needs: 4.5 for text, 3 for large or
// secondary marks.
func pairs() []struct {
	name     string
	fg, bg   color.RGBA
	minRatio float64
} {
	card, dialog, selCard := panelRamp(0.10).base, panelRamp(0.13).base, panelRamp(0.16).base
	return []struct {
		name     string
		fg, bg   color.RGBA
		minRatio float64
	}{
		{"Text on Surface", Text, Surface, 4.5},
		{"Text on Raised", Text, Raised, 4.5},
		{"Text on a card", Text, card, 4.5},
		{"Text on a selected card", Text, selCard, 4.5},
		{"Text on a key", Text, keyRamp.base, 4.5},
		{"Text on the background", Text, bgLevels[3], 4.5},
		{"Muted on Surface", Muted, Surface, 4.5},
		{"Muted on a card", Muted, card, 4.5},
		{"Muted on a dialog", Muted, dialog, 4.5},
		{"Muted on the background", Muted, bgLevels[3], 4.5},
		{"Faint on Surface", Faint, Surface, 4.5}, // apps draw small text in it
		{"Faint on a card", Faint, card, 4.5},
		{"Accent on Surface", Accent, Surface, 4.5},
		{"Accent on the background", Accent, bgLevels[3], 4.5},
		{"OnFocus on the selection", OnFocus, focusRamp.base, 4.5},
		{"OnFocus2 on the selection", OnFocus2, focusRamp.base, 4.5}, // a selected row's subtitle
		{"Ink on the selection's light edge", Ink, focusRamp.hi, 4.5},
		{"Good on Surface", Good, Surface, 3},
		{"Warn on Surface", Warn, Surface, 3},
		{"Bad on Surface", Bad, Surface, 4.5}, // error lines are small text
		{"Good on a card", Good, card, 3},
		{"Bad on a card", Bad, card, 4.5},
		{"Bad on the background", Bad, bgLevels[3], 4.5},
	}
}

// Every theme, with every accent and every backdrop brightness.
func TestThemeContrast(t *testing.T) {
	for _, th := range Themes {
		for _, ac := range AccentHues {
			for _, br := range brightnesses {
				withAppearance(t, Appearance{Theme: th.ID, Accent: ac.ID, Brightness: br.ID})
				look := th.Name + "/" + ac.Name + "/" + br.Name
				for _, p := range pairs() {
					if r := contrast(p.fg, p.bg); r < p.minRatio {
						t.Errorf("%s: %s is %.2f:1, want at least %.1f:1 (%v on %v)", look, p.name, r, p.minRatio, p.fg, p.bg)
					}
				}
				// the selection stands out from a panel
				if contrast(focusRamp.base, Surface) < 3 {
					t.Errorf("%s: the selection is hard to tell from a card", look)
				}
			}
		}
	}
}

// On a dim panel (the backlight at half in a lit room) the screen's light
// halves but the room's glare, 0.05 of full white, stays, so contrast drops.
// Text, Muted and a selected row's subtitle still read at 4.5:1 there.
func TestDimPanelContrast(t *testing.T) {
	dim := func(a, b color.RGBA) float64 {
		la, lb := luminance(a)/2+0.05, luminance(b)/2+0.05
		return max(la, lb) / min(la, lb)
	}
	for _, th := range Themes {
		for _, ac := range AccentHues {
			for _, br := range brightnesses {
				withAppearance(t, Appearance{Theme: th.ID, Accent: ac.ID, Brightness: br.ID})
				card, dialog := panelRamp(0.10).base, panelRamp(0.13).base
				for _, p := range []struct {
					name     string
					fg, bg   color.RGBA
					minRatio float64
				}{
					{"Text on Surface", Text, Surface, 4.5},
					{"Text on a key", Text, keyRamp.base, 4.5},
					{"Muted on Surface", Muted, Surface, 4.5},
					{"Muted on a card", Muted, card, 4.5},
					{"Muted on a dialog", Muted, dialog, 4.5},
					{"Muted on the background", Muted, bgLevels[3], 4.5},
					{"OnFocus2 on the selection", OnFocus2, focusRamp.base, 4.5},
					{"Faint on Surface", Faint, Surface, 3},
					{"Bad on a card", Bad, card, 3},
				} {
					if r := dim(p.fg, p.bg); r < p.minRatio {
						t.Errorf("%s/%s/%s, dim panel: %s is %.2f:1, want %.1f:1", th.Name, ac.Name, br.Name, p.name, r, p.minRatio)
					}
				}
			}
		}
	}
}

// Brightness changes the backdrop only, darker or lighter, and its stars
// stay lighter than it.
func TestBrightness(t *testing.T) {
	for _, th := range Themes {
		withAppearance(t, Appearance{Theme: th.ID})
		normal, surface, stars := bgLevels, Surface, starColors
		withAppearance(t, Appearance{Theme: th.ID, Brightness: "dim"})
		dim, dimStars := bgLevels, starColors
		withAppearance(t, Appearance{Theme: th.ID, Brightness: "bright"})
		if Surface != surface {
			t.Errorf("%s: Bright changed the panels", th.Name)
		}
		for i := range normal {
			if !(luminance(dim[i]) <= luminance(normal[i]) && luminance(normal[i]) < luminance(bgLevels[i])) {
				t.Errorf("%s: level %d is %v dim, %v normal, %v bright", th.Name, i, dim[i], normal[i], bgLevels[i])
			}
		}
		for _, l := range []struct {
			name  string
			level color.RGBA
			star  color.RGBA
		}{{"dim", dim[3], dimStars[0]}, {"normal", normal[3], stars[0]}, {"bright", bgLevels[3], starColors[0]}} {
			if luminance(l.star) < 1.6*luminance(l.level) {
				t.Errorf("%s/%s: dim stars %v barely show on %v", th.Name, l.name, l.star, l.level)
			}
		}
	}
}

// The Night theme is the palette as designed, unchanged.
func TestNightIsTheDefault(t *testing.T) {
	withAppearance(t, Appearance{Theme: "nonsense"})
	if Surface != rgb(0x282b4e) || Accent != rgb(0xbec2f8) || Ink != rgb3(27, 29, 58) || bgLevels[0] != rgb3(12, 13, 28) {
		t.Error("an unknown theme didn't fall back to Night's colours")
	}
	if CurrentAppearance().Theme != "night" || CurrentAppearance().Background != "stars" {
		t.Errorf("unknown names gave %+v", CurrentAppearance())
	}
}

// Accents differ from each other in every theme, the theme's own leaves
// its colours as they are, and an accent changes only the accent family.
func TestAccents(t *testing.T) {
	for _, th := range Themes {
		seen := map[color.RGBA]string{}
		for _, ac := range AccentHues {
			p := lookPalette(Appearance{Theme: th.ID, Accent: ac.ID})
			own := th.palette()
			if ac.own {
				if p != own {
					t.Errorf("%s: the theme's own accent changed its colours", th.Name)
				}
				continue
			}
			if prev, ok := seen[p.focus.base]; ok {
				t.Errorf("%s: %s has the same selection as %s", th.Name, ac.Name, prev)
			}
			seen[p.focus.base] = ac.Name
			if p.surface != own.surface || p.text != own.text || p.levels != own.levels || p.ink != own.ink {
				t.Errorf("%s/%s: the accent changed more than the accent family", th.Name, ac.Name)
			}
		}
	}
}

// Themes differ from each other, and each sets every colour.
func TestThemesDistinct(t *testing.T) {
	seen := map[color.RGBA]string{}
	for _, th := range Themes {
		p := th.palette()
		if prev, ok := seen[p.accent]; ok {
			t.Errorf("%s has the same accent as %s", th.Name, prev)
		}
		seen[p.accent] = th.Name
		for _, c := range []color.RGBA{p.bg, p.surface, p.text, p.ink, p.focus.base, p.levels[0]} {
			if c.A != 255 {
				t.Errorf("%s leaves a colour unset", th.Name)
			}
		}
	}
}

func TestHSLRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		c := rgb3(uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)))
		h, s, l := toHSL(c)
		got := fromHSL(h, s, l)
		d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
		if d(got.R, c.R) > 1 || d(got.G, c.G) > 1 || d(got.B, c.B) > 1 {
			t.Fatalf("%v -> %v %v %v -> %v", c, h, s, l, got)
		}
	}
}

// Every theme and scene: the background is 2x2 blocks of the theme's
// background and mark colours only.
func TestEveryBackground(t *testing.T) {
	for _, th := range Themes {
		for _, sc := range Scenes {
			withAppearance(t, Appearance{Theme: th.ID, Background: sc.ID})
			img := ambient()
			allowed := map[color.RGBA]bool{starColors[0]: true, starColors[1]: true}
			for _, c := range bgLevels {
				allowed[c] = true
			}
			for y := 0; y < H; y += 3 {
				for x := 0; x < W; x += 3 {
					i := img.PixOffset(x, y)
					c := color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
					if !allowed[c] {
						t.Fatalf("%s/%s: (%d,%d) is %v", th.ID, sc.ID, x, y, c)
					}
					j := img.PixOffset(x&^1, y&^1)
					if !bytes.Equal(img.Pix[i:i+4], img.Pix[j:j+4]) {
						t.Fatalf("%s/%s: (%d,%d) differs from its 2x2 block", th.ID, sc.ID, x, y)
					}
				}
			}
		}
	}
}

// Every scene, run for a long time on a screen with a panel: the panel is
// never touched, the number of sprites stays bounded, every sprite stays
// within reach of the screen, and once they finish the pages are the
// plain background again.
func TestScenesStress(t *testing.T) {
	ticks := 3000 // 200 s at 15 a second
	if testing.Short() {
		ticks = 600
	}
	for _, look := range []struct{ theme, motion string }{
		{"night", "normal"}, {"ember", "normal"}, {"contrast", "normal"}, {"neon", "lively"}, {"sand", "calm"},
	} {
		th := look.theme + "/" + look.motion
		for _, sc := range Scenes {
			if sc.spawn == nil {
				continue
			}
			withAppearance(t, Appearance{Theme: look.theme, Background: sc.ID, Motion: look.motion})
			s := fakeScreen(2)
			panel := image.Rect(100, 100, 400, 300)
			for y := panel.Min.Y; y < panel.Max.Y; y++ {
				for x := panel.Min.X; x < panel.Max.X; x++ {
					i := (y*W + x) * 4
					copy(s.shadow[0][i:i+4], []byte{Surface.R, Surface.G, Surface.B, 255})
				}
			}
			copyRotated(s.mem[:W*H*4], s.shadow[0])
			k := newSky(s, sc, rand.New(rand.NewPCG(uint64(len(sc.ID)), 3)))
			k.density, k.dt = motionPace(look.motion)
			now := time.Now()
			k.next = now
			moved, peak := 0, 0
			for i := range ticks {
				k.spawn(now.Add(time.Duration(i) * skyTick))
				k.apply(k.frame())
				peak = max(peak, len(k.sprites))
				if len(k.drawn) > 0 {
					moved++
				}
				if i%50 == 0 {
					for _, pt := range []image.Point{panel.Min, {250, 200}, panel.Max.Sub(image.Pt(1, 1))} {
						if got := fbAt(s, 0, pt.X, pt.Y); got != [3]byte{Surface.R, Surface.G, Surface.B} {
							t.Fatalf("%s/%s tick %d: panel pixel %v became %v", th, sc.ID, i, pt, got)
						}
					}
				}
				if len(k.drawn) > 1500 {
					t.Fatalf("%s/%s tick %d: %d cells at once", th, sc.ID, i, len(k.drawn))
				}
			}
			if peak > maxSprites {
				t.Errorf("%s/%s: %d sprites at once, cap %d", th, sc.ID, peak, maxSprites)
			}
			if moved < ticks/4 {
				t.Errorf("%s/%s: moved on only %d of %d ticks", th, sc.ID, moved, ticks)
			}
			// everything finishes: no new sprites, and within 80 s of ticks
			// at Normal motion (the slowest snowflake takes about 65 s to
			// fall, the longest cloud lasts 53 s) the sky is empty
			k.spawnFn = nil
			for i := 0; len(k.sprites) > 0; i++ {
				if float64(i)*k.dt > 1200 {
					t.Fatalf("%s/%s: %d sprites never finish (%T)", th, sc.ID, len(k.sprites), k.sprites[0])
				}
				k.apply(k.frame())
			}
			k.apply(k.frame())
			for p := range 2 {
				want := make([]byte, W*H*4)
				copyRotated(want, s.shadow[p])
				if !bytes.Equal(s.mem[p*W*H*4:(p+1)*W*H*4], want) {
					t.Errorf("%s/%s: page %d not restored", th, sc.ID, p)
				}
			}
		}
	}
}

func TestAppearanceFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "appearance.json")
	t.Setenv("MINIDECK_APPEARANCE", p)
	t.Setenv("MINIDECK_THEME", "")
	t.Setenv("MINIDECK_BACKGROUND", "")
	t.Setenv("MINIDECK_SKY", "")
	t.Setenv("MINIDECK_ACCENT", "")
	def := Appearance{Theme: "night", Accent: "theme", Background: "stars", Brightness: "normal", Motion: "normal"}
	if a := LoadAppearance(); a != def {
		t.Errorf("no file: %+v", a)
	}
	for _, want := range []Appearance{
		{Theme: "ember", Accent: "coral", Background: "rain", Brightness: "dim", Motion: "off", Still: true},
		{Theme: "neon", Accent: "theme", Background: "aurora", Brightness: "bright", Motion: "lively"},
		{Theme: "deepsea", Accent: "silver", Background: "warp", Brightness: "normal", Motion: "calm"},
	} {
		if err := SaveAppearance(want); err != nil {
			t.Fatal(err)
		}
		if a := LoadAppearance(); a != want {
			t.Errorf("saved %+v, read %+v", want, a)
		}
	}
	// what an app built before Motion reads: still is written with it
	SaveAppearance(Appearance{Motion: "off"})
	var old struct{ Still bool }
	if b, _ := os.ReadFile(p); json.Unmarshal(b, &old) != nil || !old.Still {
		t.Errorf("Motion off doesn't write still: %s", b)
	}
	for _, c := range []struct {
		file string
		want Appearance
	}{
		// names from a newer version, or a damaged file: the defaults
		{`{"theme":"lava","accent":"ultraviolet","background":"volcano","brightness":"blinding","motion":"frantic"}`, def},
		{"{not json", def},
		// a file from before accents, brightness and motion levels
		{`{"theme":"ocean","background":"snow"}`, Appearance{Theme: "ocean", Accent: "theme", Background: "snow", Brightness: "normal", Motion: "normal"}},
		{`{"theme":"ocean","background":"snow","still":true}`, Appearance{Theme: "ocean", Accent: "theme", Background: "snow", Brightness: "normal", Motion: "off", Still: true}},
		// Motion, when it is set, counts over still
		{`{"motion":"calm","still":true}`, Appearance{Theme: "night", Accent: "theme", Background: "stars", Brightness: "normal", Motion: "calm"}},
		{`{"motion":"bogus","still":true}`, def},
	} {
		os.WriteFile(p, []byte(c.file), 0644)
		if a := LoadAppearance(); a != c.want {
			t.Errorf("%s: read %+v, want %+v", c.file, a, c.want)
		}
	}
	t.Setenv("MINIDECK_THEME", "ocean")
	t.Setenv("MINIDECK_ACCENT", "lime")
	t.Setenv("MINIDECK_SKY", "0")
	if a := LoadAppearance(); a.Theme != "ocean" || a.Accent != "lime" || a.Motion != "off" || !a.Still {
		t.Errorf("environment overrides: %+v", a)
	}
}

// An older caller that sets only Still still stops the background.
func TestStillAlone(t *testing.T) {
	s := fakeScreen(2)
	defer withAppearance(t, Appearance{})
	s.SetAppearance(Appearance{Theme: "ocean", Still: true})
	if s.sky != nil {
		t.Error("the sky runs with Still set")
	}
	if a := CurrentAppearance(); a.Motion != "off" {
		t.Errorf("Still gave %+v", a)
	}
	s.SetAppearance(Appearance{Theme: "ocean", Motion: "calm"})
	if s.sky == nil || s.sky.density != 0.5 {
		t.Errorf("Calm: sky %+v", s.sky)
	}
	s.stopSky()
}

// Switching the look while the sky runs, many times over: no race (run
// with -race), and the sky ends up running the new scene.
func TestSetAppearanceWhileRunning(t *testing.T) {
	s := fakeScreen(2)
	defer withAppearance(t, Appearance{})
	applyAppearance(Appearance{})
	s.startSky()
	r := rand.New(rand.NewPCG(4, 4))
	for range 60 {
		a := Appearance{Theme: Themes[r.IntN(len(Themes))].ID, Accent: AccentHues[r.IntN(len(AccentHues))].ID,
			Background: Scenes[r.IntN(len(Scenes))].ID, Brightness: brightnesses[r.IntN(len(brightnesses))].ID,
			Motion: motions[r.IntN(len(motions))].ID}
		a.Still = a.Motion == "off"
		s.SetAppearance(a)
		s.Background()
		time.Sleep(5 * time.Millisecond)
		moving := !a.Still && sceneByID(a.Background).spawn != nil
		if (s.sky != nil) != moving {
			t.Fatalf("%+v: sky running = %v", a, s.sky != nil)
		}
		if got := CurrentAppearance(); got != a {
			t.Fatalf("current %+v, want %+v", got, a)
		}
		i := s.PixOffset(0, 0)
		if c := (color.RGBA{s.Pix[i], s.Pix[i+1], s.Pix[i+2], 255}); c != bgCell(0, 0) {
			t.Fatalf("%+v: background not redrawn in the new colours", a)
		}
	}
	s.stopSky()
}

// The Appearance screen: A and the arrows change the selected setting at
// once, B saves for every app.
func TestAppearanceScreen(t *testing.T) {
	t.Setenv("MINIDECK_APPEARANCE", filepath.Join(t.TempDir(), "appearance.json"))
	t.Setenv("MINIDECK_UI_NULL", "1")
	s, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer withAppearance(t, Appearance{})
	in := make(chan Event, 16)
	// theme +2; accent -1 (the last); background -1 (the last, Plain);
	// brightness +1; motion +1 (Plain doesn't move: no change)
	for _, b := range []Button{A, A, Down, Left, Down, Left, Down, A, Down, Right, B} {
		in <- Event{Button: b, Pressed: true}
	}
	s.Appearance(in)
	want := Appearance{Theme: Themes[2].ID, Accent: AccentHues[len(AccentHues)-1].ID, Background: Scenes[len(Scenes)-1].ID,
		Brightness: "bright", Motion: "normal"}
	if got := CurrentAppearance(); got != want {
		t.Errorf("in use %+v, want %+v", got, want)
	}
	if got := LoadAppearance(); got != want {
		t.Errorf("saved %+v, want %+v", got, want)
	}
	if it, w := AppearanceItem(), Themes[2].Name+" · "+AccentHues[len(AccentHues)-1].Name+" · "+Scenes[len(Scenes)-1].Name+" (bright)"; it.Sub != w {
		t.Errorf("settings row says %q, want %q", it.Sub, w)
	}
	// on a moving background, Motion goes round Off, Calm, Normal, Lively
	in = make(chan Event, 16)
	for _, b := range []Button{Down, Down, Right, Down, Left, Down, Left, B} {
		in <- Event{Button: b, Pressed: true}
	}
	s.Appearance(in)
	if got := CurrentAppearance(); got.Background != "stars" || got.Brightness != "normal" || got.Motion != "calm" {
		t.Errorf("in use %+v", got)
	}
	if it, w := AppearanceItem(), Themes[2].Name+" · "+AccentHues[len(AccentHues)-1].Name+" · Stars (calm)"; it.Sub != w {
		t.Errorf("settings row says %q, want %q", it.Sub, w)
	}
}

// Every row of the Appearance screen goes round all its choices and back.
func TestAppearanceChange(t *testing.T) {
	a := Appearance{}.normal()
	for row, n := range []int{len(Themes), len(AccentHues), len(Scenes), len(brightnesses), len(motions)} {
		b := a
		seen := map[Appearance]bool{}
		for range n {
			seen[b] = true
			b = b.change(row, 1).normal()
		}
		if b != a || len(seen) != n {
			t.Errorf("row %d: %d choices, back to %+v", row, len(seen), b)
		}
		if a.change(row, 1).change(row, -1) != a {
			t.Errorf("row %d: left doesn't undo right", row)
		}
	}
}

func TestWifiBars(t *testing.T) {
	head := "Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE\n" +
		" face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22\n"
	for _, c := range []struct {
		line string
		want int
	}{
		{" wlan0: 0000   62.  -48.  -256        0      0      0      0      0        0\n", 3},
		{" wlan0: 0000   31.  -79.  -256        0      0      0      0      0        0\n", 2},
		{" wlan0: 0000   12.  -98.  -256        0      0      0      0      0        0\n", 1},
		{"", 3}, // no line for wlan0: connected, strength unknown
	} {
		if got := wifiBars(head + c.line); got != c.want {
			t.Errorf("%q: %d bars, want %d", c.line, got, c.want)
		}
	}
}

func TestAxpCharging(t *testing.T) {
	for out, want := range map[string]bool{
		`{"battery":97, "voltage":4174, "charging":3}`:  true,
		`{"battery":97, "voltage":4174, "charging":0}`:  false,
		`{"battery":97, "voltage":4174, "charging":33}`: false,
		"error": false,
	} {
		if got := axpCharging(out); got != want {
			t.Errorf("%s: %v", out, got)
		}
	}
}
