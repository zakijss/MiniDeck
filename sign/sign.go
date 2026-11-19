// Package sign checks minisign signatures (https://jedisct1.github.io/minisign/).
//
// A release asset X is signed with `minisign -S -s <key> -m X`, which writes
// X.minisig beside it. Verify checks X against X.minisig and a public key:
// the signature over the file, that the key ids match, and the global
// signature over the trusted comment, which it returns.
//
// Both of minisign's algorithms are read: "ED" (the default since minisign
// 0.10) signs the file's BLAKE2b-512 hash, so the file is streamed and can
// be any size; "Ed" (legacy, -l) signs the file itself, which Ed25519 can't
// stream, so it is limited to MaxLegacy bytes.
package sign

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/blake2b"
)

// ReleaseKey is the public key MiniDeck's owner signs releases with: the
// second line of the key's minisign.pub.
const ReleaseKey = "RWTsRt37Jex0XWULZZMDBYR3DmYEdNjLWU/wh2NCJ4+Q1WwRu8TlUnfS" // TODO-KEY: the throwaway test key; put the owner's release key here before a release

// MaxLegacy is the largest file a legacy ("Ed") signature is checked
// over: the whole file is read into memory.
const MaxLegacy = 16 << 20

// PublicKey is a minisign public key.
type PublicKey struct {
	ID  [8]byte
	Key ed25519.PublicKey
}

// IDString is the key id as minisign prints it.
func (k PublicKey) IDString() string {
	return fmt.Sprintf("%016X", binary.LittleEndian.Uint64(k.ID[:]))
}

// Signature is a parsed .minisig file.
type Signature struct {
	Algorithm string // "ED" (prehashed) or "Ed" (legacy)
	KeyID     [8]byte
	Sig       [ed25519.SignatureSize]byte
	// Trusted is the trusted comment. It isn't trusted until Verify says so.
	Trusted   string
	GlobalSig [ed25519.SignatureSize]byte
}

const (
	untrusted = "untrusted comment:"
	trusted   = "trusted comment: "
)

// lines splits text into its lines, without line endings, dropping a
// trailing empty one.
func lines(text string) []string {
	ls := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(ls) > 0 && ls[len(ls)-1] == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

func decode(s string, n int) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != n {
		return nil, errors.New("bad base64 data")
	}
	return b, nil
}

// ParsePublicKey reads a public key: a minisign.pub file (an untrusted
// comment line, then the key) or the key's base64 line alone.
func ParsePublicKey(text string) (PublicKey, error) {
	ls := lines(strings.TrimSpace(text))
	switch {
	case len(ls) == 2 && strings.HasPrefix(ls[0], untrusted):
		ls = ls[1:]
	case len(ls) != 1:
		return PublicKey{}, errors.New("not a minisign public key")
	}
	b, err := decode(ls[0], 2+8+ed25519.PublicKeySize)
	if err != nil {
		return PublicKey{}, fmt.Errorf("public key: %w", err)
	}
	if string(b[:2]) != "Ed" {
		return PublicKey{}, errors.New("public key: unknown algorithm")
	}
	var k PublicKey
	copy(k.ID[:], b[2:10])
	k.Key = ed25519.PublicKey(b[10:])
	return k, nil
}

// ParseSignature reads a .minisig file.
func ParseSignature(text []byte) (Signature, error) {
	var s Signature
	ls := lines(string(text))
	if len(ls) != 4 || !strings.HasPrefix(ls[0], untrusted) || !strings.HasPrefix(ls[2], trusted) {
		return s, errors.New("not a minisign signature")
	}
	b, err := decode(ls[1], 2+8+ed25519.SignatureSize)
	if err != nil {
		return s, fmt.Errorf("signature: %w", err)
	}
	s.Algorithm = string(b[:2])
	if s.Algorithm != "ED" && s.Algorithm != "Ed" {
		return s, errors.New("signature: unknown algorithm")
	}
	copy(s.KeyID[:], b[2:10])
	copy(s.Sig[:], b[10:])
	s.Trusted = strings.TrimPrefix(ls[2], trusted)
	g, err := decode(ls[3], ed25519.SignatureSize)
	if err != nil {
		return s, fmt.Errorf("global signature: %w", err)
	}
	copy(s.GlobalSig[:], g)
	return s, nil
}

// Verify checks data against the signature and the key, reading data
// once, and returns the trusted comment. Any error means the data or the
// comment can't be trusted.
func Verify(pub PublicKey, sig Signature, data io.Reader) (string, error) {
	if len(pub.Key) != ed25519.PublicKeySize {
		return "", errors.New("no public key")
	}
	if sig.KeyID != pub.ID {
		return "", fmt.Errorf("signed with another key (id %016X, not %s)", binary.LittleEndian.Uint64(sig.KeyID[:]), pub.IDString())
	}
	var msg []byte
	switch sig.Algorithm {
	case "ED":
		h, _ := blake2b.New512(nil)
		if _, err := io.Copy(h, data); err != nil {
			return "", err
		}
		msg = h.Sum(nil)
	case "Ed":
		var buf bytes.Buffer
		n, err := io.Copy(&buf, io.LimitReader(data, MaxLegacy+1))
		if err != nil {
			return "", err
		}
		if n > MaxLegacy {
			return "", fmt.Errorf("legacy signature over a file larger than %d MB: sign it without -l", MaxLegacy>>20)
		}
		msg = buf.Bytes()
	default:
		return "", errors.New("unknown signature algorithm")
	}
	if !ed25519.Verify(pub.Key, msg, sig.Sig[:]) {
		return "", errors.New("the signature doesn't match the file")
	}
	if !ed25519.Verify(pub.Key, append(sig.Sig[:], sig.Trusted...), sig.GlobalSig[:]) {
		return "", errors.New("the trusted comment doesn't match its signature")
	}
	return sig.Trusted, nil
}
