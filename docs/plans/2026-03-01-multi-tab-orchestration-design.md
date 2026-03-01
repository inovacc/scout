# Multi-Tab Orchestration Design

## Overview

Add a `TabGroup` type to `pkg/scout/` that manages N browser tabs with shared state and coordination primitives. Supports sequential handoff, parallel scraping, and cross-tab sync.

## Core Type

```go
type TabGroup struct {
    browser *Browser
    tabs    []*Page
    store   sync.Map
    mu      sync.Mutex
    limiter *rate.Limiter // optional
    timeout time.Duration // per-action timeout
}
```

## API Surface

| Method | Purpose |
|--------|---------|
| `browser.NewTabGroup(n, opts...)` | Create N tabs |
| `tg.Tab(i) *Page` | Access tab by index |
| `tg.Len() int` | Tab count |
| `tg.Store` | `sync.Map` shared state |
| `tg.Do(i, fn) error` | Action on tab i |
| `tg.DoAll(fn) error` | Sequential on all tabs |
| `tg.DoParallel(fn) []error` | Parallel on all tabs |
| `tg.Broadcast(fn) []error` | Same action on every tab |
| `TabGroupCollect[T](tg, fn) ([]T, []error)` | Generic extract from all |
| `tg.Wait(i, cond, timeout) error` | Wait for condition |
| `tg.Navigate(urls...) []error` | Navigate tabs in parallel |
| `tg.Close() error` | Close all (idempotent) |

## Options

- `WithTabGroupRateLimit(rps float64)` — rate limit across tabs
- `WithTabGroupTimeout(d time.Duration)` — per-action timeout

## Files

- `pkg/scout/tabgroup.go` — implementation
- `pkg/scout/tabgroup_test.go` — tests

## Design Decisions

- Fixed tab count at creation (YAGNI)
- `sync.Map` for shared state
- `Collect` as package-level generic function (matches pagination pattern)
- Nil-safe `Close()` per project convention
- No channel-based API
