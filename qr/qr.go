// Package qr is a small QR code encoder: byte mode, error correction level
// M, versions 1 to 6 (up to 106 bytes), which is plenty for a token or a
// login link. It follows ISO/IEC 18004 (and Project Nayuki's reference
// implementation). Apps draw the modules themselves.
package qr

import "errors"

// qrVersions holds, for level M: total codewords, EC codewords per block,
// number of blocks (all the same size for versions 1 to 6).
var qrVersions = [...]struct{ total, ec, blocks int }{
	{}, // version 0 doesn't exist
	{26, 10, 1},
	{44, 16, 1},
	{70, 26, 1},
	{100, 18, 2},
	{134, 24, 2},
	{172, 16, 4},
}

// QR is a square of modules, Modules[y][x]; true is dark. It has no quiet
// zone: leave four light modules around it when drawing.
type QR struct {
	Size    int
	Modules [][]bool
}

func gfMul(x, y byte) byte {
	var z int
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11d)
		z ^= int((y>>uint(i))&1) * int(x)
	}
	return byte(z)
}

func rsDivisor(degree int) []byte {
	r := make([]byte, degree)
	r[degree-1] = 1
	root := byte(1)
	for range degree {
		for j := range r {
			r[j] = gfMul(r[j], root)
			if j+1 < len(r) {
				r[j] ^= r[j+1]
			}
		}
		root = gfMul(root, 2)
	}
	return r
}

func rsRemainder(data, div []byte) []byte {
	r := make([]byte, len(div))
	for _, b := range data {
		f := b ^ r[0]
		copy(r, r[1:])
		r[len(r)-1] = 0
		for i, c := range div {
			r[i] ^= gfMul(c, f)
		}
	}
	return r
}

// Encode encodes text as a QR code at level M in the smallest version
// that fits.
func Encode(text string) (*QR, error) {
	data := []byte(text)
	ver := 0
	for v := 1; v < len(qrVersions); v++ {
		capBits := (qrVersions[v].total - qrVersions[v].ec*qrVersions[v].blocks) * 8
		if 4+8+len(data)*8 <= capBits {
			ver = v
			break
		}
	}
	if ver == 0 {
		return nil, errors.New("qr: text too long")
	}
	vi := qrVersions[ver]
	dataCw := vi.total - vi.ec*vi.blocks

	// Bit stream: mode 0100 (byte), 8-bit count, data, terminator, padding.
	var bits []bool
	put := func(v, n int) {
		for i := n - 1; i >= 0; i-- {
			bits = append(bits, (v>>uint(i))&1 == 1)
		}
	}
	put(4, 4)
	put(len(data), 8)
	for _, b := range data {
		put(int(b), 8)
	}
	put(0, min(4, dataCw*8-len(bits)))
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	for pad := 0xec; len(bits) < dataCw*8; pad ^= 0xec ^ 0x11 {
		put(pad, 8)
	}
	cw := make([]byte, dataCw)
	for i, b := range bits {
		if b {
			cw[i/8] |= 1 << uint(7-i%8)
		}
	}

	// Split into blocks, add error correction, interleave.
	per := dataCw / vi.blocks
	div := rsDivisor(vi.ec)
	var blocks, ecs [][]byte
	for i := range vi.blocks {
		blk := cw[i*per : (i+1)*per]
		blocks = append(blocks, blk)
		ecs = append(ecs, rsRemainder(blk, div))
	}
	var final []byte
	for i := range per {
		for _, b := range blocks {
			final = append(final, b[i])
		}
	}
	for i := range vi.ec {
		for _, e := range ecs {
			final = append(final, e[i])
		}
	}

	size := ver*4 + 17
	q := &qrBuilder{size: size, mod: grid(size), fn: grid(size)}
	q.drawFunctionPatterns(ver)
	q.drawCodewords(final)

	best, bestPen := -1, 0
	for m := range 8 {
		q.applyMask(m)
		q.drawFormat(m)
		if p := q.penalty(); best < 0 || p < bestPen {
			best, bestPen = m, p
		}
		q.applyMask(m) // XOR again undoes it
	}
	q.applyMask(best)
	q.drawFormat(best)
	return &QR{Size: size, Modules: q.mod}, nil
}

func grid(n int) [][]bool {
	g := make([][]bool, n)
	for i := range g {
		g[i] = make([]bool, n)
	}
	return g
}

