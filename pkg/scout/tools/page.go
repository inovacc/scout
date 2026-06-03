package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// NavigateInput targets a URL on the caller's page.
type NavigateInput struct {
	URL string `json:"url" jsonschema:"the URL to navigate to"`
}

// NavigateOutput reports the landed page after load.
type NavigateOutput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Navigate drives the given page to in.URL and waits for load (best-effort).
func Navigate(_ context.Context, p *scout.Page, in NavigateInput) (*NavigateOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: navigate: nil page")
	}

	if in.URL == "" {
		return nil, fmt.Errorf("tools: navigate: url required")
	}

	if err := p.Navigate(in.URL); err != nil {
		return nil, fmt.Errorf("tools: navigate: %w", err)
	}

	_ = p.WaitLoad()

	url, _ := p.URL()
	title, _ := p.Title()

	return &NavigateOutput{URL: url, Title: title}, nil
}
