// Package procs looks at the processes running on the device: the Onion
// menu, the audio server, and whether a game is running. It reads /proc
// and Onion's files directly, since BusyBox has no pidof the apps can
// rely on and the badge must stay small.
package procs

import (
	"os"
	"strconv"
	"strings"
)

// Where to look; tests point these at a temporary directory.
var (
	procDir  = "/proc"
	cmdToRun = "/mnt/SDCARD/.tmp_update/cmd_to_run.sh"
)

// Pid returns the pid of a process whose command name is name, or 0. The
// kernel keeps only the first 15 bytes of a command name, so longer names
// never match.
func Pid(name string) int {
	ents, _ := os.ReadDir(procDir)
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if b, err := os.ReadFile(procDir + "/" + e.Name() + "/comm"); err == nil &&
			strings.TrimSuffix(string(b), "\n") == name {
			return pid
		}
	}
	return 0
}

// Running reports whether a process with this command name exists.
func Running(name string) bool { return Pid(name) > 0 }

// Exists reports whether process pid is still running.
func Exists(pid int) bool {
	_, err := os.Stat(procDir + "/" + strconv.Itoa(pid))
	return err == nil
}

// GameRunning reports whether Onion is about to run, or is running, a game
// (not an app): the command Onion's menu left for its launcher names
// RetroArch or a file under Roms or Emu.
func GameRunning() bool {
	b, err := os.ReadFile(cmdToRun)
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, "retroarch") || strings.Contains(s, "/Roms/") || strings.Contains(s, "/Emu/")
}
