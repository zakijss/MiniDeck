// Package appdir finds an app's folder on the card, where it keeps its
// settings, secrets and cache.
package appdir

import (
	"os"
	"path/filepath"
)

// Dir is the folder the running binary is in (App/<Name> on the card), or
// MINIDECK_APPDIR when that is set, so an app can run from a test folder on
// a computer.
func Dir() string {
	if d := os.Getenv("MINIDECK_APPDIR"); d != "" {
		return d
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
