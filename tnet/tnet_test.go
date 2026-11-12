package tnet

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOnTailnet(t *testing.T) {
	for host, want := range map[string]bool{
		"homeserver:4533": true, "nas": true, "box.tail1234.ts.net": true, "100.100.1.2:80": true,
		"127.0.0.1:4533": false, "localhost": false, "192.168.1.10": false, "music.example.com": false,
	} {
		if got := OnTailnet(host); got != want {
			t.Errorf("OnTailnet(%q) = %v", host, got)
		}
	}
}

// With no system CA store (as on the Miyoo), HTTPS to a public site must
// still verify, using the bundled roots. Needs the internet:
// MINIDECK_NET_TEST=1 go test ./tnet
func TestHTTPSWithoutSystemRoots(t *testing.T) {
	if os.Getenv("MINIDECK_NET_TEST") == "" {
		t.Skip("set MINIDECK_NET_TEST=1 to run")
	}
	t.Setenv("MINIDECK_SOCKS", "off")
	resp, err := Client(15 * time.Second).Get("https://github.com/")
	if err != nil {
		t.Fatal(Explain(err))
	}
	resp.Body.Close()
}

func TestExplain(t *testing.T) {
	if Explain(nil) != nil {
		t.Error("Explain(nil) != nil")
	}
	for in, want := range map[string]string{
		"dial tcp 10.0.0.1:80: connect: connection refused":       "Nothing is listening",
		"socks connect tcp 127.0.0.1:1055->nas:80: unknown error": "over the tailnet",
		"lookup nas.example: no such host":                        "wasn't found",
		"context deadline exceeded":                               "didn't answer in time",
		"http: server gave HTTP response to HTTPS client":         "plain HTTP",
		"something else entirely":                                 "something else entirely",
	} {
		err := Explain(&url.Error{Op: "Get", URL: "http://nas/?t=secret", Err: errors.New(in)})
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Explain(%q) = %q, want it to mention %q", in, err, want)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("Explain(%q) kept the URL: %q", in, err)
		}
	}
}

func TestPeers(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) {
		p := filepath.Join(dir, "status.json")
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MINIDECK_PEERS_JSON", p)
	}
	write(`{"Peer":{
		"a":{"HostName":"Laptop","DNSName":"laptop.tail1234.ts.net.","OS":"macOS","Online":false},
		"b":{"HostName":"NAS","DNSName":"","OS":"linux","Online":true}}}`)
	peers, err := Peers()
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 2 || peers[0].Name != "nas" || !peers[0].Online || peers[1].Name != "laptop" {
		t.Errorf("Peers() = %+v, want nas (online) then laptop", peers)
	}
	for _, bad := range []string{``, `{`, `{"Peer":null}`, `{"Peer":{}}`, `[]`} {
		write(bad)
		if _, err := Peers(); err == nil {
			t.Errorf("Peers() with %q: no error", bad)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	for _, c := range []struct{ in, port, want string }{
		{"homeserver", "61208", "http://homeserver:61208"},
		{" homeserver:9000/ ", "61208", "http://homeserver:9000"},
		{"homeserver/sub", "4533", "http://homeserver/sub"},
		// typed with a scheme: the scheme's own port, not the default
		{"https://music.example.com/", "4533", "https://music.example.com"},
		{"http://nas", "4533", "http://nas"},
		{"http://host/sub", "8090", "http://host/sub"},
		{"https://glances.example.com:8443", "61208", "https://glances.example.com:8443"},
		{"", "8090", ""},
	} {
		if got := NormalizeURL(c.in, c.port); got != c.want {
			t.Errorf("NormalizeURL(%q, %q) = %q, want %q", c.in, c.port, got, c.want)
		}
	}
}

func TestReadPorts(t *testing.T) {
	p := filepath.Join(t.TempDir(), "miniscale.conf")
	if s, h := readPorts(p); s != "1055" || h != "1056" {
		t.Errorf("no file: %s %s, want the defaults", s, h)
	}
	for _, c := range []struct{ conf, socks, http string }{
		{"# Local proxies\nSOCKS_PORT=2055\nHTTP_PROXY_PORT=2056\n", "2055", "2056"},
		{"export SOCKS_PORT=\"3055\"  # moved\n  HTTP_PROXY_PORT='3056'\n", "3055", "3056"},
		{"SOCKS_PORT=2055\nSOCKS_PORT=4055\n", "4055", "1056"}, // sh: the last one wins
		{"SOCKS_PORT=abc\nHTTP_PROXY_PORT=70000\n#SOCKS_PORT=9\n", "1055", "1056"},
		{"TS_HOSTNAME=miyoo\n", "1055", "1056"},
	} {
		if err := os.WriteFile(p, []byte(c.conf), 0o644); err != nil {
			t.Fatal(err)
		}
		if s, h := readPorts(p); s != c.socks || h != c.http {
			t.Errorf("%q: ports %s %s, want %s %s", c.conf, s, h, c.socks, c.http)
		}
	}
}

func TestHTTPProxyURL(t *testing.T) {
	t.Setenv("MINIDECK_HTTP_PROXY", "off")
	if u := HTTPProxyURL(); u != nil {
		t.Errorf("off: %v", u)
	}
	t.Setenv("MINIDECK_HTTP_PROXY", "127.0.0.1:9056")
	if u := HTTPProxyURL(); u == nil || u.String() != "http://127.0.0.1:9056" {
		t.Errorf("override: %v", u)
	}
	t.Setenv("MINIDECK_HTTP_PROXY", "")
	if _, err := os.Stat(confFile); err == nil {
		t.Skip(confFile + " exists: the port is the user's")
	}
	if u := HTTPProxyURL(); u == nil || u.String() != "http://127.0.0.1:1056" {
		t.Errorf("default: %v", u)
	}
}

// While MiniScale joins a WireGuard or NetBird network, its routes file
// sends those addresses and names through the proxy too.
func TestRoutesFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "routes")
	routesCache.Lock()
	routesCache.path, routesCache.at = f, time.Time{}
	routesCache.Unlock()
	defer func() {
		routesCache.Lock()
		routesCache.path, routesCache.at = "", time.Time{}
		routesCache.Unlock()
	}()
	reload := func(content string) {
		os.WriteFile(f, []byte(content), 0o644)
		routesCache.Lock()
		routesCache.at = time.Time{}
		routesCache.Unlock()
	}

	// no file: the tailnet rules alone
	if OnTailnet("10.8.0.5") || OnTailnet("nas.example.com") || !OnTailnet("100.101.1.2") || !OnTailnet("nas") {
		t.Fatal("tailnet rules changed without a routes file")
	}

	reload("# written by miniscale.sh\n100.64.0.0/10\n.netbird.cloud\n")
	if !OnTailnet("nas.netbird.cloud:443") || !OnTailnet("NAS.NetBird.Cloud") || OnTailnet("example.com") || OnTailnet("10.8.0.5") {
		t.Error("NetBird routes")
	}

	reload("10.8.0.0/24\nfd00::/64\n*\n")
	if !OnTailnet("10.8.0.5:80") || !OnTailnet("[fd00::5]:80") || OnTailnet("192.168.0.10") || !OnTailnet("example.com") {
		t.Error("WireGuard routes")
	}
	reload("0.0.0.0/0\n::/0\n*\n")
	if !OnTailnet("192.168.0.10") || OnTailnet("localhost:8080") || OnTailnet("127.0.0.1:45211") || OnTailnet("[::1]:80") {
		t.Error("full tunnel: everything but this device's own services")
	}
}
