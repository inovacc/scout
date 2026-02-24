package scout

import (
	"fmt"
	"sync"
	"time"
)

// WebFetchResult holds the result of fetching and extracting content from a URL.
type WebFetchResult struct {
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	Markdown      string    `json:"markdown"`
	HTML          string    `json:"html,omitempty"`
	Meta          *MetaData `json:"meta,omitempty"`
	Links         []string  `json:"links,omitempty"`
	RedirectChain []string  `json:"redirect_chain,omitempty"`
	StatusCode    int       `json:"status_code,omitempty"`
	FetchedAt     time.Time `json:"fetched_at"`
}

// WebFetchOption configures WebFetch behavior.
type WebFetchOption func(*webFetchOptions)

type webFetchOptions struct {
	mode        string // "markdown", "html", "text", "links", "meta", "full"
	mainOnly    bool
	includeHTML bool
	cacheTTL    time.Duration
	retries     int
	retryDelay  time.Duration
}

func webFetchDefaults() *webFetchOptions {
	return &webFetchOptions{
		mode: "full",
	}
}

// WithFetchMode sets the extraction mode: "markdown", "html", "text", "links", "meta", or "full" (default).
func WithFetchMode(mode string) WebFetchOption {
	return func(o *webFetchOptions) { o.mode = mode }
}

// WithFetchMainContent enables readability scoring to extract only main content.
func WithFetchMainContent() WebFetchOption {
	return func(o *webFetchOptions) { o.mainOnly = true }
}

// WithFetchHTML includes raw HTML in the result.
func WithFetchHTML() WebFetchOption {
	return func(o *webFetchOptions) { o.includeHTML = true }
}

// WithFetchCache enables in-memory caching with the given TTL.
func WithFetchCache(ttl time.Duration) WebFetchOption {
	return func(o *webFetchOptions) { o.cacheTTL = ttl }
}

// WithFetchRetries sets the number of retry attempts when navigation fails.
// Default is 0 (no retries).
func WithFetchRetries(n int) WebFetchOption {
	return func(o *webFetchOptions) { o.retries = n }
}

// WithFetchRetryDelay sets the delay between retry attempts.
func WithFetchRetryDelay(d time.Duration) WebFetchOption {
	return func(o *webFetchOptions) { o.retryDelay = d }
}

// fetchCache provides simple in-memory URL caching.
type fetchCache struct {
	mu      sync.RWMutex
	entries map[string]*fetchCacheEntry
}

type fetchCacheEntry struct {
	result    *WebFetchResult
	expiresAt time.Time
}

var globalFetchCache = &fetchCache{entries: make(map[string]*fetchCacheEntry)}

func (c *fetchCache) get(url string) (*WebFetchResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[url]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.result, true
}

func (c *fetchCache) set(url string, result *WebFetchResult, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[url] = &fetchCacheEntry{
		result:    result,
		expiresAt: time.Now().Add(ttl),
	}
}

