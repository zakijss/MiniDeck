package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// AppearanceItem is the row an app puts in its settings to open
// Appearance (see Screen.Appearance). Its second line names the theme,
// the accent when it isn't the theme's own, and the background, with how
// it moves and how light it is when those aren't the defaults.
func AppearanceItem() Item {
	a := CurrentAppearance()
	sub := themeByID(a.Theme).Name
	if ac := accentByID(a.Accent); !ac.own {
		sub += " · " + ac.Name
	}
	sc := sceneByID(a.Background)
	sub += " · " + sc.Name
	var more []string
	if sc.spawn != nil {
		switch a.Motion {
		case "off":
			more = append(more, "still")
		case "calm", "lively":
			more = append(more, strings.ToLower(optionByID(motions, a.Motion, 2).Name))
		}
	}
	if a.Brightness != "normal" {
		more = append(more, strings.ToLower(optionByID(brightnesses, a.Brightness, 1).Name))
	}
	if len(more) > 0 {
		sub += " (" + strings.Join(more, ", ") + ")"
	}
	return Item{Label: "Appearance", Sub: sub, Icon: "palette", Chevron: true}
}

// AppearanceHint is the footer hint for opening Appearance with SELECT,
// on screens that have no settings list of their own.
var AppearanceHint = Hint{Button: "SELECT", Label: "Look"}

// The Appearance screen's rows, in order.
const (
	rowTheme = iota
	rowAccent
	rowBackground
	rowBrightness
	rowMotion
)

// appearanceRows are the Appearance screen's rows for look a.
func appearanceRows(a Appearance) []Item {
	th, ac, sc := themeByID(a.Theme), accentByID(a.Accent), sceneByID(a.Background)
	accent := Item{Label: "Accent", Icon: "drop", Tint: Accent, Detail: ac.Name, Sub: "The selection and highlights"}
	if ac.own {
		accent.Sub = "The theme's own, " + th.accent
	}
	br := optionByID(brightnesses, a.Brightness, 1)
	mo := optionByID(motions, a.Motion, 2)
	motion := Item{Label: "Motion", Icon: "play", Detail: mo.Name, Sub: mo.About}
	if sc.spawn == nil {
		motion.Detail, motion.Sub, motion.Tint = "-", sc.Name+" doesn't move", Faint
	}
	return []Item{
		rowTheme:      {Label: "Theme", Icon: "palette", Detail: th.Name, Sub: th.About},
		rowAccent:     accent,
		rowBackground: {Label: "Background", Icon: "star", Detail: sc.Name, Sub: sc.About},
		rowBrightness: {Label: "Brightness", Icon: "sun", Detail: br.Name, Sub: br.About},
		rowMotion:     motion,
	}
}

// change steps the setting on row row of look a by step (1 or -1),
// round in a circle.
func (a Appearance) change(row, step int) Appearance {
	switch row {
	case rowTheme:
		a.Theme = Themes[cycle(themeIndex(a.Theme), step, len(Themes))].ID
	case rowAccent:
		a.Accent = AccentHues[cycle(accentIndex(a.Accent), step, len(AccentHues))].ID
	case rowBackground:
		a.Background = Scenes[cycle(sceneIndex(a.Background), step, len(Scenes))].ID
	case rowBrightness:
		a.Brightness = brightnesses[cycle(optionIndex(brightnesses, a.Brightness), step, len(brightnesses))].ID
	case rowMotion:
		if sceneByID(a.Background).spawn != nil {
			a.Motion = motions[cycle(optionIndex(motions, a.Motion), step, len(motions))].ID
			a.Still = a.Motion == "off"
		}
	}
	return a
}

// Appearance lets the user pick the theme, the accent, the background,
// its brightness and how much it moves. Every change shows at once; B
// saves it for every app and goes back. The rows scroll, with a sample
// card under them.
func (s *Screen) Appearance(input <-chan Event) {
	start := CurrentAppearance()
	a := start
	l := List{}
	for {
		l.Items = appearanceRows(a)
		top := s.StaticFrame("Appearance", "Every MiniDeck app · left and right change it",
			Hint{"A", "Next"}, Hint{"B", "Done"})
		// three rows show at once, so the sample fits below them
		lr := image.Rect(20, top, W-26, top+3*l.rowH()+16)
		l.Draw(s, lr)
		s.appearanceSample(image.Rect(lr.Min.X, lr.Max.Y+14, lr.Max.X, H-62))
		s.Present()
		ev, ok, frame := s.Next(input)
		if !ok {
			break
		}
		if frame || !ev.Pressed {
			continue
		}
		step := 0
		switch ev.Button {
		case A, Right:
			step = 1
		case Left:
			step = -1
		case B, Menu:
			if a != start {
				if err := SaveAppearance(a); err != nil {
					s.Notice(input, "Couldn't save", fmt.Sprintf("The look stays until this app closes. %v", err))
				}
			}
			return
		default:
			l.Handle(ev)
			continue
		}
		if n := a.change(l.Sel, step); n != a {
			a = n
			s.SetAppearance(a)
		}
	}
	if a != start {
		SaveAppearance(a)
	}
}

func cycle(i, step, n int) int { return ((i+step)%n + n) % n }

func themeIndex(id string) int {
	for i, t := range Themes {
		if t.ID == id {
			return i
		}
	}
	return 0
}

func accentIndex(id string) int {
	for i, a := range AccentHues {
		if a.ID == id {
			return i
		}
	}
	return 0
}

func sceneIndex(id string) int {
	for i, b := range Scenes {
		if b.ID == id {
			return i
		}
	}
	return 0
}

// appearanceSample shows the theme on a card: text, a selected and a
// plain button, and the status colours.
func (s *Screen) appearanceSample(r image.Rectangle) {
	if r.Dy() < 92 {
		return
	}
	s.Panel(r, 14)
	x, y := r.Min.X+18, r.Min.Y+14
	s.Text(x, centerY(Heading, y+12), "Sample", Heading, Text)
	s.Text(x, centerY(Small, y+40), "Secondary text", Small, Muted)
	dx := x
	for _, c := range []struct {
		label string
		col   color.RGBA
	}{{"OK", Good}, {"Low", Warn}, {"Off", Bad}} {
		s.Text(dx, centerY(Small, y+66), c.label, Small, c.col)
		dx += Measure(Small, c.label) + 14
	}
	bw := 132
	by := r.Min.Y + (r.Dy()-40)/2
	s.Capsule(image.Rect(r.Max.X-18-2*bw-12, by, r.Max.X-18-bw-12, by+40), "Button", false, Text)
	s.Capsule(image.Rect(r.Max.X-18-bw, by, r.Max.X-18, by+40), "Selected", true, Text)
}
