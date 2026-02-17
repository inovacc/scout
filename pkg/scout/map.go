package scout

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// MapOption configures URL map/link discovery behavior.
type MapOption func(*mapOptions)

type mapOptions struct {
	limit           int
	includeSubdoms  bool
	includePaths    []string
	excludePaths    []string
	search          string
	useSitemap      bool
	delay           time.Duration
	maxDepth        int
}

func mapDefaults() *mapOptions {
	return &mapOptions{
		limit:      1000,
		useSitemap: true,
		delay:      200 * time.Millisecond,
		maxDepth:   2,
	}
}

// WithMapLimit caps the number of discovered URLs. Default: 1000.
func WithMapLimit(n int) MapOption {
	return func(o *mapOptions) { o.limit = n }
}

// WithMapSubdomains includes URLs from subdomains of the start domain.
func WithMapSubdomains() MapOption {
	return func(o *mapOptions) { o.includeSubdoms = true }
}

// WithMapIncludePaths keeps only URLs whose paths start with any of the given prefixes.
func WithMapIncludePaths(paths ...string) MapOption {
	return func(o *mapOptions) { o.includePaths = paths }
}

// WithMapExcludePaths removes URLs whose paths start with any of the given prefixes.
func WithMapExcludePaths(paths ...string) MapOption {
	return func(o *mapOptions) { o.excludePaths = paths }
}

// WithMapSearch filters URLs to those containing the search term in the path or query.
func WithMapSearch(term string) MapOption {
	return func(o *mapOptions) { o.search = term }
}

// WithMapSitemap controls whether to fetch and parse sitemap.xml. Default: true.
func WithMapSitemap(v bool) MapOption {
	return func(o *mapOptions) { o.useSitemap = v }
}

// WithMapDelay sets the delay between page visits. Default: 200ms.
func WithMapDelay(d time.Duration) MapOption {
	return func(o *mapOptions) { o.delay = d }
}

// WithMapMaxDepth sets link-follow depth for on-page discovery. Default: 2.
func WithMapMaxDepth(n int) MapOption {
	return func(o *mapOptions) { o.maxDepth = n }
}

// Map discovers all URLs on a site by combining sitemap.xml parsing with on-page
// link harvesting. It returns a deduplicated list of URLs.
func (b *Browser) Map(startURL string, opts ...MapOption) ([]string, error) {
	o := mapDefaults()
	for _, fn := range opts {
		fn(o)
	}

	startParsed, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("scout: map: invalid URL: %w", err)
	}

	baseDomain := startParsed.Hostname()
	visited := &visitedSet{urls: make(map[string]bool)}

	var result []string

	addURL := func(rawURL string) bool {
		normalized := normalizeURL(rawURL)
		if visited.has(normalized) {
			return false
		}

		if !mapDomainAllowed(normalized, baseDomain, o.includeSubdoms) {
			return false
		}

		if !mapPathAllowed(normalized, o) {
			return false
		}

		if o.search != "" && !mapSearchMatch(normalized, o.search) {
			return false
		}

		visited.add(normalized)
		result = append(result, normalized)

		return len(result) < o.limit
	}

	// Phase 1: Sitemap discovery
	if o.useSitemap {
		sitemapURL := fmt.Sprintf("%s://%s/sitemap.xml", startParsed.Scheme, startParsed.Host)

		sitemapURLs, err := b.ParseSitemap(sitemapURL)
		if err == nil {
			for _, su := range sitemapURLs {
				if !addURL(su.Loc) {
					return result, nil
				}
			}
		}
	}

	// Phase 2: On-page link harvesting (BFS, lightweight — no handler, no content extraction)
	type item struct {
		url   string
		depth int
	}

	// Ensure start URL is in the set
	addURL(startURL)

	queue := []item{{url: normalizeURL(startURL), depth: 0}}
	crawled := make(map[string]bool)
	crawled[normalizeURL(startURL)] = true

	for len(queue) > 0 && len(result) < o.limit {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth > o.maxDepth {
			continue
		}

		page, err := b.NewPage(cur.url)
		if err != nil {
			continue
		}

		_ = page.WaitLoad()

		links, err := page.ExtractLinks()
		_ = page.Close()

		if err != nil {
			continue
		}

		pageURL := cur.url
		for _, link := range links {
			absURL := resolveLink(pageURL, link)
			if absURL == "" {
				continue
			}

			if addURL(absURL) && cur.depth < o.maxDepth {
				norm := normalizeURL(absURL)
				if !crawled[norm] && mapDomainAllowed(norm, baseDomain, o.includeSubdoms) {
					crawled[norm] = true
					queue = append(queue, item{url: norm, depth: cur.depth + 1})
				}
			}

			if len(result) >= o.limit {
				break
			}
		}

		if len(queue) > 0 && o.delay > 0 {
			time.Sleep(o.delay)
		}
	}

	return result, nil
}

func mapDomainAllowed(rawURL, baseDomain string, includeSubs bool) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := u.Hostname()
	if host == baseDomain {
		return true
	}

	if includeSubs && strings.HasSuffix(host, "."+baseDomain) {
		return true
	}

	return false
}

func mapPathAllowed(rawURL string, o *mapOptions) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	path := u.Path

	if len(o.excludePaths) > 0 {
		for _, ex := range o.excludePaths {
			if strings.HasPrefix(path, ex) {
				return false
			}
		}
	}

	if len(o.includePaths) > 0 {
		for _, inc := range o.includePaths {
			if strings.HasPrefix(path, inc) {
				return true
			}
		}

		return false
	}

	return true
}

func mapSearchMatch(rawURL, term string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	lower := strings.ToLower(u.Path + "?" + u.RawQuery)

	return strings.Contains(lower, strings.ToLower(term))
}
