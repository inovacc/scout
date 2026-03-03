package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKnowledgeOptionDefaults(t *testing.T) {
	o := knowledgeDefaults()
	if o.maxDepth != 3 {
		t.Fatalf("expected maxDepth 3, got %d", o.maxDepth)
	}
	if o.maxPages != 100 {
		t.Fatalf("expected maxPages 100, got %d", o.maxPages)
	}
	if o.concurrency != 1 {
		t.Fatalf("expected concurrency 1, got %d", o.concurrency)
	}
	if o.timeout != 30*time.Second {
		t.Fatalf("expected timeout 30s, got %v", o.timeout)
	}
	if o.outputDir != "" {
		t.Fatalf("expected empty outputDir, got %q", o.outputDir)
	}
}

func TestKnowledgeOptions(t *testing.T) {
	o := knowledgeDefaults()
	WithKnowledgeDepth(5)(o)
	WithKnowledgeMaxPages(50)(o)
	WithKnowledgeConcurrency(4)(o)
	WithKnowledgeTimeout(60 * time.Second)(o)
	WithKnowledgeOutput("/tmp/out")(o)

	if o.maxDepth != 5 {
		t.Fatalf("expected maxDepth 5, got %d", o.maxDepth)
	}
	if o.maxPages != 50 {
		t.Fatalf("expected maxPages 50, got %d", o.maxPages)
	}
	if o.concurrency != 4 {
		t.Fatalf("expected concurrency 4, got %d", o.concurrency)
	}
	if o.timeout != 60*time.Second {
		t.Fatalf("expected timeout 60s, got %v", o.timeout)
	}
	if o.outputDir != "/tmp/out" {
		t.Fatalf("expected outputDir /tmp/out, got %q", o.outputDir)
	}
}

func TestKnowledgeResultTypes(t *testing.T) {
	r := KnowledgeResult{
		URL:    "https://example.com",
		Domain: "example.com",
		Pages: []KnowledgePage{{
			URL:      "https://example.com",
			Title:    "Example",
			Depth:    0,
			Markdown: "# Hello",
		}},
		Summary: KnowledgeSummary{
			PagesTotal:   1,
			PagesSuccess: 1,
		},
	}

	if r.Domain != "example.com" {
		t.Fatal("domain mismatch")
	}
	if len(r.Pages) != 1 {
		t.Fatal("expected 1 page")
	}
	if r.Summary.PagesTotal != 1 {
		t.Fatal("expected total 1")
	}
}

func TestKnowledgeWriter(t *testing.T) {
	dir := t.TempDir()
	w := NewKnowledgeWriter(dir)

	if err := w.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, sub := range []string{"pages", "screenshots", "har", "snapshots"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil {
			t.Fatalf("expected %s dir: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", sub)
		}
	}

	kp := &KnowledgePage{
		URL:        "https://example.com/about",
		Title:      "About",
		Markdown:   "# About\nHello world",
		Screenshot: "aVZCT1J3MEtHZ28=",
		Snapshot:   "- document\n  - heading: About",
		HAR:        []byte(`{"log":{}}`),
	}
	if err := w.WritePage(kp); err != nil {
		t.Fatalf("WritePage failed: %v", err)
	}

	md, err := os.ReadFile(filepath.Join(dir, "pages", "about.md"))
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if string(md) != "# About\nHello world" {
		t.Fatalf("unexpected markdown: %q", md)
	}

	result := &KnowledgeResult{
		URL:     "https://example.com",
		Domain:  "example.com",
		Summary: KnowledgeSummary{PagesTotal: 1, PagesSuccess: 1},
	}
	if err := w.WriteManifest(result); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	mf, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest KnowledgeResult
	if err := json.Unmarshal(mf, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest.Domain != "example.com" {
		t.Fatalf("expected domain example.com, got %s", manifest.Domain)
	}
}

func TestURLToSlug(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/", "index"},
		{"https://example.com/about", "about"},
		{"https://example.com/blog/post-1", filepath.Join("blog", "post-1")},
		{"https://example.com", "index"},
	}

	for _, tt := range tests {
		got := urlToSlug(tt.url)
		if got != tt.want {
			t.Errorf("urlToSlug(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestKnowledgeIntegration(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	defer b.Close()

	dir := t.TempDir()
	result, err := b.Knowledge(srv.URL, WithKnowledgeDepth(1), WithKnowledgeMaxPages(5), WithKnowledgeOutput(dir))
	if err != nil {
		t.Fatalf("Knowledge failed: %v", err)
	}

	if result.Domain == "" {
		t.Fatal("expected non-empty domain")
	}

	if result.Summary.PagesTotal == 0 {
		t.Fatal("expected at least one page")
	}

	if result.Summary.PagesSuccess == 0 {
		t.Fatal("expected at least one successful page")
	}

	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "pages", "index.md")); err != nil {
		t.Fatalf("expected pages/index.md: %v", err)
	}
}
