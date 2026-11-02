package ui

import "testing"

// Prompt trims spaces around ordinary text, but a password keeps them.
func TestPromptTrim(t *testing.T) {
	t.Setenv("MINIDECK_UI_NULL", "1")
	t.Setenv("MINIDECK_SKY", "0")
	s, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		secret bool
		want   string
	}{
		{false, "pw"},
		{true, " pw  "},
	} {
		in := make(chan Event, 2)
		in <- Event{Button: Y, Pressed: true} // a space at the end
		in <- Event{Button: Start, Pressed: true}
		got, ok := s.Prompt(in, "Password", " pw ", "", tc.secret)
		if !ok || got != tc.want {
			t.Errorf("secret=%v: got %q %v, want %q", tc.secret, got, ok, tc.want)
		}
	}
}

// B deletes; on an empty field a fresh B goes back, but a held B (repeats)
// never does, so holding it to clear the text can't close the keyboard.
func TestPromptBack(t *testing.T) {
	t.Setenv("MINIDECK_UI_NULL", "1")
	t.Setenv("MINIDECK_SKY", "0")
	s, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	b := Event{Button: B, Pressed: true}
	held := Event{Button: B, Pressed: true, Repeat: true}
	start := Event{Button: Start, Pressed: true}
	for _, tc := range []struct {
		name    string
		initial string
		events  []Event
		want    string
		wantOK  bool
	}{
		{"B on an empty field goes back", "", []Event{b}, "", false},
		{"B deletes, then B on empty goes back", "ab", []Event{b, b, b}, "ab", false},
		{"held B clears but stays", "ab", []Event{b, held, held, held, start}, "", true},
		{"menu cancels with text", "ab", []Event{{Button: Menu, Pressed: true}}, "ab", false},
	} {
		in := make(chan Event, len(tc.events))
		for _, e := range tc.events {
			in <- e
		}
		close(in)
		got, ok := s.Prompt(in, "Search", tc.initial, "", false)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("%s: got %q %v, want %q %v", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}
