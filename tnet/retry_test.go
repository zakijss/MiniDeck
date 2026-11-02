package tnet

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A request that timed out waiting for the answer may have reached the
// server, which may have acted on it: it must not be sent again.
func TestNoRetryAfterTimeout(t *testing.T) {
	t.Setenv("MINIDECK_SOCKS", "off")
	var hits atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
	}))
	defer srv.Close()
	defer close(release)

	c := Client(100 * time.Millisecond)
	if _, err := c.Get(srv.URL + "/rest/scrobble?submission=true"); err == nil {
		t.Fatal("no error from a server that never answers")
	}
	if _, err := c.Post(srv.URL+"/login", "application/json", strings.NewReader(`{}`)); err == nil {
		t.Fatal("no error from a server that never answers")
	}
	time.Sleep(50 * time.Millisecond) // a second try would have arrived by now
	if n := hits.Load(); n != 2 {
		t.Errorf("the server got %d requests, want 2 (one each, no retries)", n)
	}
}

// A dial that fails sends nothing, so the request is tried again, body and
// all when the body can be read again.
func TestRetryAfterDialFailure(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		b, _ := io.ReadAll(r.Body)
		w.Write(b)
	}))
	defer srv.Close()

	var dials atomic.Int32
	d := &net.Dialer{}
	rt := &retry{&http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if dials.Add(1)%2 == 1 { // every first try fails
			return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("connection refused")}
		}
		return d.DialContext(ctx, network, addr)
	}}}
	c := &http.Client{Transport: rt}

	resp, err := c.Post(srv.URL, "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "hello" || hits.Load() != 1 || dials.Load() != 2 {
		t.Errorf("reply %q after %d requests and %d dials, want \"hello\", 1 and 2", b, hits.Load(), dials.Load())
	}

	// A body that can't be read again can't be sent again.
	req, _ := http.NewRequest("POST", srv.URL, io.NopCloser(strings.NewReader("x")))
	if _, err := c.Do(req); err == nil {
		t.Error("a body without GetBody was sent again")
	}
	if dials.Load() != 3 {
		t.Errorf("%d dials, want 3", dials.Load())
	}
}
