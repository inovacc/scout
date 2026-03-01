# Multi-Tab Orchestration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `TabGroup` type to `pkg/scout/` for coordinating actions across multiple browser tabs with shared state and sync primitives.

**Architecture:** `TabGroup` wraps N `*Page` instances created from a single `*Browser`. Methods provide sequential, parallel, and broadcast execution patterns. A `sync.Map` field enables cross-tab state sharing. Generic `TabGroupCollect[T]` follows the existing `PaginateByClick[T]` pattern.

**Tech Stack:** Go stdlib (`sync`, `time`, `context`), `x/time/rate` (already a dependency)

---

### Task 1: TabGroup struct, options, and constructor

**Files:**
- Create: `pkg/scout/tabgroup.go`
- Modify: `pkg/scout/browser.go` (add `NewTabGroup` method)
- Test: `pkg/scout/tabgroup_test.go`

**Step 1: Write the failing test**

Create `pkg/scout/tabgroup_test.go`:

```go
package scout

import (
	"testing"
)

func TestNewTabGroup(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(3)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	if tg.Len() != 3 {
		t.Errorf("Len() = %d, want 3", tg.Len())
	}

	for i := 0; i < 3; i++ {
		if tg.Tab(i) == nil {
			t.Errorf("Tab(%d) is nil", i)
		}
	}
}

func TestNewTabGroupZero(t *testing.T) {
	b := newTestBrowser(t)

	_, err := b.NewTabGroup(0)
	if err == nil {
		t.Error("NewTabGroup(0) should return error")
	}
}

func TestTabGroupNilBrowser(t *testing.T) {
	var b *Browser
	_, err := b.NewTabGroup(1)
	if err == nil {
		t.Error("nil browser should return error")
	}
}

func TestTabGroupOptions(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1,
		WithTabGroupRateLimit(10),
		WithTabGroupTimeout(5e9),
	)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	if tg.limiter == nil {
		t.Error("limiter should be set")
	}
	if tg.timeout != 5e9 {
		t.Errorf("timeout = %v, want 5s", tg.timeout)
	}
}

func TestTabGroupCloseIdempotent(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}

	if err := tg.Close(); err != nil {
		t.Errorf("first Close() error: %v", err)
	}
	if err := tg.Close(); err != nil {
		t.Errorf("second Close() error: %v", err)
	}

	// Nil-safe
	var nilTG *TabGroup
	if err := nilTG.Close(); err != nil {
		t.Errorf("nil Close() error: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/ -run TestNewTabGroup -v -count=1`
Expected: FAIL — `NewTabGroup` not defined

**Step 3: Write implementation**

Create `pkg/scout/tabgroup.go`:

```go
package scout

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// TabGroup manages N browser tabs with shared state and coordination.
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

// WithTabGroupRateLimit sets a shared rate limiter across all tabs.
func WithTabGroupRateLimit(rps float64) TabGroupOption {
	return func(tg *TabGroup) {
		tg.limiter = rate.NewLimiter(rate.Limit(rps), 1)
	}
}

// WithTabGroupTimeout sets the per-action timeout.
func WithTabGroupTimeout(d time.Duration) TabGroupOption {
	return func(tg *TabGroup) {
		tg.timeout = d
	}
}

// NewTabGroup creates a TabGroup with n blank tabs.
func (b *Browser) NewTabGroup(n int, opts ...TabGroupOption) (*TabGroup, error) {
	if b == nil || b.browser == nil {
		return nil, fmt.Errorf("scout: browser is nil")
	}
	if n < 1 {
		return nil, fmt.Errorf("scout: tab group requires at least 1 tab")
	}

	tg := &TabGroup{browser: b}
	for _, o := range opts {
		o(tg)
	}

	tg.tabs = make([]*Page, n)
	for i := 0; i < n; i++ {
		p, err := b.NewPage("about:blank")
		if err != nil {
			// Close any already-opened tabs.
			for j := 0; j < i; j++ {
				_ = tg.tabs[j].Close()
			}
			return nil, fmt.Errorf("scout: tab group: create tab %d: %w", i, err)
		}
		tg.tabs[i] = p
	}

	return tg, nil
}

// Tab returns the page at index i. Panics if i is out of range.
func (tg *TabGroup) Tab(i int) *Page {
	return tg.tabs[i]
}

// Len returns the number of tabs.
func (tg *TabGroup) Len() int {
	if tg == nil {
		return 0
	}
	return len(tg.tabs)
}

// Close closes all tabs. Nil-safe and idempotent.
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
	for _, p := range tg.tabs {
		_ = p.Close()
	}
	return nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/scout/ -run TestNewTabGroup -v -count=1 -timeout=120s`
