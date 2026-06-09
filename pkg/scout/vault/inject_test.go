package vault

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestHandleApplyToPageInjectsCookieAndHeader(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()

	var mu sync.Mutex
	var gotCookie, gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("X-Vault-Token")
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := &SecretProfile{
		ID:      "p",
		Cookies: []Cookie{{Name: "sid", Value: "cookie-val-42", Domain: "127.0.0.1", Path: "/"}},
		Headers: map[string]*LockedBuffer{"X-Vault-Token": NewLockedBuffer([]byte("hdr-val-99"))},
	}
	defer p.Close()
	h := &Handle{profile: p}
	defer func() { _ = h.Close() }()

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := h.ApplyToPage(page); err != nil {
		t.Fatalf("ApplyToPage: %v", err)
	}
	if err := page.Navigate(srv.URL + "/echo"); err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(gotAuth, "hdr-val-99") {
		t.Fatalf("header not injected; server saw X-Vault-Token=%q", gotAuth)
	}
	if !strings.Contains(gotCookie, "cookie-val-42") {
		t.Fatalf("cookie not injected; server saw Cookie=%q", gotCookie)
	}
}

func TestHandleCloseZerosSecrets(t *testing.T) {
	lb := NewLockedBuffer([]byte("top-secret"))
	backing := lb.Bytes()
	p := &SecretProfile{ID: "p", Secrets: map[string]*LockedBuffer{"k": lb}}
	h := &Handle{profile: p}

	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for i, b := range backing {
		if b != 0 {
			t.Fatalf("secret byte %d not zeroed after Handle.Close", i)
		}
	}
}

func TestHandleSecret(t *testing.T) {
	p := &SecretProfile{ID: "p", Secrets: map[string]*LockedBuffer{"k": NewLockedBuffer([]byte("v"))}}
	defer p.Close()
	h := &Handle{profile: p}
	lb, err := h.Secret("k")
	if err != nil {
		t.Fatalf("Secret: %v", err)
	}
	if !lb.Equal([]byte("v")) {
		t.Fatal("Secret returned wrong value")
	}
	if _, err := h.Secret("missing"); err == nil {
		t.Fatal("Secret succeeded for missing key")
	}
}

func TestHandleApplyStorageToPageSeedsWebStorage(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	origin := originFrom(srv.URL)
	p := &SecretProfile{
		ID: "p",
		Storage: map[string]OriginStore{
			origin: {
				LocalStorage:   map[string]string{"lk": "seeded-lv"},
				SessionStorage: map[string]string{"sk": "seeded-sv"},
			},
		},
	}
	defer p.Close()
	h := &Handle{profile: p}
	defer func() { _ = h.Close() }()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := h.ApplyStorageToPage(page); err != nil {
		t.Fatalf("ApplyStorageToPage: %v", err)
	}

	ls, err := page.LocalStorageGetAll()
	if err != nil {
		t.Fatalf("LocalStorageGetAll: %v", err)
	}
	if ls["lk"] != "seeded-lv" {
		t.Errorf("localStorage lk = %q, want seeded-lv", ls["lk"])
	}
	ss, err := page.SessionStorageGetAll()
	if err != nil {
		t.Fatalf("SessionStorageGetAll: %v", err)
	}
	if ss["sk"] != "seeded-sv" {
		t.Errorf("sessionStorage sk = %q, want seeded-sv", ss["sk"])
	}
}
