package ui

import (
	"sort"

	"golang.org/x/text/unicode/bidi"
)

// Bidirectional text: the Unicode Bidirectional Algorithm (UAX #9) for one
// line, enough for song titles, names and messages that mix Arabic or
// Hebrew with Latin and digits. It follows the standard's rules P2-P3,
// X1-X10, W1-W7, N1-N2, I1-I2 and L1-L4, with these simplifications:
//
//   - a string is one paragraph and one line (Wrap passes each line its
//     paragraph's direction with a mark, see Wrap);
//   - brackets are not paired (rule N0): a bracket takes its direction
//     from the text around it like any other neutral, which gives the same
//     result unless one script sits inside brackets in a run of another;
//   - combining marks stay after their base in right-to-left runs (L3), so
//     they draw over the right letter.
//
// golang.org/x/text supplies the bidi classes; its own reordering is not
// used.
//
// Layout positions don't change with direction: a right-to-left label is
// still drawn from the left edge of its box, as the interface is laid out
// left to right; only the characters inside it are in right-to-left order.

const maxBidiDepth = 125

func bidiClass(r rune) bidi.Class {
	p, _ := bidi.LookupRune(r)
	return p.Class()
}

func isIsolateInit(c bidi.Class) bool { return c == bidi.LRI || c == bidi.RLI || c == bidi.FSI }

// firstStrong returns 0 or 1 for the first L or R/AL in t[i:end], skipping
// isolates, and -1 if there is none (rules P2-P3).
func firstStrong(t []bidi.Class, match []int, i, end int) int {
	for ; i < end; i++ {
		switch t[i] {
		case bidi.L:
			return 0
		case bidi.R, bidi.AL:
			return 1
		case bidi.LRI, bidi.RLI, bidi.FSI:
			if match[i] < 0 {
				return -1
			}
			i = match[i]
		}
	}
	return -1
}

// rtlText reports whether s reads right to left: its first strong
// character is Arabic or Hebrew (or another right-to-left script).
func rtlText(s string) bool {
	if !needsLayout(s) {
		return false
	}
	rs := []rune(s)
	t := make([]bidi.Class, len(rs))
	for i, r := range rs {
		t[i] = bidiClass(r)
	}
	return firstStrong(t, matchIsolates(t), 0, len(t)) == 1
}

// matchIsolates pairs isolate initiators with their PDIs (BD9): match[i]
// is the index of the partner, or -1.
func matchIsolates(t []bidi.Class) []int {
	match := make([]int, len(t))
	var stack []int
	for i, c := range t {
		match[i] = -1
		switch c {
		case bidi.LRI, bidi.RLI, bidi.FSI:
			stack = append(stack, i)
		case bidi.PDI:
			if n := len(stack); n > 0 {
				match[i], match[stack[n-1]] = stack[n-1], i
				stack = stack[:n-1]
			}
		}
	}
	return match
}

// bidiReorder returns rs in visual order (left to right), with mirrored
// brackets in right-to-left runs and the invisible directional controls
// left out.
func bidiReorder(rs []rune) []rune {
	n := len(rs)
	orig := make([]bidi.Class, n)
	for i, r := range rs {
		orig[i] = bidiClass(r)
	}
	match := matchIsolates(orig)
	para := uint8(0)
	if firstStrong(orig, match, 0, n) == 1 {
		para = 1
	}
	t := append([]bidi.Class(nil), orig...)
	lv := explicitLevels(orig, t, match, para)

	// X9: embedding controls and boundary neutrals are left out of the
	// resolution; they take a neighbour's level for ordering only.
	kept := make([]int, 0, n)
	for i := range t {
		if t[i] != bidi.BN {
			kept = append(kept, i)
		}
	}
	for _, seq := range runSequences(kept, lv, orig, match) {
		resolveSequence(seq, rs, t, orig, lv, para)
	}

	// Removed characters sit at their predecessor's level.
	for i := range t {
		if t[i] == bidi.BN {
			if i > 0 {
				lv[i] = lv[i-1]
			} else {
				lv[i] = para
			}
		}
	}
	// L1: separators, and whitespace and isolate controls at the end of
	// the line or before a separator, go back to the paragraph level.
	trailing := true
	for i := n - 1; i >= 0; i-- {
		switch c := orig[i]; {
		case c == bidi.S || c == bidi.B:
			lv[i] = para
			trailing = true
		case c == bidi.WS || isIsolateInit(c) || c == bidi.PDI || t[i] == bidi.BN:
			if trailing {
				lv[i] = para
			}
		default:
			trailing = false
		}
	}
	return visualOrder(rs, orig, lv)
}

