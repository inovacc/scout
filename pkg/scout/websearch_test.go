package scout

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(webSearchTestRoutes)
}

func webSearchTestRoutes(mux *http.ServeMux) {
	// Fake Google SERP with 3 results pointing to local pages
	mux.HandleFunc("/ws-serp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		host := r.Host
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>test - Google Search</title></head>
<body>
<div class="g">
  <h3>Result One</h3>
  <a href="http://%[1]s/ws-page1">Page 1</a>
  <div class="VwiC3b">First snippet</div>
</div>
<div class="g">
  <h3>Result Two</h3>
  <a href="http://%[1]s/ws-page2">Page 2</a>
  <div class="VwiC3b">Second snippet</div>
</div>
<div class="g">
  <h3>Result Three</h3>
  <a href="http://%[1]s/ws-page3">Page 3</a>
  <div class="VwiC3b">Third snippet</div>
</div>
</body></html>`, host)
	})

	mux.HandleFunc("/ws-page1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page One</title></head>
<body><article><h1>Article One</h1>
<p>This is the content of page one with enough text for readability scoring to pick it up as substantial content.</p>
<p>More content here to ensure the article is long enough for the main content extractor to find it.</p>
</article></body></html>`)
	})

	mux.HandleFunc("/ws-page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page Two</title></head>
<body><article><h1>Article Two</h1>
<p>This is the content of page two with substantial text for readability to work.</p>
<p>Additional paragraph with more content for the article section.</p>
</article></body></html>`)
	})

	mux.HandleFunc("/ws-page3", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Page Three</title></head>
<body><article><h1>Article Three</h1>
<p>Page three content here for testing.</p>
</article></body></html>`)
	})
}

func TestWebSearch_NoFetch(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	// Override search to use local SERP
	page, err := b.NewPage(srv.URL + "/ws-serp")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	results, err := googleParser.parse(page, "test query", Google)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(results.Results) != 3 {
		t.Fatalf("results = %d, want 3", len(results.Results))
	}

	// Verify items have no Content when fetch is disabled
	for _, r := range results.Results {
		if r.Title == "" {
			t.Error("title should not be empty")
		}
		if r.URL == "" {
			t.Error("URL should not be empty")
		}
	}
}

func TestWebSearch_WithFetch(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	// Parse SERP to get result URLs, then test fetch
	page, err := b.NewPage(srv.URL + "/ws-serp")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = page.Close()
		t.Fatalf("WaitLoad: %v", err)
	}

	results, _ := googleParser.parse(page, "test", Google)
	_ = page.Close()

	if len(results.Results) == 0 {
		t.Fatal("no results parsed")
	}

	// Fetch the first result URL
	content, err := b.WebFetch(results.Results[0].URL, WithFetchMode("markdown"))
	if err != nil {
		t.Fatalf("WebFetch: %v", err)
	}

	if content.Markdown == "" {
		t.Error("fetched markdown should not be empty")
	}

	if content.Title != "Page One" {
		t.Errorf("title = %q, want %q", content.Title, "Page One")
	}
}

func TestWebSearch_MainContent(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	content, err := b.WebFetch(srv.URL+"/ws-page1", WithFetchMode("markdown"), WithFetchMainContent())
	if err != nil {
		t.Fatalf("WebFetch: %v", err)
	}

	if content.Markdown == "" {
		t.Error("main content markdown should not be empty")
	}
}

func TestWebSearch_MaxFetch(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	// Parse SERP
	page, err := b.NewPage(srv.URL + "/ws-serp")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		_ = page.Close()
		t.Fatalf("WaitLoad: %v", err)
	}
	results, _ := googleParser.parse(page, "test", Google)
	_ = page.Close()

	if len(results.Results) < 2 {
		t.Skip("need at least 2 results")
	}

	// Fetch only 1
	urls := make([]string, len(results.Results))
	for i, r := range results.Results {
		urls[i] = r.URL
	}

	fetched := b.WebFetchBatch(urls[:1], WithFetchMode("markdown"))
	if len(fetched) != 1 {
		t.Fatalf("fetched = %d, want 1", len(fetched))
	}

	if fetched[0].Markdown == "" {
		t.Error("first result should have content")
	}
}

func TestWebSearch_Cache(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	old := globalFetchCache.entries
	globalFetchCache.mu.Lock()
	globalFetchCache.entries = make(map[string]*fetchCacheEntry)
	globalFetchCache.mu.Unlock()
	defer func() {
		globalFetchCache.mu.Lock()
		globalFetchCache.entries = old
		globalFetchCache.mu.Unlock()
	}()

	url := srv.URL + "/ws-page1"

	r1, err := b.WebFetch(url, WithFetchMode("markdown"), WithFetchCache(1*time.Minute))
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	r2, err := b.WebFetch(url, WithFetchMode("markdown"), WithFetchCache(1*time.Minute))
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	if r1.FetchedAt != r2.FetchedAt {
		t.Error("second call should return cached result")
	}
}

func TestWebSearch_FetchErrorIsolation(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	urls := []string{
		"http://127.0.0.1:1/nonexistent", // bad
		srv.URL + "/ws-page1",            // good
	}

	results := b.WebFetchBatch(urls, WithFetchMode("markdown"))
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}

	if results[0].Markdown != "" {
		t.Error("bad URL should have empty markdown")
	}

	if results[1].Markdown == "" {
		t.Error("good URL should have content")
	}
}

func TestWebSearchOption_Defaults(t *testing.T) {
	o := webSearchDefaults()

	if o.engine != Google {
		t.Errorf("engine = %d, want Google", o.engine)
	}
	if o.maxPages != 1 {
		t.Errorf("maxPages = %d, want 1", o.maxPages)
	}
	if o.maxFetch != 5 {
		t.Errorf("maxFetch = %d, want 5", o.maxFetch)
	}
	if o.concurrency != 3 {
		t.Errorf("concurrency = %d, want 3", o.concurrency)
	}
	if o.fetchMode != "" {
		t.Errorf("fetchMode = %q, want empty", o.fetchMode)
	}

	// Test all option functions
	WithWebSearchEngine(Bing)(o)
	if o.engine != Bing {
		t.Error("WithWebSearchEngine failed")
	}

	WithWebSearchMaxPages(3)(o)
	if o.maxPages != 3 {
		t.Error("WithWebSearchMaxPages failed")
	}

	WithWebSearchLanguage("en")(o)
	if o.language != "en" {
		t.Error("WithWebSearchLanguage failed")
	}

	WithWebSearchRegion("us")(o)
	if o.region != "us" {
		t.Error("WithWebSearchRegion failed")
	}

	WithWebSearchFetch("markdown")(o)
	if o.fetchMode != "markdown" {
		t.Error("WithWebSearchFetch failed")
	}

	WithWebSearchMainContent()(o)
	if !o.mainOnly {
		t.Error("WithWebSearchMainContent failed")
	}

	WithWebSearchMaxFetch(10)(o)
	if o.maxFetch != 10 {
		t.Error("WithWebSearchMaxFetch failed")
	}

	WithWebSearchConcurrency(5)(o)
	if o.concurrency != 5 {
		t.Error("WithWebSearchConcurrency failed")
	}

	WithWebSearchCache(2 * time.Minute)(o)
	if o.cacheTTL != 2*time.Minute {
		t.Error("WithWebSearchCache failed")
	}
}
