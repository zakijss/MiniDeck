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