Run: `go test ./pkg/scout/ -run TestTabGroup -v -count=1 -timeout=120s`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/scout/tabgroup.go pkg/scout/tabgroup_test.go
git commit -m "feat: add TabGroup struct with constructor and options"
```

---

### Task 2: Do, DoAll, DoParallel, Broadcast

**Files:**
- Modify: `pkg/scout/tabgroup.go`
- Modify: `pkg/scout/tabgroup_test.go`

**Step 1: Write the failing tests**

Append to `pkg/scout/tabgroup_test.go`:

```go
func TestTabGroupDo(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	err = tg.Do(0, func(p *Page) error {
		return p.Navigate(srv.URL)
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}

	title, err := tg.Tab(0).Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}
	if title == "" {
		t.Error("Tab(0) should have navigated")
	}
}

func TestTabGroupDoAll(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(3)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	err = tg.DoAll(func(i int, p *Page) error {
		return p.Navigate(srv.URL)
	})
	if err != nil {
		t.Fatalf("DoAll() error: %v", err)
	}

	for i := 0; i < 3; i++ {
		title, _ := tg.Tab(i).Title()
		if title == "" {
			t.Errorf("Tab(%d) title empty after DoAll", i)
		}
	}
}

func TestTabGroupDoParallel(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(3)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	errs := tg.DoParallel(func(i int, p *Page) error {
		return p.Navigate(srv.URL)
	})
	for i, e := range errs {
		if e != nil {
			t.Errorf("DoParallel tab %d error: %v", i, e)
		}
	}
}

func TestTabGroupBroadcast(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	errs := tg.Broadcast(func(p *Page) error {
		return p.Navigate(srv.URL)
	})
	for i, e := range errs {
		if e != nil {
			t.Errorf("Broadcast tab %d error: %v", i, e)
		}
	}
}

func TestTabGroupDoOutOfRange(t *testing.T) {
	b := newTestBrowser(t)
	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	err = tg.Do(5, func(p *Page) error { return nil })
	if err == nil {
		t.Error("Do(5) should return error for out of range")
	}
}
```

**Step 2: Run to verify failure**

Run: `go test ./pkg/scout/ -run TestTabGroupDo -v -count=1`
Expected: FAIL — methods not defined

**Step 3: Implement**

Add to `pkg/scout/tabgroup.go`:

```go
import "sync"

// Do executes fn on tab i.
func (tg *TabGroup) Do(i int, fn func(*Page) error) error {
	if i < 0 || i >= len(tg.tabs) {
		return fmt.Errorf("scout: tab index %d out of range [0, %d)", i, len(tg.tabs))
	}
	if tg.limiter != nil {
		_ = tg.limiter.Wait(context.Background())
	}
	return fn(tg.tabs[i])
}

// DoAll executes fn sequentially on each tab.
func (tg *TabGroup) DoAll(fn func(i int, p *Page) error) error {
	for i, p := range tg.tabs {
		if tg.limiter != nil {
			_ = tg.limiter.Wait(context.Background())
		}
		if err := fn(i, p); err != nil {
			return fmt.Errorf("scout: tab %d: %w", i, err)
		}
	}
	return nil
}

// DoParallel executes fn on all tabs concurrently.
func (tg *TabGroup) DoParallel(fn func(i int, p *Page) error) []error {
	errs := make([]error, len(tg.tabs))
	var wg sync.WaitGroup
	for i, p := range tg.tabs {
		wg.Add(1)
		go func(idx int, page *Page) {
			defer wg.Done()
			if tg.limiter != nil {
				_ = tg.limiter.Wait(context.Background())
			}
			errs[idx] = fn(idx, page)
		}(i, p)
	}
	wg.Wait()
	return errs
}

// Broadcast executes the same fn on every tab concurrently.
func (tg *TabGroup) Broadcast(fn func(*Page) error) []error {
	return tg.DoParallel(func(_ int, p *Page) error {
		return fn(p)
	})
}
```

**Step 4: Run tests**

Run: `go test ./pkg/scout/ -run TestTabGroup -v -count=1 -timeout=120s`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/scout/tabgroup.go pkg/scout/tabgroup_test.go
git commit -m "feat: add Do, DoAll, DoParallel, Broadcast to TabGroup"
```

---

### Task 3: Navigate, Wait, TabGroupCollect

**Files:**
- Modify: `pkg/scout/tabgroup.go`
- Modify: `pkg/scout/tabgroup_test.go`

**Step 1: Write the failing tests**

Append to `pkg/scout/tabgroup_test.go`:

