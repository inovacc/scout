package scout

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// SearchEngine identifies a search engine.
type SearchEngine int

const (
	// Google is Google Search.
	Google SearchEngine = iota
	// Bing is Microsoft Bing.
	Bing
	// DuckDuckGo is DuckDuckGo Search.
	DuckDuckGo
)

// SearchResult represents a single organic search result.
type SearchResult struct {
	Title    string
	URL      string
	Snippet  string
	Position int
}

// SearchResults holds the full response from a search query.
type SearchResults struct {
	Query           string
	Engine          SearchEngine
	TotalResults    string
	FeaturedSnippet string
	NextPageURL     string
	Results         []SearchResult
}

// SearchOption configures search behavior.
type SearchOption func(*searchOptions)

type searchOptions struct {
	engine         SearchEngine
	maxPages       int
	language       string
	region         string
	delay          time.Duration
	ddgType        string // DuckDuckGo search type: "web", "news", "images"
	recentDuration time.Duration
}

func searchDefaults() *searchOptions {
	return &searchOptions{
		engine:   Google,
		maxPages: 1,
		delay:    1 * time.Second,
	}
}

// WithSearchEngine sets the search engine to use. Default: Google.
func WithSearchEngine(e SearchEngine) SearchOption {
	return func(o *searchOptions) { o.engine = e }
}

// WithSearchMaxPages sets the maximum number of result pages. Default: 1.
func WithSearchMaxPages(n int) SearchOption {
	return func(o *searchOptions) { o.maxPages = n }
}

// WithSearchLanguage sets the language for search results (e.g. "en", "pt-BR").
func WithSearchLanguage(lang string) SearchOption {
	return func(o *searchOptions) { o.language = lang }
}

// WithSearchRegion sets the region for search results (e.g. "us", "br").
func WithSearchRegion(region string) SearchOption {
	return func(o *searchOptions) { o.region = region }
}

// WithDDGSearchType sets the DuckDuckGo search type (web, news, images).
func WithDDGSearchType(t string) SearchOption {
	return func(o *searchOptions) { o.ddgType = t }
}

// WithSearchRecentDuration sets a time filter on search results.
// For Google this appends tbs=qdr: parameters; for Bing it appends &filters=ex1:"ez5_N"
// style filters; for DuckDuckGo it appends &df= parameters.
func WithSearchRecentDuration(d time.Duration) SearchOption {
	return func(o *searchOptions) { o.recentDuration = d }
}

// googleTBS converts a duration to a Google tbs=qdr: parameter value.
func googleTBS(d time.Duration) string {
	switch {
	case d <= time.Hour:
		return "qdr:h"
	case d <= 24*time.Hour:
		return "qdr:d"
	case d <= 7*24*time.Hour:
		return "qdr:w"
	case d <= 31*24*time.Hour:
		return "qdr:m"
	default:
		return "qdr:y"
	}
}

// bingFreshness converts a duration to a Bing freshness filter value.
func bingFreshness(d time.Duration) string {
	switch {
	case d <= 24*time.Hour:
		return "Day"
	case d <= 7*24*time.Hour:
		return "Week"
	case d <= 31*24*time.Hour:
		return "Month"
	default:
		return ""
	}
}

// ddgDateFilter converts a duration to a DuckDuckGo df= parameter value.
func ddgDateFilter(d time.Duration) string {
	switch {
	case d <= 24*time.Hour:
		return "d"
	case d <= 7*24*time.Hour:
		return "w"
	case d <= 31*24*time.Hour:
		return "m"
	default:
		return "y"
	}
}

// Search performs a search query and returns the results from the first page.
func (b *Browser) Search(query string, opts ...SearchOption) (*SearchResults, error) {
	o := searchDefaults()
	for _, fn := range opts {
		fn(o)
	}

	parser := getParser(o.engine)
	searchURL := parser.buildURL(query, o)

	page, err := b.NewPage(searchURL)
	if err != nil {
		return nil, fmt.Errorf("scout: search: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: search wait load: %w", err)
	}

	// Small delay to let dynamic content render
	time.Sleep(500 * time.Millisecond)

	return parser.parse(page, query, o.engine)
}

// SearchAll performs a search and collects results across multiple pages.
func (b *Browser) SearchAll(query string, opts ...SearchOption) ([]SearchResult, error) {
	o := searchDefaults()
	for _, fn := range opts {
		fn(o)
	}

	var allResults []SearchResult

	parser := getParser(o.engine)
	searchURL := parser.buildURL(query, o)

	for pageNum := 0; pageNum < o.maxPages; pageNum++ {
		page, err := b.NewPage(searchURL)
		if err != nil {
			return allResults, fmt.Errorf("scout: search all page %d: %w", pageNum+1, err)
		}

		if err := page.WaitLoad(); err != nil {
			_ = page.Close()
			return allResults, fmt.Errorf("scout: search all page %d wait: %w", pageNum+1, err)
		}

		time.Sleep(500 * time.Millisecond)

		results, err := parser.parse(page, query, o.engine)
		_ = page.Close()

		if err != nil {
			return allResults, err
		}

		// Re-number positions
		offset := len(allResults)
		for i := range results.Results {
			results.Results[i].Position = offset + i + 1
		}

		allResults = append(allResults, results.Results...)

		if results.NextPageURL == "" {
			break
		}

		searchURL = results.NextPageURL

		if pageNum < o.maxPages-1 {
			time.Sleep(o.delay)
		}
	}

	return allResults, nil
}

