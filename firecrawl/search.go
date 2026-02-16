package firecrawl

import (
	"context"
	"encoding/json"
	"fmt"
)

type searchParams struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
	Lang    string `json:"lang,omitempty"`
	Country string `json:"country,omitempty"`
}

// SearchOption configures a search request.
type SearchOption func(*searchParams)

// WithSearchLimit sets the maximum number of results.
func WithSearchLimit(n int) SearchOption {
	return func(p *searchParams) { p.Limit = n }
}

// WithSearchLang sets the search language (e.g. "en").
func WithSearchLang(lang string) SearchOption {
	return func(p *searchParams) { p.Lang = lang }
}

// WithSearchCountry sets the search country (e.g. "US").
func WithSearchCountry(country string) SearchOption {
	return func(p *searchParams) { p.Country = country }
}

// Search performs a web search and returns results as documents.
func (c *Client) Search(ctx context.Context, query string, opts ...SearchOption) (*SearchResult, error) {
	params := &searchParams{Query: query}
	for _, opt := range opts {
		opt(params)
	}

	data, err := c.post(ctx, "/search", params)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal search response: %w", err)
	}

	return &result, nil
}
