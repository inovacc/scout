# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scout is a Go library (`github.com/inovacc/scout`) that wraps [go-rod](https://github.com/go-rod/rod) to provide a simplified, Go-idiomatic API for headless browser automation, web scraping, search, and crawling. The core library is in the root `package scout`. A gRPC service layer (`grpc/`) and command binaries (`cmd/`) provide remote browser control with forensic capture.

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
task proto             # Generate gRPC protobuf code
task grpc:server       # Run the gRPC server
task grpc:client       # Run the interactive CLI client
task grpc:workflow     # Run the example workflow demo
task grpc:build        # Build server/client/workflow binaries to bin/
```

Run a single test:
```bash
go test -v -run TestName ./...
```

Tests require a Chromium browser available on the system. `newTestBrowser` calls `t.Skipf` if the browser is unavailable, so tests skip gracefully in environments without a browser.

## Architecture

Library code is in `package scout` (flat, single-package). The gRPC layer is in `grpc/` subtree. Command binaries are in `cmd/`.

### Core Types (rod wrappers)

| Type | Wraps | File |
|------|-------|------|
| `Browser` | `*rod.Browser` | `browser.go` |
| `Page` | `*rod.Page` | `page.go` |
| `Element` | `*rod.Element` | `element.go` |
| `EvalResult` | JS eval results | `eval.go` |
| `HijackRouter`, `HijackContext` | rod hijack types | `network.go` |
| `WindowState`, `WindowBounds` | Window state control | `window.go` |
| `NetworkRecorder` | HAR 1.2 traffic capture | `recorder.go` |

### HAR Recording Types

| Type | Purpose | File |
|------|---------|------|
| `NetworkRecorder` | Captures HTTP traffic via CDP, exports HAR | `recorder.go` |
| `HARLog`, `HAREntry`, `HARRequest`, `HARResponse` | HAR 1.2 data model | `recorder.go` |
| `HARHeader`, `HARContent`, `HARTimings`, `HARCreator` | HAR sub-types | `recorder.go` |
| `RecorderOption` | Functional options (`WithCaptureBody`, `WithCreatorName`) | `recorder.go` |

### Scraping Toolkit Types

| Type | Purpose | File |
|------|---------|------|
| `TableData`, `MetaData` | Extraction results | `extract.go` |
| `Form`, `FormField`, `FormWizard` | Form interaction | `form.go` |
| `RateLimiter` | Rate limiting + retry | `ratelimit.go` |
| `PaginateByClick/URL/Scroll/LoadMore` | Generic pagination | `paginate.go` |
| `SearchResults`, `SearchResult` | SERP parsing | `search.go` |
| `CrawlResult`, `SitemapURL` | Web crawling | `crawl.go` |
| `storageAPI`, `SessionState` | Web storage & session persistence | `storage.go` |

### gRPC Service Layer

```
grpc/
  proto/scout.proto        # Protocol buffer definitions (ScoutService)
  scoutpb/                 # Generated Go code (committed for consumer convenience)
  server/server.go         # gRPC service implementation (ScoutServer)