// --- SERP parsers ---

type serpParser struct {
	resultSelector  string
	titleSelector   string
	linkSelector    string
	snippetSelector string
	nextSelector    string
	buildURL        func(query string, opts *searchOptions) string
}

func getParser(engine SearchEngine) *serpParser {
	switch engine { //nolint:exhaustive // Google is the default case
	case Bing:
		return &bingParser
	case DuckDuckGo:
		return &ddgParser
	case Wikipedia:
		return &wikipediaParser
	default:
		return &googleParser
	}
}

var googleParser = serpParser{
	resultSelector:  "div.g",
	titleSelector:   "h3",
	linkSelector:    "a[href]",
	snippetSelector: "div[data-sncf], div.VwiC3b, span.aCOpRe",
	nextSelector:    "a#pnnext",
	buildURL: func(query string, opts *searchOptions) string {
		u := "https://www.google.com/search?q=" + url.QueryEscape(query)
		if opts.language != "" {
			u += "&hl=" + url.QueryEscape(opts.language)
		}

		if opts.region != "" {
			u += "&gl=" + url.QueryEscape(opts.region)
		}

		if opts.recentDuration > 0 {
			u += "&tbs=" + url.QueryEscape(googleTBS(opts.recentDuration))
		}

		return u
	},
}

var bingParser = serpParser{
	resultSelector:  "li.b_algo",
	titleSelector:   "h2",
	linkSelector:    "h2 a[href]",
	snippetSelector: "p, div.b_caption p",
	nextSelector:    "a.sb_pagN",
	buildURL: func(query string, opts *searchOptions) string {
		u := "https://www.bing.com/search?q=" + url.QueryEscape(query)
		if opts.language != "" {
			u += "&setlang=" + url.QueryEscape(opts.language)
		}

		if opts.recentDuration > 0 {
			if f := bingFreshness(opts.recentDuration); f != "" {
				u += "&filters=ex1%3a%22ez5_" + url.QueryEscape(f) + "%22"
			}
		}

		return u
	},
}

var ddgParser = serpParser{
	resultSelector:  "[data-result], .result, .results_links",
	titleSelector:   "a.result__a, h2 a, .result__title a",
	linkSelector:    "a.result__a, h2 a, .result__title a",
	snippetSelector: "a.result__snippet, .result__body, .result__snippet",
	nextSelector:    ".result--more__btn, button#more-results",
	buildURL: func(query string, opts *searchOptions) string {
		var u string

		switch opts.ddgType {
		case "news":
			u = "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query) + "&iar=news&ia=news"
		case "images":
			u = "https://duckduckgo.com/?q=" + url.QueryEscape(query) + "&iar=images&iax=images&ia=images"
		default:
			u = "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
		}

		if opts.region != "" {
			u += "&kl=" + url.QueryEscape(opts.region)
		}

		if opts.recentDuration > 0 {
			u += "&df=" + url.QueryEscape(ddgDateFilter(opts.recentDuration))
		}

		return u
	},
}

func (sp *serpParser) parse(page *Page, query string, engine SearchEngine) (*SearchResults, error) { //nolint:unparam // error kept for future use
	sr := &SearchResults{
		Query:  query,
		Engine: engine,
	}

	// Extract results
	resultEls, err := page.Elements(sp.resultSelector)
	if err != nil {
		return sr, nil //nolint:nilerr // no results is not an error
	}

	position := 1
	for _, el := range resultEls {
		result := SearchResult{Position: position}

		// Title
		titleEl, err := el.Element(sp.titleSelector)
		if err == nil {
			result.Title, _ = titleEl.Text()
		}

		// URL
		linkEl, err := el.Element(sp.linkSelector)
		if err == nil {
			href, _, _ := linkEl.Attribute("href")
			result.URL = cleanSearchURL(href)
		}

		// Snippet
		snippetEl, err := el.Element(sp.snippetSelector)
		if err == nil {
			result.Snippet, _ = snippetEl.Text()
		}

		if result.Title != "" && result.URL != "" {
			sr.Results = append(sr.Results, result)
			position++
		}
	}

	// Next page URL — use Has() to avoid blocking retry on missing element
	if has, _ := page.Has(sp.nextSelector); has {
		nextEl, err := page.Element(sp.nextSelector)
		if err == nil {
			href, _, _ := nextEl.Attribute("href")
			if href != "" {
				sr.NextPageURL = resolveURL(page, href)
			}
		}
	}

	return sr, nil
}

func cleanSearchURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	// Google wraps URLs in redirects
	if strings.HasPrefix(rawURL, "/url?") {
		u, err := url.Parse(rawURL)
		if err == nil {
			if q := u.Query().Get("q"); q != "" {
				return q
			}

			if q := u.Query().Get("url"); q != "" {
				return q
			}
		}
	}
	// DuckDuckGo uses uddg parameter
	if strings.Contains(rawURL, "uddg=") {
		u, err := url.Parse(rawURL)
		if err == nil {
			if q := u.Query().Get("uddg"); q != "" {
				return q
			}
		}
	}

	return rawURL
}

func resolveURL(page *Page, href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}

	pageURL, err := page.URL()
	if err != nil {
		return href
	}

	base, err := url.Parse(pageURL)
	if err != nil {
		return href
	}

	ref, err := url.Parse(href)
	if err != nil {
		return href
	}

	return base.ResolveReference(ref).String()
}
