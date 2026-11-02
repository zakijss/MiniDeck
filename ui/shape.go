package ui

import "unicode"

// Arabic shaping: each letter becomes the presentation form (isolated,
// final, initial or medial) its neighbours call for, using the joining
// types of ArabicShaping.txt, and lam followed by alef becomes one
// ligature. Unifont draws the presentation forms, so the letters connect.
// Marks (tashkeel) are transparent: a letter joins across them, and they
// stay where they are, to be drawn over the letter before them. Letters
// without presentation forms (most of the supplements) keep their nominal
// shape but still count for their neighbours' joining.

// joiningType returns r's joining type: U, T, R, L, D or C.
func joiningType(r rune) byte {
	switch {
	case r >= 0x600 && r < 0x600+rune(len(arabicJoining)):
		return arabicJoining[r-0x600]
	case r == 0x200d: // zero width joiner
		return 'C'
	case r == 0x200c: // zero width non-joiner
		return 'U'
	case r >= 0x300 && unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 'T'
	}
	return 'U'
}

// arabicFormsOf returns the isolated, final, initial and medial forms of
// r, or nil.
func arabicFormsOf(r rune) *[5]uint16 {
	lo, hi := 0, len(arabicForms)
	for lo < hi {
		m := (lo + hi) / 2
		switch v := rune(arabicForms[m][0]); {
		case v == r:
			return &arabicForms[m]
		case v < r:
			lo = m + 1
		default:
			hi = m
		}
	}
	return nil
}

// hasArabic reports whether rs holds a letter that shaping would change.
func hasArabic(rs []rune) bool {
	for _, r := range rs {
		if r >= 0x620 && r < 0x900 {
			return true
		}
	}
	return false
}

// shapeArabic returns rs with Arabic letters in their contextual forms,
// in logical order. It returns rs itself when there is nothing to shape.
func shapeArabic(rs []rune) []rune {
	if !hasArabic(rs) {
		return rs
	}
	jt := make([]byte, len(rs))
	for i, r := range rs {
		jt[i] = joiningType(r)
	}
	// the nearest non-transparent neighbours
	prev := func(i int) int {
		for i--; i >= 0 && jt[i] == 'T'; i-- {
		}
		return i
	}
	next := func(i int) int {
		for i++; i < len(rs) && jt[i] == 'T'; i++ {
		}
		if i == len(rs) {
			return -1
		}
		return i
	}
	out := make([]rune, 0, len(rs))
	for i := 0; i < len(rs); i++ {
		r, t := rs[i], jt[i]
		if t != 'R' && t != 'D' && t != 'L' {
			out = append(out, r)
			continue
		}
		p, n := prev(i), next(i)
		joinPrev := (t == 'R' || t == 'D') && p >= 0 && (jt[p] == 'D' || jt[p] == 'L' || jt[p] == 'C')
		if r == 0x644 && n >= 0 {
			if lig, ok := lamAlef[rs[n]]; ok {
				if joinPrev {
					lig++ // the final form
				}
				out = append(out, lig)
				out = append(out, rs[i+1:n]...) // marks on the lam
				i = n
				continue
			}
		}
		joinNext := (t == 'L' || t == 'D') && n >= 0 && (jt[n] == 'R' || jt[n] == 'D' || jt[n] == 'C')
		f := arabicFormsOf(r)
		if f == nil {
			out = append(out, r)
			continue
		}
		var form uint16
		switch {
		case joinPrev && joinNext:
			form = f[4]
		case joinPrev:
			form = f[2]
		case joinNext:
			form = f[3]
		}
		if form == 0 {
			form = f[1]
		}
		out = append(out, rune(form))
	}
	return out
}
