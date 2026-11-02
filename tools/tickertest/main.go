// tickertest draws a screen with the now-playing ticker to a PNG without a
// device (debugging aid, not shipped).
//
//	tickertest out.png
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/zakijss/minideck/ui"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: tickertest out.png")
		os.Exit(2)
	}
	os.Setenv("MINIDECK_UI_PNG", os.Args[1])
	s, err := ui.Open()
	if err != nil {
		log.Fatal(err)
	}
	s.Background()
	s.SetTicker("Northbound   ·   Paper Satellites   ·   Low Orbit")
	s.Header("MiniTape", "")
	for i := 0; i < 3; i++ { // redraw the strip as the marquee does
		s.DrawTicker()
	}
	s.Present()
}
