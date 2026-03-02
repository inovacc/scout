package scout

import (
	"context"
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

	for i := range n {
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

// Do executes fn on the i-th tab. It returns an error if i is out of range.
func (tg *TabGroup) Do(i int, fn func(*Page) error) error {
	if i < 0 || i >= len(tg.tabs) {
		return fmt.Errorf("scout: tab %d: index out of range [0, %d)", i, len(tg.tabs))
	}

	if tg.limiter != nil {
		if err := tg.limiter.Wait(context.Background()); err != nil {
			return fmt.Errorf("scout: tab %d: rate limiter: %w", i, err)
		}
	}

	if err := fn(tg.tabs[i]); err != nil {
		return fmt.Errorf("scout: tab %d: %w", i, err)
	}

	return nil
}

// DoAll executes fn sequentially on all tabs, stopping on the first error.
func (tg *TabGroup) DoAll(fn func(i int, p *Page) error) error {
	for i, p := range tg.tabs {
		if tg.limiter != nil {
			if err := tg.limiter.Wait(context.Background()); err != nil {
				return fmt.Errorf("scout: tab %d: rate limiter: %w", i, err)
			}
		}

		if err := fn(i, p); err != nil {
			return fmt.Errorf("scout: tab %d: %w", i, err)
		}
	}

	return nil
}

// DoParallel executes fn concurrently on all tabs. It returns a slice of errors
// (nil entries for successful tabs).
func (tg *TabGroup) DoParallel(fn func(i int, p *Page) error) []error {
	errs := make([]error, len(tg.tabs))

	var wg sync.WaitGroup
	for i, p := range tg.tabs {
		wg.Add(1)

		go func(i int, p *Page) {
			defer wg.Done()

			if tg.limiter != nil {
				if err := tg.limiter.Wait(context.Background()); err != nil {
					errs[i] = fmt.Errorf("scout: tab %d: rate limiter: %w", i, err)
					return
				}
			}

			if err := fn(i, p); err != nil {
				errs[i] = fmt.Errorf("scout: tab %d: %w", i, err)
			}
		}(i, p)
	}

	wg.Wait()

	return errs
}

// Broadcast executes fn concurrently on all tabs. It is a convenience wrapper
// around DoParallel that adapts a single-page function.
func (tg *TabGroup) Broadcast(fn func(*Page) error) []error {
	return tg.DoParallel(func(_ int, p *Page) error {
		return fn(p)
	})
}

// Navigate navigates tab[i] to urls[i] in parallel. If len(urls) != len(tabs),
// all error slots are filled with a mismatch error.
func (tg *TabGroup) Navigate(urls ...string) []error {
	errs := make([]error, len(tg.tabs))
	if len(urls) != len(tg.tabs) {
		mismatch := fmt.Errorf("scout: tab group: %d urls for %d tabs", len(urls), len(tg.tabs))
		for i := range errs {
			errs[i] = mismatch
		}

		return errs
	}

	var wg sync.WaitGroup
	for i, p := range tg.tabs {
		wg.Add(1)

		go func(i int, p *Page, url string) {
			defer wg.Done()

			if tg.limiter != nil {
				if err := tg.limiter.Wait(context.Background()); err != nil {
					errs[i] = fmt.Errorf("scout: tab %d: rate limiter: %w", i, err)
					return
				}
			}

			if err := p.Navigate(url); err != nil {
				errs[i] = fmt.Errorf("scout: tab %d: navigate: %w", i, err)
			}
		}(i, p, urls[i])
	}

	wg.Wait()

	return errs
}

// Wait polls every 50ms until cond returns true for tab i, or timeout elapses.
func (tg *TabGroup) Wait(i int, cond func(*Page) bool, timeout time.Duration) error {
	if i < 0 || i >= len(tg.tabs) {
		return fmt.Errorf("scout: tab %d: index out of range [0, %d)", i, len(tg.tabs))
	}

	deadline := time.After(timeout)

	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()

	for {
		if cond(tg.tabs[i]) {
			return nil
		}

		select {
		case <-deadline:
			return fmt.Errorf("scout: tab %d: wait timed out after %v", i, timeout)
		case <-tick.C:
		}
	}
}

// TabGroupCollect extracts T from each tab in parallel using fn.
func TabGroupCollect[T any](tg *TabGroup, fn func(*Page) (T, error)) ([]T, []error) {
	results := make([]T, len(tg.tabs))
	errs := make([]error, len(tg.tabs))

	var wg sync.WaitGroup
	for i, p := range tg.tabs {
		wg.Add(1)

		go func(i int, p *Page) {
			defer wg.Done()

			if tg.limiter != nil {
				if err := tg.limiter.Wait(context.Background()); err != nil {
					errs[i] = fmt.Errorf("scout: tab %d: rate limiter: %w", i, err)
					return
				}
			}

			val, err := fn(p)
			if err != nil {
				errs[i] = fmt.Errorf("scout: tab %d: collect: %w", i, err)
				return
			}

			results[i] = val
		}(i, p)
	}

	wg.Wait()

	return results, errs
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
