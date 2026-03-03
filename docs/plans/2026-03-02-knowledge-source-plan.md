# Knowledge Source Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** `scout knowledge <url>` crawls a site and collects all possible intelligence per page (markdown, HTML, links, meta, cookies, screenshots, accessibility snapshots, HAR traffic, tech stack, console logs, Swagger/API docs, PDFs), outputting both a structured directory and a single JSON blob.

**Architecture:** Reuses the existing `Crawl()` BFS engine with a custom `CrawlHandler` that calls `Gather()` (all options) per page, plus `DetectTechStack()` (first page only), `ExtractSwagger()` (silently skipped if not API docs), and `PDF()`. A directory writer streams each page to disk as it completes. `KnowledgeResult` aggregates all pages with a summary.

**Tech Stack:** Go, existing scout library (`Crawl`, `Gather`, `DetectTechStack`, `ExtractSwagger`, `SessionHijacker`, `ParseSitemap`), Cobra CLI.

---

### Task 1: Knowledge Options

**Files:**
- Create: `pkg/scout/knowledge_option.go`

**Step 1: Write the failing test**

Create `pkg/scout/knowledge_test.go`:

```go
package scout

import (
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/ -run TestKnowledgeOption -v`
Expected: FAIL — `knowledgeDefaults` not defined

**Step 3: Write implementation**

Create `pkg/scout/knowledge_option.go`:

```go
package scout

import "time"

// KnowledgeOption configures a Knowledge operation.
type KnowledgeOption func(*knowledgeOptions)

type knowledgeOptions struct {
	maxDepth    int
	maxPages    int
	concurrency int
	timeout     time.Duration
	outputDir   string
}

func knowledgeDefaults() *knowledgeOptions {
	return &knowledgeOptions{
		maxDepth:    3,
		maxPages:    100,
		concurrency: 1,
		timeout:     30 * time.Second,
	}
}

// WithKnowledgeDepth sets the BFS crawl depth. Default: 3.
func WithKnowledgeDepth(n int) KnowledgeOption {
	return func(o *knowledgeOptions) { o.maxDepth = n }
}

// WithKnowledgeMaxPages sets the maximum pages to visit. Default: 100.
func WithKnowledgeMaxPages(n int) KnowledgeOption {
	return func(o *knowledgeOptions) { o.maxPages = n }
}

// WithKnowledgeConcurrency sets concurrent page processing. Default: 1.
func WithKnowledgeConcurrency(n int) KnowledgeOption {
	return func(o *knowledgeOptions) {
		if n < 1 {
			n = 1
		}
		o.concurrency = n
	}
}

// WithKnowledgeTimeout sets per-page timeout. Default: 30s.
func WithKnowledgeTimeout(d time.Duration) KnowledgeOption {
	return func(o *knowledgeOptions) { o.timeout = d }
}

// WithKnowledgeOutput sets the output directory for streaming pages to disk.
func WithKnowledgeOutput(dir string) KnowledgeOption {
	return func(o *knowledgeOptions) { o.outputDir = dir }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/ -run TestKnowledgeOption -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/scout/knowledge_option.go pkg/scout/knowledge_test.go
git commit -m "feat: add knowledge source functional options"
```

---

### Task 2: Knowledge Data Model & Core Method Skeleton

**Files:**
- Create: `pkg/scout/knowledge.go`
- Modify: `pkg/scout/knowledge_test.go`

**Step 1: Write the failing test**

Append to `pkg/scout/knowledge_test.go`:

```go
func TestKnowledgeResultTypes(t *testing.T) {
	// Verify struct fields compile and JSON tags work.
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/ -run TestKnowledgeResultTypes -v`
Expected: FAIL — `KnowledgeResult` undefined

**Step 3: Write implementation**

Create `pkg/scout/knowledge.go`:

```go
package scout

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/scout/rod/lib/proto"
)

// KnowledgeResult holds the complete knowledge collection for a site.
type KnowledgeResult struct {
	URL       string           `json:"url"`
	Domain    string           `json:"domain"`
	CrawledAt time.Time        `json:"crawled_at"`
	Duration  string           `json:"duration"`
	TechStack *TechStack       `json:"tech_stack,omitempty"`
	Sitemap   []SitemapURL     `json:"sitemap,omitempty"`
	Pages     []KnowledgePage  `json:"pages"`
	Summary   KnowledgeSummary `json:"summary"`
}

// KnowledgePage holds all intelligence collected from a single page.
type KnowledgePage struct {
	URL        string          `json:"url"`
	Title      string          `json:"title"`
	Depth      int             `json:"depth"`
	Markdown   string          `json:"markdown"`
	HTML       string          `json:"html"`
	Links      []string        `json:"links"`
	Meta       *MetaData       `json:"meta,omitempty"`
	Cookies    []Cookie        `json:"cookies"`
	Screenshot string          `json:"screenshot"`
	Snapshot   string          `json:"snapshot"`
	HAR        []byte          `json:"har,omitempty"`
	HAREntries int             `json:"har_entries"`
	Frameworks []FrameworkInfo `json:"frameworks"`
	PageInfo   *PageInfo       `json:"page_info,omitempty"`
	ConsoleLog []string        `json:"console_log"`
	Swagger    *SwaggerSpec    `json:"swagger,omitempty"`
	PDF        []byte          `json:"pdf,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// KnowledgeSummary provides aggregate stats for the knowledge collection.
type KnowledgeSummary struct {
	PagesTotal   int           `json:"pages_total"`
	PagesSuccess int           `json:"pages_success"`
	PagesFailed  int           `json:"pages_failed"`
	UniqueLinks  int           `json:"unique_links"`
	Issues       []HealthIssue `json:"issues,omitempty"`
}

// Knowledge crawls a site and collects all possible intelligence per page.
func (b *Browser) Knowledge(targetURL string, opts ...KnowledgeOption) (*KnowledgeResult, error) {
	o := knowledgeDefaults()
	for _, fn := range opts {
		fn(o)
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("scout: knowledge: invalid URL: %w", err)
	}

	start := time.Now()
	result := &KnowledgeResult{
		URL:       targetURL,
		Domain:    parsed.Hostname(),
		CrawledAt: start,
	}

	// Try to fetch sitemap.
	sitemapURL := fmt.Sprintf("%s://%s/sitemap.xml", parsed.Scheme, parsed.Host)
	result.Sitemap, _ = b.ParseSitemap(sitemapURL)

	// Set up directory writer if output dir specified.
	var writer *KnowledgeWriter
	if o.outputDir != "" {
		writer = NewKnowledgeWriter(o.outputDir)
		if err := writer.Init(); err != nil {
			return nil, fmt.Errorf("scout: knowledge: init writer: %w", err)
		}
	}

	var (
		techStack   *TechStack
		techOnce    sync.Once
		mu          sync.Mutex
		allLinks    = make(map[string]bool)
		issues      []HealthIssue
	)

	// CrawlHandler: gather all intelligence per page.
	handler := func(page *Page, cr *CrawlResult) error {
		kp := KnowledgePage{
			URL:   cr.URL,
			Title: cr.Title,
			Depth: cr.Depth,
			Links: cr.Links,
		}

		// Detect tech stack on first page only.
		techOnce.Do(func() {
			techStack, _ = page.DetectTechStack()
		})

		// Wait for framework readiness.
		_ = page.WaitFrameworkReady()

		// Set up HAR recorder.
		var recorder *HijackRecorder
		hijacker, hijackErr := page.NewSessionHijacker(WithHijackBodyCapture())
		if hijackErr == nil {
			recorder = NewHijackRecorder()
			go recorder.RecordAll(hijacker.Events())
			defer hijacker.Stop()
		}

		// Set up console capture.
		var consoleLog []string
		rodPage := page.RodPage()
		rodPage.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
			msg := consoleArgsToString(e.Args)
			consoleLog = append(consoleLog, fmt.Sprintf("[%s] %s", e.Type, msg))
		})
		_ = proto.RuntimeEnable{}.Call(rodPage)

		// Collect meta.
		kp.Meta, _ = page.ExtractMeta()

		// Collect cookies.
		kp.Cookies, _ = page.GetCookies()

		// Detect frameworks.
		kp.Frameworks, _ = page.DetectFrameworks()

		// Collect page info.
		kp.PageInfo, _ = page.CollectInfo()

		// HTML.
		kp.HTML, _ = page.HTML()

		// Markdown.
		kp.Markdown, _ = page.Markdown()

		// Screenshot.
		if data, err := page.ScreenshotPNG(); err == nil {
			kp.Screenshot = base64.StdEncoding.EncodeToString(data)
		}

		// Snapshot (accessibility tree).
		kp.Snapshot, _ = page.Snapshot()

		// PDF.
		kp.PDF, _ = page.PDF()

		// Swagger (skip silently if not API docs).
		kp.Swagger, _ = page.ExtractSwagger()

		// Console log.
		kp.ConsoleLog = consoleLog

		// HAR.
		if recorder != nil {
			time.Sleep(500 * time.Millisecond)
			if harData, count, err := recorder.ExportHAR(); err == nil {
				kp.HAR = harData
				kp.HAREntries = count
			}
		}

		// Collect health issues from console errors and network failures.
		for _, msg := range consoleLog {
			if len(msg) > 7 && msg[:7] == "[error]" {
				issues = append(issues, HealthIssue{
					URL:      cr.URL,
					Source:   "console",
					Severity: "error",
					Message:  msg,
				})
			}
		}

		// Track unique links.
		mu.Lock()
		for _, link := range cr.Links {
			allLinks[link] = true
		}
		mu.Unlock()

		// Stream to disk if writer is set.
		if writer != nil {
			_ = writer.WritePage(&kp)
		}

		mu.Lock()
		result.Pages = append(result.Pages, kp)
		mu.Unlock()

		return nil
	}

	// Run the crawl.
	_, err = b.Crawl(targetURL, handler,
		WithCrawlMaxDepth(o.maxDepth),
		WithCrawlMaxPages(o.maxPages),
		WithCrawlConcurrent(o.concurrency),
	)
	if err != nil {
		return nil, fmt.Errorf("scout: knowledge: crawl: %w", err)
	}

	result.TechStack = techStack
	result.Duration = time.Since(start).Round(time.Millisecond).String()

	// Build summary.
	var failed int
	for _, p := range result.Pages {
		if p.Error != "" {
			failed++
		}
	}
	result.Summary = KnowledgeSummary{
		PagesTotal:   len(result.Pages),
		PagesSuccess: len(result.Pages) - failed,
		PagesFailed:  failed,
		UniqueLinks:  len(allLinks),
		Issues:       issues,
	}

	// Write manifest if writer is set.
	if writer != nil {
		_ = writer.WriteManifest(result)
	}

	return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/ -run TestKnowledgeResultTypes -v`
Expected: PASS (struct types compile)

**Step 5: Verify build**

Run: `go build ./pkg/scout/`
Expected: Success (may need KnowledgeWriter stub — see Task 3)

**Step 6: Commit**

```bash
git add pkg/scout/knowledge.go pkg/scout/knowledge_test.go
git commit -m "feat: add Knowledge() method and data model"
```

---

### Task 3: Knowledge Directory Writer

**Files:**
- Create: `pkg/scout/knowledge_writer.go`
- Modify: `pkg/scout/knowledge_test.go`

**Step 1: Write the failing test**

Append to `pkg/scout/knowledge_test.go`:

```go
import (
	"encoding/json"
	"os"
	"path/filepath"
)