// explicitLevels applies rules X1-X8, returning each character's embedding
// level and setting t to the classes after overrides (BN for the removed
// embedding controls).
func explicitLevels(orig, t []bidi.Class, match []int, para uint8) []uint8 {
	type entry struct {
		level    uint8
		override bidi.Class // L, R, or ON for none
		isolate  bool
	}
	n := len(t)
	lv := make([]uint8, n)
	stack := []entry{{para, bidi.ON, false}}
	overIsolate, overEmbed, validIsolate := 0, 0, 0
	next := func(level uint8, rtl bool) uint8 {
		if rtl {
			return (level + 1) | 1
		}
		return (level + 2) &^ 1
	}
	for i := 0; i < n; i++ {
		top := stack[len(stack)-1]
		switch c := orig[i]; c {
		case bidi.RLE, bidi.LRE, bidi.RLO, bidi.LRO:
			nl := next(top.level, c == bidi.RLE || c == bidi.RLO)
			if nl <= maxBidiDepth && overIsolate == 0 && overEmbed == 0 {
				ov := bidi.ON
				if c == bidi.RLO {
					ov = bidi.R
				} else if c == bidi.LRO {
					ov = bidi.L
				}
				stack = append(stack, entry{nl, ov, false})
			} else if overIsolate == 0 {
				overEmbed++
			}
			lv[i], t[i] = top.level, bidi.BN
		case bidi.RLI, bidi.LRI, bidi.FSI:
			lv[i] = top.level
			if top.override != bidi.ON {
				t[i] = top.override
			}
			rtl := c == bidi.RLI
			if c == bidi.FSI {
				end := match[i]
				if end < 0 {
					end = n
				}
				rtl = firstStrong(orig, match, i+1, end) == 1
			}
			nl := next(top.level, rtl)
			if nl <= maxBidiDepth && overIsolate == 0 && overEmbed == 0 {
				validIsolate++
				stack = append(stack, entry{nl, bidi.ON, true})
			} else {
				overIsolate++
			}
		case bidi.PDI:
			if overIsolate > 0 {
				overIsolate--
			} else if validIsolate > 0 {
				overEmbed = 0
				for !stack[len(stack)-1].isolate {
					stack = stack[:len(stack)-1]
				}
				stack = stack[:len(stack)-1]
				validIsolate--
			}
			top = stack[len(stack)-1]
			lv[i] = top.level
			if top.override != bidi.ON {
				t[i] = top.override
			}
		case bidi.PDF:
			if overIsolate == 0 {
				if overEmbed > 0 {
					overEmbed--
				} else if !top.isolate && len(stack) >= 2 {
					stack = stack[:len(stack)-1]
				}
			}
			lv[i], t[i] = top.level, bidi.BN
		case bidi.B:
			lv[i] = para
		case bidi.BN:
			lv[i] = top.level
		default:
			lv[i] = top.level
			if top.override != bidi.ON {
				t[i] = top.override
			}
		}
	}
	return lv
}

// runSequences splits the kept characters into level runs and chains the
// runs an isolate separates into isolating run sequences (X10).
func runSequences(kept []int, lv []uint8, orig []bidi.Class, match []int) [][]int {
	var runs [][]int
	n := len(orig)
	startOf := make([]int, n) // character -> run it starts, or -1
	endsRun := make([]bool, n)
	for i := range startOf {
		startOf[i] = -1
	}
	for k := 0; k < len(kept); {
		e := k + 1
		for e < len(kept) && lv[kept[e]] == lv[kept[k]] {
			e++
		}
		startOf[kept[k]] = len(runs)
		endsRun[kept[e-1]] = true
		runs = append(runs, kept[k:e])
		k = e
	}
	var seqs [][]int
	for _, run := range runs {
		// a run that starts with the partner of an initiator ending another
		// run continues that run's sequence
		if f := run[0]; orig[f] == bidi.PDI && match[f] >= 0 && endsRun[match[f]] {
			continue
		}
		seq := append([]int(nil), run...)
		for {
			last := seq[len(seq)-1]
			if !isIsolateInit(orig[last]) || match[last] < 0 || startOf[match[last]] < 0 {
				break
			}
			seq = append(seq, runs[startOf[match[last]]]...)
		}
		seqs = append(seqs, seq)
	}
	return seqs
}

