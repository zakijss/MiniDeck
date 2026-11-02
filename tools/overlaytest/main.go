// overlaytest draws the menu ticker over the Onion menu for a few seconds
// without playing anything (debugging aid, not shipped).
//
//	overlaytest [seconds]
package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/zakijss/minideck/ui"
)

func main() {
	secs := 6
	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil || n <= 0 {
			log.Fatalf("overlaytest: %q is not a number of seconds", os.Args[1])
		}
		secs = n
	}
	ov, err := ui.OpenOverlay()
	if err != nil {
		log.Fatal(err)
	}
	ov.Begin()
	end := time.Now().Add(time.Duration(secs) * time.Second)
	frame := time.NewTicker(80 * time.Millisecond)
	guard := time.NewTicker(8 * time.Millisecond)
	for time.Now().Before(end) {
		select {
		case <-frame.C:
			ov.Draw("Northbound   ·   Paper Satellites   ·   Low Orbit")
		case <-guard.C:
			ov.Guard()
		}
	}
}
