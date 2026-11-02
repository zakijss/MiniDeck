package qr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const loginURL = "https://login.tailscale.com/a/1a2b3c4d5e6f7a"

// Each version holds up to its level M byte capacity; one byte more needs
// the next version, and nothing above version 6 fits.
func TestSizes(t *testing.T) {
	for _, c := range []struct{ n, size int }{
		{0, 21}, {1, 21}, {14, 21},
		{15, 25}, {26, 25},
		{27, 29}, {42, 29},
		{43, 33}, {62, 33},
		{63, 37}, {84, 37},
		{85, 41}, {106, 41},
	} {
		q, err := Encode(strings.Repeat("x", c.n))
		if err != nil {
			t.Errorf("%d bytes: %v", c.n, err)
			continue
		}
		if q.Size != c.size || len(q.Modules) != c.size || len(q.Modules[0]) != c.size {
			t.Errorf("%d bytes: size %d (%d rows), want %d", c.n, q.Size, len(q.Modules), c.size)
		}
	}
	for _, n := range []int{107, 200} {
		if _, err := Encode(strings.Repeat("x", n)); err == nil {
			t.Errorf("%d bytes: no error", n)
		}
	}
}

// The finder patterns sit in three corners with light separators, and the
// timing patterns alternate between them.
func TestFunctionPatterns(t *testing.T) {
	for _, text := range []string{"a", loginURL, strings.Repeat("y", 106)} {
		q, err := Encode(text)
		if err != nil {
			t.Fatal(err)
		}
		n := q.Size
		at := func(x, y int) bool { return q.Modules[y][x] }
		for _, c := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
			for dy := -1; dy <= 7; dy++ {
				for dx := -1; dx <= 7; dx++ {
					x, y := c[0]+dx, c[1]+dy
					if x < 0 || y < 0 || x >= n || y >= n {
						continue
					}
					d := max(abs(dx-3), abs(dy-3))
					want := d != 2 && d != 4 // 3x3 core, light ring, dark ring, light separator
					if at(x, y) != want {
						t.Fatalf("%d bytes: finder at %v: module (%d,%d) wrong", len(text), c, x, y)
					}
				}
			}
		}
		for i := 8; i < n-8; i++ {
			if at(i, 6) != (i%2 == 0) || at(6, i) != (i%2 == 0) {
				t.Fatalf("%d bytes: timing pattern wrong at %d", len(text), i)
			}
		}
		if !at(8, n-8) {
			t.Errorf("%d bytes: dark module missing", len(text))
		}
	}
}

// formatBits reads both copies of the 15 format bits, as drawFormat lays
// them out.
func formatBits(q *QR) (a, b int) {
	n := q.Size
	bit := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	at := func(x, y int) int { return bit(q.Modules[y][x]) }
	for i := 0; i <= 5; i++ {
		a |= at(8, i) << i
	}
	a |= at(8, 7)<<6 | at(8, 8)<<7 | at(7, 8)<<8
	for i := 9; i < 15; i++ {
		a |= at(14-i, 8) << i
	}
	for i := range 8 {
		b |= at(n-1-i, 8) << i
	}
	for i := 8; i < 15; i++ {
		b |= at(8, n-15+i) << i
	}
	return a, b
}

// The format bits pass the BCH check, name level M, and agree in both
// copies.
func TestFormatBits(t *testing.T) {
	for _, text := range []string{"", "a", loginURL, strings.Repeat("z", 90)} {
		q, err := Encode(text)
		if err != nil {
			t.Fatal(err)
		}
		a, b := formatBits(q)
		if a != b {
			t.Fatalf("%q: format copies differ: %015b, %015b", text, a, b)
		}
		raw := a ^ 0x5412
		rem := raw
		for i := 14; i >= 10; i-- {
			if rem>>i&1 == 1 {
				rem ^= 0x537 << (i - 10)
			}
		}
		if rem != 0 {
			t.Errorf("%q: format bits %015b fail the BCH check", text, a)
		}
		if level := raw >> 13; level != 0 {
			t.Errorf("%q: error correction level bits %02b, want 00 (M)", text, level)
		}
	}
}

// decode reads q back: it undoes the mask named in the format bits, reads
// the codewords in placement order, checks each block's error correction
// and returns the byte-mode payload.
func decode(t *testing.T, q *QR) []byte {
	t.Helper()
	ver := (q.Size - 17) / 4
	vi := qrVersions[ver]
	a, _ := formatBits(q)
	mask := (a ^ 0x5412) >> 10 & 7

	b := &qrBuilder{size: q.Size, mod: grid(q.Size), fn: grid(q.Size)}
	b.drawFunctionPatterns(ver)
	for y := range q.Size {
		copy(b.mod[y], q.Modules[y])
	}
	b.applyMask(mask)

	var cw []byte
	var cur byte
	nbits := 0
	n := q.Size
	for right := n - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := range n {
			for j := range 2 {
				x, y := right-j, vert
				if (right+1)&2 == 0 {
					y = n - 1 - vert
				}
				if b.fn[y][x] {
					continue
				}
				cur <<= 1
				if b.mod[y][x] {
					cur |= 1
				}
				if nbits++; nbits%8 == 0 {
					cw = append(cw, cur)
					cur = 0
				}
			}
		}
	}
	if len(cw) < vi.total {
		t.Fatalf("read %d codewords, want %d", len(cw), vi.total)
	}
	dataCw := vi.total - vi.ec*vi.blocks
	per := dataCw / vi.blocks
	div := rsDivisor(vi.ec)
	var data []byte
	for k := range vi.blocks {
		blk := make([]byte, per)
		for i := range per {
			blk[i] = cw[i*vi.blocks+k]
		}
		ec := make([]byte, vi.ec)
		for i := range vi.ec {
			ec[i] = cw[dataCw+i*vi.blocks+k]
		}
		if !bytes.Equal(rsRemainder(blk, div), ec) {
			t.Fatalf("block %d: error correction doesn't match its data", k)
		}
		data = append(data, blk...)
	}
	if mode := data[0] >> 4; mode != 4 {
		t.Fatalf("mode %04b, want 0100 (byte)", mode)
	}
	count := int(data[0]&0xf)<<4 | int(data[1]>>4)
	out := make([]byte, count)
	for i := range out {
		out[i] = data[1+i]<<4 | data[2+i]>>4
	}
	return out
}

func TestRoundTrip(t *testing.T) {
	for _, text := range []string{"", "a", "Hello, Miyoo", loginURL, strings.Repeat("0123456789", 10) + "abcdef", "é€ ünïcode"} {
		q, err := Encode(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := decode(t, q); string(got) != text {
			t.Errorf("decoded %q, want %q", got, text)
		}
	}
}

// A login link always encodes to the same modules.
func TestGolden(t *testing.T) {
	q, err := Encode(loginURL)
	if err != nil {
		t.Fatal(err)
	}
	if q.Size != 33 {
		t.Errorf("size %d, want 33 (version 4)", q.Size)
	}
	var sb strings.Builder
	for _, row := range q.Modules {
		for _, m := range row {
			if m {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(sb.String()))
	const want = "f9fb50efdbe3804e966df8765d8dbc2d8ecb049db13642119ce60b96f0298e35"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("module grid hash %s, want %s\n%s", got, want, sb.String())
	}
}
