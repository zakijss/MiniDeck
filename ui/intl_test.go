package ui

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/image/font"
)

func hexRunes(rs []rune) string {
	var b []string
	for _, r := range rs {
		b = append(b, fmt.Sprintf("%04X", r))
	}
	return strings.Join(b, " ")
}

func TestShapeArabic(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		// seen initial, lam-alef final (the lam joins the seen), meem isolated
		{"سلام", "FEB3 FEFC FEE1"},
		{"لا", "FEFB"},
		{"لأ", "FEF7"},
		{"بلا", "FE91 FEFC"},
		// meem initial, hah medial, meem medial, dal final
		{"محمد", "FEE3 FEA4 FEE4 FEAA"},
		// the same with tashkeel: the marks stay, the letters join across them
		{"مُحَمَّد", "FEE3 064F FEA4 064E FEE4 064E 0651 FEAA"},
		// a mark between lam and alef still makes the ligature
		{"لَا", "FEFB 064E"},
		// right-joining letters don't join the next: reh, alef, then dal
		// isolated after alef
		{"راد", "FEAD FE8D FEA9"},
		// Persian peh initial, dal final, reh isolated
		{"پدر", "FB58 FEAA FEAD"},
		// zero width non-joiner breaks the join, zero width joiner forces it
		{"ب\u200cب", "FE8F 200C FE8F"},
		{"ب\u200d", "FE91 200D"},
		// tatweel joins on both sides
		{"بـب", "FE91 0640 FE90"},
		// Latin and spaces between words don't join
		{"ب a ب", "FE8F 0020 0061 0020 FE8F"},
		// a letter without presentation forms keeps its own code
		{"ݐب", "0750 FE90"},
	} {
		if got := hexRunes(shapeArabic([]rune(c.in))); got != c.want {
			t.Errorf("shape %q = %s, want %s", c.in, got, c.want)
		}
	}
	if rs := []rune("Latin"); &shapeArabic(rs)[0] != &rs[0] {
		t.Error("text with no Arabic was copied")
	}
}

