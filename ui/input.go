package ui

import (
	"encoding/binary"
	"os"
	"strings"
	"time"
)

// Button is one of the Miyoo Mini's buttons.
type Button int

const (
	BtnNone Button = iota
	Up
	Down
	Left
	Right
	A
	B
	X
	Y
	L1
	R1
	L2
	R2
	Select
	Start
	Menu
	Power
	VolUp
	VolDown
)

// Linux key codes the Miyoo Mini's gpio_keys driver sends.
var keycodes = map[uint16]Button{
	103: Up, 108: Down, 105: Left, 106: Right,
	57: A, 29: B, 42: X, 56: Y,
	18: L1, 20: R1, 15: L2, 14: R2,
	97: Select, 28: Start, 1: Menu,
	116: Power, 115: VolUp, 114: VolDown,
}

// Event is a button going down (Pressed) or up.
type Event struct {
	Button  Button
	Pressed bool // key down or auto-repeat
	Repeat  bool
}

// Input streams button events from /dev/input/event0. Without the device
// (for example on a computer) the channel just never delivers.
func Input() <-chan Event {
	ch := make(chan Event, 16)
	if keys := os.Getenv("MINIDECK_UI_KEYS"); keys != "" {
		go scripted(ch, keys)
		return ch
	}
	f, err := os.Open("/dev/input/event0")
	if err != nil {
		return ch
	}
	go func() {
		// struct input_event on 32-bit ARM: timeval (2 x int32), type u16,
		// code u16, value s32 = 16 bytes.
		buf := make([]byte, 16)
		for {
			if _, err := f.Read(buf); err != nil {
				close(ch)
				return
			}
			typ := binary.LittleEndian.Uint16(buf[8:])
			code := binary.LittleEndian.Uint16(buf[10:])
			val := int32(binary.LittleEndian.Uint32(buf[12:]))
			if typ != 1 { // EV_KEY
				continue
			}
			b, ok := keycodes[code]
			if !ok {
				continue
			}
			// Releases are dropped: nothing uses them, and each one cost a
			// full redraw. Auto-repeats are dropped while earlier events
			// still wait, so holding a direction can't build up a backlog
			// that keeps scrolling after the button is let go.
			if val == 0 || (val == 2 && len(ch) > 0) {
				continue
			}
			notePress()
			ch <- Event{Button: b, Pressed: true, Repeat: val == 2}
		}
	}()
	return ch
}

var names = map[string]Button{"up": Up, "down": Down, "left": Left, "right": Right,
	"a": A, "b": B, "x": X, "y": Y, "l1": L1, "r1": R1, "l2": L2, "r2": R2,
	"select": Select, "start": Start, "menu": Menu}

// scripted replays MINIDECK_UI_KEYS for tests, e.g. "wait,a,wait,down,b":
// one entry every 1.5 s ("wait" presses nothing), then closes the channel.
// Put %d in MINIDECK_UI_PNG to keep every frame (frame1.png, frame2.png...).
func scripted(ch chan Event, keys string) {
	for _, k := range strings.Split(keys, ",") {
		time.Sleep(1500 * time.Millisecond)
		if b, ok := names[strings.TrimSpace(k)]; ok {
			ch <- Event{Button: b, Pressed: true}
			ch <- Event{Button: b, Pressed: false}
		}
	}
	time.Sleep(1500 * time.Millisecond)
	close(ch)
}
