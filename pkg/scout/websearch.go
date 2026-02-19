package scout

import (
	"fmt"
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
	Content  *WebFetchResult `json:"content,omitempty"`
}

// WebSearchOption configures WebSearch behavior.
type WebSearchOption func(*webSearchOptions)

type webSearchOptions struct {
	engine      SearchEngine
	maxPages    int
	language    string
	region      string
	fetchMode   string // "" = no fetch, "markdown", "text", "full", etc.
	mainOnly    bool
	maxFetch    int
	concurrency int
	cacheTTL    time.Duration
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

// WebSearch performs a search query and optionally fetches result pages.
func (b *Browser) WebSearch(query string, opts ...WebSearchOption) (*WebSearchResult, error) {
	o := webSearchDefaults()
	for _, fn := range opts {
		fn(o)
	}

	// Build search options
	var searchOpts []SearchOption
	searchOpts = append(searchOpts, WithSearchEngine(o.engine))
	searchOpts = append(searchOpts, WithSearchMaxPages(o.maxPages))
	if o.language != "" {
		searchOpts = append(searchOpts, WithSearchLanguage(o.language))
	}
	if o.region != "" {
		searchOpts = append(searchOpts, WithSearchRegion(o.region))
	}

	results, err := b.SearchAll(query, searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("scout: websearch: %w", err)
	}

	// Convert to WebSearchItems
	items := make([]WebSearchItem, len(results))
	for i, r := range results {
		items[i] = WebSearchItem{
			Title:    r.Title,
			URL:      r.URL,
			Snippet:  r.Snippet,
			Position: r.Position,
		}
	}

	// Fetch result pages if requested
	if o.fetchMode != "" && len(items) > 0 {
		fetchCount := o.maxFetch
		if fetchCount > len(items) {
			fetchCount = len(items)
		}

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

		for i := 0; i < fetchCount; i++ {
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
		Query:   query,
		Engine:  o.engine,
		Results: items,
	}, nil
}
