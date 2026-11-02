package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	if err := Write(p, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(p, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "two" {
		t.Fatalf("got %q", b)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatalf("left behind: %v", ents)
	}
	// a folder that doesn't exist: an error, nothing written
	if err := Write(filepath.Join(dir, "no", "x"), []byte("x"), 0644); err == nil {
		t.Fatal("no error")
	}
}

func TestWriteChecked(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "vault.enc")
	if err := WriteChecked(p, []byte("sealed"), 0600); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "sealed" {
		t.Fatalf("got %q", b)
	}
	// A rename that fails (a folder in the way) leaves no temporary file.
	q := filepath.Join(dir, "busy")
	os.MkdirAll(filepath.Join(q, "inside"), 0755)
	if err := WriteChecked(q, []byte("x"), 0600); err == nil {
		t.Fatal("no error")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 2 {
		t.Fatalf("left behind: %v", ents)
	}
}
