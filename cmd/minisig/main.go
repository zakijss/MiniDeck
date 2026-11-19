// Command minisig checks a file against its minisign signature, for the
// device and for shell scripts:
//
//	minisig [-p PUBKEY_FILE] -m FILE -x SIGFILE
//
// It prints the trusted comment and exits 0 when the signature is good,
// prints one line saying why on stderr and exits 1 when it isn't (or a file
// can't be read), and exits 2 on a usage error. Without -p it uses the
// release key built into it (sign.ReleaseKey). -x defaults to FILE.minisig.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/zakijss/minideck/sign"
)

func main() {
	pubFile := flag.String("p", "", "the public key file (default: the release key)")
	file := flag.String("m", "", "the file to check")
	sigFile := flag.String("x", "", "its signature (default: FILE.minisig)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: minisig [-p PUBKEY_FILE] -m FILE [-x SIGFILE]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *file == "" || flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *sigFile == "" {
		*sigFile = *file + ".minisig"
	}
	comment, err := check(*pubFile, *file, *sigFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "minisig:", err)
		os.Exit(1)
	}
	fmt.Println(comment)
}

func check(pubFile, file, sigFile string) (string, error) {
	keyText := sign.ReleaseKey
	if pubFile != "" {
		b, err := readSmall(pubFile)
		if err != nil {
			return "", err
		}
		keyText = string(b)
	}
	pub, err := sign.ParsePublicKey(keyText)
	if err != nil {
		return "", err
	}
	b, err := readSmall(sigFile)
	if err != nil {
		return "", err
	}
	sig, err := sign.ParseSignature(b)
	if err != nil {
		return "", err
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return sign.Verify(pub, sig, f)
}

// readSmall reads a key or signature file, which is a few hundred bytes.
func readSmall(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 64<<10))
	return b, err
}