func TestKnowledgeWriter(t *testing.T) {
	dir := t.TempDir()
	w := NewKnowledgeWriter(dir)

	if err := w.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify subdirectories created.
	for _, sub := range []string{"pages", "screenshots", "har", "snapshots"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil {
			t.Fatalf("expected %s dir: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", sub)
		}
	}

	// Write a page.
	kp := &KnowledgePage{
		URL:        "https://example.com/about",
		Title:      "About",
		Markdown:   "# About\nHello world",
		Screenshot: "iVBORw0KGgo=", // fake base64
		Snapshot:   "- document\n  - heading: About",
		HAR:        []byte(`{"log":{}}`),
	}
	if err := w.WritePage(kp); err != nil {
		t.Fatalf("WritePage failed: %v", err)
	}

	// Verify markdown file.
	md, err := os.ReadFile(filepath.Join(dir, "pages", "about.md"))
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if string(md) != "# About\nHello world" {
		t.Fatalf("unexpected markdown: %q", md)
	}

	// Write manifest.
	result := &KnowledgeResult{
		URL:    "https://example.com",
		Domain: "example.com",
		Summary: KnowledgeSummary{PagesTotal: 1, PagesSuccess: 1},
	}
	if err := w.WriteManifest(result); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Verify manifest.json.
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/ -run TestKnowledgeWriter -v`
Expected: FAIL — `NewKnowledgeWriter` undefined

**Step 3: Write implementation**

Create `pkg/scout/knowledge_writer.go`:

```go
package scout

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// KnowledgeWriter streams knowledge pages to a structured directory.
type KnowledgeWriter struct {
	dir string
}

// NewKnowledgeWriter creates a writer for the given output directory.
func NewKnowledgeWriter(dir string) *KnowledgeWriter {
	return &KnowledgeWriter{dir: dir}
}

// Init creates the directory structure.
func (w *KnowledgeWriter) Init() error {
	dirs := []string{"pages", "screenshots", "har", "snapshots"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(w.dir, d), 0o755); err != nil {
			return fmt.Errorf("scout: knowledge writer: mkdir %s: %w", d, err)
		}
	}
	return nil
}

