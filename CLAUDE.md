# CLAUDE.md

## Project Overview

Scout is a Go browser automation library with the core engine in `internal/engine/` (internalized rod fork) and a public facade at `pkg/scout/`. A gRPC service layer (`grpc/`) provides remote browser control. A unified Cobra CLI (`cmd/scout/`) exposes all features with a background daemon for session persistence.

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
internal/engine/              Core engine (internalized rod fork + scout API)
internal/engine/browser/      Browser detection, download, cache management
internal/engine/detect/       Framework/tech-stack detection
internal/engine/fingerprint/  Fingerprint rotation strategies + store
internal/engine/hijack/       Session hijacking (HTTP + WebSocket capture, HAR)
internal/engine/llm/          LLM provider interface + implementations
internal/engine/session/      Session tracking, orphan cleanup, gops process checks
internal/engine/stealth/      Anti-bot-detection (internalized go-rod/stealth + ExtraJS)
internal/engine/vpn/          VPN integration (Surfshark)
internal/engine/lib/          Internalized rod: launcher, CDP, proto, input, utils
internal/flags/               Feature flag persistence (~/.cache/scout/)
internal/logger/              Command logging (KSUID log files, stdout/stderr capture)
internal/idle/                Idle timer for auto-shutdown
pkg/scout/                    Public facade (type aliases + New/Option re-exports)
pkg/scout/identity/           Device identity, Luhn check digits
pkg/scout/discovery/          mDNS service discovery
pkg/scout/browser/            Browser path resolution (public API)
pkg/scout/runbook/            Runbook system (extract + automate + analyze + Plan/Apply)
pkg/scout/recipe/             Deprecated compat aliases → runbook package
pkg/scout/mcp/                MCP server (33 tools, 3 resources, stdio + SSE transport)
pkg/scout/scraper/            Scraper framework + AES-256-GCM auth + 19 modes
pkg/scout/archive/            Archive/compression utilities
runbooks/                     Embedded preset runbooks (26 JSON files)
extensions/                   Embedded Chrome extensions (scout-bridge)
cmd/scout/                    Unified Cobra CLI (50+ subcommands, gops agent, logger)
grpc/                         gRPC service (proto, server, mTLS, pairing)
examples/                     18 runnable examples (simple/ and advanced/)
```

Import: `github.com/inovacc/scout/pkg/scout`. Public facade re-exports `internal/engine` types. Core does NOT import gRPC or Cobra.

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
- **Runbook selectors**: `$name` references resolved at parse time. `+` sibling prefix and `@attr` suffix preserved.
- **Runbook Plan/Apply**: `Plan()` dry-runs selectors on live page, `Apply()` executes. CLI: `scout runbook plan -f`, `scout runbook apply -f`.
- **Smart wait**: `WaitFrameworkReady()` detects framework and waits for readiness.
- **Snapshot JS**: Lives in `snapshot_script.go` (not `_js.go` — that suffix triggers GOOS=js build constraint).
- **Fingerprint rotation**: `WithFingerprintRotation(cfg)` with strategies: PerSession, PerPage, PerDomain, Interval. `FingerprintStore` persists to `~/.scout/fingerprints/`.
- **Research presets**: `WithResearchPreset(ResearchShallow|Medium|Deep)`. `ResearchCache` with TTL. `WithResearchPrior(result)` for incremental research.
- **Stealth evasions**: 17 evasions in `internal/engine/stealth/stealth_extra.go` including languages, plugins/mimeTypes, timezone, canvas/audio noise, WebGL, WebRTC, fonts, screen, battery, hasFocus, outer dimensions, toString integrity.
- **Session hijacking**: `Page.NewSessionHijacker(opts...)` captures real-time HTTP + WebSocket traffic via CDP events. `HijackEvent` discriminated union with `CapturedRequest`/`CapturedResponse`/`WebSocketFrame`. Auto-attach via `WithSessionHijack()`. Channel-based: `hijacker.Events()` returns `<-chan HijackEvent`. Filter with `WithHijackURLFilter()`, capture bodies with `WithHijackBodyCapture()`. gRPC: `StartHijack`/`StopHijack`/`StreamHijack` RPCs. CLI: `scout hijack watch <url>`.
- **Electron support**: `WithElectronApp(path)`, `WithElectronVersion(ver)`, `WithElectronCDP(endpoint)`. Auto-downloads Electron runtime to `~/.cache/scout/electron/`. CLI: `--electron-app`, `--electron-version`, `--electron-cdp` flags.
- **Command logging**: `scout logger --path <dir>` enables KSUID-based log files with stdout/stderr capture. `internal/flags/` persists feature flags in `~/.cache/scout/`. `internal/logger/` writes structured JSON logs via `slog`. Root `PersistentPreRunE` auto-captures all command output.
- **Session reset**: `ResetSession(id)` and `ResetAllSessions()` in `session_track.go`. CLI: `scout session reset [id]`, `scout session reset --all`. Kills browser process and removes session dir.
- **Health check**: `Browser.HealthCheck(url, opts...)` crawls site detecting broken links, console errors, JS exceptions, network failures. CLI: `scout test-site <url> [--depth N] [--concurrency N] [--click] [--json] [--timeout 30s]`.
- **REPL mode**: `scout repl [url]` standalone local browser shell with 20 commands (navigate, eval, click, type, extract, screenshot, markdown, cookies, tabs, health, etc.). No daemon required.
- **Page gather**: `Browser.Gather(url, opts...)` one-shot page intelligence collector. Returns DOM, HAR, links, screenshots, cookies, metadata, console log, frameworks, accessibility snapshot. CLI: `scout gather <url>` with `--html`, `--har`, `--screenshot`, `--links`, etc.
- **Cloud upload**: `Uploader` with OAuth2 for Google Drive and OneDrive. CLI: `scout upload auth --sink gdrive`, `scout upload file <path>`, `scout upload status`. Config in `~/.scout/upload.json`.
- **gops agent**: `github.com/google/gops/agent` started in `main()` with `ShutdownCleanup: true`. Makes every scout process discoverable. `IsScoutProcess(pid)` in `session/process_gops.go` uses `goprocess.Find()` to confirm a PID is a scout Go binary (avoids PID reuse false positives).
- **Browser close detection**: `Page.WaitClose()` returns a channel closed when the page target is destroyed (CDP `TargetTargetDestroyed`). Used by `mcp open` to exit when user closes browser window. `Launcher.Exit()` exposes process-exit channel. `Browser.Done()` delegates to launcher.
- **Session cleanup**: `launcher.Cleanup()` called synchronously (not `go`) for non-reusable sessions, ensuring session dir is removed before process exits. `EnrichSessionInfo()` populates `Exec` and `BuildVersion` from gops metadata.
- **Process platform files**: `process_windows.go` and `process_linux.go` in `internal/engine/session/` — each contains platform-specific `ProcessAlive` + shared gops-based `IsScoutProcess`/`ScoutProcessInfo`.

## Dependencies

Core: `ysmood/gson`, `x/time/rate`, `x/net/html`, `ollama/ollama`, `go-sdk/mcp` (rod internalized in `internal/engine/lib/`).
Stealth: internalized `go-rod/stealth` + `extract-stealth-evasions` v2.7.3.
Identity: `x/crypto`, `grandcat/zeroconf`.
gRPC/CLI: `google.golang.org/grpc`, `google.golang.org/protobuf`, `google/uuid`, `spf13/cobra`.
Process management: `google/gops` (agent registration + `goprocess.Find` for orphan detection).
Logger: `segmentio/ksuid`.

## CI

GitHub Actions (`.github/workflows/test.yml`) via reusable `inovacc/workflows` — tests, lint, vuln checks on push/PR to non-main branches.
