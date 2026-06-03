package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newSwarmSite serves a tiny 2-page site: "/" links to "/page2".
func newSwarmSite(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var base string

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>home</title></head><body>` +
			`<a href="` + base + `/page2">page two</a></body></html>`))
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>two</title></head><body><p>two</p></body></html>`))
	})

	srv := httptest.NewServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)

	return srv
}

func TestSwarmCrawl(t *testing.T) {
	b, _ := newPageTestBrowser(t)
	if b == nil {
		t.Fatal("nil browser")
	}

	srv := newSwarmSite(t)

	out, err := SwarmCrawl(context.Background(), b, SwarmCrawlInput{
		URL:      srv.URL,
		Workers:  2,
		Depth:    2,
		MaxPages: 5,
	})
	if err != nil {
		t.Fatalf("SwarmCrawl: %v", err)
	}

	if out.PagesCrawled < 1 {
		t.Errorf("PagesCrawled = %d, want >= 1", out.PagesCrawled)
	}

	if out.URL != srv.URL {
		t.Errorf("URL = %q, want %q", out.URL, srv.URL)
	}

	if out.Workers != 2 {
		t.Errorf("Workers = %d, want 2", out.Workers)
	}
}

func TestSwarmCrawlErrors(t *testing.T) {
	if _, err := SwarmCrawl(context.Background(), nil, SwarmCrawlInput{URL: "http://example.com"}); err == nil {
		t.Error("nil browser should error")
	}

	b, _ := newPageTestBrowser(t)
	if b == nil {
		t.Fatal("nil browser")
	}

	if _, err := SwarmCrawl(context.Background(), b, SwarmCrawlInput{}); err == nil {
		t.Error("empty url should error")
	}
}
