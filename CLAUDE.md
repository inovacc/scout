# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scout is a Go library (`github.com/inovacc/scout`) that wraps [go-rod](https://github.com/go-rod/rod) to provide a simplified, Go-idiomatic API for headless browser automation, web scraping, search, and crawling. It is a **library package** (not a binary) — there is no `main` package or `cmd/` directory.

## Build & Test Commands

Uses Taskfile (`task`). Key commands:

```bash
task test              # Run all tests with -race and coverage
task test:unit         # Run tests with -short flag (skip integration)
task check             # Full quality check: fmt → vet → lint → test
task lint              # golangci-lint run ./...
task lint:fix          # golangci-lint run --fix ./...
task fmt               # go fmt + goimports
task vet               # go vet ./...
```

Run a single test:
```bash
go test -v -run TestName ./...
```

Tests require a Chromium browser available on the system. `newTestBrowser` calls `t.Skipf` if the browser is unavailable, so tests skip gracefully in environments without a browser.

## Architecture

All code is in package `scout` (flat, single-package library). The types are:

### Core Types (rod wrappers)

| Type | Wraps | File |
|------|-------|------|
| `Browser` | `*rod.Browser` | `browser.go` |
| `Page` | `*rod.Page` | `page.go` |
| `Element` | `*rod.Element` | `element.go` |
| `EvalResult` | JS eval results | `eval.go` |
| `HijackRouter`, `HijackContext` | rod hijack types | `network.go` |

### Scraping Toolkit Types

| Type | Purpose | File |
|------|---------|------|
| `TableData`, `MetaData` | Extraction results | `extract.go` |
| `Form`, `FormField`, `FormWizard` | Form interaction | `form.go` |
| `RateLimiter` | Rate limiting + retry | `ratelimit.go` |
| `PaginateByClick/URL/Scroll/LoadMore` | Generic pagination | `paginate.go` |
| `SearchResults`, `SearchResult` | SERP parsing | `search.go` |
| `CrawlResult`, `SitemapURL` | Web crawling | `crawl.go` |

**Functional options pattern** for configuration: `New(opts ...Option)` with `With*()` functions in `option.go`. Each feature area has its own options (`ExtractOption`, `SearchOption`, `PaginateOption`, `CrawlOption`, `RateLimitOption`). Defaults: headless=true, 1920x1080, 30s timeout.

**Escape hatches**: `RodPage()` and `RodElement()` expose the underlying rod instances when the wrapper API is insufficient.

## Conventions

- **Error wrapping**: All errors use `fmt.Errorf("scout: action: %w", err)` — consistent `scout:` prefix.
- **Nil-safety**: `Browser.Close()` and key methods are nil-safe and idempotent. Methods guard with `if b == nil || b.browser == nil`.
- **Cleanup patterns**: `SetHeaders()` and `EvalOnNewDocument()` return cleanup functions. `HijackRouter` has `Run()` (blocking, use in goroutine) and `Stop()`.
- **Struct tags**: `scout:"selector"` or `scout:"selector@attr"` for extraction; `form:"field_name"` for form filling.
- **Generics**: Pagination functions use type parameters (`PaginateByClick[T]`) — package-level functions because Go methods can't have type params.
- **Nolint**: `Element.Interactable()` uses `//nolint:nilerr`; `RateLimiter.calculateBackoff` uses `//nolint:gosec` for jitter rand.

## Testing

- `testutil_test.go` provides `newTestServer()` and `newTestBrowser(t)` (headless, no-sandbox, auto-cleanup).
- Route registration: test files call `registerTestRoutes(fn)` in `init()` to add httptest routes. The `newTestServer()` function collects all registered routes.
- Core routes: `/`, `/page2`, `/json`, `/echo-headers`, `/set-cookie`, `/redirect`, `/slow`
- Extract routes: `/extract`, `/table`, `/meta`, `/links`, `/nested`, `/products-list`
- Form routes: `/form`, `/form-csrf`, `/wizard-step1`, `/wizard-step2`, `/submit`
- Paginate routes: `/products-page{1,2,3}`, `/api/products`, `/infinite`, `/load-more`
- Search routes: `/serp-google`, `/serp-google-page2`, `/serp-bing`, `/serp-ddg`
- Crawl routes: `/crawl-start`, `/crawl-page{1,2,3}`, `/sitemap.xml`
- Tests use `t.Skipf` when browser is unavailable — they will not fail in headless CI without Chrome, they skip.
- No mocking framework; tests run against a real headless browser and local HTTP test server.

## Dependencies

- `github.com/go-rod/rod` — core browser automation via Chrome DevTools Protocol
- `github.com/go-rod/stealth` — anti-bot-detection page creation (enabled via `WithStealth()`)
- `github.com/ysmood/gson` — JSON number handling for `EvalResult`
- `golang.org/x/time/rate` — token bucket rate limiter for `RateLimiter`

## CI

GitHub Actions (`.github/workflows/test.yml`) uses reusable `inovacc/workflows` — runs tests, lint, and vulnerability checks on push/PR to non-main branches.
