// bundle assembles MiniDeck's release from the apps' own releases: it
// fetches each app's latest app.json (its entry in MiniMart's catalogue)
// and zip, checks the zip against the entry's SHA-256 and size, and writes
// to the output folder:
//
//   - catalog.json: MiniMart's catalogue of MiniDeck's apps (MiniMart's
//     built-in "MiniDeck" source reads it from MiniDeck's latest release);
//   - minideck-all.zip: every app in one zip, laid out to unzip onto the SD
//     card root;
//   - the apps' zips, for checking the catalogue offline;
//   - SHA256SUMS.
//
// Run it from the repository root:
//
//	go run ./tools/bundle -o dist
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// repos are the apps' repositories, in the order MiniMart lists them.
var repos = []string{"MiniScale", "MiniRack", "MiniTape", "MiniFin", "MiniKey", "MiniTop", "MiniMart", "MiniBook", "MiniRadar", "MiniPulse"}

const owner = "zakijss"

var client = &http.Client{Timeout: 2 * time.Minute}

func main() {
	log.SetFlags(0)
	out := flag.String("o", "dist", "output folder")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	var entries []json.RawMessage
	all := map[string]file{} // path in the all-apps zip
	var zips []string
	for _, repo := range repos {
		base := fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/", owner, repo)
		raw, err := get(base + "app.json")
		if err != nil {
			log.Fatalf("%s: %v", repo, err)
		}
		var e struct {
			Name, Version, Zip, SHA256 string
			Size                       int64
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			log.Fatalf("%s: app.json: %v", repo, err)
		}
		z, err := get(e.Zip)
		if err != nil {
			log.Fatalf("%s: %v", repo, err)
		}
		sum := sha256.Sum256(z)
		if hex.EncodeToString(sum[:]) != e.SHA256 || int64(len(z)) != e.Size {
			log.Fatalf("%s: %s doesn't match app.json's SHA-256 and size", repo, path.Base(e.Zip))
		}
		if err := unpack(z, all); err != nil {
			log.Fatalf("%s: %v", repo, err)
		}
		name := path.Base(e.Zip)
		if err := os.WriteFile(filepath.Join(*out, name), z, 0o644); err != nil {
			log.Fatal(err)
		}
		zips = append(zips, name)
		entries = append(entries, json.RawMessage(bytes.TrimSpace(raw)))
		log.Printf("%-10s %s  %s", repo, e.Version, name)
	}

	cat, err := json.MarshalIndent(struct {
		Name    string            `json:"name"`
		Version string            `json:"version"`
		Apps    []json.RawMessage `json:"apps"`
	}{"MiniDeck", time.Now().UTC().Format("2006.01.02"), entries}, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	write(*out, "catalog.json", append(cat, '\n'))
	write(*out, "minideck-all.zip", pack(all))

	var sums strings.Builder
	for _, n := range append(zips, "minideck-all.zip", "catalog.json") {
		b, err := os.ReadFile(filepath.Join(*out, n))
		if err != nil {
			log.Fatal(err)
		}
		s := sha256.Sum256(b)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(s[:]), n)
	}
	write(*out, "SHA256SUMS", []byte(sums.String()))
	fmt.Print(sums.String())
}

func get(u string) ([]byte, error) {
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", u, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// file is one file of an app's zip, kept with its permissions.
type file struct {
	data []byte
	mode os.FileMode
}

// unpack adds the files of zip z to all. Two apps may not ship the same
// file.
func unpack(z []byte, all map[string]file) error {
	r, err := zip.NewReader(bytes.NewReader(z), int64(len(z)))
	if err != nil {
		return err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(f.Name)
		if name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
			return fmt.Errorf("unsafe path %q", f.Name)
		}
		if _, dup := all[name]; dup {
			return fmt.Errorf("%s is in two apps' zips", name)
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		all[name] = file{b, f.Mode()}
	}
	return nil
}

// pack zips files, in name order.
func pack(files map[string]file) []byte {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, n := range names {
		h := &zip.FileHeader{Name: n, Method: zip.Deflate}
		h.SetMode(files[n].mode)
		fw, err := w.CreateHeader(h)
		if err != nil {
			log.Fatal(err)
		}
		fw.Write(files[n].data)
	}
	if err := w.Close(); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func write(dir, name string, b []byte) {
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		log.Fatal(err)
	}
}
