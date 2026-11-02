package ui

import (
	"image"
	"image/color"
	"strings"
	"time"
)

// ---- status bar: clock, MiniScale, Wi-Fi, battery ----

// The status bar's glyphs, drawn like the app icons: solid colours with a
// hard shadow one art pixel down and right.
var (
	// Wi-Fi: a dot and three arcs, lit up to the signal's strength.
	wifiGlyph = rowsOf(`
.3333333.
3.......3
..22222..
.2.....2.
...111...
..1...1..
....0....`)
	// MiniScale's mark: three linked nodes, as on its app icon; 'O' is
	// each node's lit middle.
	nodeGlyph = rowsOf(`
...ooo...
...oOo...
...ooo...
...#.#...
..#...#..
ooo...ooo
oOo###oOo
ooo...ooo`)
	// a lightning bolt beside the battery while it charges
	boltGlyph = rowsOf(`
...##
..##.
.####
####.
..##.
.##..
.#...`)
)

// statusGap is the space between the status bar's items.
const statusGap = 12

// glyph draws a status glyph at (x, y) with its hard shadow.
func (s *Screen) glyph(x, y int, rows []string, c color.RGBA, pal map[byte]color.RGBA) {
	black := color.RGBA{0, 0, 0, 0xff}
	for j, row := range rows {
		for i := 0; i < len(row); i++ {
			if row[i] != '.' && row[i] != ' ' {
				s.blendRect(image.Rect(x+(i+1)*art, y+(j+1)*art, x+(i+2)*art, y+(j+2)*art), black, 110)
			}
		}
	}
	s.bitmap(x, y, art, rows, c, pal)
}

// wifiPal colours the Wi-Fi arcs lit up to bars (1-3).
func wifiPal(bars int) map[byte]color.RGBA {
	pal := map[byte]color.RGBA{'0': Text}
	for i := byte(1); i <= 3; i++ {
		pal['0'+i] = Text
		if int(i) > bars {
			pal['0'+i] = Line
		}
	}
	return pal
}

// battery draws the battery, 15 art pixels wide and 8 tall, at (x, y):
// a bevelled case with an ink outline, a cap, and its charge as a bar of
// colour inside.
func (s *Screen) battery(x, y, pct int, charging bool) {
	const w, h = 13, 8
	body := rectShape(image.Rect(x, y, x+w*art, y+h*art), art)
	cp := image.Rect(x+w*art, y+2*art, x+(w+2)*art, y+(h-2)*art)
	s.dropShadow(body, 1, 110)
	s.dropShadow(rectShape(cp, 0), 1, 110)
	s.bevel(body, key2Ramp, Ink)
	s.fillRect(cp, Ink)
	c := greenRamp
	switch {
	case charging:
	case pct <= 15:
		c = redRamp
	case pct <= 30:
		c = orangeRamp
	}
	// the charge: inside the outline, whole art pixels, at least one
	in := image.Rect(x+art, y+art, x+(w-1)*art, y+(h-1)*art)
	n := (in.Dx()/art*min(100, pct) + 50) / 100
	if n > 0 {
		f := image.Rect(in.Min.X, in.Min.Y, in.Min.X+max(1, n)*art, in.Max.Y)
		s.fillRect(f, c.base)
		s.fillRect(image.Rect(f.Min.X, f.Min.Y, f.Max.X, f.Min.Y+art), c.hi)
		s.fillRect(image.Rect(f.Min.X, f.Max.Y-art, f.Max.X, f.Max.Y), c.lo)
	}
}

// StatusBar draws the clock, MiniScale's state, Wi-Fi and the battery at
// the top right, ending at x = W-20, as pixel glyphs centred on one line.
// It returns the left edge it used.
func (s *Screen) StatusBar() int {
	st := readStatusInfo()
	x := W - 20
	cy := 26 + s.inset() // the row's centre line
	if st.battery >= 0 {
		x -= 15 * art
		s.battery(x, cy-4*art, st.battery, st.charging)
		x -= 6
		txt := itoa(st.battery) + "%"
		x -= Measure(Caps, txt)
		s.Text(x, centerY(Caps, cy), txt, Caps, Muted)
		if st.charging {
			x -= 6 + 5*art
			s.glyph(x, cy-7*art/2, boltGlyph, yellowRamp.base, nil)
		}
		x -= statusGap
	}
	if st.wifi >= 0 {
		x -= 9 * art
		pal := wifiPal(st.wifi)
		if st.wifi == 0 {
			pal['0'] = Line
		}
		s.glyph(x, cy-7*art/2, wifiGlyph, Text, pal)
		x -= statusGap
	}
	x -= 9 * art
	if st.tailnet {
		s.glyph(x, cy-4*art, nodeGlyph, lavRamp.hi, map[byte]color.RGBA{'o': skyRamp.base, 'O': whiteRamp.hi})
	} else {
		s.glyph(x, cy-4*art, nodeGlyph, Line, map[byte]color.RGBA{'o': Line, 'O': Line})
	}
	x -= statusGap + 2
	clock := time.Now().In(st.loc).Format("15:04")
	x -= Measure(Heading, clock)
	s.Text(x+art, centerY(Heading, cy)+art, clock, Heading, titleShadow)
	s.Text(x, centerY(Heading, cy), clock, Heading, Text)
	return x
}

