package scout

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/webfetch", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head>
<title>WebFetch Test Page</title>
<meta name="description" content="A test page for webfetch">
<meta property="og:title" content="OG Title">
<meta property="og:description" content="OG Description">
</head><body>
<article>
<h1>Main Article</h1>
<p>This is the main content of the article with enough text to be considered substantial content for readability scoring purposes.</p>
<p>Another paragraph with more meaningful content to ensure the readability algorithm picks this up as the main content area.</p>
</article>
<nav>
<a href="/page1">Page 1</a>
<a href="/page2">Page 2</a>
<a href="/page1">Page 1 Duplicate</a>
<a href="">Empty</a>
<a href="/page3">Page 3</a>
</nav>
</body></html>`)
		})

		mux.HandleFunc("/webfetch-redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/webfetch", http.StatusFound)
		})

		var webfetchFailCount atomic.Int32
		mux.HandleFunc("/webfetch-flaky", func(w http.ResponseWriter, _ *http.Request) {
			n := webfetchFailCount.Add(1)
			if n <= 2 {
				// Return a connection reset-like error by hijacking and closing
				hj, ok := w.(http.Hijacker)
				if ok {
					conn, _, err := hj.Hijack()
					if err == nil {
						_ = conn.Close()
						return
					}
				}
				// Fallback: 500
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Flaky OK</title></head><body><p>Success</p></body></html>`)
		})

		mux.HandleFunc("/webfetch-minimal", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Minimal Page</title></head>
<body><p>Just a paragraph.</p></body></html>`)
		})
	})
}

func TestWebFetch_FullMode(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL + "/webfetch")
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if result.Title != "WebFetch Test Page" {
		t.Errorf("title = %q, want %q", result.Title, "WebFetch Test Page")
	}
	if result.Markdown == "" {
		t.Error("markdown should not be empty in full mode")
	}
	if result.Meta == nil {
		t.Error("meta should not be nil in full mode")
	}
	if len(result.Links) == 0 {
		t.Error("links should not be empty in full mode")
	}
	if result.HTML != "" {
		t.Error("html should be empty unless WithFetchHTML is set")
	}
	if result.URL != ts.URL+"/webfetch" {
		t.Errorf("url = %q, want %q", result.URL, ts.URL+"/webfetch")
	}
	if result.FetchedAt.IsZero() {
		t.Error("fetched_at should be set")
	}
}

func TestWebFetch_MarkdownMode(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL+"/webfetch", WithFetchMode("markdown"))
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if result.Markdown == "" {
		t.Error("markdown should not be empty")
	}
	if result.Meta != nil {
		t.Error("meta should be nil in markdown mode")
	}
	if len(result.Links) > 0 {
		t.Error("links should be empty in markdown mode")
	}
}

func TestWebFetch_MarkdownMainOnly(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL+"/webfetch", WithFetchMode("markdown"), WithFetchMainContent())
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if result.Markdown == "" {
		t.Error("markdown should not be empty")
	}
}

func TestWebFetch_HTMLMode(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL+"/webfetch", WithFetchMode("html"))
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if result.HTML == "" {
		t.Error("html should not be empty in html mode")
	}
	if !strings.Contains(result.HTML, "<article>") {
		t.Error("html should contain raw HTML")
	}
}

func TestWebFetch_TextMode(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL+"/webfetch", WithFetchMode("text"))
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if result.Markdown == "" {
		t.Error("markdown (main content) should not be empty in text mode")
	}
}

func TestWebFetch_LinksMode(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL+"/webfetch", WithFetchMode("links"))
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if len(result.Links) == 0 {
		t.Fatal("links should not be empty")
	}

	// Check deduplication: /page1 appears twice in HTML but should appear once
	count := 0
	for _, l := range result.Links {
		if l == "/page1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("/page1 appears %d times, want 1 (dedup)", count)
	}
}

func TestWebFetch_MetaMode(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL+"/webfetch", WithFetchMode("meta"))
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if result.Meta == nil {
		t.Fatal("meta should not be nil")
	}
	if result.Meta.Description != "A test page for webfetch" {
		t.Errorf("description = %q, want %q", result.Meta.Description, "A test page for webfetch")
	}
	if result.Markdown != "" {
		t.Error("markdown should be empty in meta mode")
	}
}

func TestWebFetch_FullWithHTML(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL+"/webfetch", WithFetchHTML())
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if result.HTML == "" {
		t.Error("html should not be empty when WithFetchHTML is set")
	}
	if result.Markdown == "" {
		t.Error("markdown should still be populated in full mode")
	}
}

func TestWebFetch_Cache(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	url := ts.URL + "/webfetch-minimal"

	// Use a dedicated cache to avoid pollution
	old := globalFetchCache.entries
	globalFetchCache.mu.Lock()
	globalFetchCache.entries = make(map[string]*fetchCacheEntry)
	globalFetchCache.mu.Unlock()
	defer func() {
		globalFetchCache.mu.Lock()
		globalFetchCache.entries = old
		globalFetchCache.mu.Unlock()
	}()

	r1, err := b.WebFetch(url, WithFetchCache(1*time.Minute))
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}

	r2, err := b.WebFetch(url, WithFetchCache(1*time.Minute))
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}

	// Cached result should have the same FetchedAt
	if r1.FetchedAt != r2.FetchedAt {
		t.Error("second call should return cached result with same FetchedAt")
	}
}

func TestFetchCache_Expiry(t *testing.T) {
	c := &fetchCache{entries: make(map[string]*fetchCacheEntry)}

	r := &WebFetchResult{URL: "http://example.com", Title: "Test"}
	c.set("http://example.com", r, 1*time.Millisecond)

	// Should be found immediately
	got, ok := c.get("http://example.com")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Title != "Test" {
		t.Errorf("title = %q, want %q", got.Title, "Test")
	}

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	_, ok = c.get("http://example.com")
	if ok {
		t.Error("expected cache miss after expiry")
	}
}

func TestWebFetchBatch(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	urls := []string{
		ts.URL + "/webfetch",
		ts.URL + "/webfetch-minimal",
	}

	results := b.WebFetchBatch(urls)

	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}

	// Results should be in input order
	if results[0].URL != urls[0] {
		t.Errorf("result 0 url = %q, want %q", results[0].URL, urls[0])
	}
	if results[1].URL != urls[1] {
		t.Errorf("result 1 url = %q, want %q", results[1].URL, urls[1])
	}

	if results[0].Title != "WebFetch Test Page" {
		t.Errorf("result 0 title = %q, want %q", results[0].Title, "WebFetch Test Page")
	}
	if results[1].Title != "Minimal Page" {
		t.Errorf("result 1 title = %q, want %q", results[1].Title, "Minimal Page")
	}
}

func TestWebFetchBatch_ErrorIsolation(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	urls := []string{
		"http://127.0.0.1:1/nonexistent", // bad URL
		ts.URL + "/webfetch-minimal",      // good URL
	}

	results := b.WebFetchBatch(urls)

	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}

	// Bad URL should have empty fields but not nil
	if results[0] == nil {
		t.Fatal("result 0 should not be nil")
	}
	if results[0].Title != "" {
		t.Errorf("bad url title = %q, want empty", results[0].Title)
	}

	// Good URL should succeed
	if results[1].Title != "Minimal Page" {
		t.Errorf("result 1 title = %q, want %q", results[1].Title, "Minimal Page")
	}
}

func TestWebFetch_RedirectChain(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL + "/webfetch-redirect")
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if len(result.RedirectChain) < 2 {
		t.Fatalf("RedirectChain length = %d, want >= 2", len(result.RedirectChain))
	}
	if result.RedirectChain[0] != ts.URL+"/webfetch-redirect" {
		t.Errorf("RedirectChain[0] = %q, want %q", result.RedirectChain[0], ts.URL+"/webfetch-redirect")
	}
	if !strings.HasSuffix(result.RedirectChain[1], "/webfetch") {
		t.Errorf("RedirectChain[1] = %q, want suffix /webfetch", result.RedirectChain[1])
	}
	if result.Title != "WebFetch Test Page" {
		t.Errorf("title = %q, want %q", result.Title, "WebFetch Test Page")
	}
}

func TestWebFetch_NoRedirectChain(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	result, err := b.WebFetch(ts.URL + "/webfetch")
	if err != nil {
		t.Fatalf("WebFetch failed: %v", err)
	}

	if len(result.RedirectChain) != 0 {
		t.Errorf("RedirectChain should be empty for non-redirect, got %v", result.RedirectChain)
	}
}

func TestWebFetch_RetryOption(t *testing.T) {
	o := webFetchDefaults()

	WithFetchRetries(3)(o)
	if o.retries != 3 {
		t.Errorf("retries = %d, want 3", o.retries)
	}

	WithFetchRetryDelay(500 * time.Millisecond)(o)
	if o.retryDelay != 500*time.Millisecond {
		t.Errorf("retryDelay = %v, want 500ms", o.retryDelay)
	}
}

func TestWebFetch_RetryOnError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}

	b := newTestBrowser(t)

	// This should fail even with retries since the port is invalid
	_, err := b.WebFetch("http://127.0.0.1:1/nonexistent",
		WithFetchRetries(2),
		WithFetchRetryDelay(50*time.Millisecond),
	)
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
	if !strings.Contains(err.Error(), "scout: webfetch:") {
		t.Errorf("error = %q, want scout: webfetch: prefix", err.Error())
	}
}

func TestExtractPageLinks_Dedup(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/webfetch")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	links, err := extractPageLinks(page)
	if err != nil {
		t.Fatalf("extractPageLinks failed: %v", err)
	}

	// /page1 appears twice in HTML — should be deduped
	seen := make(map[string]int)
	for _, l := range links {
		seen[l]++
	}

	if seen["/page1"] != 1 {
		t.Errorf("/page1 count = %d, want 1", seen["/page1"])
	}

	// Empty hrefs should be filtered
	for _, l := range links {
		if l == "" {
			t.Error("empty link should be filtered")
		}
	}
}
