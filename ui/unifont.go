package ui

import (
	"bytes"
	"compress/flate"
	_ "embed"
	"encoding/binary"
	"io"
	"sync"
)

// GNU Unifont (SIL Open Font License 1.1, see licenses/unifont-OFL.txt)
// is the last fallback: a bitmap glyph, 8 or 16 pixels wide and 16 tall,
// for nearly every character of the Basic Multilingual Plane, so Arabic,
// Hebrew, Thai, Devanagari, Chinese, Japanese and Korean titles show in
// the same pixel style instead of empty boxes.
//
// fonts/unifont.bin (written by mkintl.go) keeps the glyphs in pages of
// 256 characters, each compressed on its own. The embedded bytes cost no
// memory until read; a page is inflated (to at most 9.5 KB) the first time
// one of its characters is drawn, and only the most recently used pages
// are kept.

//go:embed fonts/unifont.bin
var unifontData []byte

// Unifont's cell: 16 rows, the baseline under row 13.
const (
	unifontRows   = 16
	unifontAscent = 14
)

// unifontPage is one inflated page: per character 0 (absent), 1 or 2 (its
// width in bytes) plus 4 when it combines with the character before, and
// where its rows start in rows.
type unifontPage struct {
	kind [256]byte
	off  [256]uint32
	rows []byte
}

// unifontKeep is how many recently used pages stay inflated: up to twice
// as many, as the full set is kept as the old one when a new one starts
// (like lru). Older pages are inflated again if needed, which is rare: the
// faces keep the glyphs they have drawn. 32 pages of Chinese take about
// 300 KB.
const unifontKeep = 16

var unifontPages = struct {
	sync.Mutex
	cur, old map[uint16]*unifontPage
}{cur: map[uint16]*unifontPage{}}

// unifontGlyph returns r's rows (16 rows of w/8 bytes, leftmost pixel in
// the top bit), its width in pixels and whether it is a combining mark.
// ok is false when Unifont has no glyph for r.
func unifontGlyph(r rune) (rows []byte, w int, mark, ok bool) {
	if r < 0 || r > 0xffff {
		return nil, 0, false, false
	}
	pg := unifontLoad(uint16(r >> 8))
	if pg == nil {
		return nil, 0, false, false
	}
	k := pg.kind[r&0xff]
	bw := int(k & 3)
	if bw == 0 {
		return nil, 0, false, false
	}
	o := pg.off[r&0xff]
	return pg.rows[o : o+uint32(bw*unifontRows)], bw * 8, k&4 != 0, true
}

func unifontLoad(p uint16) *unifontPage {
	c := &unifontPages
	c.Lock()
	defer c.Unlock()
	if pg, ok := c.cur[p]; ok {
		return pg
	}
	pg, ok := c.old[p]
	if !ok {
		pg = inflatePage(p)
	}
	if len(c.cur) >= unifontKeep {
		c.old, c.cur = c.cur, map[uint16]*unifontPage{}
	}
	c.cur[p] = pg // nil too: a page Unifont lacks is looked up once
	return pg
}

// inflatePage finds page p in the index and inflates it, or returns nil.
func inflatePage(p uint16) *unifontPage {
	d := unifontData
	if len(d) < 2 {
		return nil
	}
	n := int(binary.LittleEndian.Uint16(d))
	const entry = 6
	base := 2 + n*entry
	if len(d) < base {
		return nil
	}
	lo, hi := 0, n
	for lo < hi {
		m := (lo + hi) / 2
		if binary.LittleEndian.Uint16(d[2+m*entry:]) < p {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo == n || binary.LittleEndian.Uint16(d[2+lo*entry:]) != p {
		return nil
	}
	start := uint32(0)
	if lo > 0 {
		start = binary.LittleEndian.Uint32(d[2+(lo-1)*entry+2:])
	}
	end := binary.LittleEndian.Uint32(d[2+lo*entry+2:])
	if start > end || int(end) > len(d)-base {
		return nil
	}
	// a new inflater each time (some 40 KB, freed after): pages are
	// inflated seldom, and keeping one would cost that for good
	zr := flate.NewReader(bytes.NewReader(d[base+int(start) : base+int(end)]))
	pg := &unifontPage{}
	if _, err := io.ReadFull(zr, pg.kind[:]); err != nil {
		return nil
	}
	o := uint32(0)
	for i, k := range pg.kind {
		pg.off[i] = o
		o += uint32(k&3) * unifontRows
	}
	pg.rows = make([]byte, o) // exactly: ReadAll would leave spare capacity
	if _, err := io.ReadFull(zr, pg.rows); err != nil {
		return nil
	}
	return pg
}
