package ui

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---- status: clock, battery, tailnet ----

var status struct {
	mu       sync.Mutex
	read     time.Time
	battery  int  // -1 unknown
	charging bool // the charger is plugged in and charging
	wifi     int  // 1-3 connected, with that many bars of signal; 0 not; -1 unknown
	tailnet  bool
	loc      *time.Location
	axpBusy  bool // a charging check is running (see readCharging)
}

func onionLocation() *time.Location {
	// Onion keeps the zone as a POSIX string such as "UTC+04:00", where the
	// sign is inverted: UTC+04:00 is four hours behind UTC.
	b, err := os.ReadFile("/mnt/SDCARD/.tmp_update/config/.tz")
	if err != nil {
		return time.Local
	}
	tz := strings.TrimSpace(string(b))
	if !strings.HasPrefix(tz, "UTC") || len(tz) < 4 {
		return time.Local
	}
	sign := 1
	if tz[3] == '+' {
		sign = -1
	}
	var h, m int
	fmt.Sscanf(tz[4:], "%d:%d", &h, &m)
	return time.FixedZone(tz, sign*(h*3600+m*60))
}

func readStatus() (int, bool, *time.Location) {
	b, _, tn, loc := readStatusAll()
	return b, tn, loc
}

// readStatusAll also reports Wi-Fi (1-3 connected, with that signal; 0
// not; -1 unknown).
func readStatusAll() (int, int, bool, *time.Location) {
	st := readStatusInfo()
	return st.battery, st.wifi, st.tailnet, st.loc
}

// statusInfo is what the status bar shows.
type statusInfo struct {
	battery  int
	charging bool
	wifi     int
	tailnet  bool
	loc      *time.Location
}

// readStatusInfo reads the status, at most every 20 s: Onion's battery
// percentage, whether it is charging, Wi-Fi and its signal, and whether
// MiniScale is running.
func readStatusInfo() statusInfo {
	status.mu.Lock()
	defer status.mu.Unlock()
	if time.Since(status.read) > 20*time.Second {
		status.read = time.Now()
		status.battery = -1
		if b, err := os.ReadFile("/tmp/percBat"); err == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				status.battery = v
			}
		}
		readCharging()
		status.wifi = -1
		if b, err := os.ReadFile("/sys/class/net/wlan0/operstate"); err == nil {
			status.wifi = 0
			if strings.TrimSpace(string(b)) == "up" {
				status.wifi = 3
				if b, err := os.ReadFile("/proc/net/wireless"); err == nil {
					status.wifi = wifiBars(string(b))
				}
			}
		}
		_, err := os.Stat("/tmp/miniscale/tailscaled.sock")
		status.tailnet = err == nil
		if status.loc == nil {
			status.loc = onionLocation()
		}
	}
	return statusInfo{status.battery, status.charging, status.wifi, status.tailnet, status.loc}
}

// wifiBars turns wlan0's link quality in /proc/net/wireless (out of 70)
// into 1-3 bars; 3 if the line isn't there.
func wifiBars(proc string) int {
	for line := range strings.Lines(proc) {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "wlan0:" {
			continue
		}
		q, err := strconv.ParseFloat(strings.TrimSuffix(f[2], "."), 64)
		switch {
		case err != nil:
			return 3
		case q >= 45:
			return 3
		case q >= 25:
			return 2
		}
		return 1
	}
	return 3
}

// axpTest is the Mini Plus's power chip tool, which Onion also asks
// whether the battery is charging; on the Mini, GPIO 59 tells.
const (
	axpTest = "/customer/app/axp_test"
	gpio59  = "/sys/devices/gpiochip0/gpio/gpio59/value"
)

// readCharging updates status.charging. The Mini's GPIO is a file read;
// the Mini Plus's tool takes a moment, so it runs in the background and
// the bar shows the answer from then on. The caller holds status.mu.
func readCharging() {
	if _, err := os.Stat(axpTest); err != nil {
		b, err := os.ReadFile(gpio59)
		status.charging = err == nil && strings.TrimSpace(string(b)) == "1"
		return
	}
	if status.axpBusy {
		return
	}
	status.axpBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, axpTest)
		cmd.Dir = filepath.Dir(axpTest)
		out, err := cmd.Output()
		status.mu.Lock()
		defer status.mu.Unlock()
		status.axpBusy = false
		if err == nil {
			status.charging = axpCharging(string(out))
		}
	}()
}

// axpCharging reads axp_test's answer, such as
// {"battery":97, "voltage":4174, "charging":3}: 3 is charging.
func axpCharging(out string) bool {
	_, after, ok := strings.Cut(out, `"charging":`)
	if !ok {
		return false
	}
	after = strings.TrimSpace(after)
	end := strings.IndexFunc(after, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		end = len(after)
	}
	return after[:end] == "3"
}

// statusKey changes whenever the status bar would look different.
func statusKey() string {
	st := readStatusInfo()
	return fmt.Sprintf("%d|%v|%d|%v|%s", st.battery, st.charging, st.wifi, st.tailnet, time.Now().In(st.loc).Format("15:04"))
}

// StaticFrame draws a screen's fixed parts (background, large title,
// subtitle, status bar, button bar) and returns where content starts, like
// Background+Header+Footer. The result is kept, so later frames with the
// same title, subtitle and buttons (and the same clock minute and battery)
// copy it instead of drawing it again; then only the content is drawn.
func (s *Screen) StaticFrame(title, sub string, hints ...Hint) int {
	key := fmt.Sprint(title, "\x00", sub, "\x00", hints, "\x00", s.ticker != "", "\x00", statusKey())
	if s.static != nil && s.staticKey == key {
		copy(s.Pix, s.static.Pix)
		s.bgID = s.staticBg
		s.DrawTicker() // it scrolls: always fresh
		return s.staticTop
	}
	s.Background()
	top := s.Header(title, sub)
	s.Footer(hints...)
	if s.static == nil {
		s.static = image.NewRGBA(s.Rect)
	}
	copy(s.static.Pix, s.Pix)
	s.staticKey, s.staticBg, s.staticTop = key, s.bgID, top
	return top
}
