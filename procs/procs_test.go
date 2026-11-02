package procs

import (
	"os"
	"path/filepath"
	"testing"
)

func fakeProc(t *testing.T, comms map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for pid, comm := range comms {
		if err := os.MkdirAll(filepath.Join(dir, pid), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, pid, "comm"), []byte(comm), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := procDir
	procDir = dir
	t.Cleanup(func() { procDir = old })
}

func TestPid(t *testing.T) {
	fakeProc(t, map[string]string{
		"1":    "init\n",
		"123":  "MainUI\n",
		"456":  "audioserver\n",
		"789":  "Main UI\n",
		"self": "MainUI\n",   // not a pid
		"900":  "tailscaled", // no newline
	})
	for name, want := range map[string]int{
		"MainUI": 123, "audioserver": 456, "tailscaled": 900, "Main": 0, "MainU": 0, "": 0, "retroarch": 0,
	} {
		if got := Pid(name); got != want {
			t.Errorf("Pid(%q) = %d, want %d", name, got, want)
		}
	}
	if !Running("audioserver") || Running("ffplay") {
		t.Error("Running")
	}
	if !Exists(123) || Exists(124) {
		t.Error("Exists")
	}
}

func TestGameRunning(t *testing.T) {
	cmd := filepath.Join(t.TempDir(), "cmd_to_run.sh")
	old := cmdToRun
	cmdToRun = cmd
	t.Cleanup(func() { cmdToRun = old })
	if GameRunning() {
		t.Error("a game runs without cmd_to_run.sh")
	}
	for s, want := range map[string]bool{
		`cd /mnt/SDCARD/RetroArch; ./retroarch -L core.so "/mnt/SDCARD/Roms/GBA/x.gba"`: true,
		`"/mnt/SDCARD/Emu/PICO/launch.sh" "/mnt/SDCARD/Roms/PICO/a.p8"`:                 true,
		`cd "/mnt/SDCARD/App/MiniRack"; ./launch.sh`:                                    false,
	} {
		if err := os.WriteFile(cmd, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := GameRunning(); got != want {
			t.Errorf("GameRunning with %q = %v", s, got)
		}
	}
}
