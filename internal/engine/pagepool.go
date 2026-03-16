package engine

import (
	"context"
	"fmt"
	"sync"
)

// ManagedPagePool manages a fixed-size pool of browser pages for concurrent scraping.
// Unlike the low-level Pool[Page] from utils.go, ManagedPagePool pre-creates real
// browser pages and handles lifecycle (state reset on release, cleanup on close).
type ManagedPagePool struct {
	browser *Browser
	pool    chan *Page
	size    int

	mu     sync.Mutex
	closed bool
}

// NewManagedPagePool creates a pool of reusable browser pages. It pre-creates size
// pages so they are ready for immediate use. Returns an error if any page
// fails to be created; already-created pages are cleaned up on failure.
func NewManagedPagePool(browser *Browser, size int) (*ManagedPagePool, error) {
	if browser == nil {
		return nil, fmt.Errorf("scout: page pool: browser is nil")
	}

	if size < 1 {
		return nil, fmt.Errorf("scout: page pool: size must be >= 1, got %d", size)
	}

	pp := &ManagedPagePool{
		browser: browser,
		pool:    make(chan *Page, size),
		size:    size,
	}

	for i := 0; i < size; i++ {
		page, err := browser.NewPage("")
		if err != nil {
			// Clean up pages created so far.
			pp.Close()

			return nil, fmt.Errorf("scout: page pool: create page %d: %w", i, err)
		}

		pp.pool <- page
	}

	return pp, nil
}

// Acquire retrieves a page from the pool. It blocks until a page becomes
// available or the context is cancelled.
func (pp *ManagedPagePool) Acquire(ctx context.Context) (*Page, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("scout: page pool: acquire: %w", ctx.Err())
	case page, ok := <-pp.pool:
		if !ok {
			return nil, fmt.Errorf("scout: page pool: pool is closed")
		}

		return page, nil
	}
}

// Release returns a page to the pool after navigating it to about:blank
// to reset its state.
func (pp *ManagedPagePool) Release(page *Page) {
	if page == nil {
		return
	}

	pp.mu.Lock()
	if pp.closed {
		pp.mu.Unlock()
		_ = page.Close()

		return
	}
	pp.mu.Unlock()

	// Navigate to about:blank to reset page state.
	_ = page.Navigate("about:blank")

	pp.pool <- page
}

// Close closes all pages currently in the pool and marks the pool as closed.
// Pages that are currently acquired will be closed when they are released.
func (pp *ManagedPagePool) Close() {
	pp.mu.Lock()
	if pp.closed {
		pp.mu.Unlock()
		return
	}

	pp.closed = true
	pp.mu.Unlock()

	close(pp.pool)

	for page := range pp.pool {
		_ = page.Close()
	}
}

// Size returns the total number of pages the pool was created with.
func (pp *ManagedPagePool) Size() int {
	return pp.size
}

// Available returns the number of pages currently available in the pool.
func (pp *ManagedPagePool) Available() int {
	return len(pp.pool)
}

// NewManagedPagePool is a convenience method that creates a ManagedPagePool from this browser.
func (b *Browser) NewManagedPagePool(size int) (*ManagedPagePool, error) {
	return NewManagedPagePool(b, size)
}
