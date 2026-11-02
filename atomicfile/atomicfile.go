// Package atomicfile writes files so that a crash, a power cut or a pulled
// SD card leaves either the old file or the new one, never half of one.
package atomicfile

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

// ErrReadBack means the data read back from the card wasn't what was
// written; the old file is left in place.
var ErrReadBack = errors.New("the card didn't store the file correctly")

// Write writes b to a hidden temporary file beside path, flushes it to the
// card and renames it over path. The flush matters on the Miyoo's FAT card:
// a rename without it can survive a power cut while the data doesn't.
func Write(path string, b []byte, perm os.FileMode) error {
	return write(path, b, perm, false)
}

// WriteChecked is Write that also reads the temporary file back before the
// rename, for files whose loss would be worse than a failed save: a card
// that corrupts data on the way gives ErrReadBack instead of replacing a
// good file with a bad one.
func WriteChecked(path string, b []byte, perm os.FileMode) error {
	return write(path, b, perm, true)
}

func write(path string, b []byte, perm os.FileMode, check bool) error {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if check {
		back, err := os.ReadFile(tmp)
		if err != nil || !bytes.Equal(back, b) {
			os.Remove(tmp)
			return ErrReadBack
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
