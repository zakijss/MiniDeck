//go:build ignore

// mkintl writes the international text data the UI kit embeds:
//
//   - fonts/unifont.bin, GNU Unifont's Basic Multilingual Plane packed in
//     pages of 256 characters, each page compressed on its own so a screen
//     that shows one Japanese title inflates one page, not the whole font;
//   - intl_tables.go, the Arabic joining types and presentation forms and
//     the bidi mirroring pairs, from the Unicode Character Database.
//
// Run it from ui/ with the release files downloaded (and their
// signatures checked) beforehand:
//
//	go run mkintl.go -unifont DIR -ucd DIR
//
// The unifont directory holds unifont-VERSION.hex.gz and
// unifont-combining-VERSION.txt from https://unifoundry.com/pub/unifont/
// (VERSION below); the ucd directory ArabicShaping.txt, UnicodeData.txt and
// BidiMirroring.txt from https://www.unicode.org/Public/UCD/latest/ucd/.
package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const unifontVersion = "18.0.01"

func main() {
	uni := flag.String("unifont", "", "directory with the Unifont release files")
	ucd := flag.String("ucd", "", "directory with the UCD files")
	flag.Parse()
	if *uni == "" || *ucd == "" {
		flag.Usage()
		os.Exit(2)
	}
	writeUnifont(*uni)
	writeTables(*ucd)
}

// ---- Unifont ----

// A page inflates to a 256-byte header, one byte per character (0 absent,
// 1 eight pixels wide, 2 sixteen wide, plus 4 when it combines with the
// character before), then the present glyphs' rows in order: 16 rows of
// one or two bytes, most significant bit leftmost.
//
// fonts/unifont.bin is a count of pages (uint16), then per page its number
// (code point >> 8, uint16) and where its compressed bytes end (uint32,
// from the start of the data), then the data.
func writeUnifont(dir string) {
	f, err := os.Open(filepath.Join(dir, "unifont-"+unifontVersion+".hex.gz"))
	check(err)
	zr, err := gzip.NewReader(f)
	check(err)
	glyphs := map[rune][]byte{}
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), ":")
		if !ok {
			continue
		}
		c, err := strconv.ParseUint(k, 16, 32)
		check(err)
		b, err := hex.DecodeString(v)
		check(err)
		if c > 0xffff || (len(b) != 16 && len(b) != 32) {
			continue
		}
		glyphs[rune(c)] = b
	}
	check(sc.Err())
	f.Close()

	comb := map[rune]bool{}
	cb, err := os.ReadFile(filepath.Join(dir, "unifont-combining-"+unifontVersion+".txt"))
	check(err)
	for _, l := range strings.Split(string(cb), "\n") {
		k, _, ok := strings.Cut(strings.TrimSpace(l), ":")
		if !ok {
			continue
		}
		c, err := strconv.ParseUint(k, 16, 32)
		check(err)
		comb[rune(c)] = true
	}

	var index, data bytes.Buffer
	pages := 0
	for p := 0; p < 256; p++ {
		var hdr [256]byte
		var body []byte
		for i := 0; i < 256; i++ {
			c := rune(p<<8 | i)
			b, ok := glyphs[c]
			if !ok {
				continue
			}
			hdr[i] = byte(len(b) / 16)
			if comb[c] {
				hdr[i] |= 4
			}
			body = append(body, b...)
		}
		if len(body) == 0 {
			continue
		}
		var z bytes.Buffer
		w, err := flate.NewWriter(&z, flate.BestCompression)
		check(err)
		w.Write(hdr[:])
		w.Write(body)
		check(w.Close())
		data.Write(z.Bytes())
		binary.Write(&index, binary.LittleEndian, uint16(p))
		binary.Write(&index, binary.LittleEndian, uint32(data.Len()))
		pages++
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(pages))
	out.Write(index.Bytes())
	out.Write(data.Bytes())
	check(os.WriteFile("fonts/unifont.bin", out.Bytes(), 0o644))
	log.Printf("fonts/unifont.bin: %d glyphs in %d pages, %d bytes", len(glyphs), pages, out.Len())
}

// ---- Unicode tables ----