// titleShadow is the colour of the drop shadow under screen titles.
var titleShadow = nightPalette.titleShadow

// Header draws a large title (and an optional subtitle under it) with the
// status bar, on the background. It returns the y where content can start.
func (s *Screen) Header(title, sub string) int {
	in := s.inset()
	s.DrawTicker()
	left := s.StatusBar()
	// A title too long for the large size is set a size smaller, on the
	// same line, rather than cut short.
	f, y, sh := Title, 10+in, 3
	if Measure(Title, title) > left-48 {
		f, y, sh = Heading, 16+in, 2
	}
	t := Fit(f, title, left-48)
	s.Text(24, y+sh, t, f, titleShadow)
	s.Text(24, y, t, f, Text)
	if sub != "" {
		s.Text(25, 50+in, Fit(Small, sub, W-50), Small, Muted)
		return 78 + in
	}
	return 58 + in
}

// ---- footer: button hints drawn like the buttons ----

// Hint is a button and what it does, for the footer: {"A", "Open"}.
type Hint struct{ Button, Label string }

// The face buttons in the Super Famicom's colours, and the letter's colour.
var faceButtons = map[string]struct {
	c  ramp
	fg color.RGBA
}{
	"A": {redRamp, whiteRamp.hi},
	"B": {yellowRamp, Ink},
	"X": {skyRamp, Ink},
	"Y": {greenRamp, Ink},
}

// buttonGlyph draws a button as a pixel-art cap: A/B/X/Y as round caps in
// their colours, START, SELECT, L1... as grey pills with small capitals. It
// returns its width.
func (s *Screen) buttonGlyph(x, cy int, b string) int {
	if fb, ok := faceButtons[b]; ok {
		const d = 11 * art // diameter
		sh := diskShape(float64(x)+d/2, float64(cy), d/2-1)
		s.dropShadow(sh, 1, 110)
		s.bevel(sh, fb.c, Ink)
		s.micro(sh.x0+(sh.w-microWidth(b)*art)/2, sh.y0+(sh.h-5*art)/2, b, fb.fg)
		return sh.w
	}
	label := strings.ToUpper(b)
	tw := microWidth(label)
	w := (tw + 8) * art
	r := image.Rect(x, cy-11*art/2, x+w, cy+11*art/2)
	sh := rectShape(r, 12)
	s.dropShadow(sh, 1, 110)
	s.bevel(sh, steelRamp, Ink)
	s.micro(x+4*art, cy-5*art/2, label, whiteRamp.hi)
	return w
}

// hintsWidth is how wide the hints are with gap pixels between them.
func hintsWidth(hints []Hint, gap int) int {
	w := 0
	for i, h := range hints {
		if i > 0 {
			w += gap
		}
		w += buttonWidth(h.Button) + 6 + Measure(Small, h.Label)
	}
	return w
}

func buttonWidth(b string) int {
	if _, ok := faceButtons[b]; ok {
		return 11 * art
	}
	return (microWidth(strings.ToUpper(b)) + 8) * art
}