type qrBuilder struct {
	size    int
	mod, fn [][]bool // modules, and which are function patterns
}

func (q *qrBuilder) set(x, y int, dark bool) {
	q.mod[y][x] = dark
	q.fn[y][x] = true
}

func (q *qrBuilder) drawFunctionPatterns(ver int) {
	n := q.size
	for i := range n {
		q.set(6, i, i%2 == 0)
		q.set(i, 6, i%2 == 0)
	}
	finder := func(cx, cy int) {
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= n || y >= n {
					continue
				}
				d := max(abs(dx), abs(dy))
				q.set(x, y, d != 2 && d != 4)
			}
		}
	}
	finder(3, 3)
	finder(n-4, 3)
	finder(3, n-4)
	if ver >= 2 {
		c := n - 7
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				q.set(c+dx, c+dy, max(abs(dx), abs(dy)) != 1)
			}
		}
	}
	q.drawFormat(0) // reserve the format areas; redrawn per mask
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (q *qrBuilder) drawFormat(mask int) {
	data := 0<<3 | mask // level M is 00
	rem := data
	for range 10 {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return (bits>>uint(i))&1 == 1 }
	n := q.size
	for i := 0; i <= 5; i++ {
		q.set(8, i, bit(i))
	}
	q.set(8, 7, bit(6))
	q.set(8, 8, bit(7))
	q.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		q.set(14-i, 8, bit(i))
	}
	for i := range 8 {
		q.set(n-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		q.set(8, n-15+i, bit(i))
	}
	q.set(8, n-8, true) // the dark module
}

func (q *qrBuilder) drawCodewords(data []byte) {
	n := q.size
	i := 0
	for right := n - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := range n {
			for j := range 2 {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = n - 1 - vert
				}
				if !q.fn[y][x] && i < len(data)*8 {
					q.mod[y][x] = (data[i>>3]>>uint(7-i&7))&1 == 1
					i++
				}
			}
		}
	}
}

func (q *qrBuilder) applyMask(m int) {
	for y := range q.size {
		for x := range q.size {
			var inv bool
			switch m {
			case 0:
				inv = (x+y)%2 == 0
			case 1:
				inv = y%2 == 0
			case 2:
				inv = x%3 == 0
			case 3:
				inv = (x+y)%3 == 0
			case 4:
				inv = (x/3+y/2)%2 == 0
			case 5:
				inv = x*y%2+x*y%3 == 0
			case 6:
				inv = (x*y%2+x*y%3)%2 == 0
			case 7:
				inv = ((x+y)%2+x*y%3)%2 == 0
			}
			if inv && !q.fn[y][x] {
				q.mod[y][x] = !q.mod[y][x]
			}
		}
	}
}

// penalty scores a masked symbol (lower is better), using the standard's
// four rules in a simplified form.
func (q *qrBuilder) penalty() int {
	n := q.size
	at := func(x, y int, row bool) bool {
		if row {
			return q.mod[y][x]
		}
		return q.mod[x][y]
	}
	p := 0
	for _, row := range []bool{true, false} {
		for y := range n {
			run := 1
			for x := 1; x <= n; x++ {
				if x < n && at(x, y, row) == at(x-1, y, row) {
					run++
					continue
				}
				if run >= 5 {
					p += 3 + run - 5
				}
				run = 1
			}
			// finder-like 1:1:3:1:1 with four light modules on one side
			for x := 0; x+11 <= n; x++ {
				a := []bool{true, false, true, true, true, false, true, false, false, false, false}
				b := []bool{false, false, false, false, true, false, true, true, true, false, true}
				ma, mb := true, true
				for k := range 11 {
					v := at(x+k, y, row)
					ma = ma && v == a[k]
					mb = mb && v == b[k]
				}
				if ma {
					p += 40
				}
				if mb {
					p += 40
				}
			}
		}
	}
	dark := 0
	for y := range n - 1 {
		for x := range n - 1 {
			c := q.mod[y][x]
			if c == q.mod[y][x+1] && c == q.mod[y+1][x] && c == q.mod[y+1][x+1] {
				p += 3
			}
		}
	}
	for y := range n {
		for x := range n {
			if q.mod[y][x] {
				dark++
			}
		}
	}
	k := (abs(dark*20-n*n*10) + n*n - 1) / (n * n)
	return p + k*10
}
