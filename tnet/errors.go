package tnet

import (
	"crypto/x509"
	"errors"
	"net/url"
	"strings"

	// The Miyoo has no CA certificate store: use Go's copy of Mozilla's roots
	// so HTTPS servers with public certificates (Let's Encrypt, tailscale
	// serve's *.ts.net) verify. Only used when the system has no roots.
	_ "golang.org/x/crypto/x509roots/fallback"
)

// sentence is an error written for the screen: a full sentence, capital
// letter and full stop included, unlike errors meant to be wrapped.
type sentence string

func (s sentence) Error() string { return string(s) }

// Explain turns a network error into a short sentence for the screen,
// without the request URL (Subsonic URLs carry an auth token).
func Explain(err error) error {
	if err == nil {
		return nil
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var ua x509.UnknownAuthorityError
	var hn x509.HostnameError
	var ci x509.CertificateInvalidError
	s := err.Error()
	switch {
	case errors.As(err, &ua):
		return sentence("The server's HTTPS certificate isn't from a known authority (self-signed?). On your tailnet, use http:// instead: Tailscale already encrypts the connection.")
	case errors.As(err, &hn):
		return sentence("The server's HTTPS certificate is for a different name. Use the name on the certificate, or http:// on your tailnet.")
	case errors.As(err, &ci):
		return sentence("The server's HTTPS certificate isn't valid (expired?). Check the server, or use http:// on your tailnet.")
	case strings.Contains(s, "socks connect"):
		return sentence("Can't reach it over the tailnet. Check that MiniScale is on and that your tailnet policy lets this device reach that server and port.")
	case strings.Contains(s, "Client.Timeout"), strings.Contains(s, "deadline exceeded"), strings.Contains(s, "i/o timeout"):
		return sentence("The server didn't answer in time. Check the address and port.")
	case strings.Contains(s, "connection refused"):
		return sentence("Nothing is listening at that address and port.")
	case strings.Contains(s, "no such host"):
		return sentence("That server name wasn't found. Check the spelling, or pick it from the list.")
	case strings.Contains(s, "server gave HTTP response to HTTPS client"):
		return sentence("That server speaks plain HTTP: use http:// instead of https://.")
	}
	return err
}
