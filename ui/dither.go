package ui

import "image"

// PixelArt redraws a picture in the interface's pixel style: one colour per
// art pixel (art x art screen pixels, the average of the pixels it covers),
// each channel reduced to the given number of levels with the same 4x4
// ordered dither as the background. The result has the picture's size.
// levels below 2 are taken as 2.
//
// It runs once per picture (a 232 px cover takes a few milliseconds on the
// device), not per frame: keep the result.
func PixelArt(src *image.RGBA, levels int) *image.RGBA {
	levels = max(levels, 2)
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	step := 255 / float64(levels-1)
	for cy := 0; cy*art < b.Dy(); cy++ {
		for cx := 0; cx*art < b.Dx(); cx++ {
			// the cell's average colour
			var sum [3]int
			n := 0
			for dy := 0; dy < art && cy*art+dy < b.Dy(); dy++ {
				i := src.PixOffset(b.Min.X+cx*art, b.Min.Y+cy*art+dy)
				for dx := 0; dx < art && cx*art+dx < b.Dx(); dx++ {
					sum[0] += int(src.Pix[i])
					sum[1] += int(src.Pix[i+1])
					sum[2] += int(src.Pix[i+2])
					i += 4
					n++
				}
			}
			threshold := (float64(bayer4[cy&3][cx&3]) + 0.5) / 16
			var c [3]uint8
			for k := range c {
				v := float64(sum[k]) / float64(n) / step
				q := float64(int(v))
				if v-q > threshold {
					q++
				}
				c[k] = uint8(min(255, q*step+0.5))
			}
			for dy := 0; dy < art && cy*art+dy < b.Dy(); dy++ {
				i := out.PixOffset(cx*art, cy*art+dy)
				for dx := 0; dx < art && cx*art+dx < b.Dx(); dx++ {
					out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = c[0], c[1], c[2], 255
					i += 4
				}
			}
		}
	}
	return out
}
