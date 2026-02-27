# CLAUDE.md

## Project Overview

Scout is a Go library (`github.com/inovacc/scout/pkg/scout`) wrapping [go-rod](https://github.com/go-rod/rod) for headless browser automation, web scraping, search, and crawling. A gRPC service layer (`grpc/`) provides remote browser control. A unified Cobra CLI (`cmd/scout/`) exposes all features with a background daemon for session persistence.

## Build & Test

Uses Taskfile. Key commands: `task build`, `task test`, `task test:unit`, `task check`, `task lint`, `task lint:fix`, `task fmt`, `task vet`, `task proto`, `task generate:stealth`.

Run a single test: `go test -v -run TestName ./...`

Build: `go build ./cmd/scout/` and `go build ./pkg/...` (not `go build ./...` — root has no main).

Tests require Chromium; `newTestBrowser` calls `t.Skipf` if unavailable. No mocking — real browser + httptest server.

### Browser Support

- `BrowserChrome` (default), `BrowserBrave`, `BrowserEdge` via `WithBrowser()`. Firefox unsupported (CDP removed).
- Extensions: `WithExtension(paths...)`, `WithExtensionByID(ids...)`, `DownloadExtension(id)`.
- Docker: full image (debian+Chromium) and slim image (distroless CLI-only).

## Architecture

```
pkg/scout/          Core library (flat package)
pkg/stealth/        Anti-bot-detection (internalized go-rod/stealth + ExtraJS)
pkg/identity/       Device identity, Luhn check digits
pkg/discovery/      mDNS service discovery
pkg/browser/        Browser detection, download, cache management
pkg/scout/recipe/   Recipe system (extract + automate + analyze)
pkg/scout/mcp/      MCP server (stdio transport)
extensions/         Embedded Chrome extensions (scout-bridge)
cmd/scout/          Unified Cobra CLI (50+ subcommands)
grpc/               gRPC service (proto, server, mTLS, pairing)
scraper/            Scraper framework + AES-256-GCM auth
examples/           18 runnable examples (simple/ and advanced/)
```

Import: `github.com/inovacc/scout/pkg/scout`. Core does NOT import gRPC or Cobra.

## Conventions

- **Functional options**: `New(opts ...Option)` with `With*()` in `option.go`. Defaults: headless=true, 1920×1080, 30s timeout.
- **WaitLoad**: `NewPage()` doesn't wait for DOM. Call `page.WaitLoad()` before extraction on external sites.
- **Error wrapping**: `fmt.Errorf("scout: action: %w", err)` — consistent prefix.
- **Nil-safety**: `Browser.Close()` and key methods are nil-safe and idempotent.
- **Cleanup patterns**: `SetHeaders()`, `EvalOnNewDocument()` return cleanup functions. `HijackRouter` has `Run()` (goroutine) and `Stop()`.
- **Struct tags**: `scout:"selector"` or `scout:"selector@attr"` for extraction; `form:"field_name"` for forms.
- **Generics**: Pagination uses type params (`PaginateByClick[T]`) — package-level functions.
- **Escape hatches**: `RodPage()` and `RodElement()` expose underlying rod instances.
- **CLI baseOpts**: `baseOpts(cmd)` in `helpers.go` combines headless/sandbox/browser/stealth options.
- **Stealth**: `WithStealth()` or `SCOUT_STEALTH=true/1`. Adds `disable-blink-features=AutomationControlled` + JS evasions via `stealth.Page()`.
- **Bridge**: Enabled by default. Embedded via `embed.FS`. Disable with `WithoutBridge()` or `SCOUT_BRIDGE=false`.
- **Remote CDP**: `WithRemoteCDP(endpoint)` connects to existing Chrome DevTools endpoint.
- **Platform-specific**: `WithXvfb()` in `option_unix.go`. gRPC `platform_*.go` for OS defaults.
- **gRPC port**: Default `9551`. Daemon state in `~/.scout/`.
- **LLM providers**: `LLMProvider` interface with `Name()` + `Complete()`. Ollama, OpenAI-compatible, Anthropic implementations.
- **Recipe selectors**: `$name` references resolved at parse time. `+` sibling prefix and `@attr` suffix preserved.
- **Smart wait**: `WaitFrameworkReady()` detects framework and waits for readiness.
- **Snapshot JS**: Lives in `snapshot_script.go` (not `_js.go` — that suffix triggers GOOS=js build constraint).

## Dependencies

Core: `go-rod/rod`, `ysmood/gson`, `x/time/rate`, `x/net/html`, `ollama/ollama`, `go-sdk/mcp`.
Stealth: internalized `go-rod/stealth` + `extract-stealth-evasions` v2.7.3.
Identity: `x/crypto`, `grandcat/zeroconf`.
gRPC/CLI: `google.golang.org/grpc`, `google.golang.org/protobuf`, `google/uuid`, `spf13/cobra`.

## CI

GitHub Actions (`.github/workflows/test.yml`) via reusable `inovacc/workflows` — tests, lint, vuln checks on push/PR to non-main branches.
