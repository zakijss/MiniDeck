package ui

import (
	"image"
	"image/color"
)

// Item is one row of a List.
type Item struct {
	Label  string
	Sub    string // optional second line
	Detail string // optional, on the right
	Icon   string // optional icon tile on the left (see Icon)
	// Image, if set, gives a thumbnail for the left (nil while loading).
	Image func() image.Image
	Value any
	Tint  color.RGBA // icon colour; zero means white
	// Chevron shows a disclosure chevron: the row opens another screen.
	Chevron bool
}

// List is a scrolling, selectable list, drawn as one panel of rows with the
// selected row highlighted.
type List struct {
	Items []Item
	Sel   int
	top   int
}

// Handle moves the selection; it reports whether the event was used.
func (l *List) Handle(ev Event) bool {
	if !ev.Pressed || len(l.Items) == 0 {
		return false
	}
	switch ev.Button {
	case Up:
		l.Sel = (l.Sel - 1 + len(l.Items)) % len(l.Items)
	case Down:
		l.Sel = (l.Sel + 1) % len(l.Items)
	case Left, L1:
		l.Sel = max(0, l.Sel-6)
	case Right, R1:
		l.Sel = min(len(l.Items)-1, l.Sel+6)
	default:
		return false
	}
	return true
}

// Selected returns the selected item, or nil.
func (l *List) Selected() *Item {
	if l.Sel < 0 || l.Sel >= len(l.Items) {
		return nil
	}
	return &l.Items[l.Sel]
}

func (l *List) rowH() int {
	for _, it := range l.Items {
		if it.Sub != "" || it.Image != nil || it.Icon != "" {
			return 66
		}
	}
	return 54
}

// layout picks how many rows fit in height h and stretches them a little to
// fill it, so a list never leaves a gap at the bottom.
func (l *List) layout(h int) (rows, rh int) {
	base := l.rowH()
	rows = max(1, (h+base/3)/base)
	rh = h / rows
	if rh > base+10 { // too stretched: keep the natural height
		rh = base
		rows = max(1, h/base)
	}
	return
}

// cursorGlyph is the arrow beside the selected row.
var cursorGlyph = rowsOf(`
#...
##..
###.
####
###.
##..
#...`)

// Draw renders the list inside r: a card of rows, the selected one a bright
// lavender block with a pixel arrow beside it.
func (l *List) Draw(s *Screen, r image.Rectangle) {
	rows, rh := l.layout(r.Dy())
	if len(l.Items) < rows {
		rh = l.rowH()
	}
	if l.Sel < l.top {
		l.top = l.Sel
	}
	if l.Sel >= l.top+rows {
		l.top = l.Sel - rows + 1
	}
	shown := min(rows, len(l.Items)-l.top)
	if shown <= 0 {
		return
	}
	section := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+shown*rh)
	s.Panel(section, 18)
	for i := 0; i < shown; i++ {
		it := l.Items[l.top+i]
		y := r.Min.Y + i*rh
		focused := l.top+i == l.Sel
		row := image.Rect(r.Min.X+6, y+4, r.Max.X-6, y+rh-4)
		fg, fg2 := Text, Muted
		if focused {
			s.Focus(row, 12)
			fg, fg2 = OnFocus, OnFocus2
			if r.Min.X >= 14 {
				s.bitmap(r.Min.X-12, y+rh/2-7, art, cursorGlyph, Accent, nil)
			}
		} else if i > 0 && l.top+i-1 != l.Sel {
			// separator (none next to the selected row)
			s.fillRect(image.Rect(r.Min.X+18, y-1, r.Max.X-18, y+1), nightRamp.lo)
		}
		x := row.Min.X + 12
		th := rh - 22
		ty := y + 11
		switch {
		case it.Image != nil:
			pr := image.Rect(x, ty, x+th, ty+th)
			if img := it.Image(); img != nil {
				s.dropShadow(rectShape(pr, 8), 1, 110)
				s.RoundPicture(img, pr, 8)
			} else {
				s.tile("disc", pr, key2Ramp)
			}
			x += th + 14
		case it.Icon != "":
			c, ok := iconColors[it.Icon]
			if it.Tint.A != 0 {
				c = rampOf(it.Tint)
			} else if !ok {
				c = accentRamp
			}
			s.tile(it.Icon, image.Rect(x, ty, x+th, ty+th), c)
			x += th + 14
		}
		right := row.Max.X - 12
		if it.Chevron {
			s.Icon("chev-right", right-20, y+(rh-24)/2, 24, fg2)
			right -= 26
		}
		if it.Detail != "" {
			// the label comes first: a long detail is cut, never drawn over it
			d := Fit(Small, it.Detail, right-x-Measure(Heading, it.Label)-14)
			dw := Measure(Small, d)
			s.Text(right-dw, centerY(Small, y+rh/2), d, Small, fg2)
			right -= dw + 14
		}
		room := right - x
		if it.Sub != "" {
			s.Text(x, y+10, Fit(Heading, it.Label, room), Heading, fg)
			s.Text(x, y+36, Fit(Small, it.Sub, room), Small, fg2)
		} else {
			s.Text(x, centerY(Heading, y+rh/2), Fit(Heading, it.Label, room), Heading, fg)
		}
	}
	if len(l.Items) > rows {
		// a pixel scroll bar: a dark track and a lavender thumb
		track := image.Rect(r.Max.X+6, r.Min.Y, r.Max.X+6+2*art, section.Max.Y)
		s.fillRect(track, nightRamp.lo)
		h := max(24, section.Dy()*rows/len(l.Items))
		top := r.Min.Y + (section.Dy()-h)*l.top/max(1, len(l.Items)-rows)
		s.fillRect(image.Rect(track.Min.X, top, track.Max.X, top+h), accentRamp.base)
		s.fillRect(image.Rect(track.Min.X, top, track.Min.X+art, top+h), accentRamp.hi)
	}
}
