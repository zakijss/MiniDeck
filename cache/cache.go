// Package cache keeps downloaded data (server replies, images) on the SD
// card, least recently used dropped first.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Disk keeps downloads on the SD card, so screens open at once instead of
// waiting on the network. Files are named by a hash of the key; reading one
// marks it recently used, and Prune drops the least recently used beyond a
// size limit.
type Disk struct {
	dir string
	mu  sync.Mutex
}

// New returns the cache kept in dir, creating the folder if needed.
func New(dir string) *Disk {
	os.MkdirAll(dir, 0755)
	return &Disk{dir: dir}
}

func (c *Disk) path(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(h[:16]))
}

// Get returns the data stored under key, if there is any.
func (c *Disk) Get(key string) ([]byte, bool) {
	p := c.path(key)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	now := time.Now()
	os.Chtimes(p, now, now)
	return b, true
}

// Put stores b under key. It is best effort: a full card only means the
// data is fetched again next time. Each write goes to its own temporary
// file, so two loaders storing the same key at once can't mix their data.
func (c *Disk) Put(key string, b []byte) {
	f, err := os.CreateTemp(c.dir, ".put-*")
	if err != nil {
		return
	}
	_, werr := f.Write(b)
	if cerr := f.Close(); werr != nil || cerr != nil || os.Rename(f.Name(), c.path(key)) != nil {
		os.Remove(f.Name())
	}
}

// Dir is the cache's folder.
func (c *Disk) Dir() string { return c.dir }

// Open opens the file stored under key, for reading in place (a song too
// big to hold in memory), and marks it recently used.
func (c *Disk) Open(key string) (*os.File, bool) {
	p := c.path(key)
	f, err := os.Open(p)
	if err != nil {
		return nil, false
	}
	now := time.Now()
	os.Chtimes(p, now, now)
	return f, true
}

// Temp creates a file in the cache's folder to fill and then Commit. Prune
// leaves it alone while it is being written.
func (c *Disk) Temp() (*os.File, error) {
	return os.CreateTemp(c.dir, ".part-*")
}

// Commit stores the temporary file at path under key.
func (c *Disk) Commit(path, key string) error {
	return os.Rename(path, c.path(key))
}

// Prune deletes the least recently used files until the cache is at most
// max bytes.
func (c *Disk) Prune(max int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ents, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
		used time.Time
	}
	var files []file
	var total int64
	for _, e := range ents {
		fi, err := e.Info()
		if err != nil || e.IsDir() {
			continue
		}
		// a file still being written (Temp), unless it was left by a crash
		if strings.HasPrefix(e.Name(), ".part-") && time.Since(fi.ModTime()) < time.Hour {
			continue
		}
		files = append(files, file{filepath.Join(c.dir, e.Name()), fi.Size(), fi.ModTime()})
		total += fi.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].used.Before(files[j].used) })
	for _, f := range files {
		if total <= max {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
