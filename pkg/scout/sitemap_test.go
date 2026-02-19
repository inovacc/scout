package scout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSitemapExtract(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	result, err := browser.SitemapExtract(ts.URL+"/crawl-start",
		WithSitemapMaxDepth(2),
		WithSitemapMaxPages(10),
		WithSitemapDelay(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("SitemapExtract() error: %v", err)
	}

	if result.Total < 3 {
		t.Errorf("expected >= 3 pages, got %d", result.Total)
	}

	foundStart := false
	for _, p := range result.Pages {
		t.Logf("page: %s (depth=%d, title=%q, dom=%v, md=%d, err=%q)",
			p.URL, p.Depth, p.Title, p.DOM != nil, len(p.Markdown), p.Error)

		if p.Title == "Crawl Start" {
			foundStart = true
		}

		if p.Error != "" {
			continue
		}

		if p.DOM == nil {
			t.Errorf("page %s: expected DOM node", p.URL)
		}

		if p.Markdown == "" {
			t.Errorf("page %s: expected non-empty markdown", p.URL)
		}
	}

	if !foundStart {
		t.Error("expected to find 'Crawl Start' page")
	}
}

func TestSitemapExtractOutputDir(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()

	result, err := browser.SitemapExtract(ts.URL+"/crawl-start",
		WithSitemapMaxDepth(1),
		WithSitemapMaxPages(5),
		WithSitemapDelay(50*time.Millisecond),
		WithSitemapOutputDir(dir),
	)
	if err != nil {
		t.Fatalf("SitemapExtract() error: %v", err)
	}

	// Verify index files exist.
	indexJSON := filepath.Join(dir, "index.json")
	if _, err := os.Stat(indexJSON); err != nil {
		t.Errorf("expected index.json: %v", err)
	}

	indexMD := filepath.Join(dir, "index.md")
	if _, err := os.Stat(indexMD); err != nil {
		t.Errorf("expected index.md: %v", err)
	}

	// Verify index.json is valid and DOM/Markdown are omitted.
	data, err := os.ReadFile(indexJSON)
	if err != nil {
		t.Fatalf("read index.json: %v", err)
	}

	var idx SitemapResult
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("unmarshal index.json: %v", err)
	}

	for _, p := range idx.Pages {
		if p.DOM != nil {
			t.Errorf("index.json should not contain DOM for %s", p.URL)
		}

		if p.Markdown != "" {
			t.Errorf("index.json should not contain Markdown for %s", p.URL)
		}
	}

	// Verify per-page directories exist for successful pages.
	for _, p := range result.Pages {
		if p.Error != "" {
			continue
		}

		pageDir := filepath.Join(dir, urlToDir(p.URL))
		if _, err := os.Stat(pageDir); err != nil {
			t.Errorf("expected page dir %s: %v", pageDir, err)
		}
	}

	t.Logf("output dir: %s, pages: %d", dir, result.Total)
}

func TestSitemapExtractOptions(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	t.Run("skip_json", func(t *testing.T) {
		result, err := browser.SitemapExtract(ts.URL+"/crawl-start",
			WithSitemapMaxDepth(0),
			WithSitemapMaxPages(1),
			WithSitemapDelay(50*time.Millisecond),
			WithSitemapSkipJSON(),
		)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		for _, p := range result.Pages {
			if p.DOM != nil {
				t.Errorf("expected nil DOM with SkipJSON, got %+v", p.DOM)
			}

			if p.Error == "" && p.Markdown == "" {
				t.Error("expected markdown when not skipped")
			}
		}
	})

	t.Run("skip_markdown", func(t *testing.T) {
		result, err := browser.SitemapExtract(ts.URL+"/crawl-start",
			WithSitemapMaxDepth(0),
			WithSitemapMaxPages(1),
			WithSitemapDelay(50*time.Millisecond),
			WithSitemapSkipMarkdown(),
		)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		for _, p := range result.Pages {
			if p.Markdown != "" {
				t.Errorf("expected empty markdown with SkipMarkdown")
			}

			if p.Error == "" && p.DOM == nil {
				t.Error("expected DOM when not skipped")
			}
		}
	})

	t.Run("main_only", func(t *testing.T) {
		result, err := browser.SitemapExtract(ts.URL+"/crawl-start",
			WithSitemapMaxDepth(0),
			WithSitemapMaxPages(1),
			WithSitemapDelay(50*time.Millisecond),
			WithSitemapMainOnly(),
			WithSitemapSkipJSON(),
		)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		if len(result.Pages) == 0 {
			t.Fatal("expected at least one page")
		}
	})
}

func TestSitemapOptionDefaults(t *testing.T) {
	o := sitemapDefaults()

	if o.maxDepth != 3 {
		t.Errorf("default maxDepth = %d, want 3", o.maxDepth)
	}

	if o.maxPages != 100 {
		t.Errorf("default maxPages = %d, want 100", o.maxPages)
	}

	if o.delay != 500*time.Millisecond {
		t.Errorf("default delay = %v", o.delay)
	}

	if o.domDepth != 50 {
		t.Errorf("default domDepth = %d, want 50", o.domDepth)
	}

	WithSitemapMaxDepth(5)(o)
	if o.maxDepth != 5 {
		t.Errorf("maxDepth = %d, want 5", o.maxDepth)
	}

	WithSitemapMaxPages(50)(o)
	if o.maxPages != 50 {
		t.Errorf("maxPages = %d, want 50", o.maxPages)
	}

	WithSitemapDOMDepth(10)(o)
	if o.domDepth != 10 {
		t.Errorf("domDepth = %d, want 10", o.domDepth)
	}

	WithSitemapSelector("main")(o)
	if o.selector != "main" {
		t.Errorf("selector = %q, want 'main'", o.selector)
	}

	WithSitemapMainOnly()(o)
	if !o.mainOnly {
		t.Error("expected mainOnly = true")
	}

	WithSitemapSkipJSON()(o)
	if !o.skipJSON {
		t.Error("expected skipJSON = true")
	}

	WithSitemapSkipMarkdown()(o)
	if !o.skipMarkdown {
		t.Error("expected skipMarkdown = true")
	}

	WithSitemapOutputDir("/tmp/out")(o)
	if o.outputDir != "/tmp/out" {
		t.Errorf("outputDir = %q", o.outputDir)
	}
}

func TestURLToDir(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/", "example.com"},
		{"https://example.com/about", "example.com-about"},
		{"https://example.com/a/b/c", "example.com-a-b-c"},
		{"https://sub.example.com/page", "sub.example.com-page"},
	}

	for _, tt := range tests {
		got := urlToDir(tt.url)
		if got != tt.want {
			t.Errorf("urlToDir(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestSitemapExtractIndexMarkdown(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()

	_, err := browser.SitemapExtract(ts.URL+"/crawl-start",
		WithSitemapMaxDepth(0),
		WithSitemapMaxPages(1),
		WithSitemapDelay(50*time.Millisecond),
		WithSitemapOutputDir(dir),
	)
	if err != nil {
		t.Fatalf("SitemapExtract() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatalf("read index.md: %v", err)
	}

	md := string(data)
	if !strings.Contains(md, "# Sitemap Extract") {
		t.Error("expected '# Sitemap Extract' header in index.md")
	}

	if !strings.Contains(md, "Total pages:") {
		t.Error("expected 'Total pages:' in index.md")
	}
}
