package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// WebSearchResult holds the combined search + fetch results.
type WebSearchResult struct {
	Query   string          `json:"query"`
	Engine  SearchEngine    `json:"engine"`
	Results []WebSearchItem `json:"results"`
}

// WebSearchItem is a single search result with optional fetched content.
type WebSearchItem struct {
	Title    string          `json:"title"`
	URL      string          `json:"url"`
	Snippet  string          `json:"snippet"`
	Position int             `json:"position"`
	RRFScore float64         `json:"rrf_score,omitempty"`
	Content  *WebFetchResult `json:"content,omitempty"`
}

// WebSearchOption configures WebSearch behavior.
type WebSearchOption func(*webSearchOptions)

type webSearchOptions struct {
	engine         SearchEngine
	engines        []SearchEngine
	maxPages       int
	language       string
	region         string
	domain         string
	excludeDomains []string
	recentDuration time.Duration
	fetchMode      string // "" = no fetch, "markdown", "text", "full", etc.
	mainOnly       bool
	maxFetch       int
	concurrency    int
	cacheTTL       time.Duration
}

func webSearchDefaults() *webSearchOptions {
	return &webSearchOptions{
		engine:      Google,
		maxPages:    1,
		maxFetch:    5,
		concurrency: 3,
	}
}

// WithWebSearchEngine sets the search engine. Default: Google.
func WithWebSearchEngine(e SearchEngine) WebSearchOption {
	return func(o *webSearchOptions) { o.engine = e }
}

// WithWebSearchMaxPages sets the max number of search result pages. Default: 1.
func WithWebSearchMaxPages(n int) WebSearchOption {
	return func(o *webSearchOptions) { o.maxPages = n }
}

// WithWebSearchLanguage sets the language for search results.
func WithWebSearchLanguage(lang string) WebSearchOption {
	return func(o *webSearchOptions) { o.language = lang }
}

// WithWebSearchRegion sets the region for search results.
func WithWebSearchRegion(region string) WebSearchOption {
	return func(o *webSearchOptions) { o.region = region }
}

// WithWebSearchFetch enables fetching result pages with the given mode.
// Mode can be "markdown", "text", "full", etc. Empty string disables fetch.
func WithWebSearchFetch(mode string) WebSearchOption {
	return func(o *webSearchOptions) { o.fetchMode = mode }
}

// WithWebSearchMainContent enables readability filtering on fetched pages.
func WithWebSearchMainContent() WebSearchOption {
	return func(o *webSearchOptions) { o.mainOnly = true }
}

// WithWebSearchMaxFetch limits how many results to fetch. Default: 5.
func WithWebSearchMaxFetch(n int) WebSearchOption {
	return func(o *webSearchOptions) { o.maxFetch = n }
}

// WithWebSearchConcurrency sets fetch parallelism. Default: 3.
func WithWebSearchConcurrency(n int) WebSearchOption {
	return func(o *webSearchOptions) { o.concurrency = n }
}

// WithWebSearchCache sets cache TTL for fetched pages.
func WithWebSearchCache(ttl time.Duration) WebSearchOption {
	return func(o *webSearchOptions) { o.cacheTTL = ttl }
}

// WithSearchEngines sets multiple engines for multi-engine aggregation with RRF merging.
// Accepted values: "google", "bing", "duckduckgo"/"ddg".
func WithSearchEngines(engines ...string) WebSearchOption {
	return func(o *webSearchOptions) {
		o.engines = nil
		for _, name := range engines {
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "google":
				o.engines = append(o.engines, Google)
			case "bing":
				o.engines = append(o.engines, Bing)
			case "duckduckgo", "ddg":
				o.engines = append(o.engines, DuckDuckGo)
			}
		}
	}
}

// WithSearchDomain appends "site:domain" to the query to restrict results.
func WithSearchDomain(domain string) WebSearchOption {
	return func(o *webSearchOptions) { o.domain = domain }
}

// WithSearchExcludeDomain appends "-site:domain" for each domain to exclude.
func WithSearchExcludeDomain(domains ...string) WebSearchOption {
	return func(o *webSearchOptions) { o.excludeDomains = domains }
}

// WithSearchRecent restricts search results to a recent time window.
// For Google: appends tbs=qdr: parameter (h/d/w/m/y based on duration).
// For Bing: appends freshness filter (Day/Week/Month).
// For DuckDuckGo: appends df= parameter (d/w/m/y).
func WithSearchRecent(d time.Duration) WebSearchOption {
	return func(o *webSearchOptions) { o.recentDuration = d }
}