```

| Type | Purpose | File |
|------|---------|------|
| `ScoutServer` | Multi-session gRPC service | `grpc/server/server.go` |
| `ScoutService` (proto) | 25+ RPCs: session, nav, interact, capture, record, stream | `grpc/proto/scout.proto` |

### Command Binaries

| Binary | Purpose | Path |
|--------|---------|------|
| `scout-server` | gRPC server with reflection, graceful shutdown | `cmd/server/` |
| `scout-client` | Interactive CLI (nav, click, type, key, eval, shot, har) | `cmd/client/` |
| `scout-workflow` | Bidirectional streaming demo (form automation + HAR) | `cmd/example-workflow/` |

### Examples

`examples/` contains 18 standalone runnable programs (not part of the library build due to `_` prefix):
- `examples/simple/` — 8 examples: navigation, screenshots, extraction, JS eval, forms, cookies
- `examples/advanced/` — 10 examples: search, pagination, crawling, rate limiting, hijacking, stealth, PDF, HAR recording
- Each is a separate `package main` with its own implicit dependency on the parent module
- Build individually: `cd examples/simple/basic-navigation && go build .`

**Functional options pattern** for configuration: `New(opts ...Option)` with `With*()` functions in `option.go`. Each feature area has its own options (`ExtractOption`, `SearchOption`, `PaginateOption`, `CrawlOption`, `RateLimitOption`). Defaults: headless=true, 1920x1080, 30s timeout.

**Escape hatches**: `RodPage()` and `RodElement()` expose the underlying rod instances when the wrapper API is insufficient.

## Conventions

- **WaitLoad**: `NewPage()` does not wait for DOM load. Call `page.WaitLoad()` before `Extract()`, `ExtractMeta()`, `PDF()`, etc. when targeting external sites.
- **extractAll[T] (pagination)**: Finds first `scout:` tag match, walks to `parentElement`, extracts remaining fields within that parent. All struct fields must be resolvable within the parent of the first field's match.
- **Error wrapping**: All errors use `fmt.Errorf("scout: action: %w", err)` — consistent `scout:` prefix.
- **Nil-safety**: `Browser.Close()` and key methods are nil-safe and idempotent. Methods guard with `if b == nil || b.browser == nil`.
- **Cleanup patterns**: `SetHeaders()` and `EvalOnNewDocument()` return cleanup functions. `HijackRouter` has `Run()` (blocking, use in goroutine) and `Stop()`.
- **Struct tags**: `scout:"selector"` or `scout:"selector@attr"` for extraction; `form:"field_name"` for form filling.
- **Generics**: Pagination functions use type parameters (`PaginateByClick[T]`) — package-level functions because Go methods can't have type params.
- **Nolint**: `Element.Interactable()` uses `//nolint:nilerr`; `RateLimiter.calculateBackoff` uses `//nolint:gosec` for jitter rand.
- **Platform-specific options**: `WithXvfb()` lives in `option_unix.go` (`//go:build !windows`). The `xvfb`/`xvfbArgs` fields compile on all platforms but the option function is only available on Unix.
- **Window state transitions**: Chrome requires restoring to `normal` before changing between non-normal states. `setWindowState()` handles this automatically.
- **NetworkRecorder**: Attach to a page, records all HTTP traffic via CDP events. `Stop()` is nil-safe and idempotent. `ExportHAR()` produces HAR 1.2 JSON. `Clear()` resets entries.
- **Page keyboard methods**: `KeyPress(key)` and `KeyType(keys...)` operate at the page level (not element-scoped). Used by the gRPC server for `PressKey` RPC.

## Testing

- `testutil_test.go` provides `newTestServer()` and `newTestBrowser(t)` (headless, no-sandbox, auto-cleanup).
- Route registration: test files call `registerTestRoutes(fn)` in `init()` to add httptest routes. The `newTestServer()` function collects all registered routes.
- Core routes: `/`, `/page2`, `/json`, `/echo-headers`, `/set-cookie`, `/redirect`, `/slow`
- Extract routes: `/extract`, `/table`, `/meta`, `/links`, `/nested`, `/products-list`
- Form routes: `/form`, `/form-csrf`, `/wizard-step1`, `/wizard-step2`, `/submit`
- Paginate routes: `/products-page{1,2,3}`, `/api/products`, `/infinite`, `/load-more`
- Search routes: `/serp-google`, `/serp-google-page2`, `/serp-bing`, `/serp-ddg`
- Crawl routes: `/crawl-start`, `/crawl-page{1,2,3}`, `/sitemap.xml`
- Recorder routes: `/recorder-page`, `/recorder-asset`, `/recorder-api`
- Window tests: no routes needed — window control operates on the browser window itself
- Tests use `t.Skipf` when browser is unavailable — they will not fail in headless CI without Chrome, they skip.
- No mocking framework; tests run against a real headless browser and local HTTP test server.

## Dependencies

### Core library (package scout)
- `github.com/go-rod/rod` — core browser automation via Chrome DevTools Protocol
- `github.com/go-rod/stealth` — anti-bot-detection page creation (enabled via `WithStealth()`)
- `github.com/ysmood/gson` — JSON number handling for `EvalResult`
- `golang.org/x/time/rate` — token bucket rate limiter for `RateLimiter`

### gRPC layer (grpc/ and cmd/)
- `google.golang.org/grpc` — gRPC framework
- `google.golang.org/protobuf` — Protocol Buffers runtime
- `github.com/google/uuid` — session ID generation

Note: The core library does NOT import gRPC. Library-only consumers pull zero gRPC dependencies.

## CI

GitHub Actions (`.github/workflows/test.yml`) uses reusable `inovacc/workflows` — runs tests, lint, and vulnerability checks on push/PR to non-main branches.
