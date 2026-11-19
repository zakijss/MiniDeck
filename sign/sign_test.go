package sign

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
)

// testdata holds the public half of a throwaway key (test.pub) and files
// it signed with minisign 0.12: msg.txt prehashed (msg.txt.minisig) and
// legacy (msg.legacy.minisig), and bigFile() prehashed (big.txt.minisig).

func read(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bigFile is the 429 kB file big.txt.minisig signs, made here rather than
// kept in the repository.
func bigFile() []byte {
	var b bytes.Buffer
	for i := range 40000 {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.Bytes()
}

func testKey(t *testing.T) PublicKey {
	k, err := ParsePublicKey(string(read(t, "test.pub")))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestMinisignVectors(t *testing.T) {
	k := testKey(t)
	if k.IDString() != "5D74EC25FBDD46EC" {
		t.Fatalf("key id %s", k.IDString())
	}
	if bare, err := ParsePublicKey(ReleaseKey + "\n"); err != nil || bare.ID != k.ID {
		t.Fatalf("bare key line: %v", err)
	}
	for _, c := range []struct {
		sig, alg, comment string
		data              []byte
	}{
		{"msg.txt.minisig", "ED", "test file", read(t, "msg.txt")},
		{"msg.legacy.minisig", "Ed", "timestamp:1795107198\tfile:msg.txt", read(t, "msg.txt")},
		{"big.txt.minisig", "ED", "big file", bigFile()},
	} {
		s, err := ParseSignature(read(t, c.sig))
		if err != nil {
			t.Fatalf("%s: %v", c.sig, err)
		}
		if s.Algorithm != c.alg {
			t.Errorf("%s: algorithm %q", c.sig, s.Algorithm)
		}
		got, err := Verify(k, s, bytes.NewReader(c.data))
		if err != nil || got != c.comment {
			t.Errorf("%s: %q, %v", c.sig, got, err)
		}
		// one byte changed, added or cut
		for _, bad := range [][]byte{
			append([]byte{c.data[0] ^ 1}, c.data[1:]...),
			append(append([]byte{}, c.data...), '\n'),
			c.data[:len(c.data)-1],
		} {
			if _, err := Verify(k, s, bytes.NewReader(bad)); err == nil {
				t.Errorf("%s: tampered file verified", c.sig)
			}
		}
		// the trusted comment changed
		s2 := s
		s2.Trusted += "x"
		if _, err := Verify(k, s2, bytes.NewReader(c.data)); err == nil || !strings.Contains(err.Error(), "trusted comment") {
			t.Errorf("%s: tampered comment: %v", c.sig, err)
		}
		// the comment changed in the file itself
		txt := strings.Replace(string(read(t, c.sig)), "\ntrusted comment: ", "\ntrusted comment: !", 1)
		if s3, err := ParseSignature([]byte(txt)); err != nil {
			t.Error(err)
		} else if _, err := Verify(k, s3, bytes.NewReader(c.data)); err == nil {
			t.Errorf("%s: edited comment line verified", c.sig)
		}
	}
}

func TestWrongKey(t *testing.T) {
	k := testKey(t)
	s, err := ParseSignature(read(t, "msg.txt.minisig"))
	if err != nil {
		t.Fatal(err)
	}
	data := read(t, "msg.txt")
	// another key that claims the same id
	other, _, _ := ed25519.GenerateKey(nil)
	if _, err := Verify(PublicKey{ID: k.ID, Key: other}, s, bytes.NewReader(data)); err == nil {
		t.Error("verified with another key")
	}
	// the right key under another id
	k2 := k
	k2.ID[0] ^= 1
	if _, err := Verify(k2, s, bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "another key") {
		t.Errorf("wrong key id: %v", err)
	}
	if _, err := Verify(PublicKey{}, s, bytes.NewReader(data)); err == nil {
		t.Error("verified with no key")
	}
}

// sigFile makes a .minisig the way minisign does, with a generated key.
func sigFile(priv ed25519.PrivateKey, id [8]byte, alg string, msg []byte, comment string) []byte {
	sig := ed25519.Sign(priv, msg)
	line := append(append([]byte(alg), id[:]...), sig...)
	global := ed25519.Sign(priv, append(sig, comment...))
	return []byte("untrusted comment: x\n" + base64.StdEncoding.EncodeToString(line) + "\ntrusted comment: " + comment + "\n" + base64.StdEncoding.EncodeToString(global) + "\n")
}

func TestLegacyLimit(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	id := [8]byte{1, 2, 3}
	big := make([]byte, MaxLegacy+1)
	s, err := ParseSignature(sigFile(priv, id, "Ed", big, "c"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(PublicKey{ID: id, Key: pub}, s, bytes.NewReader(big)); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("legacy over the limit: %v", err)
	}
}

func TestMalformed(t *testing.T) {
	good := string(read(t, "msg.txt.minisig"))
	ls := strings.Split(good, "\n")
	keyLine := strings.Split(string(read(t, "test.pub")), "\n")[1]
	for name, txt := range map[string]string{
		"empty":           "",
		"three lines":     strings.Join(ls[:3], "\n"),
		"no untrusted":    strings.Join(append([]string{"comment"}, ls[1:]...), "\n"),
		"no trusted":      strings.Join([]string{ls[0], ls[1], "comment: x", ls[3]}, "\n"),
		"bad base64":      strings.Join([]string{ls[0], "!!!!", ls[2], ls[3]}, "\n"),
		"short sig":       strings.Join([]string{ls[0], ls[1][:40], ls[2], ls[3]}, "\n"),
		"bad global":      strings.Join([]string{ls[0], ls[1], ls[2], ls[3][:20]}, "\n"),
		"public key":      string(read(t, "test.pub")),
		"other algorithm": strings.Join([]string{ls[0], base64.StdEncoding.EncodeToString(append([]byte("XX"), make([]byte, 72)...)), ls[2], ls[3]}, "\n"),
		"extra line":      good + "more\n",
	} {
		if _, err := ParseSignature([]byte(txt)); err == nil {
			t.Errorf("signature %s: accepted", name)
		}
	}
	for name, txt := range map[string]string{
		"empty":         "",
		"bad base64":    "untrusted comment: x\n!!!",
		"short":         keyLine[:20],
		"no comment":    "comment\n" + keyLine,
		"three lines":   "untrusted comment: x\n" + keyLine + "\n" + keyLine,
		"signature":     good,
		"other algo":    base64.StdEncoding.EncodeToString(append([]byte("ED"), make([]byte, 40)...)),
		"a signature's": ls[1],
	} {
		if _, err := ParsePublicKey(txt); err == nil {
			t.Errorf("key %s: accepted", name)
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add(read(f, "msg.txt.minisig"))
	f.Add(read(f, "test.pub"))
	k, _ := ParsePublicKey(ReleaseKey)
	f.Fuzz(func(t *testing.T, b []byte) {
		if s, err := ParseSignature(b); err == nil {
			Verify(k, s, bytes.NewReader(b))
		}
		ParsePublicKey(string(b))
	})
}

// TestReleaseKey stops a release built with the throwaway test key: the
// release workflows run it with RELEASE=1.
func TestReleaseKey(t *testing.T) {
	if os.Getenv("RELEASE") != "1" {
		t.Skip("RELEASE=1 checks the release key")
	}
	k, err := ParsePublicKey(ReleaseKey)
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("sign.go")
	if err != nil {
		t.Fatal(err)
	}
	if k.IDString() == "5D74EC25FBDD46EC" || strings.Contains(string(src), "TODO-"+"KEY") {
		t.Fatal("sign.ReleaseKey is still the test key: put the owner's release key there")
	}
}
