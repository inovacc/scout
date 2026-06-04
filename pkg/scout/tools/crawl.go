package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// CrawlInput drives a single-host BFS crawl. Distinct from `swarm` — no
// distributed workers, no domain partitioning queue.
type CrawlInput struct {
	URL            string   `json:"url"                      jsonschema:"URL to start crawling from"`
	MaxDepth       int      `json:"maxDepth,omitempty"       jsonschema:"maximum crawl depth (default 3)"`
	MaxPages       int      `json:"maxPages,omitempty"       jsonschema:"maximum pages to crawl (default 100)"`
	Delay          string   `json:"delay,omitempty"          jsonschema:"per-page delay as Go duration (default 500ms)"`
	AllowedDomains []string `json:"allowedDomains,omitempty" jsonschema:"restrict crawl to these domains (default: same host)"`
	Concurrent     int      `json:"concurrent,omitempty"     jsonschema:"concurrent page limit (default 1 = sequential)"`
}

// CrawlOutput is the per-page crawl result list.
type CrawlOutput struct {
	StartURL string              `json:"start_url"`
	Pages    []scout.CrawlResult `json:"pages"`
	Total    int                 `json:"total"`
}

// Crawl drives Browser.Crawl with a nil handler (collect results, no
// per-page callback). For AI workflows that need per-page processing,
// pipe the output through gather/extract on the result URLs.
func Crawl(_ context.Context, b *scout.Browser, in CrawlInput) (*CrawlOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: crawl: nil browser")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("tools: crawl: url required")
	}
	opts, err := in.toCrawlOptions()
	if err != nil {
		return nil, err
	}
	results, err := b.Crawl(in.URL, nil, opts...)
	if err != nil {
		return nil, err
	}
	return &CrawlOutput{StartURL: in.URL, Pages: results, Total: len(results)}, nil
}

func (in CrawlInput) toCrawlOptions() ([]scout.CrawlOption, error) {
	var opts []scout.CrawlOption
	depth := in.MaxDepth
	if depth == 0 {
		depth = 3
	}
	pages := in.MaxPages
	if pages == 0 {
		pages = 100
	}
	opts = append(opts, scout.WithCrawlMaxDepth(depth), scout.WithCrawlMaxPages(pages))
	if in.Delay != "" {
		d, err := time.ParseDuration(in.Delay)
		if err != nil {
			return nil, fmt.Errorf("tools: crawl: parse delay %q: %w", in.Delay, err)
		}
		opts = append(opts, scout.WithCrawlDelay(d))
	}
	if len(in.AllowedDomains) > 0 {
		opts = append(opts, scout.WithCrawlAllowedDomains(in.AllowedDomains...))
	}
	if in.Concurrent > 0 {
		opts = append(opts, scout.WithCrawlConcurrent(in.Concurrent))
	}
	return opts, nil
}
