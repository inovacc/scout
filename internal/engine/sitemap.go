package engine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SitemapPage holds DOM extraction results for a single crawled page.
type SitemapPage struct {
	URL      string   `json:"url"`
	Title    string   `json:"title"`
	Depth    int      `json:"depth"`
	Links    []string `json:"links,omitempty"`
	DOM      *DOMNode `json:"dom,omitempty"`
	Markdown string   `json:"markdown,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// SitemapResult holds the complete sitemap extraction output.
type SitemapResult struct {
	StartURL string        `json:"start_url"`
	Pages    []SitemapPage `json:"pages"`
	Total    int           `json:"total"`
}

// SitemapOption configures SitemapExtract behavior.
type SitemapOption func(*sitemapOptions)

type sitemapOptions struct {
	maxDepth       int
	maxPages       int
	delay          time.Duration
	allowedDomains []string
	domDepth       int
	selector       string
	mainOnly       bool
	skipJSON       bool
	skipMarkdown   bool
	outputDir      string
}

func sitemapDefaults() *sitemapOptions {
	return &sitemapOptions{
		maxDepth: 3,
		maxPages: 100,
		delay:    500 * time.Millisecond,
		domDepth: 50,
	}
}

// WithSitemapMaxDepth sets the maximum crawl depth. Default: 3.
func WithSitemapMaxDepth(n int) SitemapOption {
	return func(o *sitemapOptions) { o.maxDepth = n }
}

// WithSitemapMaxPages sets the maximum number of pages to extract. Default: 100.
func WithSitemapMaxPages(n int) SitemapOption {
	return func(o *sitemapOptions) { o.maxPages = n }
}

// WithSitemapDelay sets the delay between page visits. Default: 500ms.
func WithSitemapDelay(d time.Duration) SitemapOption {
	return func(o *sitemapOptions) { o.delay = d }
}

// WithSitemapAllowedDomains restricts crawling to the specified domains.
func WithSitemapAllowedDomains(domains ...string) SitemapOption {
	return func(o *sitemapOptions) { o.allowedDomains = domains }
}

// WithSitemapDOMDepth sets the maximum DOM tree depth for JSON extraction. Default: 50.
func WithSitemapDOMDepth(n int) SitemapOption {
	return func(o *sitemapOptions) { o.domDepth = n }
}

// WithSitemapSelector scopes DOM extraction to a CSS selector.
func WithSitemapSelector(s string) SitemapOption {
	return func(o *sitemapOptions) { o.selector = s }
}

// WithSitemapMainOnly uses a heuristic to find main content (markdown only).
func WithSitemapMainOnly() SitemapOption {
	return func(o *sitemapOptions) { o.mainOnly = true }
}

// WithSitemapSkipJSON disables DOM JSON extraction.
func WithSitemapSkipJSON() SitemapOption {
	return func(o *sitemapOptions) { o.skipJSON = true }
}

// WithSitemapSkipMarkdown disables markdown extraction.
func WithSitemapSkipMarkdown() SitemapOption {
	return func(o *sitemapOptions) { o.skipMarkdown = true }
}

// WithSitemapOutputDir enables writing per-page files and an index to the given directory.
func WithSitemapOutputDir(dir string) SitemapOption {
	return func(o *sitemapOptions) { o.outputDir = dir }
}

// SitemapExtract crawls a site starting from startURL and extracts DOM JSON
// and Markdown for every page using the bridge extension.
func (b *Browser) SitemapExtract(startURL string, opts ...SitemapOption) (*SitemapResult, error) {
	o := sitemapDefaults()
	for _, fn := range opts {
		fn(o)
	}

	startParsed, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("scout: sitemap: invalid start URL: %w", err)
	}

	if len(o.allowedDomains) == 0 {
		o.allowedDomains = []string{startParsed.Hostname()}
	}

	// Open a single page and bridge for reuse across navigations.
	page, err := b.NewPage("about:blank")
	if err != nil {
		return nil, fmt.Errorf("scout: sitemap: new page: %w", err)
	}

	defer func() { _ = page.Close() }()

	bridge, err := page.Bridge(WithQueryTimeout(15 * time.Second))
	if err != nil {
		return nil, fmt.Errorf("scout: sitemap: bridge init: %w", err)
	}

	visited := &visitedSet{urls: make(map[string]bool)}

	type queueItem struct {
		url   string
		depth int
	}

	queue := []queueItem{{url: normalizeURL(startURL), depth: 0}}
	visited.add(normalizeURL(startURL))

	result := &SitemapResult{StartURL: startURL}

	if o.outputDir != "" {
		if err := os.MkdirAll(o.outputDir, 0o755); err != nil {
			return nil, fmt.Errorf("scout: sitemap: create output dir: %w", err)
		}
	}

	for len(queue) > 0 && len(result.Pages) < o.maxPages {
		item := queue[0]
		queue = queue[1:]

		if item.depth > o.maxDepth {
			continue
		}

		sp := SitemapPage{URL: item.url, Depth: item.depth}

		if err := page.Navigate(item.url); err != nil {
			sp.Error = err.Error()
			result.Pages = append(result.Pages, sp)

			continue
		}

		if err := page.WaitLoad(); err != nil {
			sp.Error = err.Error()
			result.Pages = append(result.Pages, sp)

			continue
		}

		sp.Title, _ = page.Title()

		// Wait for bridge ready.
		if err := waitBridgeReady(bridge, 5*time.Second); err != nil {
			sp.Error = err.Error()
			result.Pages = append(result.Pages, sp)

			continue
		}

		// Build DOM options.
		var domOpts []DOMOption
		if o.selector != "" {
			domOpts = append(domOpts, WithDOMSelector(o.selector))
		}

		if o.domDepth != 50 {
			domOpts = append(domOpts, WithDOMDepth(o.domDepth))
		}

		// Extract DOM JSON.
		if !o.skipJSON {
			node, err := bridge.DOM(domOpts...)
			if err != nil {
				sp.Error = err.Error()
			} else {
				sp.DOM = node
			}
		}

		// Extract Markdown.
		if !o.skipMarkdown {
			var mdOpts []DOMOption
			if o.selector != "" {
				mdOpts = append(mdOpts, WithDOMSelector(o.selector))
			}

			if o.mainOnly {
				mdOpts = append(mdOpts, WithDOMMainOnly())
			}

			md, err := bridge.DOMMarkdown(mdOpts...)
			if err != nil {
				if sp.Error == "" {
					sp.Error = err.Error()
				}
			} else {
				sp.Markdown = md
			}
		}

		// Extract links for BFS.
		links, err := page.ExtractLinks()
		if err == nil {
			pageURL, _ := page.URL()
			for _, link := range links {
				absURL := resolveLink(pageURL, link)
				if absURL != "" {
					sp.Links = append(sp.Links, absURL)
				}
			}
		}

		// Write per-page files if outputDir is set.
		if o.outputDir != "" {
			writeSitemapPageFiles(o.outputDir, &sp)
		}

		result.Pages = append(result.Pages, sp)

		// Enqueue new links.
		if item.depth < o.maxDepth {
			for _, link := range sp.Links {
				normalized := normalizeURL(link)
				if !visited.has(normalized) && isDomainAllowed(normalized, o.allowedDomains) {
					visited.add(normalized)
					queue = append(queue, queueItem{url: normalized, depth: item.depth + 1})
				}
			}
		}

		if o.delay > 0 {
			time.Sleep(o.delay)
		}
	}

	result.Total = len(result.Pages)

	// Write index files.
	if o.outputDir != "" {
		writeSitemapIndex(o.outputDir, result)
	}

	return result, nil
}

func waitBridgeReady(bridge *Bridge, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for !bridge.Available() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	if !bridge.Available() {
		return fmt.Errorf("scout: sitemap: bridge not available")
	}

	return nil
}

// urlToDir converts a URL to a filesystem-safe directory name.
func urlToDir(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}

	name := u.Hostname()
	if u.Path != "" && u.Path != "/" {
		name += strings.ReplaceAll(u.Path, "/", "-")
	}

	name = strings.TrimRight(name, "-")

	return name
}

func writeSitemapPageFiles(outputDir string, sp *SitemapPage) {
	dir := filepath.Join(outputDir, urlToDir(sp.URL))
	_ = os.MkdirAll(dir, 0o755)

	if sp.DOM != nil {
		data, err := json.MarshalIndent(sp.DOM, "", "  ")
		if err == nil {
			_ = os.WriteFile(filepath.Join(dir, "dom.json"), data, 0o644)
		}
	}

	if sp.Markdown != "" {
		_ = os.WriteFile(filepath.Join(dir, "dom.md"), []byte(sp.Markdown), 0o644)
	}
}

func writeSitemapIndex(outputDir string, result *SitemapResult) {
	// index.json — SitemapResult with DOM/Markdown omitted per page.
	indexPages := make([]SitemapPage, len(result.Pages))
	for i, p := range result.Pages {
		indexPages[i] = SitemapPage{
			URL:   p.URL,
			Title: p.Title,
			Depth: p.Depth,
			Links: p.Links,
			Error: p.Error,
		}
	}

	indexResult := &SitemapResult{
		StartURL: result.StartURL,
		Pages:    indexPages,
		Total:    result.Total,
	}

	data, err := json.MarshalIndent(indexResult, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(outputDir, "index.json"), data, 0o644)
	}

	// index.md — all pages' markdown concatenated.
	var sb strings.Builder
	sb.WriteString("# Sitemap Extract\n\n")
	fmt.Fprintf(&sb, "Start URL: %s\n", result.StartURL)
	fmt.Fprintf(&sb, "Total pages: %d\n\n", result.Total)

	for _, p := range result.Pages {
		fmt.Fprintf(&sb, "---\n\n## %s\n\n", p.URL)

		if p.Title != "" {
			fmt.Fprintf(&sb, "**Title:** %s\n\n", p.Title)
		}

		if p.Error != "" {
			fmt.Fprintf(&sb, "**Error:** %s\n\n", p.Error)
		}

		if p.Markdown != "" {
			sb.WriteString(p.Markdown)
			sb.WriteString("\n\n")
		}
	}

	_ = os.WriteFile(filepath.Join(outputDir, "index.md"), []byte(sb.String()), 0o644)
}