// resolveSequence applies W1-W7, N0-N2 and I1-I2 to one isolating run
// sequence.
func resolveSequence(seq []int, rs []rune, t, orig []bidi.Class, lv []uint8, para uint8) {
	level := lv[seq[0]]
	dir := func(l uint8) bidi.Class {
		if l&1 == 1 {
			return bidi.R
		}
		return bidi.L
	}
	// sos and eos from the levels on either side (X10)
	before := para
	for i := seq[0] - 1; i >= 0; i-- {
		if t[i] != bidi.BN {
			before = lv[i]
			break
		}
	}
	after := para
	if last := seq[len(seq)-1]; !isIsolateInit(orig[last]) {
		for i := last + 1; i < len(t); i++ {
			if t[i] != bidi.BN {
				after = lv[i]
				break
			}
		}
	}
	sos, eos := dir(max(before, level)), dir(max(after, level))

	c := make([]bidi.Class, len(seq))
	for k, i := range seq {
		c[k] = t[i]
	}
	// W1: marks take the class of what they follow
	for k := range c {
		if c[k] != bidi.NSM {
			continue
		}
		switch {
		case k == 0:
			c[k] = sos
		case isIsolateInit(c[k-1]) || c[k-1] == bidi.PDI:
			c[k] = bidi.ON
		default:
			c[k] = c[k-1]
		}
	}
	// W2, W3: European digits after Arabic letters are Arabic numbers
	strong := sos
	for k := range c {
		switch c[k] {
		case bidi.L, bidi.R, bidi.AL:
			strong = c[k]
		case bidi.EN:
			if strong == bidi.AL {
				c[k] = bidi.AN
			}
		}
	}
	for k := range c {
		if c[k] == bidi.AL {
			c[k] = bidi.R
		}
	}
	// W4: one separator between two numbers of the same kind
	for k := 1; k+1 < len(c); k++ {
		switch {
		case c[k] == bidi.ES && c[k-1] == bidi.EN && c[k+1] == bidi.EN:
			c[k] = bidi.EN
		case c[k] == bidi.CS && c[k-1] == bidi.EN && c[k+1] == bidi.EN:
			c[k] = bidi.EN
		case c[k] == bidi.CS && c[k-1] == bidi.AN && c[k+1] == bidi.AN:
			c[k] = bidi.AN
		}
	}
	// W5: terminators next to European digits
	for k := 0; k < len(c); k++ {
		if c[k] != bidi.ET {
			continue
		}
		e := k
		for e < len(c) && c[e] == bidi.ET {
			e++
		}
		if (k > 0 && c[k-1] == bidi.EN) || (e < len(c) && c[e] == bidi.EN) {
			for j := k; j < e; j++ {
				c[j] = bidi.EN
			}
		}
		k = e - 1
	}
	// W6: other separators and terminators are neutral
	for k := range c {
		switch c[k] {
		case bidi.ES, bidi.ET, bidi.CS:
			c[k] = bidi.ON
		}
	}
	// W7: European digits in left-to-right context are left to right
	strong = sos
	for k := range c {
		switch c[k] {
		case bidi.L, bidi.R:
			strong = c[k]
		case bidi.EN:
			if strong == bidi.L {
				c[k] = bidi.L
			}
		}
	}
	resolveBrackets(seq, rs, orig, c, dir(level), sos)
	// N1, N2: neutrals between two of the same direction take it,
	// otherwise the embedding direction
	neutral := func(x bidi.Class) bool {
		switch x {
		case bidi.B, bidi.S, bidi.WS, bidi.ON, bidi.LRI, bidi.RLI, bidi.FSI, bidi.PDI:
			return true
		}
		return false
	}
	strongDir := func(x bidi.Class) bidi.Class {
		if x == bidi.L {
			return bidi.L
		}
		return bidi.R // R, EN, AN
	}
	for k := 0; k < len(c); k++ {
		if !neutral(c[k]) {
			continue
		}
		e := k
		for e < len(c) && neutral(c[e]) {
			e++
		}
		lead, trail := sos, eos
		if k > 0 {
			lead = strongDir(c[k-1])
		}
		if e < len(c) {
			trail = strongDir(c[e])
		}
		d := dir(level)
		if lead == trail {
			d = lead
		}
		for j := k; j < e; j++ {
			c[j] = d
		}
		k = e - 1
	}
	// I1, I2
	for k, i := range seq {
		switch {
		case level&1 == 0 && c[k] == bidi.R:
			lv[i] = level + 1
		case level&1 == 0 && (c[k] == bidi.AN || c[k] == bidi.EN):
			lv[i] = level + 2
		case level&1 == 1 && (c[k] == bidi.L || c[k] == bidi.AN || c[k] == bidi.EN):
			lv[i] = level + 1
		default:
			lv[i] = level
		}
	}
}