// WebFetch navigates to a URL and extracts structured content in a single call.
// Combines navigation, readability, metadata, and markdown conversion.
func (b *Browser) WebFetch(url string, opts ...WebFetchOption) (*WebFetchResult, error) {
	o := webFetchDefaults()
	for _, fn := range opts {
		fn(o)
	}

	// Check cache
	if o.cacheTTL > 0 {
		if cached, ok := globalFetchCache.get(url); ok {
			return cached, nil
		}
	}

	var page *Page
	var lastErr error

	for attempt := 0; attempt <= o.retries; attempt++ {
		if attempt > 0 {
			if o.retryDelay > 0 {
				time.Sleep(o.retryDelay)
			}
		}

		page, lastErr = b.NewPage(url)
		if lastErr != nil {
			continue
		}

		lastErr = page.WaitLoad()
		if lastErr != nil {
			_ = page.Close()
			page = nil
			continue
		}

		break
	}

	if lastErr != nil {
		if page != nil {
			_ = page.Close()
		}
		return nil, fmt.Errorf("scout: webfetch: navigate: %w", lastErr)
	}
	defer func() { _ = page.Close() }()

	result := &WebFetchResult{
		URL:       url,
		FetchedAt: time.Now(),
	}

	// Redirect tracking: compare final URL to requested URL
	if finalURL, urlErr := page.URL(); urlErr == nil && finalURL != url {
		result.RedirectChain = []string{url, finalURL}
		result.URL = finalURL
	}

	// Title
	title, err := page.Title()
	if err == nil {
		result.Title = title
	}

	switch o.mode {
	case "links":
		links, linkErr := extractPageLinks(page)
		if linkErr != nil {
			return nil, fmt.Errorf("scout: webfetch: links: %w", linkErr)
		}
		result.Links = links

	case "meta":
		meta, metaErr := page.ExtractMeta()
		if metaErr != nil {
			return nil, fmt.Errorf("scout: webfetch: meta: %w", metaErr)
		}
		result.Meta = meta

	case "html":
		h, htmlErr := page.HTML()
		if htmlErr != nil {
			return nil, fmt.Errorf("scout: webfetch: html: %w", htmlErr)
		}
		result.HTML = h

	case "text":
		md, mdErr := page.MarkdownContent(WithMainContentOnly())
		if mdErr != nil {
			return nil, fmt.Errorf("scout: webfetch: text: %w", mdErr)
		}
		result.Markdown = md

	case "markdown":
		var mdOpts []MarkdownOption
		if o.mainOnly {
			mdOpts = append(mdOpts, WithMainContentOnly())
		}
		md, mdErr := page.Markdown(mdOpts...)
		if mdErr != nil {
			return nil, fmt.Errorf("scout: webfetch: markdown: %w", mdErr)
		}
		result.Markdown = md

	default: // "full"
		// Markdown
		var mdOpts []MarkdownOption
		if o.mainOnly {
			mdOpts = append(mdOpts, WithMainContentOnly())
		}
		md, mdErr := page.Markdown(mdOpts...)
		if mdErr == nil {
			result.Markdown = md
		}

		// Meta
		meta, metaErr := page.ExtractMeta()
		if metaErr == nil {
			result.Meta = meta
		}

		// Links
		links, linkErr := extractPageLinks(page)
		if linkErr == nil {
			result.Links = links
		}

		// HTML (only if requested)
		if o.includeHTML {
			h, htmlErr := page.HTML()
			if htmlErr == nil {
				result.HTML = h
			}
		}
	}

	// Cache result
	if o.cacheTTL > 0 {
		globalFetchCache.set(url, result, o.cacheTTL)
	}

	return result, nil
}

// WebFetchBatch fetches multiple URLs concurrently and returns results.
func (b *Browser) WebFetchBatch(urls []string, opts ...WebFetchOption) []*WebFetchResult {
	results := make([]*WebFetchResult, len(urls))
	var wg sync.WaitGroup

	sem := make(chan struct{}, 3) // default concurrency

	for i, u := range urls {
		wg.Add(1)

		go func(idx int, rawURL string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := b.WebFetch(rawURL, opts...)
			if err != nil {
				results[idx] = &WebFetchResult{
					URL:       rawURL,
					FetchedAt: time.Now(),
				}

				return
			}

			results[idx] = result
		}(i, u)
	}

	wg.Wait()

	return results
}

// extractPageLinks extracts all href links from the page.
func extractPageLinks(page *Page) ([]string, error) {
	elems, err := page.Elements("a[href]")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var links []string

	for _, el := range elems {
		href, ok, attrErr := el.Attribute("href")
		if attrErr != nil || !ok || href == "" {
			continue
		}

		if _, ok := seen[href]; ok {
			continue
		}

		seen[href] = struct{}{}
		links = append(links, href)
	}

	return links, nil
}
