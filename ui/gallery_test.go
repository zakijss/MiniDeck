package ui

import (
	"image"
	"image/draw"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font"
)

// TestGallery draws the package's own screens (keyboard, alert, form,
// chooser, loading, gauges, ticker) to DIR/ui-*.png for review. It runs
// only with MINIDECK_GALLERY=DIR set.
func TestGallery(t *testing.T) {
	dir := os.Getenv("MINIDECK_GALLERY")
	if dir == "" {
		t.Skip("set MINIDECK_GALLERY=DIR to draw the gallery")
	}
	os.MkdirAll(dir, 0o755)
	// a status bar as on the device: battery, Wi-Fi and MiniScale on
	status.mu.Lock()
	status.read, status.battery, status.charging, status.wifi, status.tailnet, status.loc = time.Now().Add(time.Hour), 76, false, 2, true, time.UTC
	status.mu.Unlock()
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	shot := func(name string) {
		f, err := os.Create(filepath.Join(dir, "ui-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, s.RGBA)
		f.Close()
	}

	// the status bar in each state, one under another (actual size)
	bars := image.NewRGBA(image.Rect(0, 0, W, 8*52))
	for i, st := range []statusInfo{
		{battery: 76, wifi: 3, tailnet: true}, {battery: 76, wifi: 2, tailnet: true}, {battery: 54, wifi: 1, tailnet: false},
		{battery: 28, wifi: 0, tailnet: true}, {battery: 9, wifi: -1, tailnet: false}, {battery: 41, charging: true, wifi: 3, tailnet: true},
		{battery: 100, charging: true, wifi: 3, tailnet: true}, {battery: -1, wifi: 3, tailnet: true},
	} {
		status.mu.Lock()
		status.battery, status.charging, status.wifi, status.tailnet = st.battery, st.charging, st.wifi, st.tailnet
		status.mu.Unlock()
		s.Background()
		s.StatusBar()
		draw.Draw(bars, image.Rect(0, i*52, W, (i+1)*52), s.RGBA, image.Point{}, draw.Src)
	}
	status.mu.Lock()
	status.battery, status.charging, status.wifi, status.tailnet = 76, false, 2, true
	status.mu.Unlock()
	if f, err := os.Create(filepath.Join(dir, "ui-status.png")); err == nil {
		png.Encode(f, bars)
		f.Close()
	}
	present := func(name string) { s.pngPath = filepath.Join(dir, "ui-"+name+".png") }

	// keyboard
	present("keyboard")
	keys := keysFor(0, false, false)
	s.drawPrompt("Repository address", []rune("https://github.com/example"), 26, "An https:// address of a repository", false, false, keys, 2, 3, 0)

	// keyboard with a secret
	present("keyboard-secret")
	keys = keysFor(1, true, false)
	s.drawPrompt("Password", []rune("hunter22"), 8, "", true, false, keys, 4, 1, 1)

	// alert
	present("alert")
	s.Alert("End minirack?", "It stops at once. Anything it hasn't saved is lost.", "End", "Back")

	// form
	fm := &Form{Title: "Add a server", Intro: "MiniRack reads Glances, Beszel or Vantage.", Fields: []*Field{
		{Label: "Type", Value: "Beszel", Options: []string{"Glances", "Beszel", "Vantage"}},
		{Label: "Name", Value: "homeserver"},
		{Label: "Server", Value: "", Placeholder: "http://name:8090", Hint: "The address of your server"},
		{Label: "Password", Value: "secret", Secret: true},
	}}
	present("form")
	fm.draw(s, fm.rows(), 1, "")

	// chooser (a list with icons)
	s.pngPath = ""
	items := []Item{
		{Label: "Type the address", Sub: "Use the keyboard", Icon: "list", Tint: Muted},
		{Label: "homeserver", Sub: "100.64.0.2 · linux", Icon: "server", Tint: Accent},
		{Label: "laptop", Sub: "offline", Icon: "server", Tint: Faint},
		{Label: "gaming-pc", Sub: "100.64.0.9 · windows", Icon: "server", Tint: Accent},
	}
	l := List{Items: items, Sel: 1}
	top := s.StaticFrame("Pick the server", "", Hint{"A", "Choose"}, Hint{"B", "Back"})
	l.Draw(s, image.Rect(20, top, W-26, H-58))
	shot("choose")

	// plain rows (no icons), with details
	var plain []Item
	for _, n := range []string{"Wi-Fi", "Bluetooth", "Sleep after", "Brightness", "Volume", "Language", "Clock"} {
		plain = append(plain, Item{Label: n, Detail: "On"})
	}
	l = List{Items: plain, Sel: 2}
	top = s.StaticFrame("Settings", "Changes save at once", Hint{"A", "Change"}, Hint{"L1", "Prev"}, Hint{"R1", "Next"}, Hint{"START", "Save"}, Hint{"B", "Back"})
	l.Draw(s, image.Rect(20, top, W-26, H-58))
	shot("plain-list")

	// loading, gauges, bar, ring, capsules
	s.Background()
	top = s.Header("Widgets", "Loading, gauges, bars and buttons")
	s.Loading(top+4, "Loading")
	for i, v := range []float64{35, 78, 93, -1} {
		s.Gauge(70+i*95, top+90, 30, v, []string{"CPU", "MEMORY", "DISK", "TEMP"}[i])
	}
	s.Bar(image.Rect(24, top+160, 300, top+166), 0.62, Text)
	s.Bar(image.Rect(24, top+180, 300, top+190), 0.3, Good)
	s.Ring(480, float64(top+90), 42, 6, 0.7, Text)
	s.TextCenter(480, top+90-LineHeight(Heading)/2, "21", Heading, Text)
	s.Capsule(image.Rect(330, top+150, 610, top+200), "Save", true, Text)
	s.Capsule(image.Rect(330, top+210, 610, top+260), "Remove", false, Bad)
	s.Panel(image.Rect(24, top+210, 300, top+300), 18)
	s.Text(40, top+222, "A panel", Heading, Text)
	s.Text(40, top+250, "with some small text · 47°C", Small, Muted)
	s.Footer(Hint{"A", "Open"}, Hint{"X", "Refresh"}, Hint{"Y", "Settings"}, Hint{"SELECT", "Show"}, Hint{"B", "Exit"})
	shot("widgets")

	// icons, plain and on tiles
	s.Background()
	top = s.Header("Icons", "Every glyph, at 24, 36 and on a tile")
	names := []string{"play", "pause", "prev", "next", "note", "disc", "dice", "chart", "clock", "star", "person", "list", "shuffle",
		"gear", "plus", "server", "check", "cross", "wifi", "chev-left", "chev-right", "chev-up", "chev-down", "eye", "backspace"}
	for i, n := range names {
		x, y := 24+(i%13)*46, top+(i/13)*150
		s.Icon(n, x, y, 24, Text)
		s.Icon(n, x-6, y+34, 36, Accent)
		s.tile(n, image.Rect(x-8, y+80, x+36, y+124), iconColors[n])
	}
	shot("icons")

	// type specimen
	s.Background()
	y := 12
	for _, sp := range []struct {
		f    font.Face
		text string
	}{{Display, "Display 123 456"}, {Title, "Title Running"}, {Big, "Big 42%"}, {Heading, "Heading Featured themes"},
		{Body, "Body The quick brown fox jumps"}, {Small, "Small Over 200 from the community"}, {Caps, "CAPS NOW PLAYING · UP NEXT"}} {
		s.Text(12, y, sp.text, sp.f, Text)
		y += LineHeight(sp.f) + 6
	}
	s.Text(12, y, "Accents: é è à ü ö ç ñ É ß · — – … • › ‹ ° “quotes” ✓ ✗ ← → ↑ ↓ ♥", Small, Text)
	y += LineHeight(Small) + 4
	s.Text(12, y, "Fallback: Привет мир · Ελληνικά", Body, Text)
	shot("type")

	// the overlay strip over a theme's top bar (as Overlay draws it)
	o := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, OverlayH))}
	bg := image.NewRGBA(image.Rect(0, 0, W, H))
	for i := 0; i < len(bg.Pix); i += 4 {
		bg.Pix[i], bg.Pix[i+1], bg.Pix[i+2], bg.Pix[i+3] = 40, 40, 44, 255
	}
	o.bgID, o.ticker, o.tickerCompact, o.tickerW = bg, "Northbound   ·   Paper Satellites   ·   Low Orbit", true, OverlayW
	o.DrawTicker()
	s.Fill(s.Rect, rgb3(40, 40, 44))
	copy(s.Pix, o.Pix)
	shot("overlay")

	// ticker
	s.SetTicker("Northbound   ·   Paper Satellites   ·   Low Orbit")
	l = List{Items: listItems(), Sel: 0}
	top = s.StaticFrame("MiniMart", "Themes, icon packs, apps and games", Hint{"A", "Open"}, Hint{"B", "Exit"})
	l.Draw(s, image.Rect(20, top, W-26, H-58))
	shot("ticker")
	s.SetTicker("")

	intlGallery(s, shot)

	// Appearance, and a list, in every theme, each over its own background
	for i, th := range Themes {
		sc := Scenes[i%len(Scenes)]
		applyAppearance(Appearance{Theme: th.ID, Background: sc.ID})
		s.static = nil
		present("appearance-" + th.ID)
		in := make(chan Event, 1)
		in <- Event{Button: B, Pressed: true}
		s.Appearance(in)
		l = List{Items: listItems(), Sel: 1}
		top = s.StaticFrame("MiniMart", "Themes, icon packs, apps and games", Hint{"A", "Open"}, AppearanceHint, Hint{"B", "Exit"})
		l.Draw(s, image.Rect(20, top, W-26, H-58))
		shot("list-" + th.ID)
	}
	// some accents and brightnesses, and the Appearance screen scrolled
	// down to Motion
	for _, a := range []Appearance{
		{Theme: "night", Accent: "coral", Background: "clouds", Brightness: "bright"},
		{Theme: "slate", Accent: "lime", Background: "aurora", Brightness: "dim"},
		{Theme: "contrast", Accent: "pink", Background: "meteors", Brightness: "bright"},
	} {
		applyAppearance(a)
		s.static = nil
		name := a.Theme + "-" + a.Accent + "-" + a.Brightness
		present("appearance-" + name)
		in := make(chan Event, 6)
		for _, b := range []Button{Down, Down, Down, Down, B} {
			in <- Event{Button: b, Pressed: true}
		}
		s.Appearance(in)
		l = List{Items: listItems(), Sel: 2}
		top = s.StaticFrame("MiniMart", "Themes, icon packs, apps and games", Hint{"A", "Open"}, AppearanceHint, Hint{"B", "Exit"})
		l.Draw(s, image.Rect(20, top, W-26, H-58))
		shot("list-" + name)
	}
	s.pngPath = ""
	// each moving background after 20 s, its sprites on the plain backdrop
	// (on a device they show only around the panels)
	for _, sc := range Scenes {
		if sc.spawn == nil {
			continue
		}
		applyAppearance(Appearance{Background: sc.ID})
		f := fakeScreen(1)
		k := newSky(f, sc, rand.New(rand.NewPCG(1, 2)))
		now := time.Now()
		k.next = now.Add(10 * time.Second)
		for i := range 300 {
			k.spawn(now.Add(time.Duration(i) * skyTick))
			k.apply(k.frame())
		}
		for y := range H {
			for x := range W {
				c := fbAt(f, 0, x, y)
				i := s.PixOffset(x, y)
				s.Pix[i], s.Pix[i+1], s.Pix[i+2], s.Pix[i+3] = c[0], c[1], c[2], 255
			}
		}
		shot("scene-" + sc.ID)
	}
	applyAppearance(Appearance{})
}
