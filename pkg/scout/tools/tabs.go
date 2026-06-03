package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// TabInfo describes a single open browser tab.
type TabInfo struct {
	Index int    `json:"index"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// TabsInput takes no fields (lists the browser's open tabs).
type TabsInput struct{}

// TabsOutput holds the list of open tabs.
type TabsOutput struct {
	Tabs []TabInfo `json:"tabs"`
}

// Tabs lists every open tab in the browser.
func Tabs(_ context.Context, b *scout.Browser, _ TabsInput) (*TabsOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: tabs: nil browser")
	}

	pages, err := b.Pages()
	if err != nil {
		return nil, fmt.Errorf("tools: tabs: %w", err)
	}

	out := &TabsOutput{Tabs: make([]TabInfo, 0, len(pages))}
	for i, p := range pages {
		u, _ := p.URL()
		t, _ := p.Title()
		out.Tabs = append(out.Tabs, TabInfo{Index: i, URL: u, Title: t})
	}

	return out, nil
}

// NewTabInput optionally carries a URL to open in the new tab.
type NewTabInput struct {
	URL string `json:"url,omitempty" jsonschema:"optional URL to open in the new tab"`
}

// NewTabOutput reports the landed page in the newly opened tab.
type NewTabOutput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// NewTab opens a new tab, optionally navigating it to in.URL.
func NewTab(_ context.Context, b *scout.Browser, in NewTabInput) (*NewTabOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: new-tab: nil browser")
	}

	p, err := b.NewPage(in.URL)
	if err != nil {
		return nil, fmt.Errorf("tools: new-tab: %w", err)
	}

	if in.URL != "" {
		waitLoadBounded(p, 15*time.Second)
	}

	u, _ := p.URL()
	t, _ := p.Title()

	return &NewTabOutput{URL: u, Title: t}, nil
}
