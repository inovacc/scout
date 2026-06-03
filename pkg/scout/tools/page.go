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

// BackInput, ForwardInput and ReloadInput take no fields (operate on the page).
type (
	BackInput    struct{}
	ForwardInput struct{}
	ReloadInput  struct{}
)

// EmptyOutput is returned by verbs with no payload.
type EmptyOutput struct {
	OK bool `json:"ok"`
}

// Back navigates the page back in history.
func Back(_ context.Context, p *scout.Page, _ BackInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: back: nil page")
	}

	if err := p.NavigateBack(); err != nil {
		return nil, fmt.Errorf("tools: back: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// Forward navigates the page forward in history.
func Forward(_ context.Context, p *scout.Page, _ ForwardInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: forward: nil page")
	}

	if err := p.NavigateForward(); err != nil {
		return nil, fmt.Errorf("tools: forward: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// Reload reloads the current page.
func Reload(_ context.Context, p *scout.Page, _ ReloadInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: reload: nil page")
	}

	if err := p.Reload(); err != nil {
		return nil, fmt.Errorf("tools: reload: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}

// WaitInput waits for page load (Selector empty) or for an element to appear.
type WaitInput struct {
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector to wait for; empty waits for page load"`
}

// Wait blocks until the page finishes loading or the selector resolves.
func Wait(_ context.Context, p *scout.Page, in WaitInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: wait: nil page")
	}

	if in.Selector == "" {
		_ = p.WaitLoad()
		return &EmptyOutput{OK: true}, nil
	}

	if _, err := p.Element(in.Selector); err != nil {
		return nil, fmt.Errorf("tools: wait: %w", err)
	}

	return &EmptyOutput{OK: true}, nil
}
