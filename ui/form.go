package ui

import (
	"image"
	"strings"
)

// Field is one row of a Form.
type Field struct {
	Label       string
	Value       string
	Hint        string   // shown on the keyboard screen and under the form
	Placeholder string   // shown greyed out while Value is empty
	Secret      bool     // typed hidden, shown as asterisks
	Options     []string // a choice: A or Left/Right cycles through these
	Optional    bool
	// Pick, if set, offers a list (for example tailnet devices) before the
	// keyboard. It returns the new value, or false to type instead.
	Pick func() (string, bool)
	// Hidden hides the field (for example Beszel's login for other types).
	Hidden func() bool
	// Changed runs after the value changes.
	Changed func()
}

// Form is a settings screen: fields, then a Save button. Save calls Check
// (which may test the connection) and then Commit.
type Form struct {
	Title  string
	Intro  string
	Fields []*Field
	Check  func() error // nil means OK; may take a while
	Commit func() error
	Delete func() // if set, offers a Remove button
}

type formRow struct {
	f      *Field
	button string // "save", "remove"
}

func (fm *Form) rows() []formRow {
	var out []formRow
	for _, f := range fm.Fields {
		if f.Hidden == nil || !f.Hidden() {
			out = append(out, formRow{f: f})
		}
	}
	out = append(out, formRow{button: "save"})
	if fm.Delete != nil {
		out = append(out, formRow{button: "remove"})
	}
	return out
}

// Run shows the form until it is saved (true) or cancelled (false).
func (fm *Form) Run(s *Screen, input <-chan Event) bool {
	sel := 0
	msg := ""
	for {
		rows := fm.rows()
		if sel >= len(rows) {
			sel = len(rows) - 1
		}
		fm.draw(s, rows, sel, msg)
		ev, ok, frame := s.Next(input)
		if !ok {
			return false
		}
		if frame || !ev.Pressed {
			continue
		}
		msg = ""
		r := rows[sel]
		switch ev.Button {
		case Up:
			sel = (sel - 1 + len(rows)) % len(rows)
		case Down:
			sel = (sel + 1) % len(rows)
		case B, Menu:
			return false
		case Left, Right:
			if r.f != nil && len(r.f.Options) > 0 {
				r.f.cycle(ev.Button == Left)
			}
		case Start:
			sel = len(rows) - 1
			if fm.Delete != nil {
				sel--
			}
			if fm.save(s, input) {
				return true
			}
		case A:
			switch {
			case r.button == "save":
				if fm.save(s, input) {
					return true
				}
			case r.button == "remove":
				if s.Confirm(input, "Remove this?", "It can't be undone.", "Remove") {
					fm.Delete()
					return true
				}
			case len(r.f.Options) > 0:
				r.f.cycle(false)
			default:
				if r.f.Pick != nil {
					if v, ok := r.f.Pick(); ok {
						r.f.Value = v
						if r.f.Changed != nil {
							r.f.Changed()
						}
						break
					}
				}
				if v, ok := s.Prompt(input, r.f.Label, r.f.Value, r.f.Hint, r.f.Secret); ok {
					r.f.Value = v
					if r.f.Changed != nil {
						r.f.Changed()
					}
				}
			}
		}
	}
}

func (f *Field) cycle(back bool) {
	i := 0
	for j, o := range f.Options {
		if o == f.Value {
			i = j
		}
	}
	if back {
		i = (i - 1 + len(f.Options)) % len(f.Options)
	} else {
		i = (i + 1) % len(f.Options)
	}
	f.Value = f.Options[i]
	if f.Changed != nil {
		f.Changed()
	}
}

