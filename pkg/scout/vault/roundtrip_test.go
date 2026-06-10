package vault

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCaptureSetUseRoundTripReproducesCookie(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()

	var mu sync.Mutex
	var sawCookie string
	mux := http.NewServeMux()
	mux.HandleFunc("/set", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: "rt-token-9", Path: "/"})
		_, _ = w.Write([]byte("<html><body>set</body></html>"))
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sawCookie = r.Header.Get("Cookie")
		mu.Unlock()
		_, _ = w.Write([]byte("<html><body>echo</body></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Authenticated session sets a cookie; capture it.
	p1, err := b.NewPage(srv.URL + "/set")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := p1.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}
	in, err := CaptureFromPage(p1, "rt")
	if err != nil {
		t.Fatalf("CaptureFromPage: %v", err)
	}
	_ = p1.Close()

	// 2. Persist into a temp vault, then Use it.
	vpath := filepath.Join(t.TempDir(), "vault.bin")
	if _, err := Create([]byte("pw"), WithPath(vpath)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	v, err := Open([]byte("pw"), WithPath(vpath))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = v.Close() }()
	id, err := v.Set(in)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	h, err := v.Use(id)
	if err != nil {
		t.Fatalf("Use: %v", err)
	}
	defer func() { _ = h.Close() }()

	// 3. Apply to a fresh page; the server must see the captured cookie.
	p2, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage2: %v", err)
	}
	defer func() { _ = p2.Close() }()
	if err := h.ApplyToPage(p2); err != nil {
		t.Fatalf("ApplyToPage: %v", err)
	}
	if err := p2.Navigate(srv.URL + "/echo"); err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if err := p2.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(sawCookie, "rt-token-9") {
		t.Fatalf("round-trip failed: server saw Cookie=%q, want auth=rt-token-9", sawCookie)
	}
}
