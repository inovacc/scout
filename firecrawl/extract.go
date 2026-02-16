package firecrawl

import (
	"context"
	"encoding/json"
	"fmt"
)

type extractParams struct {
	URLs      []string `json:"urls"`
	Prompt    string   `json:"prompt,omitempty"`
	Schema    any      `json:"schema,omitempty"`
	WebSearch bool     `json:"enableWebSearch,omitempty"`
}

// ExtractOption configures an extraction request.
type ExtractOption func(*extractParams)

// WithExtractPrompt sets the AI extraction prompt.
func WithExtractPrompt(prompt string) ExtractOption {
	return func(p *extractParams) { p.Prompt = prompt }
}

// WithExtractSchema sets a JSON schema for structured extraction.
func WithExtractSchema(schema any) ExtractOption {
	return func(p *extractParams) { p.Schema = schema }
}

// WithWebSearch enables web search to supplement extraction.
func WithWebSearch() ExtractOption {
	return func(p *extractParams) { p.WebSearch = true }
}

// Extract performs AI-powered data extraction from URLs.
func (c *Client) Extract(ctx context.Context, urls []string, opts ...ExtractOption) (*ExtractResult, error) {
	params := &extractParams{URLs: urls}
	for _, opt := range opts {
		opt(params)
	}

	data, err := c.post(ctx, "/extract", params)
	if err != nil {
		return nil, err
	}

	var result ExtractResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal extract response: %w", err)
	}

	return &result, nil
}
