package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// SitemapInput configures a sitemap extraction (crawl + DOM + markdown per page).
type SitemapInput struct {
	URL            string   `json:"url"                       jsonschema:"the URL to start sitemap extraction from"`
	MaxDepth       int      `json:"maxDepth,omitempty"        jsonschema:"maximum crawl depth"`
	MaxPages       int      `json:"maxPages,omitempty"        jsonschema:"maximum pages to visit"`
	Delay          string   `json:"delay,omitempty"           jsonschema:"per-page delay as Go duration (politeness)"`
	AllowedDomains []string `json:"allowedDomains,omitempty"  jsonschema:"only follow links matching these domains (default: same host)"`
	DOMDepth       int      `json:"domDepth,omitempty"        jsonschema:"per-page DOM tree depth limit (0 = unlimited)"`
	Selector       string   `json:"selector,omitempty"        jsonschema:"CSS selector to scope DOM extraction per page"`
	MainOnly       bool     `json:"mainOnly,omitempty"        jsonschema:"extract only the page's <main> element"`
	SkipJSON       bool     `json:"skipJson,omitempty"        jsonschema:"omit DOM JSON from output (markdown only)"`
	SkipMarkdown   bool     `json:"skipMarkdown,omitempty"    jsonschema:"omit markdown rendering"`
	OutputDir      string   `json:"outputDir,omitempty"       jsonschema:"if set, also write per-page artifacts to this directory"`
}

// SitemapOutput re-exports the engine result.
type SitemapOutput = scout.SitemapResult

// Sitemap drives Browser.SitemapExtract with options derived from input.
func Sitemap(_ context.Context, b *scout.Browser, in SitemapInput) (*SitemapOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: sitemap: nil browser")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("tools: sitemap: url required")
	}
	opts, err := in.toSitemapOptions()
	if err != nil {
		return nil, err
	}
	return b.SitemapExtract(in.URL, opts...)
}

func (in SitemapInput) toSitemapOptions() ([]scout.SitemapOption, error) {
	var opts []scout.SitemapOption
	if in.MaxDepth > 0 {
		opts = append(opts, scout.WithSitemapMaxDepth(in.MaxDepth))
	}
	if in.MaxPages > 0 {
		opts = append(opts, scout.WithSitemapMaxPages(in.MaxPages))
	}
	if in.Delay != "" {
		d, err := time.ParseDuration(in.Delay)
		if err != nil {
			return nil, fmt.Errorf("tools: sitemap: parse delay %q: %w", in.Delay, err)
		}
		opts = append(opts, scout.WithSitemapDelay(d))
	}
	if len(in.AllowedDomains) > 0 {
		opts = append(opts, scout.WithSitemapAllowedDomains(in.AllowedDomains...))
	}
	if in.DOMDepth > 0 {
		opts = append(opts, scout.WithSitemapDOMDepth(in.DOMDepth))
	}
	if in.Selector != "" {
		opts = append(opts, scout.WithSitemapSelector(in.Selector))
	}
	if in.MainOnly {
		opts = append(opts, scout.WithSitemapMainOnly())
	}
	if in.SkipJSON {
		opts = append(opts, scout.WithSitemapSkipJSON())
	}
	if in.SkipMarkdown {
		opts = append(opts, scout.WithSitemapSkipMarkdown())
	}
	if in.OutputDir != "" {
		opts = append(opts, scout.WithSitemapOutputDir(in.OutputDir))
	}
	return opts, nil
}
