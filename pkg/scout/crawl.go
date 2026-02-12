package scout

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// CrawlResult holds information about a crawled page.
type CrawlResult struct {
	URL   string
	Title string
	Depth int
	Links []string
	Error error
}

// CrawlHandler is called for each page visited during crawling.
// Return a non-nil error to stop the crawl.
type CrawlHandler func(page *Page, result *CrawlResult) error

// CrawlOption configures crawl behavior.
type CrawlOption func(*crawlOptions)

type crawlOptions struct {
	maxDepth       int
	maxPages       int
	allowedDomains []string
	delay          time.Duration
	concurrent     int
}

func crawlDefaults() *crawlOptions {
	return &crawlOptions{
		maxDepth:   3,
		maxPages:   100,
		delay:      500 * time.Millisecond,
		concurrent: 1,
	}
}

// WithCrawlMaxDepth sets the maximum crawl depth from the start URL. Default: 3.
func WithCrawlMaxDepth(n int) CrawlOption {
	return func(o *crawlOptions) { o.maxDepth = n }
}

// WithCrawlMaxPages sets the maximum number of pages to crawl. Default: 100.
func WithCrawlMaxPages(n int) CrawlOption {
	return func(o *crawlOptions) { o.maxPages = n }
}

// WithCrawlAllowedDomains restricts crawling to the specified domains.
func WithCrawlAllowedDomains(domains ...string) CrawlOption {
	return func(o *crawlOptions) { o.allowedDomains = domains }
}

// WithCrawlDelay sets the delay between page visits. Default: 500ms.
func WithCrawlDelay(d time.Duration) CrawlOption {
	return func(o *crawlOptions) { o.delay = d }
}

// WithCrawlConcurrent sets the number of concurrent pages for crawling. Default: 1.
func WithCrawlConcurrent(n int) CrawlOption {
	return func(o *crawlOptions) {
		if n < 1 {
			n = 1
		}

		o.concurrent = n
	}
}

// Crawl performs a BFS crawl starting from startURL, calling handler for each page.
func (b *Browser) Crawl(startURL string, handler CrawlHandler, opts ...CrawlOption) ([]CrawlResult, error) {
	o := crawlDefaults()
	for _, fn := range opts {
		fn(o)
	}

	// Parse start URL to determine default allowed domain
	startParsed, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("scout: crawl: invalid start URL: %w", err)
	}

	if len(o.allowedDomains) == 0 {
		o.allowedDomains = []string{startParsed.Hostname()}
	}

	visited := &visitedSet{urls: make(map[string]bool)}

	var (
		results []CrawlResult
		mu      sync.Mutex
	)

	type crawlItem struct {
		url   string
		depth int
	}

	queue := []crawlItem{{url: normalizeURL(startURL), depth: 0}}
	visited.add(normalizeURL(startURL))

	sem := make(chan struct{}, o.concurrent)

	for len(queue) > 0 {
		mu.Lock()

		if len(results) >= o.maxPages {
			mu.Unlock()
			break
		}

		mu.Unlock()

		// Take next item from queue
		item := queue[0]
		queue = queue[1:]

		if item.depth > o.maxDepth {
			continue
		}

		sem <- struct{}{}

		// Process the page
		page, err := b.NewPage(item.url)
		if err != nil {
			<-sem

			result := CrawlResult{URL: item.url, Depth: item.depth, Error: err}

			mu.Lock()

			results = append(results, result)

			mu.Unlock()

			continue
		}

		if err := page.WaitLoad(); err != nil {
			_ = page.Close()

			<-sem

			result := CrawlResult{URL: item.url, Depth: item.depth, Error: err}

			mu.Lock()

			results = append(results, result)

			mu.Unlock()

			continue
		}

		result := CrawlResult{URL: item.url, Depth: item.depth}

		// Get title
		result.Title, _ = page.Title()

		// Get links
		links, err := page.ExtractLinks()
		if err == nil {
			pageURL, _ := page.URL()
			for _, link := range links {
				absURL := resolveLink(pageURL, link)
				if absURL != "" {
					result.Links = append(result.Links, absURL)
				}
			}
		}

		// Call handler
		if handler != nil {
			if err := handler(page, &result); err != nil {
				_ = page.Close()

				<-sem
				mu.Lock()

				results = append(results, result)

				mu.Unlock()

				return results, err
			}
		}

		_ = page.Close()

		<-sem

		mu.Lock()

		results = append(results, result)

		mu.Unlock()

		// Enqueue new links
		if item.depth < o.maxDepth {
			for _, link := range result.Links {
				normalized := normalizeURL(link)
				if !visited.has(normalized) && isDomainAllowed(normalized, o.allowedDomains) {
					visited.add(normalized)
					queue = append(queue, crawlItem{url: normalized, depth: item.depth + 1})
				}
			}
		}

		time.Sleep(o.delay)
	}

	return results, nil
}

// SitemapURL represents a URL entry in a sitemap.
type SitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

type sitemapURLSet struct {
	URLs []SitemapURL `xml:"url"`
}

type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// ParseSitemap fetches and parses a sitemap.xml, returning all URLs found.
// Supports both sitemap index files and regular sitemaps.
func (b *Browser) ParseSitemap(sitemapURL string) ([]SitemapURL, error) {
	resp, err := http.Get(sitemapURL) //nolint:gosec // user-provided URL is intentional
	if err != nil {
		return nil, fmt.Errorf("scout: parse sitemap: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("scout: parse sitemap read: %w", err)
	}

	// Try as sitemap index first
	var index sitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		var allURLs []SitemapURL

		for _, sm := range index.Sitemaps {
			urls, err := b.ParseSitemap(sm.Loc)
			if err != nil {
				continue
			}

			allURLs = append(allURLs, urls...)
		}

		return allURLs, nil
	}

	// Parse as regular sitemap
	var urlSet sitemapURLSet
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, fmt.Errorf("scout: parse sitemap xml: %w", err)
	}

	return urlSet.URLs, nil
}

// --- internal helpers ---

type visitedSet struct {
	mu   sync.RWMutex
	urls map[string]bool
}

func (v *visitedSet) has(u string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.urls[u]
}

func (v *visitedSet) add(u string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.urls[u] = true
}

func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// Remove fragment
	u.Fragment = ""
	// Remove trailing slash (unless path is just "/")
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	return u.String()
}

func isDomainAllowed(rawURL string, allowed []string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := u.Hostname()
	for _, d := range allowed {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}

	return false
}

func resolveLink(pageURL, href string) string {
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") {
		return ""
	}

	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}

	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}

	return base.ResolveReference(ref).String()
}
