package engine

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/scout/urlpolicy"
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
	jobManager     *AsyncJobManager
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

// WithCrawlJobManager attaches an async job manager to track crawl progress.
func WithCrawlJobManager(m *AsyncJobManager) CrawlOption {
	return func(o *crawlOptions) { o.jobManager = m }
}

// CrawlOutput holds the overall results of a crawl, including a job ID
// when an AsyncJobManager is attached.
type CrawlOutput struct {
	JobID   string // non-empty when WithCrawlJobManager is used
	Results []CrawlResult
}

// Crawl performs a BFS crawl starting from startURL, calling handler for each page.
func (b *Browser) Crawl(startURL string, handler CrawlHandler, opts ...CrawlOption) ([]CrawlResult, error) {
	out, err := b.CrawlWithJob(startURL, handler, opts...)
	return out.Results, err
}

// CrawlWithJob is like Crawl but returns a CrawlOutput that includes the
// async job ID when WithCrawlJobManager is used.
func (b *Browser) CrawlWithJob(startURL string, handler CrawlHandler, opts ...CrawlOption) (CrawlOutput, error) {
	o := crawlDefaults()
	for _, fn := range opts {
		fn(o)
	}

	// Parse start URL to determine default allowed domain
	startParsed, err := url.Parse(startURL)
	if err != nil {
		return CrawlOutput{}, fmt.Errorf("scout: crawl: invalid start URL: %w", err)
	}

	if len(o.allowedDomains) == 0 {
		o.allowedDomains = []string{startParsed.Hostname()}
	}

	visited := &visitedSet{urls: make(map[string]bool)}

	var jobID string

	jm := o.jobManager

	if jm != nil {
		var err error

		jobID, err = jm.Create("crawl", map[string]any{
			"start_url": startURL,
			"max_depth": o.maxDepth,
			"max_pages": o.maxPages,
		})
		if err == nil {
			_ = jm.Start(jobID)
		} else {
			jm = nil
		}
	}

	var (
		results  []CrawlResult
		mu       sync.Mutex
		errCount int
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

			errCount++
			if jm != nil {
				_ = jm.UpdateProgress(jobID, len(results), errCount)
			}

			mu.Unlock()

			continue
		}

		if err := page.WaitLoad(); err != nil {
			_ = page.Close()

			<-sem

			result := CrawlResult{URL: item.url, Depth: item.depth, Error: err}

			mu.Lock()

			results = append(results, result)

			errCount++
			if jm != nil {
				_ = jm.UpdateProgress(jobID, len(results), errCount)
			}

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

				errCount++
				if jm != nil {
					_ = jm.UpdateProgress(jobID, len(results), errCount)
					_ = jm.Fail(jobID, err.Error())
				}

				mu.Unlock()

				return CrawlOutput{JobID: jobID, Results: results}, err
			}
		}

		_ = page.Close()

		<-sem

		mu.Lock()

		results = append(results, result)
		if jm != nil {
			_ = jm.UpdateProgress(jobID, len(results), errCount)
		}

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

	if jm != nil {
		_ = jm.Complete(jobID, results)
	}

	return CrawlOutput{JobID: jobID, Results: results}, nil
}

// SitemapURL represents a URL entry in a sitemap.
type SitemapURL struct {
	Loc        string `xml:"loc" json:"loc"`
	LastMod    string `xml:"lastmod" json:"last_mod,omitempty"`
	ChangeFreq string `xml:"changefreq" json:"change_freq,omitempty"`
	Priority   string `xml:"priority" json:"priority,omitempty"`
}

type sitemapURLSet struct {
	URLs []SitemapURL `xml:"url"`
}

type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// Sitemap parsing safety limits. A sitemap index can fan out into nested
// sitemaps; without caps a hostile (or buggy) server can exhaust memory via a
// huge body, an unbounded URL set, or a self-referential index amplification
// bomb. These bound each axis. See sitemaps.org for the 50 MiB convention.
const (
	maxSitemapBytes = 50 << 20 // per-document uncompressed body cap
	maxSitemapDepth = 5        // sitemap-index nesting depth cap
	maxSitemapURLs  = 100_000  // total URL ceiling across the whole tree
)

// ParseSitemap fetches and parses a sitemap.xml, returning all URLs found.
// Supports both sitemap index files and regular sitemaps.
//
// Fetches are gated by the SSRF url policy (urlpolicy.FromEnv) — by default
// loopback, link-local, and metadata targets are blocked; set
// SCOUT_ALLOW_LOCAL_TARGETS to permit them. The body read, recursion depth, and
// total URL count are all bounded (see the maxSitemap* limits).
func (b *Browser) ParseSitemap(sitemapURL string) ([]SitemapURL, error) {
	total := 0
	return b.parseSitemap(context.Background(), urlpolicy.FromEnv(), sitemapURL, 0, map[string]bool{}, &total)
}

func (b *Browser) parseSitemap(
	ctx context.Context,
	policy *urlpolicy.Policy,
	sitemapURL string,
	depth int,
	seen map[string]bool,
	total *int,
) ([]SitemapURL, error) {
	if depth > maxSitemapDepth || *total >= maxSitemapURLs || seen[sitemapURL] {
		return nil, nil
	}

	seen[sitemapURL] = true

	// SSRF gate: block internal/metadata targets before fetching. Applied to
	// every node in the tree so a hostile index cannot redirect a child <loc>
	// at an internal address.
	if policy != nil {
		if err := policy.Check(ctx, sitemapURL); err != nil {
			return nil, fmt.Errorf("scout: parse sitemap: %w", err)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(sitemapURL) //nolint:gosec // policy-checked URL
	if err != nil {
		return nil, fmt.Errorf("scout: parse sitemap: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	// Bound the body read (canonical +1 sentinel pattern).
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSitemapBytes+1))
	if err != nil {
		return nil, fmt.Errorf("scout: parse sitemap read: %w", err)
	}

	if len(body) > maxSitemapBytes {
		return nil, fmt.Errorf("scout: parse sitemap: response exceeds %d bytes", maxSitemapBytes)
	}

	// Try as sitemap index first
	var index sitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		var allURLs []SitemapURL

		for _, sm := range index.Sitemaps {
			urls, err := b.parseSitemap(ctx, policy, sm.Loc, depth+1, seen, total)
			if err != nil {
				continue
			}

			allURLs = append(allURLs, urls...)

			if *total >= maxSitemapURLs {
				break
			}
		}

		return allURLs, nil
	}

	// Parse as regular sitemap
	var urlSet sitemapURLSet
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, fmt.Errorf("scout: parse sitemap xml: %w", err)
	}

	out := make([]SitemapURL, 0, len(urlSet.URLs))
	for _, u := range urlSet.URLs {
		out = append(out, u)

		*total++
		if *total >= maxSitemapURLs {
			break
		}
	}

	return out, nil
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
