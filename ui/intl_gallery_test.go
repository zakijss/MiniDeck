package ui

import (
	"image"

	"golang.org/x/image/font"
)

// intlTitles are song titles in the scripts the fallback font covers.
var intlTitles = []Item{
	{Label: "جمال المحيم", Sub: "فيروز · كيفك إنت", Detail: "4:12"},
	{Label: "שלום עולם", Sub: "עידן רייכל · ממעמקים", Detail: "3:05"},
	{Label: "夜に駆ける", Sub: "YOASOBI · THE BOOK", Detail: "4:21"},
	{Label: "月亮代表我的心", Sub: "邓丽君 · 1977", Detail: "3:29"},
	{Label: "좋은 날", Sub: "아이유 · Real", Detail: "3:52"},
	{Label: "Группа крови", Sub: "Кино · 1988", Detail: "4:45"},
	{Label: "Τα παιδιά του Πειραιά", Sub: "Μάνος Χατζιδάκις", Detail: "3:10"},
}

// intlGallery draws lists and specimens in other scripts to
// ui-intl-*.png.
func intlGallery(s *Screen, shot func(string)) {
	s.SetTicker("كيفك إنت · فيروز (Live) 1994")
	l := List{Items: intlTitles, Sel: 1}
	top := s.StaticFrame("Recently played", "Seven languages, one list", Hint{"A", "Play"}, Hint{"B", "Back"})
	l.Draw(s, image.Rect(20, top, W-26, H-58))
	shot("intl-list")
	s.SetTicker("")

	var plain []Item
	for _, it := range intlTitles {
		plain = append(plain, Item{Label: it.Label, Detail: it.Detail})
	}
	plain = append(plain, Item{Label: "عمرو دياب (Live) 1998 - تملي معاك وأنا بغني لك يا حبيبي", Detail: "5:01"})
	l = List{Items: plain, Sel: len(plain) - 1}
	top = s.StaticFrame("أغاني عربية", "Arabic title, mixed lines, cut to fit", Hint{"A", "Play"}, Hint{"B", "Back"})
	l.Draw(s, image.Rect(20, top, W-26, H-58))
	shot("intl-plain")

	// each script in each face
	s.Background()
	y := 8
	for _, sp := range []struct {
		f    font.Face
		text string
	}{
		{Title, "Title مرحبا שלום 東京"},
		{Heading, "Heading سلام لا إله 1234 · עברית"},
		{Body, "Body مُحَمَّد · ภาษาไทย · नमस्ते"},
		{Small, "Small 漢字 かな カナ 한국어 Ελληνικά"},
		{Body, "Body Привет мир · Ελληνικά · Բարեւ"},
		{Small, "Small Привет мир · ქართული · ⇄ ☂ ♫"},
		{Caps, "CAPS سلام 東京 PLAYING"},
	} {
		s.Text(12, y, sp.text, sp.f, Text)
		y += LineHeight(sp.f) + 8
	}
	for _, line := range Wrap(Body, "قال: هذه أغنية جديدة من ألبوم 2024 بعنوان Night Signals وهي رائعة جداً", W-24) {
		s.Text(12, y, line, Body, Muted)
		y += LineHeight(Body)
	}
	y += 6
	for _, w := range []int{300, 150} {
		s.Text(12, y, Fit(Heading, "جمال المحيم والأغاني الجميلة", w), Heading, Text)
		s.Text(400, y, Fit(Heading, "東京の夜に駆ける歌", w-140), Heading, Text)
		y += LineHeight(Heading) + 4
	}
	shot("intl-type")
}
