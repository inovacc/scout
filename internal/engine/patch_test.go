package engine

import (
	"context"
	"sync"
	"testing"
)

// TestPageContextPropagation verifies that the page context is properly
// propagated through the rodPage type's context chain.
func TestPageContextPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	b := newRodBrowser()
	p := &rodPage{
		ctx:         ctx,
		browser:     b,
		helpersLock: &sync.Mutex{},
	}

	// Verify GetContext returns the cancelled context.
	if p.GetContext().Err() != context.Canceled {
		t.Fatal("expected page context to be cancelled")
	}

	// Verify that Browser.Context(p.ctx) propagates the page's context.
	bClone := p.browser.Context(p.ctx)
	if bClone.GetContext().Err() != context.Canceled {
		t.Fatal("browser clone should inherit page's cancelled context")
	}
}

// TestPageContextClone verifies that rodPage.Context() creates a proper clone
// with the new context without affecting the original.
func TestPageContextClone(t *testing.T) {
	ctx1 := context.Background()
	ctx2 := t.Context()

	p := &rodPage{
		ctx:         ctx1,
		helpersLock: &sync.Mutex{},
	}

	clone := p.Context(ctx2)
	if clone.GetContext() != ctx2 {
		t.Fatal("clone should have new context")
	}

	if p.GetContext() != ctx1 {
		t.Fatal("original should keep old context")
	}
}

// TestPageWithCancel verifies WithCancel creates a cancellable page clone.
func TestPageWithCancel(t *testing.T) {
	p := &rodPage{
		ctx:         context.Background(),
		helpersLock: &sync.Mutex{},
	}

	clone, cancel := p.WithCancel()
	defer cancel()

	if clone.GetContext().Err() != nil {
		t.Fatal("clone context should not be cancelled yet")
	}

	cancel()

	if clone.GetContext().Err() != context.Canceled {
		t.Fatal("clone context should be cancelled after cancel()")
	}

	if p.GetContext().Err() != nil {
		t.Fatal("original context should not be affected")
	}
}

// TestBrowserCloseNilClient verifies rodBrowser.Close behavior with nil client.
func TestBrowserCloseNilClient(t *testing.T) {
	b := newRodBrowser()

	panicked := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		_ = b.Close()
	}()

	if !panicked {
		t.Fatal("expected panic from Close without Connect (nil client)")
	}
}

// TestPageInfoContextPath verifies Info() uses p.browser.Context(p.ctx)
// pattern for context propagation.
func TestPageInfoContextPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := newRodBrowser()
	p := &rodPage{
		ctx:     ctx,
		browser: b,
	}

	bClone := p.browser.Context(p.ctx)
	if bClone.GetContext() != ctx {
		t.Fatal("Info path: browser clone should use page context")
	}

	if bClone.GetContext().Err() != context.Canceled {
		t.Fatal("Info path: browser clone context should be cancelled")
	}
}

// TestPageActivateContextPath verifies Activate() uses p.browser.Context(p.ctx).
func TestPageActivateContextPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := newRodBrowser()
	p := &rodPage{
		ctx:     ctx,
		browser: b,
	}

	bClone := p.browser.Context(p.ctx)
	if bClone.GetContext().Err() != context.Canceled {
		t.Fatal("Activate path: browser clone context should be cancelled")
	}
}

// TestPageTriggerFaviconContextPath verifies TriggerFavicon() uses
// p.browser.Context(p.ctx).
func TestPageTriggerFaviconContextPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := newRodBrowser()
	p := &rodPage{
		ctx:     ctx,
		browser: b,
	}

	bClone := p.browser.Context(p.ctx)
	if bClone.GetContext().Err() != context.Canceled {
		t.Fatal("TriggerFavicon path: browser clone context should be cancelled")
	}
}

// TestBrowserContextClone verifies rodBrowser.Context creates isolated clones.
func TestBrowserContextClone(t *testing.T) {
	b := newRodBrowser()

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	ctx2 := t.Context()

	b1 := b.Context(ctx1)
	b2 := b.Context(ctx2)

	if b1.GetContext() != ctx1 {
		t.Fatal("b1 should have ctx1")
	}

	if b2.GetContext() != ctx2 {
		t.Fatal("b2 should have ctx2")
	}

	cancel1()

	if b1.GetContext().Err() != context.Canceled {
		t.Fatal("b1 context should be cancelled")
	}

	if b2.GetContext().Err() != nil {
		t.Fatal("b2 context should not be cancelled")
	}
}
