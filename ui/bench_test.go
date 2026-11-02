package ui

import (
	"image"
	"testing"
)

// listItems is a typical list: icons, two lines, a detail and chevrons.
func listItems() []Item {
	labels := []string{"Featured themes", "All themes", "Icon packs", "Apps", "Homebrew games", "Ports", "Repositories"}
	icons := []string{"star", "list", "gear", "plus", "play", "dice", "server"}
	var items []Item
	for i, l := range labels {
		items = append(items, Item{Label: l, Sub: "Picked by the Onion team", Icon: icons[i], Detail: "12", Chevron: true})
	}
	return items
}

// BenchmarkListScreen draws a whole list screen from nothing: background,
// title, status bar, footer and a five-row list.
func BenchmarkListScreen(b *testing.B) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	l := List{Items: listItems(), Sel: 1}
	for i := 0; i < b.N; i++ {
		s.Background()
		top := s.Header("MiniMart", "Themes, icon packs, apps and games")
		s.Footer(Hint{"A", "Open"}, Hint{"B", "Exit"})
		l.Draw(s, image.Rect(20, top, W-26, H-58))
	}
}

// BenchmarkListPress is what a press on a list costs: the kept frame
// (StaticFrame) and the list drawn again with the selection moved.
func BenchmarkListPress(b *testing.B) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	l := List{Items: listItems()}
	for i := 0; i < b.N; i++ {
		top := s.StaticFrame("MiniMart", "Themes, icon packs, apps and games", Hint{"A", "Open"}, Hint{"B", "Exit"})
		l.Sel = i % len(l.Items)
		l.Draw(s, image.Rect(20, top, W-26, H-58))
	}
}

// BenchmarkCoverList is a list of album covers (MiniTape).
func BenchmarkCoverList(b *testing.B) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	cover := image.NewRGBA(image.Rect(0, 0, 240, 240))
	for i := range cover.Pix {
		cover.Pix[i] = uint8(i * 7)
	}
	var items []Item
	for i := 0; i < 20; i++ {
		items = append(items, Item{Label: "Album title", Sub: "Artist · 2001", Image: func() image.Image { return cover }})
	}
	l := List{Items: items}
	for i := 0; i < b.N; i++ {
		top := s.StaticFrame("Recently added", "", Hint{"A", "Open"}, Hint{"B", "Back"})
		l.Sel = i % 5
		l.Draw(s, image.Rect(20, top, W-26, H-58))
	}
}

// BenchmarkText draws a line of body text.
func BenchmarkText(b *testing.B) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	for i := 0; i < b.N; i++ {
		s.Text(20, 100, "Community apps, checked before installing", Body, Text)
	}
}

// BenchmarkTextArabic draws a line of Arabic with a Latin word and digits
// (laid out once, then from the cache).
func BenchmarkTextArabic(b *testing.B) {
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, W, H))}
	for i := 0; i < b.N; i++ {
		s.Text(20, 100, "سلام لبيروت (Live) 1994", Body, Text)
	}
}

// BenchmarkFit cuts a list label that is too long, as List.Draw does for
// every row it draws.
func BenchmarkFit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Fit(Heading, "Community apps, checked before installing them", 300)
	}
}
