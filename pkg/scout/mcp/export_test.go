package mcp

import (
	"context"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/aria"
)

// NewStateForTest constructs a minimal mcpState for integration tests.
func NewStateForTest(t *testing.T) (*mcpState, func()) {
	t.Helper()
	state := &mcpState{
		config:    ServerConfig{Headless: true},
		ariaStore: aria.NewStore(),
		hooks:     newHookRegistry(),
	}
	cleanup := func() {
		if state.browser != nil {
			_ = state.browser.Close()
		}
	}
	return state, cleanup
}

// AriaStore exposes the unexported ariaStore field for tests.
func (s *mcpState) AriaStore() *aria.Store { return s.ariaStore }

// OpenPage creates/reuses the browser page and navigates to url.
func (s *mcpState) OpenPage(url string) (*scout.Page, error) {
	page, err := s.ensurePage(context.Background())
	if err != nil {
		return nil, err
	}
	if err := page.Navigate(url); err != nil {
		return nil, err
	}
	return page, nil
}

// InstallInvalidationHooks exposes installInvalidationHooks for tests.
func (s *mcpState) InstallInvalidationHooks(page *scout.Page) {
	installInvalidationHooks(page, s)
}