func (fm *Form) save(s *Screen, input <-chan Event) bool {
	for _, r := range fm.rows() {
		if r.f != nil && !r.f.Optional && strings.TrimSpace(r.f.Value) == "" && !r.f.Secret {
			s.Notice(input, "Missing "+strings.ToLower(r.f.Label), "Fill it in, then save again.")
			return false
		}
	}
	if fm.Check != nil {
		s.Alert("Checking…", "Connecting to your server.")
		if err := fm.Check(); err != nil {
			if !s.Confirm(input, "Couldn't connect", err.Error(), "Save anyway") {
				return false
			}
		}
	}
	if fm.Commit != nil {
		if err := fm.Commit(); err != nil {
			s.Notice(input, "Couldn't save", err.Error())
			return false
		}
	}
	return true
}

func (fm *Form) draw(s *Screen, rows []formRow, sel int, msg string) {
	s.Background()
	top := s.Header(fm.Title, "")
	if fm.Intro != "" {
		for _, line := range Wrap(Small, fm.Intro, W-50) {
			s.Text(25, top-6, line, Small, Muted)
			top += LineHeight(Small)
		}
		top += 4
	}
	bottom := H - 86 // the hint line and the footer go below
	const fh, bh = 62, 52
	// Fields form one panel; Save and Remove are capsules after it.
	var fields []int
	var buttons []int
	for i, r := range rows {
		if r.f != nil {
			fields = append(fields, i)
		} else {
			buttons = append(buttons, i)
		}
	}
	total := len(fields)*fh + 12 + len(buttons)*(bh+10)
	pos := func(i int) int { // y of row i before scrolling
		y := 0
		for j := 0; j < i; j++ {
			if rows[j].f != nil {
				y += fh
			} else {
				y += bh + 10
			}
			if j == len(fields)-1 {
				y += 12
			}
		}
		return y
	}
	off := 0
	if h := pos(sel) + fh; h > bottom-top {
		off = h - (bottom - top)
	}
	group := image.Rect(20, top-off, W-20, top-off+len(fields)*fh)
	clip := image.Rect(0, top-2, W, bottom+2)
	if g := group.Intersect(clip); !g.Empty() {
		s.Panel(group, 18)
	}
	for _, i := range append(fields, buttons...) {
		r := rows[i]
		y := top - off + pos(i)
		focused := i == sel
		if r.f == nil {
			if y < top-2 || y+bh > bottom+2 {
				continue
			}
			rect := image.Rect(20, y, W-20, y+bh)
			if r.button == "remove" {
				s.Capsule(rect, "Remove", focused, Bad)
			} else {
				s.Capsule(rect, "Save", focused, Text)
			}
			continue
		}
		if y < top-2 || y+fh > bottom+2 {
			continue
		}
		rect := image.Rect(25, y+4, W-25, y+fh-4)
		fg, fg2 := Text, Muted
		if focused {
			s.Focus(rect, 13)
			fg, fg2 = OnFocus, OnFocus2
		} else if i > 0 && rows[i-1].f != nil && i-1 != sel {
			s.FillAlpha(image.Rect(40, y, W-40, y+1), Text, 0.12)
		}
		s.Text(rect.Min.X+16, rect.Min.Y+6, r.f.Label, Caps, fg2)
		val, col := r.f.Value, fg
		switch {
		case len(r.f.Options) > 0:
			s.Icon("chev-left", rect.Max.X-170, rect.Min.Y+(rect.Dy()-22)/2, 22, fg2)
			s.Icon("chev-right", rect.Max.X-40, rect.Min.Y+(rect.Dy()-22)/2, 22, fg2)
			s.TextCenter(rect.Max.X-99, rect.Min.Y+(rect.Dy()-LineHeight(Heading))/2, val, Heading, fg)
			val = ""
		case r.f.Secret && val != "":
			val = strings.Repeat("•", min(14, len([]rune(val))))
		case val == "" && r.f.Placeholder != "":
			val, col = r.f.Placeholder, fg2
		case val == "" && r.f.Optional:
			val, col = "Optional", fg2
		}
		if val != "" {
			if focused && !r.f.Secret {
				s.Marquee(rect.Min.X+16, rect.Min.Y+24, rect.Dx()-70, val, Body, col)
			} else {
				s.Text(rect.Min.X+16, rect.Min.Y+24, Fit(Body, val, rect.Dx()-70), Body, col)
			}
		}
		switch {
		case r.f.Pick != nil && len(r.f.Options) == 0:
			s.Icon("list", rect.Max.X-40, rect.Min.Y+(rect.Dy()-22)/2, 22, fg2)
		case len(r.f.Options) == 0:
			s.Icon("chev-right", rect.Max.X-34, rect.Min.Y+(rect.Dy()-18)/2, 18, fg2)
		}
	}
	if top-off+total > bottom+2 { // more below
		s.Circle(float64(W-40), float64(bottom-6), 14, Raised)
		s.Icon("chev-down", W-50, bottom-16, 20, Text)
	}
	hint := ""
	if sel < len(rows) && rows[sel].f != nil {
		hint = rows[sel].f.Hint
	}
	if msg != "" {
		s.Text(25, H-80, Fit(Small, msg, W-50), Small, Bad)
	} else if hint != "" {
		s.Text(25, H-80, Fit(Small, hint, W-50), Small, Muted)
	}
	edit := "Edit"
	if sel < len(rows) && rows[sel].f != nil && len(rows[sel].f.Options) > 0 {
		edit = "Change"
	}
	if sel < len(rows) && rows[sel].f == nil {
		edit = "Select"
	}
	s.Footer(Hint{"A", edit}, Hint{"START", "Save"}, Hint{"B", "Cancel"})
	s.Present()
}

