package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// TestSiteInput configures a site-health check.
type TestSiteInput struct {
	URL           string `json:"url"                     jsonschema:"the URL to health-check"`
	Depth         int    `json:"depth,omitempty"         jsonschema:"maximum crawl depth (default 2)"`
	Concurrency   int    `json:"concurrency,omitempty"   jsonschema:"concurrent page limit (default 3)"`
	Timeout       string `json:"timeout,omitempty"       jsonschema:"overall timeout as Go duration (default 60s)"`
	ClickElements bool   `json:"clickElements,omitempty" jsonschema:"click interactive elements to surface latent JS errors"`
}

// TestSiteOutput is the health-check result. Re-exports the engine type.
type TestSiteOutput = scout.HealthReport

// TestSite runs Browser.HealthCheck with options derived from the input.
func TestSite(_ context.Context, b *scout.Browser, in TestSiteInput) (*TestSiteOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: test-site: nil browser")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("tools: test-site: url required")
	}
	opts, err := in.toHealthOptions()
	if err != nil {
		return nil, err
	}
	return b.HealthCheck(in.URL, opts...)
}

func (in TestSiteInput) toHealthOptions() ([]scout.HealthCheckOption, error) {
	var opts []scout.HealthCheckOption
	depth := in.Depth
	if depth == 0 {
		depth = 2
	}
	conc := in.Concurrency
	if conc == 0 {
		conc = 3
	}
	timeout := 60 * time.Second
	if in.Timeout != "" {
		d, err := time.ParseDuration(in.Timeout)
		if err != nil {
			return nil, fmt.Errorf("tools: test-site: parse timeout %q: %w", in.Timeout, err)
		}
		timeout = d
	}
	opts = append(opts,
		scout.WithHealthDepth(depth),
		scout.WithHealthConcurrency(conc),
		scout.WithHealthTimeout(timeout),
	)
	if in.ClickElements {
		opts = append(opts, scout.WithHealthClickElements())
	}
	return opts, nil
}
