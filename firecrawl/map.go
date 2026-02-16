package firecrawl

import (
	"context"
	"encoding/json"
	"fmt"
)

type mapParams struct {
	URL               string `json:"url"`
	Search            string `json:"search,omitempty"`
	Limit             int    `json:"limit,omitempty"`
	IncludeSubdomains bool   `json:"includeSubdomains,omitempty"`
}

// MapOption configures a map request.
type MapOption func(*mapParams)

// WithMapSearch filters discovered URLs by a search term.
func WithMapSearch(term string) MapOption {
	return func(p *mapParams) { p.Search = term }
}

// WithMapLimit sets the maximum number of URLs to return.
func WithMapLimit(n int) MapOption {
	return func(p *mapParams) { p.Limit = n }
}

// WithIncludeSubdomains includes URLs from subdomains.
func WithIncludeSubdomains() MapOption {
	return func(p *mapParams) { p.IncludeSubdomains = true }
}

// Map discovers URLs on a website without scraping content.
func (c *Client) Map(ctx context.Context, url string, opts ...MapOption) (*MapResult, error) {
	params := &mapParams{URL: url}
	for _, opt := range opts {
		opt(params)
	}

	data, err := c.post(ctx, "/map", params)
	if err != nil {
		return nil, err
	}

	var result MapResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal map response: %w", err)
	}

	return &result, nil
}