```go
func TestTabGroupNavigate(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	errs := tg.Navigate(srv.URL, srv.URL+"/page2")
	for i, e := range errs {
		if e != nil {
			t.Errorf("Navigate tab %d error: %v", i, e)
		}
	}
}

func TestTabGroupNavigateMismatch(t *testing.T) {
	b := newTestBrowser(t)
	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	errs := tg.Navigate("http://example.com") // 1 URL for 2 tabs
	hasErr := false
	for _, e := range errs {
		if e != nil {
			hasErr = true
		}
	}
	if !hasErr {
		t.Error("Navigate with wrong URL count should produce errors")
	}
}

func TestTabGroupWait(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	_ = tg.Tab(0).Navigate(srv.URL)
	_ = tg.Tab(0).WaitLoad()

	err = tg.Wait(0, func(p *Page) bool {
		title, _ := p.Title()
		return title != ""
	}, 5*time.Second)
	if err != nil {
		t.Errorf("Wait() error: %v", err)
	}
}

func TestTabGroupWaitTimeout(t *testing.T) {
	b := newTestBrowser(t)
	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	err = tg.Wait(0, func(p *Page) bool {
		return false // never true
	}, 100*time.Millisecond)
	if err == nil {
		t.Error("Wait() should timeout")
	}
}

func TestTabGroupCollect(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	_ = tg.Navigate(srv.URL, srv.URL)
	_ = tg.Broadcast(func(p *Page) error { return p.WaitLoad() })

	titles, errs := TabGroupCollect(tg, func(p *Page) (string, error) {
		return p.Title()
	})
	for i, e := range errs {
		if e != nil {
			t.Errorf("Collect tab %d error: %v", i, e)
		}
	}
	if len(titles) != 2 {
		t.Fatalf("Collect() returned %d results, want 2", len(titles))
	}
	for i, title := range titles {
		if title == "" {
			t.Errorf("Collect tab %d title empty", i)
		}
	}
}

func TestTabGroupStore(t *testing.T) {
	b := newTestBrowser(t)
	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup() error: %v", err)
	}
	defer func() { _ = tg.Close() }()

	tg.Store.Store("token", "abc123")
	val, ok := tg.Store.Load("token")
	if !ok || val.(string) != "abc123" {
		t.Error("Store should persist values across tabs")
	}
}
```

**Step 2: Run to verify failure**

Run: `go test ./pkg/scout/ -run TestTabGroupNavigate -v -count=1`
Expected: FAIL

**Step 3: Implement**

Add to `pkg/scout/tabgroup.go`:

```go
import "context"

// Navigate sends each tab to the corresponding URL in parallel.
// len(urls) must equal Len().
func (tg *TabGroup) Navigate(urls ...string) []error {
	errs := make([]error, len(tg.tabs))
	if len(urls) != len(tg.tabs) {
		for i := range errs {
			errs[i] = fmt.Errorf("scout: tab group: %d urls for %d tabs", len(urls), len(tg.tabs))
		}
		return errs
	}
	var wg sync.WaitGroup
	for i, p := range tg.tabs {
		wg.Add(1)
		go func(idx int, page *Page, url string) {
			defer wg.Done()
			if tg.limiter != nil {
				_ = tg.limiter.Wait(context.Background())
			}
			errs[idx] = page.Navigate(url)
		}(i, p, urls[i])
	}
	wg.Wait()
	return errs
}

// Wait polls until cond returns true for tab i, or timeout expires.
func (tg *TabGroup) Wait(i int, cond func(*Page) bool, timeout time.Duration) error {
	if i < 0 || i >= len(tg.tabs) {
		return fmt.Errorf("scout: tab index %d out of range [0, %d)", i, len(tg.tabs))
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

// TabGroupCollect extracts a value from each tab in parallel.
func TabGroupCollect[T any](tg *TabGroup, fn func(*Page) (T, error)) ([]T, []error) {
	results := make([]T, len(tg.tabs))
	errs := make([]error, len(tg.tabs))
	var wg sync.WaitGroup
	for i, p := range tg.tabs {
		wg.Add(1)
		go func(idx int, page *Page) {
			defer wg.Done()
			if tg.limiter != nil {
				_ = tg.limiter.Wait(context.Background())
			}
			results[idx], errs[idx] = fn(page)
		}(i, p)
	}
	wg.Wait()
	return results, errs
}
```

**Step 4: Run tests**

Run: `go test ./pkg/scout/ -run TestTabGroup -v -count=1 -timeout=120s`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/scout/tabgroup.go pkg/scout/tabgroup_test.go
git commit -m "feat: add Navigate, Wait, TabGroupCollect to TabGroup"
```

---

### Task 4: Final vet, build, update BACKLOG

**Files:**
- Modify: `docs/BACKLOG.md`

**Step 1: Full verification**

Run: `go vet ./pkg/scout/`
Run: `go build ./pkg/scout/`
Run: `go test ./pkg/scout/ -run TestTabGroup -v -count=1 -timeout=120s`
Expected: All pass

**Step 2: Update BACKLOG.md**

Mark multi-tab orchestration as done:
```
| ~~Multi-tab orchestration~~ | ~~P3~~ | ~~Large~~ | ~~Done — TabGroup with Do/DoAll/DoParallel/Broadcast/Navigate/Wait/Collect~~ |
```

**Step 3: Commit**

```bash
git add docs/BACKLOG.md
git commit -m "docs: mark multi-tab orchestration done in BACKLOG"
```
