package scout

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(crawlTestRoutes)
}

func crawlTestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/crawl-start", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Crawl Start</title></head>
<body>
<h1>Start Page</h1>
<a href="/crawl-page1">Page 1</a>
<a href="/crawl-page2">Page 2</a>
<a href="https://external.example.com/nope">External</a>
</body></html>`)
	})

	mux.HandleFunc("/crawl-page1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Crawl Page 1</title></head>
<body>
<h1>Page 1</h1>
<a href="/crawl-page3">Page 3</a>
<a href="/crawl-start">Back to Start</a>
</body></html>`)
	})

	mux.HandleFunc("/crawl-page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Crawl Page 2</title></head>
<body>
<h1>Page 2</h1>
<a href="/crawl-start">Back to Start</a>
</body></html>`)
	})

	mux.HandleFunc("/crawl-page3", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Crawl Page 3</title></head>
<body>
<h1>Page 3</h1>
<a href="/crawl-start">Back to Start</a>
</body></html>`)
	})

	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")

		sitemap := sitemapURLSet{
			URLs: []SitemapURL{
				{Loc: "https://example.com/page1", LastMod: "2024-01-01", ChangeFreq: "daily", Priority: "1.0"},
				{Loc: "https://example.com/page2", LastMod: "2024-01-02", ChangeFreq: "weekly", Priority: "0.8"},
				{Loc: "https://example.com/page3", LastMod: "2024-01-03"},
			},
		}
		data, _ := xml.MarshalIndent(struct {
			XMLName xml.Name     `xml:"urlset"`
			URLs    []SitemapURL `xml:"url"`
		}{URLs: sitemap.URLs}, "", "  ")
		_, _ = w.Write([]byte(xml.Header))
		_, _ = w.Write(data)
	})
}

func TestCrawl(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	var titles []string

	results, err := b.Crawl(srv.URL+"/crawl-start", func(_ *Page, result *CrawlResult) error {
		titles = append(titles, result.Title)
		return nil
	},
		WithCrawlMaxDepth(2),
		WithCrawlMaxPages(10),
		WithCrawlDelay(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}

	if len(results) < 3 {
		t.Errorf("Crawl() visited %d pages, want >= 3", len(results))
	}

	// Should have visited start, page1, page2, and possibly page3
	foundStart := false
	foundPage1 := false
	foundPage2 := false

	for _, r := range results {
		t.Logf("Crawled: %s (depth=%d, title=%q, links=%d)", r.URL, r.Depth, r.Title, len(r.Links))

		if r.Title == "Crawl Start" {
			foundStart = true
		}

		if r.Title == "Crawl Page 1" {
			foundPage1 = true
		}

		if r.Title == "Crawl Page 2" {
			foundPage2 = true
		}
	}

	if !foundStart {
		t.Error("should have crawled start page")
	}

	if !foundPage1 {
		t.Error("should have crawled page 1")
	}

	if !foundPage2 {
		t.Error("should have crawled page 2")
	}
}

func TestCrawlMaxPages(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	results, err := b.Crawl(srv.URL+"/crawl-start", nil,
		WithCrawlMaxDepth(10),
		WithCrawlMaxPages(2),
		WithCrawlDelay(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}

	if len(results) > 2 {
		t.Errorf("Crawl() visited %d pages, want <= 2", len(results))
	}
}

func TestCrawlHandlerStop(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	stopErr := fmt.Errorf("stop crawling")
	results, err := b.Crawl(srv.URL+"/crawl-start", func(_ *Page, _ *CrawlResult) error {
		return stopErr
	},
		WithCrawlMaxDepth(3),
		WithCrawlDelay(50*time.Millisecond),
	)

	if err != stopErr {
		t.Errorf("Crawl() error = %v, want stopErr", err)
	}

	if len(results) != 1 {
		t.Errorf("Crawl() visited %d pages, want 1 (stopped by handler)", len(results))
	}
}

func TestParseSitemap(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	urls, err := b.ParseSitemap(srv.URL + "/sitemap.xml")
	if err != nil {
		t.Fatalf("ParseSitemap() error: %v", err)
	}

	if len(urls) != 3 {
		t.Fatalf("ParseSitemap() returned %d URLs, want 3", len(urls))
	}

	if urls[0].Loc != "https://example.com/page1" {
		t.Errorf("urls[0].Loc = %q", urls[0].Loc)
	}

	if urls[0].LastMod != "2024-01-01" {
		t.Errorf("urls[0].LastMod = %q", urls[0].LastMod)
	}

	if urls[0].ChangeFreq != "daily" {
		t.Errorf("urls[0].ChangeFreq = %q", urls[0].ChangeFreq)
	}

	if urls[0].Priority != "1.0" {
		t.Errorf("urls[0].Priority = %q", urls[0].Priority)
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/page#section", "https://example.com/page"},
		{"https://example.com/page/", "https://example.com/page"},
		{"https://example.com/", "https://example.com/"},
		{"https://example.com/a/b/c/", "https://example.com/a/b/c"},
	}
	for _, tt := range tests {
		got := normalizeURL(tt.input)
		if got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsDomainAllowed(t *testing.T) {
	allowed := []string{"example.com", "sub.test.org"}

	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com/page", true},
		{"https://sub.example.com/page", true},
		{"https://other.com/page", false},
		{"https://sub.test.org/page", true},
	}
	for _, tt := range tests {
		got := isDomainAllowed(tt.url, allowed)
		if got != tt.want {
			t.Errorf("isDomainAllowed(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestResolveLink(t *testing.T) {
	base := "https://example.com/dir/page"

	tests := []struct {
		href string
		want string
	}{
		{"/other", "https://example.com/other"},
		{"sibling", "https://example.com/dir/sibling"},
		{"https://absolute.com/path", "https://absolute.com/path"},
		{"#fragment", ""},
		{"javascript:void(0)", ""},
		{"mailto:test@test.com", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := resolveLink(base, tt.href)
		if got != tt.want {
			t.Errorf("resolveLink(%q, %q) = %q, want %q", base, tt.href, got, tt.want)
		}
	}
}

func TestCrawlOptions(t *testing.T) {
	o := crawlDefaults()

	if o.maxDepth != 3 {
		t.Errorf("default maxDepth = %d, want 3", o.maxDepth)
	}

	if o.maxPages != 100 {
		t.Errorf("default maxPages = %d, want 100", o.maxPages)
	}

	if o.delay != 500*time.Millisecond {
		t.Errorf("default delay = %v", o.delay)
	}

	if o.concurrent != 1 {
		t.Errorf("default concurrent = %d, want 1", o.concurrent)
	}

	WithCrawlMaxDepth(5)(o)

	if o.maxDepth != 5 {
		t.Errorf("maxDepth = %d, want 5", o.maxDepth)
	}

	WithCrawlMaxPages(50)(o)

	if o.maxPages != 50 {
		t.Errorf("maxPages = %d, want 50", o.maxPages)
	}

	WithCrawlAllowedDomains("a.com", "b.com")(o)

	if len(o.allowedDomains) != 2 {
		t.Errorf("allowedDomains = %v", o.allowedDomains)
	}

	WithCrawlDelay(1 * time.Second)(o)

	if o.delay != 1*time.Second {
		t.Errorf("delay = %v", o.delay)
	}

	WithCrawlConcurrent(4)(o)

	if o.concurrent != 4 {
		t.Errorf("concurrent = %d, want 4", o.concurrent)
	}

	WithCrawlConcurrent(0)(o) // should clamp to 1

	if o.concurrent != 1 {
		t.Errorf("concurrent = %d, want 1 (clamped)", o.concurrent)
	}
}
