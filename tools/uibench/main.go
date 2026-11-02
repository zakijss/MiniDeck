// uibench times drawing the heaviest screens on the device (without showing
// them) and the framebuffer copy. Not shipped.
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"time"

	"github.com/zakijss/minideck/ui"
)

// timeit runs f n times and prints the average time per run.
func timeit(name string, n int, f func()) {
	start := time.Now()
	for i := 0; i < n; i++ {
		f()
	}
	fmt.Printf("%-22s %6.1f ms/frame\n", name, float64(time.Since(start).Microseconds())/1000/float64(n))
}

func main() {
	os.Setenv("MINIDECK_UI_NULL", "1")
	s, err := ui.Open()
	if err != nil {
		log.Fatal(err)
	}
	cover := image.NewRGBA(image.Rect(0, 0, 240, 240))
	for i := range cover.Pix {
		cover.Pix[i] = uint8(i * 7)
	}
	bd := ui.Backdrop(cover)
	timeit("gauge cards (4)", 10, func() {
		s.Background()
		s.Header("MiniRack", "3 of 4 up")
		for i := 0; i < 4; i++ {
			x, y := 20+(i%2)*306, 60+(i/2)*184
			r := image.Rect(x, y, x+294, y+172)
			s.Panel(r, 16)
			for g := 0; g < 3; g++ {
				s.Gauge(x+52+g*95, y+82, 30, float64(30+g*25), "CPU")
			}
		}
		s.Footer(ui.Hint{Button: "A", Label: "Details"}, ui.Hint{Button: "B", Label: "Exit"})
	})
	items := []ui.Item{}
	for i := 0; i < 20; i++ {
		items = append(items, ui.Item{Label: "Album title", Sub: "Artist · 2001", Image: func() image.Image { return cover }})
	}
	l := ui.List{Items: items}
	timeit("list with covers", 10, func() {
		s.Background()
		s.Header("Recently added", "")
		l.Draw(s, image.Rect(14, 58, 626, 430))
		s.Footer(ui.Hint{Button: "A", Label: "Open"})
	})
	timeit("now playing", 10, func() {
		s.Blit(bd)
		r := image.Rect(28, 62, 260, 294)
		s.Shadow(r, 16)
		s.RoundPicture(cover, r, 16)
		s.Text(286, 70, "Signal Fire", ui.Display, ui.Text)
		s.Circle(450, 334, 34, color.RGBA{255, 122, 89, 255})
		s.Icon("pause", 430, 314, 40, ui.BG)
		s.Icon("prev", 330, 316, 36, ui.Text)
		s.Icon("next", 518, 316, 36, ui.Text)
	})
	timeit("keyboard keys (50)", 10, func() {
		s.Clear(ui.BG)
		for i := 0; i < 50; i++ {
			x, y := 3+(i%10)*64, 150+(i/10)*54
			s.RoundRect(image.Rect(x, y, x+58, y+48), 9, ui.Raised)
			s.TextCenter(x+29, y+12, "q", ui.Heading, ui.Text)
		}
	})
	// A byte-by-byte framebuffer copy (rotate + BGRA), the baseline for the
	// word-at-a-time copy Present uses (BenchmarkCopyRotated in ui).
	dst := make([]byte, 640*480*4)
	timeit("framebuffer copy", 10, func() {
		src := s.Pix
		n := 640 * 480
		for i := 0; i < n; i++ {
			si, di := i*4, (n-1-i)*4
			dst[di], dst[di+1], dst[di+2], dst[di+3] = src[si+2], src[si+1], src[si], 0xff
		}
	})
}
