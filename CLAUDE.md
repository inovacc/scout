# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scout is a Go library (`github.com/inovacc/scout/pkg/scout`) that wraps [go-rod](https://github.com/go-rod/rod) to provide a simplified, Go-idiomatic API for headless browser automation, web scraping, search, and crawling. The core library is in `pkg/scout/`. A gRPC service layer (`grpc/`) provides remote browser control. A unified Cobra CLI (`cmd/scout/`) exposes all features as commands with a background daemon for session persistence.

## Build & Test Commands

Uses Taskfile (`task`). Key commands:

```bash
task build             # Build scout CLI binary to bin/
task test              # Run all tests with -race and coverage
task test:unit         # Run tests with -short flag (skip integration)
task check             # Full quality check: fmt → vet → lint → test
task lint              # golangci-lint run ./...
task lint:fix          # golangci-lint run --fix ./...
task fmt               # go fmt + goimports
task vet               # go vet ./...
task proto             # Generate gRPC protobuf code
task grpc:server       # Run the gRPC server via scout CLI
task grpc:client       # Run the interactive CLI client via scout CLI
```

Run a single test:
```bash
go test -v -run TestName ./...
```

Tests require a Chromium browser available on the system. `newTestBrowser` calls `t.Skipf` if the browser is unavailable, so tests skip gracefully in environments without a browser.

## Architecture

```
scout/
├── pkg/scout/          # Core library (package scout)
├── cmd/scout/          # Unified Cobra CLI binary
│   └── internal/cli/   # CLI command implementations
├── grpc/               # gRPC service layer
│   ├── proto/          # Protobuf definitions
│   ├── scoutpb/        # Generated Go code
│   └── server/         # gRPC server implementation
├── firecrawl/          # Firecrawl v2 REST API client (pure HTTP, no browser)
├── scraper/            # Scraper framework + Slack mode
├── examples/           # 18 runnable examples
└── docs/               # Documentation, ADRs, roadmap
```

Library code is in `pkg/scout/` (flat, single-package). Import as `github.com/inovacc/scout/pkg/scout`. The gRPC layer is in `grpc/`. The unified CLI is at `cmd/scout/`.

### Core Types (rod wrappers)

| Type | Wraps | File |
|------|-------|------|
| `Browser` | `*rod.Browser` | `pkg/scout/browser.go` |
| `Page` | `*rod.Page` | `pkg/scout/page.go` |
| `Element` | `*rod.Element` | `pkg/scout/element.go` |
| `EvalResult` | JS eval results | `pkg/scout/eval.go` |
| `HijackRouter`, `HijackContext` | rod hijack types | `pkg/scout/network.go` |
| `WindowState`, `WindowBounds` | Window state control | `pkg/scout/window.go` |
| `NetworkRecorder` | HAR 1.2 traffic capture | `pkg/scout/recorder.go` |

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

### Firecrawl Client

```
firecrawl/
  doc.go                # Package documentation
  client.go             # Client type, New(apiKey, ...Option)
  option.go             # WithAPIURL(), WithTimeout(), WithHTTPClient()
  types.go              # Document, DocumentMetadata, Format constants, job types
  http.go               # Internal HTTP helpers (post, get, delete, handleError)
  error.go              # APIError, AuthError, RateLimitError
  poll.go               # Generic poll[T]() for async jobs
  scrape.go             # Scrape() + ScrapeOption funcs
  crawl.go              # Crawl(), GetCrawlStatus(), WaitForCrawl(), CancelCrawl()
  search.go             # Search() + SearchOption funcs
  map.go                # Map() + MapOption funcs
  batch.go              # BatchScrape(), GetBatchStatus(), WaitForBatch()
  extract.go            # Extract() + ExtractOption funcs
```

| Type | Purpose | File |
|------|---------|------|
| `Client` | Firecrawl API client | `firecrawl/client.go` |
| `Document`, `DocumentMetadata` | Scraped page data | `firecrawl/types.go` |
| `CrawlJob`, `BatchJob` | Async job status | `firecrawl/types.go` |
| `SearchResult`, `MapResult`, `ExtractResult` | Endpoint results | `firecrawl/types.go` |
| `APIError`, `AuthError`, `RateLimitError` | Typed errors | `firecrawl/error.go` |

Pure HTTP client — no dependency on `pkg/scout/` or rod. Import as `github.com/inovacc/scout/firecrawl`. API key from `FIRECRAWL_API_KEY` env or passed directly.

### Scraper Framework

```
scraper/
  scraper.go              # Base types: Credentials, Progress, AuthError, RateLimitError, ExportJSON
  crypto.go               # AES-256-GCM + Argon2id: EncryptData, DecryptData
  slack/
    slack.go              # Scraper struct, Authenticate, ListChannels, GetMessages, etc.
    api.go                # Internal Slack web API client (apiCall, postAPI)
    auth.go               # Browser auth flow, token extraction JS, normalizeWorkspaceURL
    session.go            # CapturedSession, CaptureFromPage, SaveEncrypted, LoadEncrypted
    types.go              # Workspace, Channel, Message, Thread, File, User, SearchResult
    option.go             # Functional options: WithWorkspace, WithToken, WithDCookie, etc.
    export.go             # ExportChannelJSON
    doc.go                # Package documentation
```