// WritePage writes a single page's data to the directory structure.
func (w *KnowledgeWriter) WritePage(kp *KnowledgePage) error {
	slug := urlToSlug(kp.URL)

	// Markdown.
	if kp.Markdown != "" {
		path := filepath.Join(w.dir, "pages", slug+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(kp.Markdown), 0o644); err != nil {
			return fmt.Errorf("scout: knowledge writer: write markdown: %w", err)
		}
	}

	// Screenshot (decode base64 to PNG).
	if kp.Screenshot != "" {
		data, err := base64.StdEncoding.DecodeString(kp.Screenshot)
		if err == nil {
			path := filepath.Join(w.dir, "screenshots", slug+".png")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			_ = os.WriteFile(path, data, 0o644)
		}
	}

	// HAR.
	if len(kp.HAR) > 0 {
		path := filepath.Join(w.dir, "har", slug+".har")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		_ = os.WriteFile(path, kp.HAR, 0o644)
	}

	// Snapshot.
	if kp.Snapshot != "" {
		path := filepath.Join(w.dir, "snapshots", slug+".yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		_ = os.WriteFile(path, []byte(kp.Snapshot), 0o644)
	}

	// PDF.
	if len(kp.PDF) > 0 {
		path := filepath.Join(w.dir, "pages", slug+".pdf")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		_ = os.WriteFile(path, kp.PDF, 0o644)
	}

	return nil
}

// WriteManifest writes the KnowledgeResult (minus page content) as manifest.json.
func (w *KnowledgeWriter) WriteManifest(result *KnowledgeResult) error {
	// Create a copy without heavy page content.
	manifest := *result
	manifest.Pages = nil

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: knowledge writer: marshal manifest: %w", err)
	}

	return os.WriteFile(filepath.Join(w.dir, "manifest.json"), data, 0o644)
}

// urlToSlug converts a URL to a filesystem-safe slug.
// e.g., "https://example.com/blog/post-1" → "blog/post-1"
// "https://example.com/" → "index"
func urlToSlug(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "page"
	}

	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return "index"
	}

	// Sanitize path components.
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = sanitizeFilename(p)
	}

	return filepath.Join(parts...)
}

