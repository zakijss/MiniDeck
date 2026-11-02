// Package secrets reads and writes small JSON secret files sealed with the
// device key: OpenSSL "enc -aes-256-cbc -pbkdf2 -iter 4096" format (the same
// as `miniscale.sh seal`), keyed by the first line of
// /appconfigs/minideck.key, which lives in the device's internal flash.
//
// The device key is 256 random bits, so stretching it adds nothing: Load
// and Save use 4096 PBKDF2 rounds, about 0.1 s on the device, where 200000
// would take 4.5 s every time an app opens. Encrypt and Decrypt use 200000
// for passwords people type (MiniKey's code), where the stretching matters.
// Load also opens files sealed with 200000 rounds and re-seals them with
// 4096.
package secrets

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zakijss/minideck/atomicfile"
)

const (
	iter     = 200000 // Encrypt/Decrypt: for passwords people choose
	fastIter = 4096   // Load/Save: for the random device key
)

// DeviceKeyPath is where the device key lives; MINIDECK_DEVKEY overrides it
// for tests.
func DeviceKeyPath() string {
	if p := os.Getenv("MINIDECK_DEVKEY"); p != "" {
		return p
	}
	return "/appconfigs/minideck.key"
}

func deviceKey() ([]byte, error) {
	f, err := os.Open(DeviceKeyPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return nil, errors.New("device key is empty")
	}
	return []byte(sc.Text()), nil
}

// DeviceKey returns the device key, for apps that combine it with a code
// the user enters (MiniKey).
func DeviceKey() ([]byte, error) { return deviceKey() }

// Decrypt opens an OpenSSL salted AES-256-CBC/PBKDF2 blob with pass.
func Decrypt(blob, pass []byte) ([]byte, error) { return decrypt(blob, pass, iter) }

func decrypt(blob, pass []byte, iter int) ([]byte, error) {
	if len(blob) < 32 || string(blob[:8]) != "Salted__" || (len(blob)-16)%aes.BlockSize != 0 {
		return nil, errors.New("not an OpenSSL salted file")
	}
	salt, ct := blob[8:16], blob[16:]
	km, err := pbkdf2.Key(sha256.New, string(pass), salt, iter, 48)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(km[:32])
	if err != nil {
		return nil, err
	}
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, km[32:48]).CryptBlocks(pt, ct)
	pad := int(pt[len(pt)-1])
	if pad < 1 || pad > aes.BlockSize || !bytes.Equal(pt[len(pt)-pad:], bytes.Repeat([]byte{byte(pad)}, pad)) {
		return nil, errors.New("wrong key or damaged file")
	}
	return pt[:len(pt)-pad], nil
}

// LoadOrSetAside is Load for an app's screen: when the file can't be opened
// (the device key changed, the file is damaged), the app asks for its
// logins again, and the old file is renamed first so the next Save doesn't
// overwrite it: to path+".unreadable", or, when an earlier copy already
// has that name, to path+".unreadable.2", ".3" and so on. An earlier copy is
// never replaced, since it may be the one a key restored from a backup
// opens.
func LoadOrSetAside(path string) (map[string]string, error) {
	m, err := Load(path)
	if err == nil {
		return m, nil
	}
	if aside, ok := freeName(path + ".unreadable"); ok {
		if rerr := os.Rename(path, aside); rerr == nil {
			err = fmt.Errorf("%w (kept as %s)", err, filepath.Base(aside))
		}
	}
	return map[string]string{}, err
}

// freeName returns base, or base with ".2", ".3"... added, whichever names
// no file yet. It gives up after a thousand tries, so a card that can't be
// read doesn't keep it looping.
func freeName(base string) (string, bool) {
	for i := 1; i <= 1000; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s.%d", base, i)
		}
		if _, err := os.Lstat(name); errors.Is(err, os.ErrNotExist) {
			return name, true
		}
	}
	return "", false
}

// Load reads a sealed JSON object of strings. A missing file gives an empty
// map, so apps work without secrets.
func Load(path string) (map[string]string, error) {
	out := map[string]string{}
	blob, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := deviceKey()
	if err != nil {
		return nil, fmt.Errorf("device key: %w", err)
	}
	// A wrong round count gives garbage, which fails the padding or JSON.
	if pt, err := decrypt(blob, key, fastIter); err == nil && json.Unmarshal(pt, &out) == nil {
		return out, nil
	}
	out = map[string]string{}
	pt, err := decrypt(blob, key, iter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := json.Unmarshal(pt, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	Save(path, out) // re-seal in the fast format; best effort
	return out, nil
}

// EnsureDeviceKey creates the device key if it doesn't exist yet (the
// MiniScale app normally creates it at first login).
func EnsureDeviceKey() error {
	p := DeviceKeyPath()
	if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
		return nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	return atomicfile.Write(p, []byte(hex.EncodeToString(b)+"\n"), 0600)
}

// Encrypt seals plaintext with pass in OpenSSL's salted format, so
// `openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000` can open it too.
func Encrypt(plain, pass []byte) ([]byte, error) { return encrypt(plain, pass, iter) }

func encrypt(plain, pass []byte, iter int) ([]byte, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	km, err := pbkdf2.Key(sha256.New, string(pass), salt, iter, 48)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(km[:32])
	if err != nil {
		return nil, err
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	pt := append(append([]byte(nil), plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	ct := make([]byte, len(pt))
	cipher.NewCBCEncrypter(block, km[32:48]).CryptBlocks(ct, pt)
	return append(append([]byte("Salted__"), salt...), ct...), nil
}

// Save writes m to path, sealed with the device key (created if missing).
func Save(path string, m map[string]string) error {
	if err := EnsureDeviceKey(); err != nil {
		return fmt.Errorf("device key: %w", err)
	}
	key, err := deviceKey()
	if err != nil {
		return err
	}
	pt, err := json.Marshal(m)
	if err != nil {
		return err
	}
	blob, err := encrypt(pt, key, fastIter)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, blob, 0600)
}
