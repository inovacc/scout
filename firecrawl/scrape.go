package firecrawl

import (
	"context"
	"encoding/json"
	"fmt"
)

type scrapeParams struct {
	URL             string   `json:"url"`
	Formats         []Format `json:"formats,omitempty"`
	OnlyMainContent *bool    `json:"onlyMainContent,omitempty"`
	IncludeTags     []string `json:"includeTags,omitempty"`
	ExcludeTags     []string `json:"excludeTags,omitempty"`
	WaitFor         int      `json:"waitFor,omitempty"`
	Timeout         int      `json:"timeout,omitempty"`
}

// ScrapeOption configures a scrape request.
type ScrapeOption func(*scrapeParams)

// WithFormats sets the output formats for scraping.
func WithFormats(formats ...Format) ScrapeOption {
	return func(p *scrapeParams) { p.Formats = formats }
}

// WithOnlyMainContent extracts only the main content, removing navs/footers.
func WithOnlyMainContent() ScrapeOption {
	return func(p *scrapeParams) { v := true; p.OnlyMainContent = &v }
}

// WithIncludeTags limits extraction to specific HTML tags.
func WithIncludeTags(tags ...string) ScrapeOption {
	return func(p *scrapeParams) { p.IncludeTags = tags }
}

// WithExcludeTags excludes specific HTML tags from extraction.
func WithExcludeTags(tags ...string) ScrapeOption {
	return func(p *scrapeParams) { p.ExcludeTags = tags }
}

// WithWaitFor sets the wait time in milliseconds for dynamic content.
func WithWaitFor(ms int) ScrapeOption {
	return func(p *scrapeParams) { p.WaitFor = ms }
}

// WithScrapeTimeout sets the scrape timeout in milliseconds.
func WithScrapeTimeout(ms int) ScrapeOption {
	return func(p *scrapeParams) { p.Timeout = ms }
}

// Scrape scrapes a single URL and returns the document.
func (c *Client) Scrape(ctx context.Context, url string, opts ...ScrapeOption) (*Document, error) {
	params := &scrapeParams{URL: url}
	for _, opt := range opts {
		opt(params)
	}

	data, err := c.post(ctx, "/scrape", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Success bool     `json:"success"`
		Data    Document `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal scrape response: %w", err)
	}

	return &resp.Data, nil
}
