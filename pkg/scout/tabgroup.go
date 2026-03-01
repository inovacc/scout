package scout

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// TabGroup manages a fixed set of browser tabs for concurrent work.
type TabGroup struct {
	browser *Browser
	tabs    []*Page
	Store   sync.Map
	mu      sync.Mutex
	limiter *rate.Limiter
	timeout time.Duration
	closed  bool
}

// TabGroupOption configures a TabGroup.
type TabGroupOption func(*TabGroup)

// WithTabGroupRateLimit sets a rate limiter for the tab group.
func WithTabGroupRateLimit(rps float64) TabGroupOption {
	return func(tg *TabGroup) {
		tg.limiter = rate.NewLimiter(rate.Limit(rps), 1)
	}
}

// WithTabGroupTimeout sets a per-operation timeout for the tab group.
func WithTabGroupTimeout(d time.Duration) TabGroupOption {
	return func(tg *TabGroup) {
		tg.timeout = d
	}
}

// NewTabGroup creates a group of n blank tabs. It returns an error if n < 1
// or the browser is nil. On partial failure, already-created tabs are closed.
func (b *Browser) NewTabGroup(n int, opts ...TabGroupOption) (*TabGroup, error) {
	if b == nil {
		return nil, fmt.Errorf("scout: tab group: browser is nil")
	}
	if n < 1 {
		return nil, fmt.Errorf("scout: tab group: n must be >= 1, got %d", n)
	}

	tg := &TabGroup{
		browser: b,
		tabs:    make([]*Page, 0, n),
	}
	for _, opt := range opts {
		opt(tg)
	}

	for i := 0; i < n; i++ {
		p, err := b.NewPage("about:blank")
		if err != nil {
			// Clean up already-created tabs.
			for _, tab := range tg.tabs {
				_ = tab.Close()
			}
			return nil, fmt.Errorf("scout: tab group: create tab %d: %w", i, err)
		}
		tg.tabs = append(tg.tabs, p)
	}

	return tg, nil
}

// Tab returns the i-th tab. It panics if i is out of range.
func (tg *TabGroup) Tab(i int) *Page {
	return tg.tabs[i]
}

// Len returns the number of tabs. It is nil-safe.
func (tg *TabGroup) Len() int {
	if tg == nil {
		return 0
	}
	return len(tg.tabs)
}

// Close closes all tabs in the group. It is nil-safe and idempotent.
func (tg *TabGroup) Close() error {
	if tg == nil {
		return nil
	}
	tg.mu.Lock()
	defer tg.mu.Unlock()

	if tg.closed {
		return nil
	}
	tg.closed = true

	var firstErr error
	for _, tab := range tg.tabs {
		if err := tab.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
