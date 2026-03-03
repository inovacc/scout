package engine

import (
	"context"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/utils"
)

type (
	timeoutContextKey struct{}
	timeoutContextVal struct {
		parent context.Context //nolint:containedctx // internalized rod pattern
		cancel context.CancelFunc
	}
)

// Context returns a clone with the specified ctx for chained sub-operations.
func (b *rodBrowser) Context(ctx context.Context) *rodBrowser {
	newObj := *b
	newObj.ctx = ctx

	return &newObj
}

// GetContext of current instance.
func (b *rodBrowser) GetContext() context.Context {
	return b.ctx
}

// Timeout returns a clone with the specified total timeout of all chained sub-operations.
func (b *rodBrowser) Timeout(d time.Duration) *rodBrowser {
	ctx, cancel := context.WithTimeout(b.ctx, d)
	return b.Context(context.WithValue(ctx, timeoutContextKey{}, &timeoutContextVal{b.ctx, cancel}))
}

// CancelTimeout cancels the current timeout context and returns a clone with the parent context.
func (b *rodBrowser) CancelTimeout() *rodBrowser {
	val := b.ctx.Value(timeoutContextKey{}).(*timeoutContextVal) //nolint:forcetypeassert
	val.cancel()

	return b.Context(val.parent)
}

// WithCancel returns a clone with a context cancel function.
func (b *rodBrowser) WithCancel() (*rodBrowser, func()) {
	ctx, cancel := context.WithCancel(b.ctx)
	return b.Context(ctx), cancel
}

// Sleeper returns a clone with the specified sleeper for chained sub-operations.
func (b *rodBrowser) Sleeper(sleeper func() utils.Sleeper) *rodBrowser {
	newObj := *b
	newObj.sleeper = sleeper

	return &newObj
}

// Context returns a clone with the specified ctx for chained sub-operations.
func (p *rodPage) Context(ctx context.Context) *rodPage {
	p.helpersLock.Lock()
	newObj := *p
	p.helpersLock.Unlock()

	newObj.ctx = ctx

	return &newObj
}

// GetContext of current instance.
func (p *rodPage) GetContext() context.Context {
	return p.ctx
}

// Timeout returns a clone with the specified total timeout of all chained sub-operations.
func (p *rodPage) Timeout(d time.Duration) *rodPage {
	ctx, cancel := context.WithTimeout(p.ctx, d)
	return p.Context(context.WithValue(ctx, timeoutContextKey{}, &timeoutContextVal{p.ctx, cancel}))
}

// CancelTimeout cancels the current timeout context and returns a clone with the parent context.
func (p *rodPage) CancelTimeout() *rodPage {
	val := p.ctx.Value(timeoutContextKey{}).(*timeoutContextVal) //nolint: forcetypeassert
	val.cancel()

	return p.Context(val.parent)
}

// WithCancel returns a clone with a context cancel function.
func (p *rodPage) WithCancel() (*rodPage, func()) {
	ctx, cancel := context.WithCancel(p.ctx)
	return p.Context(ctx), cancel
}

// Sleeper returns a clone with the specified sleeper for chained sub-operations.
func (p *rodPage) Sleeper(sleeper func() utils.Sleeper) *rodPage {
	newObj := *p
	newObj.sleeper = sleeper

	return &newObj
}

// Context returns a clone with the specified ctx for chained sub-operations.
func (el *rodElement) Context(ctx context.Context) *rodElement {
	newObj := *el
	newObj.ctx = ctx

	return &newObj
}

// GetContext of current instance.
func (el *rodElement) GetContext() context.Context {
	return el.ctx
}

// Timeout returns a clone with the specified total timeout of all chained sub-operations.
func (el *rodElement) Timeout(d time.Duration) *rodElement {
	ctx, cancel := context.WithTimeout(el.ctx, d)
	return el.Context(context.WithValue(ctx, timeoutContextKey{}, &timeoutContextVal{el.ctx, cancel}))
}

// CancelTimeout cancels the current timeout context and returns a clone with the parent context.
func (el *rodElement) CancelTimeout() *rodElement {
	val := el.ctx.Value(timeoutContextKey{}).(*timeoutContextVal) //nolint: forcetypeassert
	val.cancel()

	return el.Context(val.parent)
}

// WithCancel returns a clone with a context cancel function.
func (el *rodElement) WithCancel() (*rodElement, func()) {
	ctx, cancel := context.WithCancel(el.ctx)
	return el.Context(ctx), cancel
}

// Sleeper returns a clone with the specified sleeper for chained sub-operations.
func (el *rodElement) Sleeper(sleeper func() utils.Sleeper) *rodElement {
	newObj := *el
	newObj.sleeper = sleeper

	return &newObj
}
