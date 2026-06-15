package engine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestInstallRequestFilterBlocksRedirectAndFetch verifies the request filter
// aborts a blocked target reached via an HTTP redirect AND via an in-page
// fetch() — the two SSRF bypasses that a navigate-time-only URL check misses.
func TestInstallRequestFilterBlocksRedirectAndFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	var evilHits int32

	mux := http.NewServeMux()
	mux.HandleFunc("/evil", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&evilHits, 1)
		_, _ = w.Write([]byte("EVIL"))
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/evil", http.StatusFound)
	})
	mux.HandleFunc("/fetch", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>ok<script>fetch('/evil').catch(function(){})</script></body></html>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	b, err := New(WithHeadless(true), WithNoSandbox(), WithTimeout(30e9))
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	// Block any request whose URL targets /evil.
	b.InstallRequestFilter(func(rawURL string) bool {
		return !strings.Contains(rawURL, "/evil")
	})

	// Case 1: 302 redirect to the blocked target.
	if page, perr := b.NewPage(srv.URL + "/redirect"); perr == nil {
		_ = page.WaitLoad()
	}

	// Case 2: in-page fetch() to the blocked target.
	if page, perr := b.NewPage(srv.URL + "/fetch"); perr == nil {
		_ = page.WaitLoad()
		time.Sleep(500 * time.Millisecond) // let the fetch fire and be aborted
	}

	if n := atomic.LoadInt32(&evilHits); n != 0 {
		t.Fatalf("blocked /evil was reached %d time(s); request filter did not abort it", n)
	}
}