// Footer draws button hints on a bar along the bottom.
func (s *Screen) Footer(hints ...Hint) {
	bar := image.Rect(12, H-48, W-12, H-6)
	s.block(bar, 12, panelRamp(0.10), Ink, 0)
	room := bar.Dx() - 24
	gap := 20
	for gap > 8 && hintsWidth(hints, gap) > room {
		gap -= 2
	}
	// still too wide: shorten the labels, the longest first
	labels := make([]string, len(hints))
	for i, h := range hints {
		labels[i] = h.Label
	}
	for over := hintsWidth(hints, gap) - room; over > 0; {
		li := 0
		for i := range labels {
			if Measure(Small, labels[i]) > Measure(Small, labels[li]) {
				li = i
			}
		}
		lw := Measure(Small, labels[li])
		if lw <= 24 {
			break
		}
		cut := min(over, lw-24)
		labels[li] = Fit(Small, hints[li].Label, lw-cut)
		hh := append([]Hint(nil), hints...)
		for i := range hh {
			hh[i].Label = labels[i]
		}
		over = hintsWidth(hh, gap) - room
	}
	x := bar.Min.X + 12
	cy := (bar.Min.Y+bar.Max.Y)/2 - 1
	for i, h := range hints {
		x += s.buttonGlyph(x, cy, h.Button) + 6
		x += s.Text(x, centerY(Small, cy), labels[i], Small, Text) + gap
	}
}

// Alert draws a centred card with a title, a message and buttons. The
// first button is the default (A).
func (s *Screen) Alert(title, body string, buttons ...string) {
	s.Background()
	s.StatusBar()
	lines := Wrap(Body, body, 430)
	h := 40 + LineHeight(Heading) + 10 + len(lines)*LineHeight(Body)
	if len(buttons) > 0 {
		h += 74
	}
	card := image.Rect(W/2-240, (H-h)/2, W/2+240, (H+h)/2)
	s.PanelTint(card, 24, 0.13)
	y := card.Min.Y + 24
	s.TextCenter(W/2, y, title, Heading, Text)
	y += LineHeight(Heading) + 10
	for _, l := range lines {
		s.TextCenter(W/2, y, l, Body, Muted)
		y += LineHeight(Body)
	}
	if len(buttons) > 0 {
		bw := (card.Dx() - 48 - (len(buttons)-1)*12) / len(buttons)
		for i, b := range buttons {
			x := card.Min.X + 24 + i*(bw+12)
			r := image.Rect(x, card.Max.Y-68, x+bw, card.Max.Y-22)
			glyph, fg := "A", OnFocus
			if i > 0 {
				glyph, fg = "B", Text
			}
			if i == 0 {
				s.block(r, 14, focusRamp, Ink, 1)
			} else {
				s.block(r, 14, keyRamp, Ink, 1)
			}
			lw := buttonWidth(glyph) + 8 + Measure(Heading, b)
			lx := (r.Min.X+r.Max.X)/2 - lw/2
			cy := r.Min.Y + r.Dy()/2 - 1
			lx += s.buttonGlyph(lx, cy, glyph) + 8
			s.Text(lx, centerY(Heading, cy), b, Heading, fg)
		}
	}
	s.Present()
}

// Message shows a centred title and body (errors, loading). Hints become
// the alert's buttons.
func (s *Screen) Message(title, body string, hints ...Hint) {
	var buttons []string
	for _, h := range hints {
		buttons = append(buttons, h.Label)
	}
	s.Alert(title, body, buttons...)
}

// Loading draws a centred "busy" line: three pixel blocks, the bright one
// moving along each time the screen is redrawn, and the text.
func (s *Screen) Loading(cy int, text string) {
	w := Measure(Body, text)
	x := W/2 - (w+52)/2
	lit := int(time.Now().UnixMilli()/250) % 3
	by := cy + (LineHeight(Body)-4*art)/2
	for i := 0; i < 3; i++ {
		c := Line
		if i == lit {
			c = Accent
		}
		bx := x + i*6*art
		s.fillRect(image.Rect(bx, by, bx+4*art, by+4*art), c)
	}
	s.Text(x+52, centerY(Body, by+2*art), text, Body, Muted)
}

// ---- small capitals drawn straight (button pills) ----

// microWidth is the width of s in micro-font art pixels.
func microWidth(s string) int {
	w := 0
	for i, r := range s {
		if i > 0 {
			w++
		}
		g, ok := microFont[r]
		if !ok {
			g = microFont[' ']
		}
		w += len(g[0])
	}
	return w
}

// micro draws s in the micro font, each art pixel 2x2, top-left at (x, y).
func (s *Screen) micro(x, y int, str string, c color.RGBA) {
	for _, r := range str {
		g, ok := microFont[r]
		if !ok {
			g = microFont[' ']
		}
		s.bitmap(x, y, art, g, c, nil)
		x += (len(g[0]) + 1) * art
	}
}
