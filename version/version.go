// Package version holds the release an app's binaries were built from. The
// app's scripts/build.sh stamps it:
//
//	go build -ldflags "-X github.com/zakijss/minideck/version.stamp=minideck-version=1.0.0"
package version

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// stamp carries a prefix so an app's scripts/package.sh can find the
// version inside an ARM binary it can't run (with -trimpath, Go's build
// info leaves the -X flags out).
var stamp = "minideck-version=dev"

// Version is the release, or "dev" for a build without the stamp.
var Version = strings.TrimPrefix(stamp, "minideck-version=")

// String returns Version.
func String() string { return Version }

// Flag handles -version (or --version) as the first argument: it prints
// "name VERSION" and exits. Call it first thing in main, before the screen
// opens.
func Flag(name string) {
	if len(os.Args) > 1 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Println(name, Version)
		os.Exit(0)
	}
}

// Note sets up the standard logger for the program called name, so every
// line reads "2026/11/01 12:00:00 name: message", and logs which release
// is starting. The log goes to stderr, which launch.sh sends to the app's
// log file, so a bug report's log says which release it came from.
func Note(name string) {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix(name + ": ")
	log.Printf("%s starting", Version)
}