// sanitizeFilename removes characters unsafe for filenames.
func sanitizeFilename(s string) string {
	replacer := strings.NewReplacer(
		"<", "", ">", "", ":", "", "\"", "",
		"|", "", "?", "", "*", "", "\\", "",
	)
	s = replacer.Replace(s)
	if s == "" {
		return "page"
	}
	return s
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/ -run TestKnowledgeWriter -v`
Expected: PASS

**Step 5: Verify full build**

Run: `go build ./pkg/scout/`
Expected: Success

**Step 6: Commit**

```bash
git add pkg/scout/knowledge_writer.go pkg/scout/knowledge_test.go
git commit -m "feat: add knowledge directory writer"
```

---

### Task 4: Knowledge CLI Command

**Files:**
- Create: `cmd/scout/knowledge.go`

**Step 1: Write the CLI command**

Create `cmd/scout/knowledge.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func knowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge <url>",
		Short: "Crawl a site and collect all possible intelligence",
		Long: `Crawl a site and collect all possible intelligence per page:
markdown, HTML, links, meta, cookies, screenshots, accessibility snapshots,
HAR traffic, tech stack, console logs, Swagger/API docs, and PDFs.

Output is both a structured directory and optionally a single JSON blob.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			if !strings.HasPrefix(targetURL, "http") {
				targetURL = "https://" + targetURL
			}

			depth, _ := cmd.Flags().GetInt("depth")
			maxPages, _ := cmd.Flags().GetInt("max-pages")
			concurrency, _ := cmd.Flags().GetInt("concurrency")
			timeout, _ := cmd.Flags().GetDuration("timeout")
			outputDir, _ := cmd.Flags().GetString("output")
			jsonOut, _ := cmd.Flags().GetBool("json")

			// Default output dir based on domain.
			if outputDir == "" && !jsonOut {
				outputDir = "knowledge-" + domainFromURL(targetURL)
			}

			opts := baseOpts(cmd)
			b, err := scout.New(opts...)
			if err != nil {
				return fmt.Errorf("create browser: %w", err)
			}
			defer b.Close()

			var kOpts []scout.KnowledgeOption
			kOpts = append(kOpts, scout.WithKnowledgeDepth(depth))
			kOpts = append(kOpts, scout.WithKnowledgeMaxPages(maxPages))
			kOpts = append(kOpts, scout.WithKnowledgeConcurrency(concurrency))
			kOpts = append(kOpts, scout.WithKnowledgeTimeout(timeout))
			if outputDir != "" {
				kOpts = append(kOpts, scout.WithKnowledgeOutput(outputDir))
			}

			fmt.Fprintf(os.Stderr, "Crawling %s (depth=%d, max-pages=%d)...\n", targetURL, depth, maxPages)

			result, err := b.Knowledge(targetURL, kOpts...)
			if err != nil {
				return err
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Print summary.
			fmt.Fprintf(os.Stderr, "\nKnowledge collection complete:\n")
			fmt.Fprintf(os.Stderr, "  Pages:  %d (%d ok, %d failed)\n",
				result.Summary.PagesTotal, result.Summary.PagesSuccess, result.Summary.PagesFailed)
			fmt.Fprintf(os.Stderr, "  Links:  %d unique\n", result.Summary.UniqueLinks)
			fmt.Fprintf(os.Stderr, "  Time:   %s\n", result.Duration)
			if outputDir != "" {
				fmt.Fprintf(os.Stderr, "  Output: %s/\n", outputDir)
			}
			if result.TechStack != nil && len(result.TechStack.Frameworks) > 0 {
				names := make([]string, len(result.TechStack.Frameworks))
				for i, f := range result.TechStack.Frameworks {
					names[i] = f.Name
				}
				fmt.Fprintf(os.Stderr, "  Stack:  %s\n", strings.Join(names, ", "))
			}

			return nil
		},
	}

	cmd.Flags().Int("depth", 3, "BFS crawl depth")
	cmd.Flags().Int("max-pages", 100, "Maximum pages to visit")
	cmd.Flags().Int("concurrency", 1, "Concurrent page processing")
	cmd.Flags().Duration("timeout", 30*time.Second, "Per-page timeout")
	cmd.Flags().String("output", "", "Output directory (default: knowledge-{domain}/)")
	cmd.Flags().Bool("json", false, "Output single JSON blob to stdout")

	addBaseFlags(cmd)

	return cmd
}

func domainFromURL(rawURL string) string {
	// Simple extraction — strip scheme and path.
	s := rawURL
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}
```

**Step 2: Register the command**

Find where other commands are registered (likely `cmd/scout/main.go` or `cmd/scout/root.go`) and add:

```go
rootCmd.AddCommand(knowledgeCmd())
```

**Step 3: Verify build**

Run: `go build ./cmd/scout/`
Expected: Success

**Step 4: Commit**

```bash
git add cmd/scout/knowledge.go
git commit -m "feat: add scout knowledge CLI command"
```

---

### Task 5: Integration Test with httptest

**Files:**
- Modify: `pkg/scout/knowledge_test.go`

**Step 1: Write integration test**

Append to `pkg/scout/knowledge_test.go`:

```go
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

	// Verify files written.
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "pages", "index.md")); err != nil {
		t.Fatalf("expected pages/index.md: %v", err)
	}
}
```

**Step 2: Run test**

Run: `go test ./pkg/scout/ -run TestKnowledgeIntegration -v -timeout 120s`
Expected: PASS (requires browser)

**Step 3: Commit**

```bash
git add pkg/scout/knowledge_test.go
git commit -m "test: add knowledge integration test"
```

---

### Task 6: Build Verification & Final Commit

**Step 1: Full build**

Run: `go build ./pkg/scout/ && go build ./cmd/scout/`
Expected: Success

**Step 2: Run all knowledge tests**

Run: `go test ./pkg/scout/ -run TestKnowledge -v -timeout 120s`
Expected: All pass

**Step 3: Lint**

Run: `task lint` (or `golangci-lint run --fix ./... --timeout=5m`)
Expected: Clean or only pre-existing warnings

**Step 4: Final commit if any lint fixes**

```bash
git add -A
git commit -m "style: fix lint issues in knowledge source"
```

---

## Verification Summary

```bash
go build ./pkg/scout/ && go build ./cmd/scout/
go test ./pkg/scout/ -run TestKnowledge -v -timeout 120s
# Manual: scout knowledge https://example.com --depth 1
```