// rrfMerge performs Reciprocal Rank Fusion across multiple ranked lists.
// Each item is scored as sum(1/(k+rank)) across all lists containing it.
// k=60 is the standard constant. Results are sorted by RRF score descending.
func rrfMerge(engineResults [][]WebSearchItem) []WebSearchItem {
	const k = 60.0

	type scored struct {
		item  WebSearchItem
		score float64
	}

	scores := make(map[string]*scored)
	order := make([]string, 0)

	for _, results := range engineResults {
		for _, item := range results {
			rank := float64(item.Position)
			if rank <= 0 {
				rank = 1
			}

			s, ok := scores[item.URL]
			if !ok {
				cp := item
				cp.RRFScore = 0
				s = &scored{item: cp}
				scores[item.URL] = s
				order = append(order, item.URL)
			}

			s.score += 1.0 / (k + rank)
		}
	}

	merged := make([]WebSearchItem, 0, len(scores))
	for _, url := range order {
		s := scores[url]
		s.item.RRFScore = math.Round(s.score*10000) / 10000
		merged = append(merged, s.item)
	}

	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].RRFScore > merged[j].RRFScore
	})

	// Reassign positions after sort
	for i := range merged {
		merged[i].Position = i + 1
	}

	return merged
}

// buildSearchQuery applies domain filters to the base query.
func buildSearchQuery(query string, o *webSearchOptions) string {
	var q strings.Builder
	q.WriteString(query)

	if o.domain != "" {
		q.WriteString(" site:" + o.domain)
	}

	for _, d := range o.excludeDomains {
		q.WriteString(" -site:" + d)
	}

	return q.String()
}

// WebSearch performs a search query and optionally fetches result pages.
func (b *Browser) WebSearch(query string, opts ...WebSearchOption) (*WebSearchResult, error) {
	o := webSearchDefaults()
	for _, fn := range opts {
		fn(o)
	}

	// Apply domain filters to the query
	finalQuery := buildSearchQuery(query, o)

	// Determine which engines to search
	engines := o.engines
	if len(engines) == 0 {
		engines = []SearchEngine{o.engine}
	}

	// Build base search options (shared across engines)
	var baseSearchOpts []SearchOption

	baseSearchOpts = append(baseSearchOpts, WithSearchMaxPages(o.maxPages))
	if o.language != "" {
		baseSearchOpts = append(baseSearchOpts, WithSearchLanguage(o.language))
	}

	if o.region != "" {
		baseSearchOpts = append(baseSearchOpts, WithSearchRegion(o.region))
	}

	if o.recentDuration > 0 {
		baseSearchOpts = append(baseSearchOpts, WithSearchRecentDuration(o.recentDuration))
	}

	// Search each engine sequentially and collect results
	engineResults := make([][]WebSearchItem, 0, len(engines))

	for _, eng := range engines {
		opts := make([]SearchOption, 0, len(baseSearchOpts)+1)
		opts = append(opts, baseSearchOpts...)
		opts = append(opts, WithSearchEngine(eng))

		results, err := b.SearchAll(finalQuery, opts...)
		if err != nil {
			return nil, fmt.Errorf("scout: websearch: %w", err)
		}

		items := make([]WebSearchItem, len(results))
		for i, r := range results {
			items[i] = WebSearchItem{
				Title:    r.Title,
				URL:      r.URL,
				Snippet:  r.Snippet,
				Position: r.Position,
			}
		}

		engineResults = append(engineResults, items)
	}

	// Merge results: RRF for multi-engine, plain for single
	var items []WebSearchItem
	if len(engineResults) == 1 {
		items = engineResults[0]
	} else {
		items = rrfMerge(engineResults)
	}

	// Fetch result pages if requested
	if o.fetchMode != "" && len(items) > 0 {
		fetchCount := min(o.maxFetch, len(items))

		concurrency := o.concurrency
		if concurrency <= 0 {
			concurrency = 3
		}

		var fetchOpts []WebFetchOption

		fetchOpts = append(fetchOpts, WithFetchMode(o.fetchMode))
		if o.mainOnly {
			fetchOpts = append(fetchOpts, WithFetchMainContent())
		}

		if o.cacheTTL > 0 {
			fetchOpts = append(fetchOpts, WithFetchCache(o.cacheTTL))
		}

		var wg sync.WaitGroup

		sem := make(chan struct{}, concurrency)

		for i := range fetchCount {
			wg.Add(1)

			go func(idx int) {
				defer wg.Done()

				sem <- struct{}{}

				defer func() { <-sem }()

				content, fetchErr := b.WebFetch(items[idx].URL, fetchOpts...)
				if fetchErr != nil {
					return // leave Content nil on error
				}

				items[idx].Content = content
			}(i)
		}

		wg.Wait()
	}

	return &WebSearchResult{
		Query:   finalQuery,
		Engine:  o.engine,
		Results: items,
	}, nil
}
