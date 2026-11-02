package cache

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestPutGetPrune(t *testing.T) {
	c := New(t.TempDir())
	if _, ok := c.Get("a"); ok {
		t.Fatal("Get on an empty cache found something")
	}
	old := time.Now().Add(-time.Hour)
	for i, k := range []string{"a", "b", "c"} {
		c.Put(k, bytes.Repeat([]byte{byte('a' + i)}, 100))
		at := old.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(c.path(k), at, at); err != nil {
			t.Fatal(err)
		}
	}
	if b, ok := c.Get("a"); !ok || len(b) != 100 || b[0] != 'a' {
		t.Fatalf("Get(a) = %d bytes, %v", len(b), ok)
	}
	// "a" was just read, so "b" and then "c" are the least recently used.
	c.Prune(150)
	if _, ok := c.Get("b"); ok {
		t.Error("b survived the prune")
	}
	if _, ok := c.Get("c"); ok {
		t.Error("c survived the prune")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("a, the most recently used, was pruned")
	}
	ents, _ := os.ReadDir(c.dir)
	if len(ents) != 1 {
		t.Errorf("%d files left, want 1 (no temporary files)", len(ents))
	}
}