// Confirm asks a yes/no question as an alert: A answers yes (labelled
// yes), B goes back.
func (s *Screen) Confirm(input <-chan Event, title, body, yes string) bool {
	s.Alert(title, body, yes, "Back")
	for ev := range input {
		if !ev.Pressed {
			continue
		}
		switch ev.Button {
		case A:
			return true
		case B, Menu:
			return false
		}
	}
	return false
}

// Notice shows a message as an alert until A or B is pressed.
func (s *Screen) Notice(input <-chan Event, title, body string) {
	s.Alert(title, body, "OK")
	for ev := range input {
		if ev.Pressed && (ev.Button == A || ev.Button == B || ev.Button == Menu) {
			return
		}
	}
}

// Choose shows a list and returns the chosen index, or false on B.
func (s *Screen) Choose(input <-chan Event, title string, items []Item) (int, bool) {
	l := List{Items: items}
	for {
		top := s.StaticFrame(title, "", Hint{"A", "Choose"}, Hint{"B", "Back"})
		l.Draw(s, image.Rect(20, top, W-26, H-58))
		s.Present()
		ev, ok, frame := s.Next(input)
		if !ok {
			return 0, false
		}
		if frame || !ev.Pressed {
			continue
		}
		switch ev.Button {
		case A:
			if len(items) > 0 {
				return l.Sel, true
			}
		case B, Menu:
			return 0, false
		default:
			l.Handle(ev)
		}
	}
}

// PickHost offers the tailnet's devices (peers returns their names and a
// detail line for each) and returns "http://NAME:PORT", or false to let the
// user type the address.
func (s *Screen) PickHost(input <-chan Event, peers func() ([]string, []string, error), port string) (string, bool) {
	s.Alert("Your tailnet", "Looking for devices…")
	names, details, err := peers()
	if err != nil {
		s.Notice(input, "Couldn't list devices", err.Error())
		return "", false
	}
	items := []Item{{Label: "Type the address", Sub: "Use the keyboard", Icon: "list", Tint: Muted}}
	for i, n := range names {
		detail := ""
		if i < len(details) {
			detail = details[i]
		}
		tint := Accent
		if strings.Contains(detail, "offline") {
			tint = Faint
		}
		items = append(items, Item{Label: n, Sub: detail, Icon: "server", Tint: tint})
	}
	i, ok := s.Choose(input, "Pick the server", items)
	if !ok || i == 0 {
		return "", false
	}
	return "http://" + names[i-1] + ":" + port, true
}
