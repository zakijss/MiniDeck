package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
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

// Saves of the same file from several goroutines at once each land whole:
// the file always holds one of the writes, never a mix.
func TestWriteConcurrent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "progress.json")
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				if err := Write(p, bytes.Repeat([]byte{'a' + byte(i)}, 16<<10), 0644); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	b, _ := os.ReadFile(p)
	if len(b) != 16<<10 || !bytes.Equal(b, bytes.Repeat(b[:1], len(b))) {
		t.Fatalf("mixed or short file: %d bytes", len(b))
	}
}
