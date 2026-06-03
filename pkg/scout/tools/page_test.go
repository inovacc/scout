package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

// newPageTestBrowser returns a headless browser + a blank page, skipping when
// Chromium is unavailable (project norm — real browser, no mocks).
func newPageTestBrowser(t *testing.T) (*scout.Browser, *scout.Page) {
	t.Helper()

	b, err := scout.New(scout.WithHeadless(true), scout.WithNoSandbox())
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	t.Cleanup(func() { _ = b.Close() })

	p, err := b.NewPage("")
	if err != nil {
		t.Skipf("page unavailable: %v", err)
	}

	return b, p
}

// newTestServer serves a minimal HTML document with the given body fragment.
func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>T</title></head><body>" + body + "</body></html>"))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestNavigate(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>hi</h1>")

	out, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL})
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	if out.Title != "T" {
		t.Errorf("Title = %q, want T", out.Title)
	}

	if out.URL == "" {
		t.Errorf("URL empty")
	}

	if _, err := Navigate(context.Background(), nil, NavigateInput{URL: srv.URL}); err == nil {
		t.Error("nil page should error")
	}

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: ""}); err == nil {
		t.Error("empty url should error")
	}
}
