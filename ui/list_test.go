package ui

import (
	"image"
	"os"
	"strings"
	"testing"
)

func TestListHandle(t *testing.T) {
	l := List{Items: make([]Item, 5)}
	press := func(b Button, repeat bool) int {
		l.Handle(Event{Button: b, Pressed: true, Repeat: repeat})
		return l.Sel
	}
	if press(Up, false) != 4 {
		t.Error("a fresh Up on the first row didn't go round to the last")
	}
	if press(Down, true) != 4 {
		t.Error("a held Down went round past the last row")
	}
	if press(Down, false) != 0 {
		t.Error("a fresh Down on the last row didn't go round to the first")
	}
	if press(Up, true) != 0 {
		t.Error("a held Up went round past the first row")
	}
	if press(R2, false) != 4 || press(L2, false) != 0 {
		t.Error("L2/R2 don't go to the first and last rows")
	}
	l.Items, l.Sel = l.Items[:2], 4 // the list shrank under the selection
	if press(Up, false) != 0 {
		t.Errorf("Up from a selection past the end: Sel = %d", l.Sel)
	}
}

// A list that shrank (or a Sel set out of range) draws its rows, not
// nothing or a panic.
func TestListDrawOutOfRange(t *testing.T) {
	s := newTestScreen()
	l := List{Items: make([]Item, 30)}
	for i := range l.Items {
		l.Items[i].Label = "row"
	}
	r := image.Rect(20, 60, W-26, H-58)
	l.Sel = 29
	l.Draw(s, r)
	l.Items = l.Items[:3]
	l.Draw(s, r)
	if l.Sel != 2 || l.top != 0 {
		t.Errorf("after shrinking: Sel %d, top %d", l.Sel, l.top)
	}
	l.Sel = -1
	l.Draw(s, r)
	if l.Sel != 0 {
		t.Errorf("Sel -1 drew as %d", l.Sel)
	}
}

// The A still held from the press that opened a question doesn't answer it.
func TestConfirmIgnoresRepeat(t *testing.T) {
	s := newTestScreen()
	s.pngPath = os.DevNull
	in := make(chan Event, 2)
	in <- Event{Button: A, Pressed: true, Repeat: true}
	in <- Event{Button: B, Pressed: true}
	if s.Confirm(in, "Remove this?", "It can't be undone.", "Remove") {
		t.Error("a held A confirmed")
	}
}

// A very long message stays on the screen.
func TestAlertLongBody(t *testing.T) {
	s := newTestScreen()
	s.pngPath = os.DevNull
	s.Clear(Ink)
	s.Alert(strings.Repeat("Title ", 30), strings.Repeat("word ", 400)+strings.Repeat("x", 300), "OK")
	// the card's rounded corners leave the background in the screen's
	// corners; a card taller than the screen would cover row 0 and row H-1
	// in the middle
	for _, y := range []int{0, H - 1} {
		c := s.RGBAAt(W/2, y)
		bg := ambient().RGBAAt(W/2, y)
		if c != bg {
			t.Errorf("row %d at the centre is %v, not the background %v: the card ran off the screen", y, c, bg)
		}
	}
}