func writeTables(dir string) {
	// General categories, for the transparent marks and the forms.
	cat := map[rune]string{}
	forms := map[rune]*[4]rune{} // base -> isolated, final, initial, medial
	lamAlef := map[rune]rune{}   // alef -> isolated lam-alef ligature
	ud, err := os.ReadFile(filepath.Join(dir, "UnicodeData.txt"))
	check(err)
	for _, l := range strings.Split(string(ud), "\n") {
		fs := strings.Split(l, ";")
		if len(fs) < 6 {
			continue
		}
		c := parse(fs[0])
		cat[c] = fs[2]
		if c < 0xfb50 || c > 0xfeff {
			continue
		}
		d := strings.Fields(fs[5])
		if len(d) < 2 {
			continue
		}
		form := map[string]int{"<isolated>": 0, "<final>": 1, "<initial>": 2, "<medial>": 3}
		k, ok := form[d[0]]
		if !ok {
			continue
		}
		switch {
		case len(d) == 2:
			b := parse(d[1])
			if forms[b] == nil {
				forms[b] = &[4]rune{}
			}
			if forms[b][k] == 0 {
				forms[b][k] = c
			}
		case len(d) == 3 && d[1] == "0644" && k == 0:
			lamAlef[parse(d[2])] = c
		}
	}

	// Joining types for U+0600..U+08FF: listed ones from ArabicShaping.txt,
	// unlisted marks and format characters transparent, the rest U.
	const lo, hi = 0x600, 0x900
	jt := make([]byte, hi-lo)
	for c := rune(lo); c < hi; c++ {
		switch cat[c] {
		case "Mn", "Me", "Cf":
			jt[c-lo] = 'T'
		default:
			jt[c-lo] = 'U'
		}
	}
	as, err := os.ReadFile(filepath.Join(dir, "ArabicShaping.txt"))
	check(err)
	for _, l := range strings.Split(string(as), "\n") {
		if strings.HasPrefix(l, "#") {
			continue
		}
		fs := strings.Split(l, ";")
		if len(fs) < 3 {
			continue
		}
		c := parse(fs[0])
		if c >= lo && c < hi {
			jt[c-lo] = strings.TrimSpace(fs[2])[0]
		}
	}

	// Mirrored pairs.
	var mirror [][2]rune
	bm, err := os.ReadFile(filepath.Join(dir, "BidiMirroring.txt"))
	check(err)
	for _, l := range strings.Split(string(bm), "\n") {
		l, _, _ = strings.Cut(l, "#")
		fs := strings.Split(l, ";")
		if len(fs) < 2 {
			continue
		}
		a, b := parse(fs[0]), parse(fs[1])
		if a <= 0xffff && b <= 0xffff {
			mirror = append(mirror, [2]rune{a, b})
		}
	}
	sort.Slice(mirror, func(i, j int) bool { return mirror[i][0] < mirror[j][0] })

	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by mkintl.go from the Unicode Character Database; DO NOT EDIT.\n\npackage ui\n\n")
	fmt.Fprintf(&b, "// arabicJoining is the joining type of U+%04X..U+%04X, one letter each:\n", lo, hi-1)
	fmt.Fprintf(&b, "// U non-joining, T transparent, R right, L left, D dual, C join-causing.\n")
	fmt.Fprintf(&b, "const arabicJoining = \"\" +\n")
	for i := 0; i < len(jt); i += 64 {
		sep := " +"
		if i+64 >= len(jt) {
			sep = ""
		}
		fmt.Fprintf(&b, "\t%q%s\n", jt[i:min(i+64, len(jt))], sep)
	}
	var bases []rune
	for c, f := range forms {
		if f[0] != 0 && f[1] != 0 { // a letter, not a lone form
			bases = append(bases, c)
		}
	}
	sort.Slice(bases, func(i, j int) bool { return bases[i] < bases[j] })
	fmt.Fprintf(&b, "\n// arabicForms are the presentation forms of the letters that have them:\n")
	fmt.Fprintf(&b, "// the letter, then its isolated, final, initial and medial forms (0 for\n// none: the letter doesn't join on that side).\n")
	fmt.Fprintf(&b, "var arabicForms = [...][5]uint16{\n")
	for _, c := range bases {
		f := forms[c]
		fmt.Fprintf(&b, "\t{0x%04X, 0x%04X, 0x%04X, 0x%04X, 0x%04X},\n", c, f[0], f[1], f[2], f[3])
	}
	fmt.Fprintf(&b, "}\n\n// lamAlef maps an alef to the isolated ligature of lam with it; the final\n// form follows it.\n")
	fmt.Fprintf(&b, "var lamAlef = map[rune]rune{\n")
	var alefs []rune
	for a := range lamAlef {
		alefs = append(alefs, a)
	}
	sort.Slice(alefs, func(i, j int) bool { return alefs[i] < alefs[j] })
	for _, a := range alefs {
		fmt.Fprintf(&b, "\t0x%04X: 0x%04X,\n", a, lamAlef[a])
	}
	fmt.Fprintf(&b, "}\n\n// bidiMirror pairs each character with its mirror image (BidiMirroring.txt),\n// sorted, for right-to-left runs.\n")
	fmt.Fprintf(&b, "var bidiMirror = [...][2]uint16{\n")
	for i, m := range mirror {
		if i%6 == 0 {
			b.WriteString("\t")
		}
		fmt.Fprintf(&b, "{0x%04X, 0x%04X},", m[0], m[1])
		if i%6 == 5 || i == len(mirror)-1 {
			b.WriteString("\n")
		} else {
			b.WriteString(" ")
		}
	}
	fmt.Fprintf(&b, "}\n")
	src, err := format.Source(b.Bytes())
	check(err)
	check(os.WriteFile("intl_tables.go", src, 0o644))
	log.Printf("intl_tables.go: %d letters with forms, %d lam-alef, %d mirror pairs", len(bases), len(alefs), len(mirror))
}

func parse(s string) rune {
	c, err := strconv.ParseUint(strings.TrimSpace(s), 16, 32)
	check(err)
	return rune(c)
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