| Type | Purpose | File |
|------|---------|------|
| `Credentials`, `Progress` | Base scraper types | `scraper/scraper.go` |
| `AuthError`, `RateLimitError` | Typed error conditions | `scraper/scraper.go` |
| `EncryptData`, `DecryptData` | AES-256-GCM + Argon2id encryption | `scraper/crypto.go` |
| `slack.Scraper` | Slack workspace scraper | `scraper/slack/slack.go` |
| `slack.CapturedSession` | Encrypted session persistence | `scraper/slack/session.go` |

### Unified CLI (`cmd/scout/`)

Single binary `scout` with Cobra subcommands. Communicates with a background gRPC daemon for session persistence.

```
cmd/scout/
├── main.go                 # Entry point: cli.Execute()
└── internal/cli/
    ├── root.go             # Root command + persistent flags (--addr, --session, --output, --format)
    ├── daemon.go           # Auto-start gRPC daemon, getClient(), resolveSession()
    ├── daemon_unix.go      # Unix process detach (Setsid)
    ├── daemon_windows.go   # Windows process detach (CREATE_NEW_PROCESS_GROUP)
    ├── helpers.go          # writeOutput(), readPassphrase(), truncate()
    ├── version.go          # scout version
    ├── session.go          # scout session create/destroy/list/use
    ├── server.go           # scout server (gRPC server)
    ├── client.go           # scout client (interactive REPL)
    ├── navigate.go         # scout navigate/back/forward/reload
    ├── screenshot.go       # scout screenshot/pdf
    ├── har.go              # scout har start/stop/export
    ├── interact.go         # scout click/type/select/hover/focus/clear/key
    ├── inspect.go          # scout title/url/text/attr/eval/html
    ├── window.go           # scout window get/min/max/full/restore
    ├── storage.go          # scout storage get/set/list/clear
    ├── network.go          # scout cookie/header/block
    ├── search.go           # scout search (standalone)
    ├── crawl.go            # scout crawl (standalone)
    ├── extract.go          # scout table/meta (standalone)
    ├── form.go             # scout form detect/fill/submit (standalone)
    ├── slack.go            # scout slack capture/load/decrypt
    └── firecrawl.go        # scout firecrawl scrape/crawl/search/map/batch/extract
```

Daemon state: `~/.scout/daemon.pid`, `~/.scout/current-session`, `~/.scout/sessions/`

### Examples

`examples/` contains 18 standalone runnable programs:
- `examples/simple/` — 8 examples: navigation, screenshots, extraction, JS eval, forms, cookies
- `examples/advanced/` — 10 examples: search, pagination, crawling, rate limiting, hijacking, stealth, PDF, HAR recording
- Each is a separate `package main` importing `github.com/inovacc/scout/pkg/scout`
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
- **Firecrawl error prefix**: All firecrawl errors use `"firecrawl:"` prefix. Typed errors: `*AuthError` (401/403), `*RateLimitError` (429), `*APIError` (other).
- **Firecrawl polling**: `poll[T]()` generic function reused by `WaitForCrawl` and `WaitForBatch` for async job completion.

## Testing

- `pkg/scout/testutil_test.go` provides `newTestServer()` and `newTestBrowser(t)` (headless, no-sandbox, auto-cleanup).
- Route registration: test files call `registerTestRoutes(fn)` in `init()` to add httptest routes. The `newTestServer()` function collects all registered routes.
- Core routes: `/`, `/page2`, `/json`, `/echo-headers`, `/set-cookie`, `/redirect`, `/slow`
- Extract routes: `/extract`, `/table`, `/meta`, `/links`, `/nested`, `/products-list`
- Form routes: `/form`, `/form-csrf`, `/wizard-step1`, `/wizard-step2`, `/submit`
- Paginate routes: `/products-page{1,2,3}`, `/api/products`, `/infinite`, `/load-more`
- Search routes: `/serp-google`, `/serp-google-page2`, `/serp-bing`, `/serp-ddg`
- Crawl routes: `/crawl-start`, `/crawl-page{1,2,3}`, `/sitemap.xml`
- Recorder routes: `/recorder-page`, `/recorder-asset`, `/recorder-api`
- Firecrawl tests: mock HTTP server in `firecrawl/testutil_test.go`; integration tests behind `//go:build integration` + `FIRECRAWL_API_KEY`
- Window tests: no routes needed — window control operates on the browser window itself
- Tests use `t.Skipf` when browser is unavailable — they will not fail in headless CI without Chrome, they skip.
- No mocking framework; tests run against a real headless browser and local HTTP test server.

## Dependencies

### Core library (pkg/scout/)
- `github.com/go-rod/rod` — core browser automation via Chrome DevTools Protocol
- `github.com/go-rod/stealth` — anti-bot-detection page creation (enabled via `WithStealth()`)
- `github.com/ysmood/gson` — JSON number handling for `EvalResult`
- `golang.org/x/time/rate` — token bucket rate limiter for `RateLimiter`

### gRPC layer and CLI (grpc/ and cmd/scout/)
- `google.golang.org/grpc` — gRPC framework
- `google.golang.org/protobuf` — Protocol Buffers runtime
- `github.com/google/uuid` — session ID generation
- `github.com/spf13/cobra` — CLI framework

Note: The core library does NOT import gRPC or Cobra. Library-only consumers pull zero CLI/gRPC dependencies.

## CI

GitHub Actions (`.github/workflows/test.yml`) uses reusable `inovacc/workflows` — runs tests, lint, and vulnerability checks on push/PR to non-main branches.
