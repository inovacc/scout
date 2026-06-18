package b3pipe

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

func TestBuildFetchJSBearer(t *testing.T) {
	js := buildFetchJS("https://investidor.b3.com.br/api/posicao",
		AuthConfig{Mode: "bearer", TokenStorageKey: "accessToken"})
	if !strings.Contains(js, "localStorage.getItem('accessToken')") {
		t.Errorf("bearer JS missing token read: %s", js)
	}
	if !strings.Contains(js, "Authorization") || !strings.Contains(js, "Bearer ") {
		t.Errorf("bearer JS missing Authorization header: %s", js)
	}
	if !strings.Contains(js, "/api/posicao") {
		t.Errorf("JS missing endpoint: %s", js)
	}
}

func TestBuildFetchJSCookie(t *testing.T) {
	js := buildFetchJS("https://investidor.b3.com.br/api/posicao", AuthConfig{Mode: "cookie"})
	if !strings.Contains(js, "credentials") || !strings.Contains(js, "include") {
		t.Errorf("cookie JS must use credentials:'include': %s", js)
	}
	if strings.Contains(js, "Authorization") {
		t.Errorf("cookie JS should not set Authorization: %s", js)
	}
}

func TestFetchSectionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires browser; skipped under -short")
	}
	// Minimal page that serves a token in localStorage + an API echoing it.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><script>localStorage.setItem('accessToken','T123')</script></body></html>`))
	})
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer T123" {
			w.WriteHeader(401)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"x":1}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		t.Skipf("no browser: %v", err)
	}
	defer b.Close()
	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = page.WaitLoad()
	res, err := FetchSection(page, Section{ID: "data", Endpoint: srv.URL + "/api/data"},
		AuthConfig{Mode: "bearer", TokenStorageKey: "accessToken"})
	if err != nil {
		t.Fatalf("FetchSection: %v", err)
	}
	if res.Status != 200 || string(res.Body) != `{"data":[{"x":1}]}` {
		t.Fatalf("res = %d %q", res.Status, res.Body)
	}
}