// resolveBrackets applies N0: a pair of brackets takes the direction of
// the text inside it when that is the embedding direction e, or the
// direction of the text before it when that agrees with the text inside;
// marks on a bracket follow it. c holds the sequence's classes after the
// W rules.
func resolveBrackets(seq []int, rs []rune, orig, c []bidi.Class, e, sos bidi.Class) {
	type pair struct{ open, close int } // positions in seq
	type opener struct {
		closer rune
		at     int
	}
	var pairs []pair
	var stack []opener
	for k, i := range seq {
		if c[k] != bidi.ON {
			continue
		}
		p, _ := bidi.LookupRune(rs[i])
		if !p.IsBracket() {
			continue
		}
		if p.IsOpeningBracket() {
			if len(stack) == 63 { // BD16: give up on deeper nesting
				pairs = nil
				break
			}
			stack = append(stack, opener{mirror(rs[i]), k})
			continue
		}
		for j := len(stack) - 1; j >= 0; j-- {
			if stack[j].closer == rs[i] {
				pairs = append(pairs, pair{stack[j].at, k})
				stack = stack[:j]
				break
			}
		}
	}
	sort.Slice(pairs, func(a, b int) bool { return pairs[a].open < pairs[b].open })
	strong := func(x bidi.Class) bidi.Class {
		switch x {
		case bidi.L:
			return bidi.L
		case bidi.R, bidi.EN, bidi.AN:
			return bidi.R
		}
		return bidi.ON
	}
	for _, p := range pairs {
		inside := bidi.ON
		for k := p.open + 1; k < p.close; k++ {
			if d := strong(c[k]); d == e {
				inside = e
				break
			} else if d != bidi.ON {
				inside = d
			}
		}
		if inside == bidi.ON {
			continue
		}
		d := e
		if inside != e {
			before := sos
			for k := p.open - 1; k >= 0; k-- {
				if x := strong(c[k]); x != bidi.ON {
					before = x
					break
				}
			}
			if before == inside {
				d = inside
			}
		}
		for _, k := range []int{p.open, p.close} {
			c[k] = d
			for k++; k < len(seq) && orig[seq[k]] == bidi.NSM; k++ {
				c[k] = d
			}
		}
	}
}

// visualOrder reverses the runs at each odd level and above (L2), keeping
// combining marks after their base (L3), mirrors characters in
// right-to-left runs (L4), and leaves out the directional controls.
func visualOrder(rs []rune, orig []bidi.Class, lv []uint8) []rune {
	// units: a character and the marks that follow it at the same level
	type unit struct{ start, end int }
	units := make([]unit, 0, len(rs))
	for i := 0; i < len(rs); {
		e := i + 1
		for e < len(rs) && orig[e] == bidi.NSM && lv[e] == lv[i] {
			e++
		}
		units = append(units, unit{i, e})
		i = e
	}
	var hi, lowOdd uint8 = 0, 255
	for _, u := range units {
		l := lv[u.start]
		hi = max(hi, l)
		if l&1 == 1 {
			lowOdd = min(lowOdd, l)
		}
	}
	for l := hi; l >= lowOdd && l > 0; l-- {
		for k := 0; k < len(units); {
			if lv[units[k].start] < l {
				k++
				continue
			}
			e := k
			for e < len(units) && lv[units[e].start] >= l {
				e++
			}
			for a, b := k, e-1; a < b; a, b = a+1, b-1 {
				units[a], units[b] = units[b], units[a]
			}
			k = e
		}
	}
	out := make([]rune, 0, len(rs))
	for _, u := range units {
		for i := u.start; i < u.end; i++ {
			r := rs[i]
			if invisibleControl(r, orig[i]) {
				continue
			}
			if lv[i]&1 == 1 {
				r = mirror(r)
			}
			out = append(out, r)
		}
	}
	return out
}

// invisibleControl reports the directional marks, embeddings and isolates,
// which only steer the ordering.
func invisibleControl(r rune, c bidi.Class) bool {
	switch c {
	case bidi.LRE, bidi.RLE, bidi.LRO, bidi.RLO, bidi.PDF, bidi.LRI, bidi.RLI, bidi.FSI, bidi.PDI:
		return true
	}
	return r == 0x200e || r == 0x200f || r == 0x061c
}

// mirror returns r's mirror image for right-to-left text, or r.
func mirror(r rune) rune {
	if r > 0xffff {
		return r
	}
	lo, hi := 0, len(bidiMirror)
	for lo < hi {
		m := (lo + hi) / 2
		switch v := rune(bidiMirror[m][0]); {
		case v == r:
			return rune(bidiMirror[m][1])
		case v < r:
			lo = m + 1
		default:
			hi = m
		}
	}
	return r
}
