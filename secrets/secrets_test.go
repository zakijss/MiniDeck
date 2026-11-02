package secrets

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINIDECK_DEVKEY", filepath.Join(dir, "key"))
	p := filepath.Join(dir, "secrets.enc")
	if err := Save(p, map[string]string{"navidrome": "pä$$ w0rd"}); err != nil {
		t.Fatal(err)
	}
	m, err := Load(p)
	if err != nil || m["navidrome"] != "pä$$ w0rd" {
		t.Fatalf("got %v, %v", m, err)
	}
	// The file must open with the openssl command miniscale.sh uses.
	if _, err := exec.LookPath("openssl"); err == nil {
		out, err := exec.Command("openssl", "enc", "-d", "-aes-256-cbc", "-pbkdf2", "-iter", "4096",
			"-pass", "file:"+os.Getenv("MINIDECK_DEVKEY"), "-in", p).Output()
		if err != nil || string(out) != `{"navidrome":"pä$$ w0rd"}` {
			t.Fatalf("openssl: %q %v", out, err)
		}
	}
	os.WriteFile(os.Getenv("MINIDECK_DEVKEY"), []byte("0000\n"), 0600)
	if _, err := Load(p); err == nil {
		t.Fatal("wrong key accepted")
	}
}

// Files sealed with 200000 rounds still open, and are re-sealed with 4096.
func TestSlowFileResealed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINIDECK_DEVKEY", filepath.Join(dir, "key"))
	if err := EnsureDeviceKey(); err != nil {
		t.Fatal(err)
	}
	key, _ := deviceKey()
	old, err := Encrypt([]byte(`{"beszel":"s3cret"}`), key) // 200000 rounds
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "secrets.enc")
	os.WriteFile(p, old, 0600)
	m, err := Load(p)
	if err != nil || m["beszel"] != "s3cret" {
		t.Fatalf("legacy load: %v %v", m, err)
	}
	now, _ := os.ReadFile(p)
	if pt, err := decrypt(now, key, fastIter); err != nil || string(pt) != `{"beszel":"s3cret"}` {
		t.Fatalf("not re-sealed in the fast format: %v", err)
	}
}

// A file that won't open is set aside, not left to be overwritten by the
// next Save.
func TestLoadOrSetAside(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINIDECK_DEVKEY", filepath.Join(dir, "key"))
	p := filepath.Join(dir, "secrets.enc")
	if m, err := LoadOrSetAside(p); err != nil || len(m) != 0 {
		t.Fatalf("missing file: %v %v", m, err)
	}
	if err := Save(p, map[string]string{"beszel": "pw"}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(os.Getenv("MINIDECK_DEVKEY"), []byte("0000\n"), 0600) // another device's key
	m, err := LoadOrSetAside(p)
	if err == nil || m == nil || len(m) != 0 {
		t.Fatalf("got %v, %v", m, err)
	}
	if _, err := os.Stat(p + ".unreadable"); err != nil {
		t.Fatal("not kept aside:", err)
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("still in place, to be overwritten")
	}
}

// A second unreadable file is kept beside the first, never over it: the
// first may be the only copy a restored key opens.
func TestSetAsideKeepsEarlierCopies(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MINIDECK_DEVKEY", filepath.Join(dir, "key"))
	p := filepath.Join(dir, "secrets.enc")
	var kept [][]byte
	for range 3 {
		os.WriteFile(p, []byte("damaged "+string(rune('a'+len(kept)))), 0600)
		b, _ := os.ReadFile(p)
		kept = append(kept, b)
		if _, err := LoadOrSetAside(p); err == nil {
			t.Fatal("damaged file opened")
		}
	}
	for i, name := range []string{".unreadable", ".unreadable.2", ".unreadable.3"} {
		b, err := os.ReadFile(p + name)
		if err != nil || string(b) != string(kept[i]) {
			t.Errorf("%s: %q %v, want %q", name, b, err, kept[i])
		}
	}
}
