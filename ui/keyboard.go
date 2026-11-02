package ui

import (
	"image"
	"strings"
)

// Three key layouts, switched with X or the layout key.
var layers = [][]string{
	{"1234567890", "qwertyuiop", "asdfghjkl-", "zxcvbnm./:"},
	{"!@#$%^&*()", "QWERTYUIOP", "ASDFGHJKL_", "ZXCVBNM,?;"},
	{"~`+=[]{}\\|", "'\"<>$%&*()", "1234567890", "@#!?/:;,._"},
}
var layerNames = []string{"abc", "ABC", "#+="}

// Bottom-row keys, with their width in key units. On password fields the
// "http://" key becomes "show"/"hide".
var specials = []struct {
	label string
	w     int
}{{"layout", 2}, {"http://", 2}, {"space", 3}, {"del", 1}, {"done", 2}}

type key struct {
	r     image.Rectangle
	label string // what is drawn
	text  string // what is typed ("" for special keys)
	icon  string // drawn instead of the label
}

const (
	kbTop  = 152
	keyW   = 58
	keyH   = 46
	keyGap = 6
)

func keysFor(layer int, secret, show bool) [][]key {
	var rows [][]key
	x0 := (W - (10*keyW + 9*keyGap)) / 2
	for ri, row := range layers[layer] {
		var ks []key
		for ci, ch := range []rune(row) {
			x := x0 + ci*(keyW+keyGap)
			y := kbTop + ri*(keyH+keyGap)
			ks = append(ks, key{r: image.Rect(x, y, x+keyW, y+keyH), label: string(ch), text: string(ch)})
		}
		rows = append(rows, ks)
	}
	var ks []key
	x := x0
	y := kbTop + 4*(keyH+keyGap)
	for _, sp := range specials {
		w := sp.w*keyW + (sp.w-1)*keyGap
		k := key{r: image.Rect(x, y, x+w, y+keyH), label: sp.label}
		switch sp.label {
		case "layout":
			k.label = layerNames[(layer+1)%len(layerNames)]
		case "http://":
			if secret {
				k.label = "show"
				if show {
					k.label = "hide"
				}
				k.icon = "eye"
			}
		case "del":
			k.icon = "backspace"
		}
		ks = append(ks, k)
		x += w + keyGap
	}
	return append(rows, ks)
}

// nearest picks the key in row whose centre is closest to x.
func nearest(row []key, x int) int {
	best, bd := 0, 1<<30
	for i, k := range row {
		d := (k.r.Min.X+k.r.Max.X)/2 - x
		if d < 0 {
			d = -d
		}
		if d < bd {
			best, bd = i, d
		}
	}
	return best
}

// Prompt asks for text with the on-screen keyboard. It returns the text and
// true, or false if cancelled. secret hides the text (the eye key or
// SELECT shows it). L1/R1 move the cursor, so a typo in the middle of an
// address can be fixed without retyping the rest.
func (s *Screen) Prompt(input <-chan Event, title, initial, hint string, secret bool) (string, bool) {
	text := []rune(initial)
	cur := len(text)
	layer, row, col := 0, 1, 0
	show := !secret
	insert := func(r []rune) {
		text = append(text[:cur], append(r, text[cur:]...)...)
		cur += len(r)
	}
	// Spaces around an address or a name are typos; around a password
	// they may be part of it.
	done := func() string {
		if secret {
			return string(text)
		}
		return strings.TrimSpace(string(text))
	}
	del := func() {
		if cur > 0 {
			text = append(text[:cur-1], text[cur:]...)
			cur--
		}
	}
	for {
		keys := keysFor(layer, secret, show)
		if col >= len(keys[row]) {
			col = len(keys[row]) - 1
		}
		s.drawPrompt(title, text, cur, hint, secret, show, keys, row, col, layer)
		ev, ok, frame := s.Next(input)
		if !ok {
			return "", false
		}
		if frame || !ev.Pressed {
			continue
		}
		cx := (keys[row][col].r.Min.X + keys[row][col].r.Max.X) / 2
		switch ev.Button {
		case Up:
			row = (row - 1 + len(keys)) % len(keys)
			col = nearest(keys[row], cx)
		case Down:
			row = (row + 1) % len(keys)
			col = nearest(keys[row], cx)
		case Left:
			col = (col - 1 + len(keys[row])) % len(keys[row])
		case Right:
			col = (col + 1) % len(keys[row])
		case L1:
			cur = max(0, cur-1)
		case R1:
			cur = min(len(text), cur+1)
		case L2:
			cur = 0
		case R2:
			cur = len(text)
		case B:
			del()
		case X:
			layer = (layer + 1) % len(layers)
		case Y:
			insert([]rune{' '})
		case Select:
			show = !show
		case Start:
			return done(), true
		case Menu:
			return initial, false
		case A:
			k := keys[row][col]
			switch {
			case k.text != "":
				insert([]rune(k.text))
				if layer == 1 && row > 0 { // one capital, then back to lowercase
					layer = 0
				}
			case k.label == "http://":
				if !strings.HasPrefix(string(text), "http") {
					text = append([]rune("http://"), text...)
					cur += 7
				}
			case k.label == "show", k.label == "hide":
				show = !show
			case k.label == "space":
				insert([]rune{' '})
			case k.label == "del":
				del()
			case k.label == "done":
				return done(), true
			default: // layout key
				layer = (layer + 1) % len(layers)
			}
		}
	}
}

