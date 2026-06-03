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

func TestBackForward(t *testing.T) {
	b, p := newPageTestBrowser(t)
	if b == nil {
		t.Fatal("nil browser")
	}

	srv1 := newTestServer(t, "<p>one</p>")
	srv2 := newTestServer(t, "<p>two</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv1.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv2.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Back(context.Background(), p, BackInput{}); err != nil {
		t.Errorf("Back: %v", err)
	}

	if _, err := Forward(context.Background(), p, ForwardInput{}); err != nil {
		t.Errorf("Forward: %v", err)
	}

	if _, err := Back(context.Background(), nil, BackInput{}); err == nil {
		t.Error("nil page Back should error")
	}

	if _, err := Forward(context.Background(), nil, ForwardInput{}); err == nil {
		t.Error("nil page Forward should error")
	}
}

func TestReload(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<p>x</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Reload(context.Background(), p, ReloadInput{}); err != nil {
		t.Errorf("Reload: %v", err)
	}

	if _, err := Reload(context.Background(), nil, ReloadInput{}); err == nil {
		t.Error("nil page should error")
	}
}

func TestWaitForElement(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, `<div id="ready">ok</div>`)

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Wait(context.Background(), p, WaitInput{Selector: "#ready"}); err != nil {
		t.Errorf("Wait selector: %v", err)
	}

	if _, err := Wait(context.Background(), p, WaitInput{}); err != nil {
		t.Errorf("Wait load: %v", err)
	}

	if _, err := Wait(context.Background(), nil, WaitInput{}); err == nil {
		t.Error("nil page should error")
	}
}