func TestBidiOrder(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"abc", "abc"},
		{"שלום", "םולש"},
		{"abc שלום def", "abc םולש def"},
		{"שלום abc", "abc םולש"},
		{"שלום 123", "123 םולש"},
		{"abc 123 שלום", "abc 123 םולש"},
		{"שלום 1.5% abc", "abc 1.5% םולש"},
		// a minus sign is a separator, so it stays on the right of the digits
		{"שלום -12 abc", "abc 12- םולש"},
		// Arabic-Indic digits keep their order inside right-to-left text
		{"١٢٣ אב", "בא ١٢٣"},
		// European digits after Arabic letters are Arabic numbers
		{"ﺏ 12 ﺕ", "ﺕ 12 ﺏ"},
		// brackets are mirrored and paired
		{"שלום (abc) עולם", "םלוע (abc) םולש"},
		{"abc (שלום) def", "abc (םולש) def"},
		{"א < ב", "ב > א"},
		{"א [ב]", "[ב] א"},
		// an isolate keeps a track number on the left of a Hebrew title
		{"1  \u2068שלום\u2069", "1  םולש"},
		{"1  שלום", "םולש  1"},
		// explicit overrides and embeddings
		{"\u202eabc\u202c", "cba"},
		{"\u202babc def\u202c", "abc def"},
		{"א \u202aabc ב\u202c", "abc ב א"},
		// a right-to-left mark sets the direction of neutral-led text
		{"\u200f123 abc", "abc 123"},
		// marks stay after their letter
		{"\u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd", "\u05dd\u05d5\u05b9\u05dc\u05e9\u05b8\u05c1"},
	} {
		if got := string(bidiReorder([]rune(c.in))); got != c.want {
			t.Errorf("order %q = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVisualArabic(t *testing.T) {
	// "Track 1: سلام" in a left-to-right line: the Arabic word reversed,
	// shaped, after the Latin
	got := hexRunes([]rune(visual("Track 1: سلام")))
	want := "0054 0072 0061 0063 006B 0020 0031 003A 0020 FEE1 FEFC FEB3"
	if got != want {
		t.Errorf("visual = %s, want %s", got, want)
	}
	if !rtlText("سلام abc") || rtlText("abc سلام") || rtlText("123") {
		t.Error("rtlText")
	}
	if s := "Plain ASCII · é"; visual(s) != s {
		t.Error("left-to-right text changed")
	}
}

func TestCoverage(t *testing.T) {
	// one character per script that Pixel Operator and Inter lack
	for _, r := range "中한ア あשاกअԱაஅ☂⇄" {
		if r == ' ' {
			continue
		}
		if _, _, _, ok := unifontGlyph(r); !ok {
			t.Errorf("Unifont has no %U %c", r, r)
		}
		for name, f := range map[string]font.Face{"Title": Title, "Heading": Heading, "Body": Body, "Small": Small, "Caps": Caps} {
			g := f.(glyphSource).glyphOf(r)
			if g.missing || !g.ok || g.adv == 0 {
				t.Errorf("%s draws %U %c as a stand-in", name, r, r)
			}
		}
	}
	// Cyrillic and Greek still come from Inter (scaled with the pixel font),
	// so they match the Latin around them.
	if g := Body.(glyphSource).glyphOf('Ж'); g.rect.Dy() < 16 {
		t.Errorf("Cyrillic in Body is %d px tall", g.rect.Dy())
	}
	// Unifont glyphs: 1x in the 2x faces, 2x in the 3x ones
	if w := Body.(glyphSource).glyphOf('中').adv; w != 16 {
		t.Errorf("中 in Body advances %d, want 16", w)
	}
	if w := Title.(glyphSource).glyphOf('中').adv; w != 32 {
		t.Errorf("中 in Title advances %d, want 32", w)
	}
	// marks take no room after a letter
	if Measure(Body, "مُحَمَّد") != Measure(Body, "محمد") {
		t.Error("tashkeel takes room")
	}
	// format characters are invisible
	if Measure(Body, "a\u200db\u200fc️") != Measure(Body, "abc") {
		t.Error("format characters take room")
	}
}

func TestFitIntl(t *testing.T) {
	latin := "Community apps, checked before installing"
	for w := 0; w < 400; w += 7 {
		got := Fit(Body, latin, w)
		// the longest prefix that fits, as Fit always cut
		r := []rune(latin)
		for len(r) > 0 && Measure(Body, string(r)+"…") > w {
			r = r[:len(r)-1]
		}
		if want := string(r) + "…"; got != want && Measure(Body, latin) > w {
			t.Fatalf("Fit(%d) = %q, want %q", w, got, want)
		}
	}
	ar := "جمال المحيم والأغاني الجميلة"
	for _, w := range []int{60, 100, 150} {
		got := Fit(Heading, ar, w)
		if !strings.HasSuffix(got, "…") || !strings.HasPrefix(ar, strings.TrimSuffix(got, "…")) {
			t.Fatalf("Fit(%d) = %q, not a prefix and an ellipsis", w, got)
		}
		if Measure(Heading, got) > w {
			t.Errorf("Fit(%d) = %q is %d wide", w, got, Measure(Heading, got))
		}
		// the ellipsis ends right-to-left text: on the left
		if v := visual(got); !strings.HasPrefix(v, "…") {
			t.Errorf("Fit(%d) draws as %q, ellipsis not on the left", w, v)
		}
	}
	// never between a letter and its marks
	marked := "مُحَمَّدٌ مُحَمَّدٌ مُحَمَّدٌ"
	for w := 10; w < 140; w += 2 {
		got := strings.TrimSuffix(Fit(Body, marked, w), "…")
		if rest := marked[len(got):]; rest != "" {
			if r, _ := utf8.DecodeRuneInString(rest); isMark(r) {
				t.Fatalf("Fit(%d) cut before mark %U", w, r)
			}
		}
	}
}

func TestWrapIntl(t *testing.T) {
	para := "שלום עולם abc def ghi jkl mno pqr stu"
	lines := Wrap(Body, para, 160)
	if len(lines) < 2 {
		t.Fatalf("Wrap = %q", lines)
	}
	for _, l := range lines {
		if Measure(Body, l) > 160 && strings.Contains(strings.TrimPrefix(l, "\u200f"), " ") {
			t.Errorf("line %q is %d wide", l, Measure(Body, l))
		}
		// every line reads right to left, like its paragraph
		if !rtlText(l) {
			t.Errorf("line %q lost its paragraph's direction", l)
		}
	}
	got := strings.ReplaceAll(strings.Join(lines, " "), "\u200f", "")
	if got != para {
		t.Errorf("lines join to %q", got)
	}
	// left-to-right paragraphs are left alone
	for _, l := range Wrap(Body, "abc def שלום עולם ghi jkl", 120) {
		if strings.ContainsRune(l, 0x200f) || (rtlText(l) && !strings.HasPrefix(l, "\u200e")) {
			t.Errorf("line %q of a left-to-right paragraph", l)
		}
	}
}

// FuzzText runs arbitrary bytes through shaping, ordering, measuring,
// fitting, wrapping and drawing.
func FuzzText(f *testing.F) {
	for _, s := range []string{"", "abc", "سلام 123 (abc)", "שָׁלוֹם", "\u2068\u202e\u2069\u202c", "\xff\xfe(", "لَا", "中文 한국어",
		"\u2067((\u2069]))", "ً̀", strings.Repeat("\u202b", 130) + "x"} {
		f.Add(s, 120)
	}
	s := &Screen{RGBA: image.NewRGBA(image.Rect(0, 0, 200, 60))}
	f.Fuzz(func(t *testing.T, str string, w int) {
		w = (w%400 + 400) % 400
		v := visual(str)
		if utf8.RuneCountInString(v) > utf8.RuneCountInString(str) {
			t.Fatalf("visual %q grew from %q", v, str)
		}
		for _, fc := range []font.Face{Body, Small, Heading, Caps} {
			m := Measure(fc, str)
			if lw := width(fc, str); lw != m {
				t.Fatalf("width %d, Measure %d for %q", lw, m, str)
			}
			fit := Fit(fc, str, w)
			if fw := Measure(fc, fit); fw > w && fit != str && fit != "…" {
				t.Fatalf("Fit(%q, %d) = %q, %d wide", str, w, fit, fw)
			}
			Wrap(fc, str, w)
			s.Text(4, 4, str, fc, Text)
		}
		s.Marquee(4, 30, 60, str, Small, Text)
		rtlText(str)
	})
}
