package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDir(t *testing.T) {
	t.Setenv("MINIDECK_APPDIR", "/x/App/Test")
	if got := Dir(); got != "/x/App/Test" {
		t.Errorf("Dir() = %q with MINIDECK_APPDIR set", got)
	}
	t.Setenv("MINIDECK_APPDIR", "")
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	if got := Dir(); got != filepath.Dir(exe) {
		t.Errorf("Dir() = %q, want the binary's folder %q", got, filepath.Dir(exe))
	}
}
