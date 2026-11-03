package main

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestUnpackRejectsEscapes(t *testing.T) {
	for _, name := range []string{"..", "../x", "a/../../x", "/etc/x"} {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		w.Create(name)
		w.Close()
		if err := unpack(buf.Bytes(), map[string]file{}); err == nil {
			t.Errorf("%q: unpacked, want an error", name)
		}
	}
}
