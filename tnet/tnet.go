// Package tnet gives apps an HTTP client that reaches tailnet hosts through
// MiniDeck's SOCKS5 proxy and everything else directly.
//
// A host goes through the tailnet when it is a 100.64.0.0/10 address, ends in
// .ts.net, or is a single-label name (MagicDNS). The proxies listen on the
// ports set in MiniScale's miniscale.conf (1055 and 1056 by default).
// MINIDECK_SOCKS and MINIDECK_HTTP_PROXY override the addresses; "off"
// disables a proxy.
package tnet

import (
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// confFile is MiniScale's settings file, where the user can move the
// proxies to other ports.
const confFile = "/mnt/SDCARD/App/MiniScale/miniscale.conf"

// confPorts reads the proxy ports once: miniscale.sh applies a change to
// them only when MiniScale starts again, and an app is reopened far more
// often than that.
var confPorts = sync.OnceValues(func() (socks, httpProxy string) { return readPorts(confFile) })

// readPorts reads SOCKS_PORT and HTTP_PROXY_PORT from a miniscale.conf,
// falling back to miniscale.sh's defaults for a missing file, a missing
// setting or a value that isn't a port.
func readPorts(path string) (socks, httpProxy string) {
	socks, httpProxy = "1055", "1056"
	b, err := os.ReadFile(path)
	if err != nil {
		return socks, httpProxy
	}
	// The file is sourced by sh, so the last assignment wins.
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := shellAssign(line)
		if !ok || !isPort(v) {
			continue
		}
		switch k {
		case "SOCKS_PORT":
			socks = v
		case "HTTP_PROXY_PORT":
			httpProxy = v
		}
	}
	return socks, httpProxy
}

// shellAssign splits a simple shell assignment (KEY=value, KEY="value",
// export KEY=value, with an optional trailing comment). Anything fancier
// is not understood, and reported as no assignment.
func shellAssign(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	key, value, ok = strings.Cut(line, "=")
	if !ok || key == "" || strings.ContainsAny(key, " \t#") {
		return "", "", false
	}
	if i := strings.Index(value, " #"); i >= 0 {
		value = value[:i]
	}
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

func isPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n > 0 && n < 65536
}

func socksAddr() string {
	if v := os.Getenv("MINIDECK_SOCKS"); v != "" {
		return v
	}
	p, _ := confPorts()
	return "127.0.0.1:" + p
}

// HTTPProxyURL is MiniScale's HTTP proxy to the tailnet, for clients that
// use it instead of SOCKS, or nil when MINIDECK_HTTP_PROXY is "off".
func HTTPProxyURL() *url.URL {
	addr := os.Getenv("MINIDECK_HTTP_PROXY")
	if addr == "off" {
		return nil
	}
	if addr == "" {
		_, p := confPorts()
		addr = "127.0.0.1:" + p
	}
	return &url.URL{Scheme: "http", Host: addr}
}

// OnTailnet reports whether host should be reached through the tailnet.
func OnTailnet(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return cgnat.Contains(ip)
	}
	return strings.HasSuffix(host, ".ts.net") || !strings.Contains(host, ".")
}

// ProxyFor is an http.Transport Proxy function.
func ProxyFor(r *http.Request) (*url.URL, error) {
	addr := socksAddr()
	if addr == "off" || !OnTailnet(r.URL.Host) {
		return nil, nil
	}
	return &url.URL{Scheme: "socks5", Host: addr}, nil
}

// Client returns an HTTP client with the given timeout (0 = none, for
// streams).
//
// Through Tailscale's userspace networking a kept-alive connection can go
// quietly dead (the server or tailscaled dropped it), and a request sent on
// it hangs until the timeout: the occasional "context deadline exceeded".
// So idle connections are dropped sooner and the timeout applies to each
// try until the response headers arrive. A request that failed before it
// had a connection (the dial or the proxy failed) is tried once more; one
// that may have reached the server is not, since the server may have acted
// on it already (a scrobble would count twice).
func Client(timeout time.Duration) *http.Client {
	t := &http.Transport{
		Proxy:                 ProxyFor,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 15 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       20 * time.Second,
	}
	c := &http.Client{Transport: &retry{t}}
	if timeout > 0 {
		c.Timeout = 2*timeout + 10*time.Second // both tries, and reading the body
	}
	return c
}

// retry is the transport Client uses: it sends a request once more when the
// first try failed before any of it could reach the server (see Client).
type retry struct{ t http.RoundTripper }

// RoundTrip implements http.RoundTripper.
func (r *retry) RoundTrip(req *http.Request) (*http.Response, error) {
	// The transport hands over a connection (GotConn) only once the dial,
	// the proxy handshake and TLS have all succeeded. Without one, not a
	// byte of the request was written.
	var gotConn atomic.Bool
	trace := &httptrace.ClientTrace{GotConn: func(httptrace.GotConnInfo) { gotConn.Store(true) }}
	resp, err := r.t.RoundTrip(req.WithContext(httptrace.WithClientTrace(req.Context(), trace)))
	if err == nil || gotConn.Load() || req.Context().Err() != nil {
		return resp, err
	}
	// The transport closed the body: only one that can be re-read (Beszel's
	// login) can be sent again.
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody == nil {
			return resp, err
		}
		b, gerr := req.GetBody()
		if gerr != nil {
			return resp, err
		}
		req = req.Clone(req.Context())
		req.Body = b
	}
	return r.t.RoundTrip(req)
}
