# Knowledge Source Design

## Overview

`scout knowledge <url>` crawls a site and collects all possible intelligence per page: markdown, HTML, links, meta, cookies, screenshots, accessibility snapshots, HAR traffic, tech stack, console logs, Swagger/API docs, and PDFs. Output is both an LLM-ready structured directory and a single JSON blob.

## CLI

```
scout knowledge <url> [flags]
  --depth N          BFS crawl depth (default 3)
  --max-pages N      Maximum pages to visit (default 100)
  --concurrency N    Concurrent page processing (default 1)
  --timeout 30s      Per-page timeout (default 30s)
  --output dir       Output directory (default ./knowledge-{domain}/)
  --json             Output single JSON blob to stdout
```

## Core API

```go
func (b *Browser) Knowledge(targetURL string, opts ...KnowledgeOption) (*KnowledgeResult, error)
```

### Options

- `WithKnowledgeDepth(n int)`
- `WithKnowledgeMaxPages(n int)`
- `WithKnowledgeConcurrency(n int)`
- `WithKnowledgeTimeout(d time.Duration)`
- `WithKnowledgeOutput(dir string)` — stream pages to disk as they complete

## Data Model

```go
type KnowledgeResult struct {
    URL         string              `json:"url"`
    Domain      string              `json:"domain"`
    CrawledAt   time.Time           `json:"crawled_at"`
    Duration    string              `json:"duration"`
    TechStack   *TechStack          `json:"tech_stack"`
    Sitemap     []SitemapURL        `json:"sitemap,omitempty"`
    Pages       []KnowledgePage     `json:"pages"`
    Summary     KnowledgeSummary    `json:"summary"`
}

type KnowledgePage struct {
    URL         string          `json:"url"`
    Title       string          `json:"title"`
    Depth       int             `json:"depth"`
    Markdown    string          `json:"markdown"`
    HTML        string          `json:"html"`
    Links       []string        `json:"links"`
    Meta        *MetaData       `json:"meta"`
    Cookies     []Cookie        `json:"cookies"`
    Screenshot  string          `json:"screenshot"`
    Snapshot    string          `json:"snapshot"`
    HAR         []byte          `json:"har"`
    HAREntries  int             `json:"har_entries"`
    Frameworks  []FrameworkInfo `json:"frameworks"`
    PageInfo    *PageInfo       `json:"page_info"`
    ConsoleLog  []string        `json:"console_log"`
    Swagger     *SwaggerSpec    `json:"swagger,omitempty"`
    PDF         []byte          `json:"pdf,omitempty"`
    Error       string          `json:"error,omitempty"`
}

type KnowledgeSummary struct {
    PagesTotal   int            `json:"pages_total"`
    PagesSuccess int            `json:"pages_success"`
    PagesFailed  int            `json:"pages_failed"`
    UniqueLinks  int            `json:"unique_links"`
    Issues       []HealthIssue  `json:"issues,omitempty"`
}
```

## Directory Output

```
knowledge-example.com/
  manifest.json          # KnowledgeResult minus page content
  pages/
    index.md
    about.md
    blog/
      post-1.md
  screenshots/
    index.png
    about.png
  har/
    index.har
  snapshots/
    index.yaml
  cookies.json
  links.json
  tech-stack.json
  sitemap.json
```

## Pipeline (per page)

1. Navigate + `WaitLoad()` + `WaitFrameworkReady()`
2. Start `SessionHijacker` (HAR capture) + CDP console listeners
3. Call `Gather()` with all options (markdown, HTML, links, meta, cookies, screenshot, snapshot, frameworks, console)
4. Call `DetectTechStack()` (first page only, cached)
5. Try `ExtractSwagger()` (skip silently if not API docs)
6. Generate PDF
7. Stop hijacker, export HAR
8. Stream page to disk immediately
9. Collect health issues from console + network errors

## Architecture

**Approach:** Gather-per-page inside Crawl BFS handler, stream results to disk.

Reuses: `Crawl()` BFS, `Gather()` aggregator, `DetectTechStack()`, `SessionHijacker` + `HijackRecorder`, `HealthIssue` model, `ParseSitemap()`.

## Files

| File | Purpose |
|------|---------|
| `pkg/scout/knowledge.go` | `Browser.Knowledge()`, result types, crawl handler |
| `pkg/scout/knowledge_option.go` | Functional options |
| `pkg/scout/knowledge_writer.go` | Directory writer (stream pages to disk) |
| `cmd/scout/knowledge.go` | CLI command |

## Verification

```bash
go build ./pkg/scout/ && go build ./cmd/scout/
go test ./pkg/scout/ -run TestKnowledge -v
# Manual: scout knowledge https://example.com --depth 1
```