func (s *Screen) drawPrompt(title string, text []rune, cur int, hint string, secret, show bool, keys [][]key, row, col, layer int) {
	s.Background()
	top := s.Header(title, "")
	box := image.Rect(20, top+2, W-20, top+50) // below the title (and the ticker)
	// a sunken field: dark inside, its dark edge on top, a lavender border
	sh := rectShape(box, 10)
	s.fillShape(sh, Accent)
	in := sh.erode()
	s.fillShape(in, nightRamp.lo)
	s.fillShape(in.erode(), wellColors[0])
	s.fillShape(in.andShifted(1).erode(), wellColors[1])
	shown := text
	if secret && !show {
		shown = []rune(strings.Repeat("•", len(text)))
	}
	// Scroll sideways so the cursor stays visible.
	inner := box.Dx() - 36
	start := 0
	for start < cur && Measure(Body, string(shown[start:cur])) > inner-20 {
		start++
	}
	end := len(shown)
	for end > cur && Measure(Body, string(shown[start:end])) > inner {
		end--
	}
	x := box.Min.X + 18
	ty := centerY(Body, (box.Min.Y+box.Max.Y)/2)
	if start > 0 {
		s.Text(box.Min.X+6, ty, "‹", Body, Muted)
	}
	before := Measure(Body, string(shown[start:cur]))
	s.Text(x, ty, string(shown[start:end]), Body, Text)
	s.Fill(image.Rect(x+before, box.Min.Y+10, x+before+art, box.Max.Y-10), Accent) // cursor
	if end < len(shown) {
		s.Text(box.Max.X-14, ty, "›", Body, Muted)
	}
	if hint != "" && box.Max.Y+8+LineHeight(Small) < kbTop-8 {
		s.Text(24, box.Max.Y+8, Fit(Small, hint, W-48), Small, Muted)
	}
	pad := image.Rect(0, kbTop-8, W, kbTop+5*(keyH+keyGap)+2)
	s.fillRect(pad, nightRamp.lo)
	s.fillRect(image.Rect(0, pad.Min.Y, W, pad.Min.Y+art), Ink)
	s.fillRect(image.Rect(0, pad.Min.Y+art, W, pad.Min.Y+2*art), nightRamp.base)
	for ri, r := range keys {
		for ci, k := range r {
			focused := ri == row && ci == col
			fg := Text
			c := keyRamp
			switch {
			case focused:
				c, fg = focusRamp, OnFocus
			case k.text == "":
				c = key2Ramp
			}
			// a key cap: bevelled, with a deeper dark edge at the bottom
			ksh := rectShape(k.r, 8)
			s.dropShadow(ksh, 1, 120)
			s.bevel(ksh, c, Ink)
			s.fillRect(image.Rect(k.r.Min.X+2*art, k.r.Max.Y-3*art, k.r.Max.X-2*art, k.r.Max.Y-2*art), c.lo)
			cx := (k.r.Min.X + k.r.Max.X) / 2
			cy := k.r.Min.Y + (keyH-art)/2
			if k.icon != "" {
				iw := 24
				lw := 0
				if k.icon == "eye" {
					lw = Measure(Small, k.label) + 6
				}
				ix := cx - (iw+lw)/2
				s.Icon(k.icon, ix, cy-iw/2, iw, fg)
				if lw > 0 {
					s.Text(ix+iw+6, centerY(Small, cy), k.label, Small, fg)
				}
				continue
			}
			f := Heading
			if k.text == "" {
				f = Small
			}
			s.TextCenter(cx, centerY(f, cy), k.label, f, fg)
		}
	}
	hints := []Hint{{"A", "Type"}, {"B", "Delete"}, {"L1/R1", "Move"}, {"START", "Done"}}
	if secret {
		hints = append(hints, Hint{"SELECT", "Show"})
	}
	s.Footer(hints...)
	s.Present()
}
