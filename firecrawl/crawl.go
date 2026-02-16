package firecrawl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type crawlParams struct {
	URL            string   `json:"url"`
	Limit          int      `json:"limit,omitempty"`
	MaxDepth       int      `json:"maxDepth,omitempty"`
	IncludePaths   []string `json:"includePaths,omitempty"`
	ExcludePaths   []string `json:"excludePaths,omitempty"`
	AllowBackLinks bool     `json:"allowBackwardLinks,omitempty"`
}

// CrawlOption configures a crawl request.
type CrawlOption func(*crawlParams)

// WithCrawlLimit sets the maximum number of pages to crawl.
func WithCrawlLimit(n int) CrawlOption {
	return func(p *crawlParams) { p.Limit = n }
}

// WithMaxDepth sets the maximum crawl depth.
func WithMaxDepth(n int) CrawlOption {
	return func(p *crawlParams) { p.MaxDepth = n }
}

// WithIncludePaths restricts crawling to matching URL paths.
func WithIncludePaths(paths ...string) CrawlOption {
	return func(p *crawlParams) { p.IncludePaths = paths }
}

// WithExcludePaths excludes matching URL paths from crawling.
func WithExcludePaths(paths ...string) CrawlOption {
	return func(p *crawlParams) { p.ExcludePaths = paths }
}

// WithAllowBackLinks allows crawling links that point back to parent pages.
func WithAllowBackLinks() CrawlOption {
	return func(p *crawlParams) { p.AllowBackLinks = true }
}

// Crawl starts an async crawl job and returns the initial job status.
func (c *Client) Crawl(ctx context.Context, url string, opts ...CrawlOption) (*CrawlJob, error) {
	params := &crawlParams{URL: url}
	for _, opt := range opts {
		opt(params)
	}

	data, err := c.post(ctx, "/crawl", params)
	if err != nil {
		return nil, err
	}

	var job CrawlJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal crawl response: %w", err)
	}

	return &job, nil
}

// GetCrawlStatus checks the status of a crawl job.
func (c *Client) GetCrawlStatus(ctx context.Context, id string) (*CrawlJob, error) {
	data, err := c.get(ctx, "/crawl/"+id)
	if err != nil {
		return nil, err
	}

	var job CrawlJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("firecrawl: unmarshal crawl status: %w", err)
	}

	return &job, nil
}

// WaitForCrawl polls a crawl job until it completes.
func (c *Client) WaitForCrawl(ctx context.Context, id string, interval time.Duration) (*CrawlJob, error) {
	return poll[CrawlJob](ctx, c, "/crawl/"+id, interval)
}

// CancelCrawl cancels a running crawl job.
func (c *Client) CancelCrawl(ctx context.Context, id string) error {
	_, err := c.delete(ctx, "/crawl/"+id)
	return err
}
