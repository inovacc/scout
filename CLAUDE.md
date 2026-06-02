# CLAUDE.md

## Project Overview

Scout is a Go browser automation library with the core engine in `internal/engine/` (internalized rod fork) and a public facade at `pkg/scout/`. A gRPC service layer (`grpc/`) provides remote browser control. A unified Cobra CLI (`cmd/scout/`) exposes all features with a background daemon for session persistence.

## Build & Test

Uses Taskfile. Key commands: `task build`, `task test`, `task test:unit`, `task check`, `task lint`, `task lint:fix`, `task fmt`, `task vet`, `task proto`, `task generate:stealth`.

Run a single test: `go test -v -run TestName ./...`

Build: `go build ./cmd/scout/` and `go build ./pkg/...` (not `go build ./...` — root has no main).

Tests require Chromium; `newTestBrowser` calls `t.Skipf` if unavailable. No mocking — real browser + httptest server.

### Browser Support

- `BrowserChrome` (default), `BrowserChromium`, `BrowserBrave`, `BrowserEdge` via `WithBrowser()`. Firefox unsupported (CDP removed).
- Browser isolation: by default Scout only uses `~/.scout/browsers/` cache. `BestCached()` auto-downloads Chrome for Testing if nothing cached. `--system-browser` flag allows system-installed browsers.
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
internal/engine/swarm/        Distributed crawling (coordinator, worker, domain queue)
internal/engine/lib/          Internalized rod: launcher, CDP, proto, input, utils
internal/engine/scouthome/    Per-user state root resolver (SCOUT_HOME, %LOCALAPPDATA%\Scout)
internal/flags/               Feature flag persistence (~/.cache/scout/)
internal/logger/              Command logging (KSUID log files, stdout/stderr capture)
internal/tracing/             OpenTelemetry instrumentation (Init, MCPToolSpan, ScraperSpan)
internal/idle/                Idle timer for auto-shutdown
pkg/scout/                    Public facade (type aliases + New/Option re-exports)
pkg/scout/identity/           Device identity, Luhn check digits
pkg/scout/discovery/          mDNS service discovery
pkg/scout/browser/            Browser path resolution (public API)
pkg/scout/guide/              Step-by-step guide recording (Recorder, Guide, RenderMarkdown)
pkg/scout/runbook/            Runbook system (extract + automate + analyze + Plan/Apply)
pkg/scout/mcp/                MCP server (18 built-in tools + 3 WS tools, 3 resources, stdio + SSE transport)
pkg/scout/plugin/             Plugin system (subprocess JSON-RPC, manager, 8 capability proxies)
pkg/scout/plugin/registry/    Plugin marketplace (GitHub-backed index, lock file, checksum verification)
pkg/scout/plugin/sdk/         Go SDK for plugin authors (10 handler types)
pkg/scout/proxy/              API middleware proxy (YAML routes, browser extraction, caching)
pkg/scout/strategy/           Strategy files (YAML/JSON workflows, executor, sinks)
pkg/scout/agent/              AI agent framework integration (OpenAI/Anthropic tool schemas)
pkg/scout/monitor/            Visual regression testing (baseline management, pixel diff, monitoring)
pkg/scout/scraper/            Scraper framework + AES-256-GCM auth + 20 modes (including TikTok)
pkg/scout/archive/            Archive/compression utilities
runbooks/                     Embedded preset runbooks (26 JSON files)
extensions/                   Embedded Chrome extensions (scout-bridge)
plugins/                      12 standalone plugin binaries (diag, reports, content, search, network, forms, crawl, guide, comm, email-docs, content-social, enterprise)
hacks/                        Test tools and debug utilities (not part of build)
cmd/scout/                    Unified Cobra CLI (50+ subcommands, gops agent, logger, connect.go)
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
- **Remote CDP connect**: `scout connect --cdp ws://...` connects to running browser. Uses `WithRemoteCDP()` internally.
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
- **State root**: resolved via `internal/engine/scouthome`. Precedence: `SCOUT_HOME` env → platform default (Windows `%LOCALAPPDATA%\Scout`, Darwin `~/Library/Application Support/Scout`, Linux `$XDG_DATA_HOME/scout` or `~/.local/share/scout`) → legacy `~/.scout` if it has content. New installs land in the platform default.
- **Session directory**: `<scouthome>/sessions/<encoded-id>/{scout.pid, scout.lock, monitors.json, job.json, data/}`. Session IDs use the `pkg/id` encoded format — 12-char attribute prefix (`1` version, browser code C/B/E/X/M, H/V mode, P/E lifetime, S/N stealth, B/N bridge, V/N vpn, 5 reserved `0`) + 24 random `[A-Z]`. Example: `1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA`. `scout.pid` is a 432-byte fixed-width binary record (`SCT1` magic, v1; little-endian); `scout.lock` is a 0-byte sibling carrying the OS-level advisory lock (LockFileEx on Windows, flock on Unix); `monitors.json` declares which monitors are active (HAR/hijack/console/ws/blocks). Metadata mode 0o600, dir 0o700. `SessionDir(id)` returns the session dir, `SessionDataDir(id)` returns `data/`. Use `id.New(id.Attrs{...})` / `id.Parse(s)` (`pkg/id`) for ID generation + decoding; engine aliases `NewSessionID`/`ParseSessionID` / `SessionAttrs` are also available.
- **Session startup cleanup**: `CleanStaleSessions()` runs in `main()` on every invocation. Uses a single-shot `os.RemoveAll` for the fast path; persistent failures (Windows AV / Search Indexer / OneDrive holding Chrome SQLite + LevelDB files) are enqueued for `StartCleanupRetrier` which retries every 60 s for the process lifetime. Removes non-reusable sessions unconditionally and orphaned dirs without `scout.pid`. **Reusable sessions are auto-cleaned only when `IsExpired()` trips** (H6 + bounded-lifetime); otherwise only manual `scout session reset/destroy` removes them.
- **Session monitors**: `monitors.json` sidecar persists per-session monitor config (HAR/hijack/console/ws sinks + block rules). Daemon `CreateSession` writes it; `DestroySession` reads it to flush the HAR artifact to the configured path (default `<session>/har.json`). `scout session create --har --hijack --block <pattern> --block-method POST` enables monitoring in one command.
- **Request blocking**: `scout.WithBlockRules(scout.BlockRule{Pattern, Method})` installs a `HijackRequests` router that aborts matching requests with `NetworkErrorReasonBlockedByClient`. Pattern uses CDP URLPattern syntax (`*` wildcards). Used for recon: capture the intended request payload via HAR / hijack BEFORE the abort fires so server never sees it.
- **Session reset**: `ResetSession(id)` and `ResetAllSessions()` in `session_track.go`. CLI: `scout session reset [id]`, `scout session reset --all`. Kills browser process and removes session dir.
- **Job tracking**: `job.json` in session dir tracks job type, status (pending/running/completed/failed), progress, steps, timestamps. API: `NewJob()`, `WriteJob()`, `StartJob()`, `CompleteJob()`, `FailJob()`, `AddJobStep()`.
- **Health check**: `Browser.HealthCheck(url, opts...)` crawls site detecting broken links, console errors, JS exceptions, network failures. CLI: `scout test-site <url> [--depth N] [--concurrency N] [--click] [--json] [--timeout 30s]`.
- **REPL mode**: `scout repl [url]` standalone local browser shell with 20 commands (navigate, eval, click, type, extract, screenshot, markdown, cookies, tabs, health, etc.). No daemon required.
- **Page gather**: `Browser.Gather(url, opts...)` one-shot page intelligence collector. Returns DOM, HAR, links, screenshots, cookies, metadata, console log, frameworks, accessibility snapshot. CLI: `scout gather <url>` with `--html`, `--har`, `--screenshot`, `--links`, etc.
- **Cloud upload**: `Uploader` with OAuth2 for Google Drive and OneDrive. CLI: `scout upload auth --sink gdrive`, `scout upload file <path>`, `scout upload status`. Config in `~/.scout/upload.json`.
- **gops agent**: `github.com/google/gops/agent` started in `main()` with `ShutdownCleanup: true`. Makes every scout process discoverable. `IsScoutProcess(pid)` in `session/process_gops.go` uses `goprocess.Find()` to confirm a PID is a scout Go binary (avoids PID reuse false positives).
- **Browser close detection**: `Page.WaitClose()` returns a channel closed when the page target is destroyed (CDP `TargetTargetDestroyed`). Used by `mcp open` to exit when user closes browser window. `Launcher.Exit()` exposes process-exit channel. `Browser.Done()` delegates to launcher.
- **Session cleanup**: `launcher.Cleanup()` called synchronously (not `go`) for non-reusable sessions, ensuring session dir is removed before process exits. `EnrichSessionInfo()` populates `Exec` and `BuildVersion` from gops metadata.
- **Process platform files**: `process_windows.go` and `process_unix.go` (`//go:build !windows`) in `internal/engine/session/` — each contains platform-specific `ProcessAlive` + shared gops-based `IsScoutProcess`/`ScoutProcessInfo`.
- **Plugin system**: Subprocess-based plugins communicate via JSON-RPC 2.0 on stdin/stdout. `plugin.Manager` discovers from `~/.scout/plugins/*/plugin.json` and `$SCOUT_PLUGIN_PATH`. Plugins declare capabilities (`scraper_mode`, `extractor`, `mcp_tool`) in manifest. Lazy process launch. `ModeProxy` bridges `scraper.Mode`, `ToolProxy` bridges MCP tools. Go SDK in `pkg/scout/plugin/sdk/` — `NewServer()`, `RegisterMode/Extractor/Tool()`, `Run()`. CLI: `scout plugin install <path|url>` supports local dirs and archive URLs.
- **OpenTelemetry tracing**: `internal/tracing/` package. No-op unless `SCOUT_TRACE=1` or `OTEL_EXPORTER_OTLP_ENDPOINT` is set. `tracing.Init(ctx, Config{})` in CLI bootstrap. All 37 MCP tools auto-instrumented via `addTracedTool()` wrapper in `pkg/scout/mcp/server.go`. Scraper CLI uses `ScraperSpan()`. Custom spans: `tracing.Start(ctx, "name", attrs...)`.
- **Reports**: `SaveReport()` persists AI-consumable markdown to `~/.scout/reports/{uuidv7}.txt`. Three types: `health_check`, `gather`, `crawl`. Each report includes metadata, structured findings, AI analysis instructions, and embedded raw JSON. CLI: `scout test-site --report`, `scout gather --report`, `scout report list/show/delete`.
- **Swarm mode**: `internal/engine/swarm/` distributed crawling. `Coordinator` manages domain-partitioned BFS queue with URL dedup and worker health. `Worker` pulls batches, navigates with real browser, extracts title+links. CLI: `scout swarm start <url> [--workers N --depth N --max-pages N --report]`.
- **Default browser fallback**: When no `--browser` flag is given, `launchLocal()` calls `browser.BestCached()` to find cached browsers. If none exist, `BestCached()` auto-downloads Chrome for Testing. Rod fallback is a true last resort.
- **Browser isolation**: Default mode uses only `~/.scout/browsers/` cache. `--system-browser` flag allows system-installed browsers. `scout browser list` shows only cached browsers by default; `--detect` scans system paths.
- **Bridge reset**: `Bridge.ResetReady()` clears `ready`/`available` flags before navigation when reusing a page+bridge across URLs (used by `SitemapExtract`). Chrome for Testing requires this — it kills CDP connections on stale binding access.
- **Plugin marketplace**: `pkg/scout/plugin/registry/` — `FetchIndex()` downloads GitHub-backed JSON index, `Index.Search()` filters by name/description/tags. `LockFile` tracks installed versions + SHA256 checksums in `~/.scout/plugins/lock.json`. CLI: `scout plugin search`, `scout plugin update`, `scout plugin install github:owner/plugin`.
- **WebSocket MCP tools**: `ws_listen` monitors page WS traffic for a duration, `ws_send` executes JS to send WS messages, `ws_connections` lists active connections. Built on `Page.MonitorWebSockets()` JS interceptor.
- **AI agent provider**: `pkg/scout/agent/` — `Provider` wraps Scout browser as 9 AI tools. `OpenAITools()` and `AnthropicTools()` return framework-specific schemas. `Call(ctx, name, args)` executes tools with error wrapping into `ToolResult`.
- **Visual monitor**: `pkg/scout/monitor/` — `BaselineManager` captures/loads PNG baselines with SHA256 checksums. `Compare()` does pixel-level diff with threshold. `Monitor.Run()` checks at intervals, calls `ChangeHandler` on visual change.
- **MCP tool count**: 18 built-in tools after deprecation cleanup (navigate, click, type, extract, eval, back, forward, wait, screenshot, snapshot, pdf, session_list, session_reset, open, swarm_crawl, ws_listen, ws_send, ws_connections). 28 tools migrated to plugins.
- **Mobile automation**: `WithMobile(MobileConfig{})` for ADB-connected Android Chrome, `WithTouchEmulation()` for desktop touch simulation. `Page.Touch()`, `Page.Swipe()`, `Page.PinchZoom()` for touch gestures. CLI: `scout mobile devices`, `scout mobile connect`.
- **Agent HTTP server**: `scout agent serve [--addr localhost:9000]` starts REST API for AI frameworks. Endpoints: `GET /tools` (OpenAI/Anthropic formats), `POST /call` (execute tool). `pkg/scout/agent/server.go`.
- **WebSocket HAR**: `Recorder` captures WS events (opened/sent/received/closed) alongside HTTP. `ExportHAR()` includes `_webSocketMessages` extension. `ExportWebSocketHAR()` for WS-only export.
- **Claude Code plugin**: `.claude-plugin/plugin.json` + `.mcp.json` + 6 skills + 3 agents. Test: `claude --plugin-dir .`. Skills: `/scout:scrape`, `/scout:screenshot`, `/scout:test-site`, `/scout:gather`, `/scout:crawl`, `/scout:monitor`. Validate: `task plugin:validate`.
- **Agent server auth**: `--api-key` flag or `SCOUT_AGENT_API_KEY` env var enables Bearer token auth. `/health` and `/metrics` bypass auth. CORS enabled by default. Rate limit: `--rate-limit 100` (requests/sec).
- **OpenAPI spec**: `docs/openapi.yaml` — OpenAPI 3.1.0 for all agent HTTP endpoints. Update when adding endpoints.
- **Plugin auto-update**: `scout plugin check-updates` compares lock file against registry. `ShouldCheck(24h)` throttles to daily. `MarkChecked()` persists timestamp.
- **Benchmarks**: `go test -bench=. ./internal/engine/hijack/ ./pkg/scout/agent/ ./internal/metrics/` — 11 benchmarks for hot paths.
- **Helm values schema**: `deploy/helm/scout/values.schema.json` validates chart values. Update when adding Helm options.
- **Grafana dashboard**: `deploy/grafana/scout-dashboard.json` — 15 panels, import into Grafana with Prometheus datasource.
- **npm package**: `npm/scout-browser/` published as `@inovacc/scout-browser` to GitHub Packages. Bump version in `package.json` before `npm publish`.
- **Secrets vault**: `pkg/scout/vault` stores named secret profiles in one Argon2id+AES-256-GCM file at `<scouthome>/profiles/vault.bin` (0o600 in a 0o700 dir). Secrets live in `LockedBuffer` (`[]byte` + `VirtualLock`/`Mlock` + explicit zero) and never become `string`. `Vault.Use(id)` returns a `Handle` that injects cookies/headers into a live page via CDP (`Handle.ApplyToPage`) and yields scout-internal secrets via `Handle.Secret` — never env vars. `scout vault rotate` re-keys atomically. CLI: `scout vault init/set/get/list/use/rotate/rm`. `--from-profile` imports a `.scoutprofile`'s secret fields (deprecating `UserProfile.Cookies/Storage/Headers`, removal after 2026-07-02).

## Dependencies

Core: `ysmood/gson`, `x/time/rate`, `x/net/html`, `ollama/ollama`, `go-sdk/mcp` (rod internalized in `internal/engine/lib/`).
Stealth: internalized `go-rod/stealth` + `extract-stealth-evasions` v2.7.3.
Identity: `x/crypto`, `grandcat/zeroconf`.
gRPC/CLI: `google.golang.org/grpc`, `google.golang.org/protobuf`, `google/uuid`, `spf13/cobra`.
Process management: `google/gops` (agent registration + `goprocess.Find` for orphan detection).
Logger: `segmentio/ksuid`.
Tracing: `go.opentelemetry.io/otel`, `otel/sdk`, `otel/exporters/stdout/stdouttrace`.

## CI

GitHub Actions (`.github/workflows/test.yml`) via reusable `inovacc/workflows` — tests, lint, vuln checks on push/PR to non-main branches.

<!-- GSD:project-start source:PROJECT.md -->
## Project

**Scout — Stabilization Milestone**

Scout is a Go browser automation library with an internalized rod fork, public facade API, gRPC service, MCP server, plugin system, REPL, and 50+ CLI commands. It provides headless and headed browser control for scraping, testing, monitoring, and AI agent integration.

**Core Value:** Sessions must be rock-solid: open cleanly, close cleanly, never leak processes, and never touch the user's browser without explicit permission.

### Constraints

- **Language**: Go 1.25 — no language changes
- **Architecture**: Keep layered pattern (internal engine -> pkg facade -> entry points)
- **Testing**: Real browser + httptest, no mocks. Tests require Chromium available
- **Breaking changes**: Acceptable — clean API over backwards compat
- **Build**: Taskfile-based. `task build`, `task test`, `task check`
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.26.0 - All application code (`go.mod` line 3)
- JavaScript - Stealth evasion scripts (`internal/engine/stealth/stealth_extra.go`), bridge extension (`extensions/`), snapshot scripts (`snapshot_script.go`)
- Protocol Buffers - gRPC service definition (`grpc/proto/scout.proto`)
- Node.js - npm package installer (`npm/scout-browser/install.js`), stealth asset generation (`pkg/stealth/generate/`)
## Runtime
- Go 1.26.0 (requires Go toolchain)
- Chromium/Chrome browser (headless or headed) for core functionality
- Optional: Electron runtime for `WithElectronApp()` support
- Go modules (`go.mod` / `go.sum`)
- Lockfile: `go.sum` present
## Frameworks
- Internalized rod fork (`internal/engine/lib/`) - CDP browser automation engine
- Cobra v1.10.2 (`github.com/spf13/cobra`) - CLI framework, 80+ subcommands in `cmd/scout/`
- Go standard `testing` package
- `github.com/stretchr/testify` v1.11.1 - Assertions
- `github.com/ysmood/got` v0.42.3 - Rod-style test helpers
- Real browser + `httptest` server (no mocking)
- Taskfile v3 (`Taskfile.yml`) - Task runner (preferred over Make)
- `golangci-lint` - Linting (`task lint`)
- `goimports` - Import formatting (`task fmt`)
- `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` - Protobuf code generation (`task proto`)
## Key Dependencies
- `github.com/ysmood/gson` v0.7.3 - JSON handling for CDP protocol
- `github.com/ysmood/goob` v0.4.0 - Observable pattern (event system)
- `github.com/ollama/ollama` v0.24.0 - Ollama LLM client SDK
- `github.com/modelcontextprotocol/go-sdk` v1.4.1 - MCP server implementation
- `google.golang.org/grpc` v1.81.1 - gRPC framework
- `google.golang.org/protobuf` v1.36.11 - Protobuf runtime
- `github.com/spf13/cobra` v1.10.2 - CLI framework
- `golang.org/x/oauth2` v0.36.0 - OAuth2 for cloud uploads (Google Drive, OneDrive)
- `golang.org/x/crypto` v0.51.0 - AES-256-GCM encryption for scraper auth
- `golang.org/x/net` v0.54.0 - HTML parsing (`x/net/html`)
- `golang.org/x/time` v0.14.0 - Rate limiting (`x/time/rate`)
- `github.com/google/gops` v0.3.29 - Process discovery and orphan detection
- `github.com/google/uuid` v1.6.0 - UUID generation (UUIDv7 for reports)
- `github.com/segmentio/ksuid` v1.0.4 - K-Sortable unique IDs for command logging
- `github.com/grandcat/zeroconf` v1.0.0 - mDNS service discovery (`pkg/scout/discovery/`)
- `go.opentelemetry.io/otel` v1.43.0 - OpenTelemetry tracing
- `go.opentelemetry.io/otel/sdk` v1.43.0 - OTel SDK
- `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` v1.41.0 - Trace export
- `github.com/gin-gonic/gin` v1.10.0 - HTTP framework (indirect, used by ollama dep)
- `github.com/mattn/go-sqlite3` v1.14.24 - SQLite (indirect, via ollama)
- `github.com/charmbracelet/bubbletea` v1.3.10 - TUI (indirect)
## Configuration
- `SCOUT_HEADLESS` - Enable headless mode (default: true)
- `SCOUT_NO_SANDBOX` - Disable Chrome sandbox
- `SCOUT_HOME` - Override per-user state root (default: `%LOCALAPPDATA%\Scout` on Windows, `~/Library/Application Support/Scout` on macOS, `$XDG_DATA_HOME/scout` or `~/.local/share/scout` on Linux). Honored by every persisted directory (sessions, browsers, plugins, fingerprints, extensions, reports, OAuth tokens, electron cache).
- `SCOUT_STEALTH` - Enable stealth mode (`true`/`1`)
- `SCOUT_BRIDGE` - Enable/disable bridge extension (`false` to disable)
- `SCOUT_TRACE` - Enable OpenTelemetry tracing (`1`)
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP exporter endpoint
- `SCOUT_AGENT_API_KEY` - Agent HTTP server API key
- `SCOUT_PLUGIN_PATH` - Additional plugin search paths
- `CHROME_BIN` - Chrome/Chromium binary path
- `~/.scout/` - Session data, fingerprints, plugins, reports, upload config
- `~/.scout/browsers/` - Cached browser binaries
- `~/.scout/plugins/` - Installed plugins + `lock.json`
- `~/.scout/sessions/` - Session directories with `scout.pid` and `job.json`
- `~/.scout/reports/` - Persisted markdown reports
- `~/.scout/fingerprints/` - Fingerprint store
- `~/.scout/upload.json` - Cloud upload OAuth config
- `~/.cache/scout/` - Feature flags, Electron cache
- `~/.cache/scout/electron/` - Electron runtime cache
- `Taskfile.yml` - All build/test/lint/deploy tasks
- `grpc/proto/scout.proto` - gRPC service definition
- `deploy/helm/scout/values.schema.json` - Helm values validation
- `deploy/helm/scout/Chart.yaml` - Helm chart (v0.1.0, appVersion 1.0.0)
- `docs/openapi.yaml` - OpenAPI 3.1.0 spec for agent HTTP API
## Platform Requirements
- Go 1.26.0+
- Chromium or Chrome browser (auto-downloaded via `BestCached()` if missing)
- `protoc` + Go plugins (for proto regeneration only)
- `golangci-lint` (for linting)
- `goimports` (for formatting)
- Node.js/npx (only for stealth asset regeneration)
- Full image: `debian:bookworm-slim` + Chromium + fonts (`docker/Dockerfile`)
- Slim image: `gcr.io/distroless/static-debian12:nonroot` (`docker/Dockerfile.slim`)
- Swarm image: `Dockerfile.swarm` (distributed crawling)
- Helm chart: `deploy/helm/scout/` (deployment, service, HPA, PVC, job templates)
- Grafana dashboard: `deploy/grafana/scout-dashboard.json` (15 panels, Prometheus datasource)
- `@inovacc/scout-browser` v1.0.1 (`npm/scout-browser/package.json`)
- Published to GitHub Packages (`npm.pkg.github.com`)
- Platforms: darwin, linux, win32 (x64, arm64)
- Node.js >= 16
- GitHub Actions (`.github/workflows/test.yml`)
- Reusable workflow: `inovacc/workflows/.github/workflows/reusable-go-check.yml@main`
- Runs: tests, lint, vulncheck on push/PR to non-main branches
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
- Snake_case for Go files: `browser_rod.go`, `dev_helpers.go`, `hijack_session_test.go`
- Platform-specific suffix: `option_unix.go`, `option_unix_test.go`, `process_windows.go`, `process_unix.go`
- Test files co-located: `extract.go` / `extract_test.go`
- Avoid `_js.go` suffix (triggers `GOOS=js` build constraint) -- use `snapshot_script.go` instead
- Generated files: `scout.pb.go`, `scout_grpc.pb.go` (protobuf), `scout.go` facade (via `gen-facade-full.go`)
- Lowercase, single-word where possible: `engine`, `browser`, `stealth`, `hijack`, `swarm`
- Multi-word uses no separator: `fingerprint`, `runbook`
- Internal packages under `internal/engine/` for core, `internal/flags/`, `internal/logger/`, `internal/tracing/`, `internal/idle/`
- Public packages under `pkg/scout/` with sub-packages for domains: `pkg/scout/mcp/`, `pkg/scout/plugin/`, `pkg/scout/scraper/`
- PascalCase exported, camelCase unexported (standard Go)
- Constructor: `New(opts ...Option) (*Browser, error)` -- always returns `(*T, error)`
- Helper constructors: `newTestBrowser(t *testing.T)`, `newBenchServer()`
- Boolean checks: `IsFeatureEnabled()`, `IsScoutProcess()`, `ShouldIgnoreCommand()`
- Prefixed with domain: `CleanOrphans()`, `CleanStaleSessions()`, `ResetSession()`
- PascalCase exported structs: `Browser`, `Page`, `Element`, `Option`
- Type aliases for facade re-export: `type Browser = engine.Browser`
- Constants use PascalCase with category prefix: `BrowserChrome`, `BrowserBrave`, `TraceTypeQuery`
- Enum-like constants grouped with iota or explicit values
- Package-level singletons with `sync.Once`: `sharedBrowser`/`sharedBrowserOnce`, `instance`/`once`
- Unexported package vars: `testRouteRegistrars`, `flagCache`, `appDir`
## Code Style
- `go fmt` + `goimports` (run via `task fmt`)
- No explicit `.editorconfig` or `.prettierrc` -- Go standard formatting
- `golangci-lint` v2 with `default: all` linters enabled
- Config: `.golangci.yml`
- Key disabled linters: `tagliatelle`, `gochecknoglobals`, `mnd`, `testpackage`, `varnamelen`, `paralleltest`, `funlen`, `cyclop`, `err113`, `wsl`, `errorlint`, `exhaustruct`, `forcetypeassert`, `godox`, `gosec`, `lll`, `nestif`, `wrapcheck`, `dupl`
- Package-specific exclusions: `forbidigo`/`gocritic` excluded for `examples/`, `contextcheck` excluded for `pkg/scout/scraper/` and `grpc/`
- Nolint directives used sparingly with explanation: `//nolint:maintidx`, `//nolint:staticcheck // internalized rod API`, `//nolint: forcetypeassert`
## Import Organization
- Used to avoid collisions with internalized rod packages: `launcher2`, `proto2`, `js2`
- Pattern: append `2` to the package name when aliasing internalized lib packages
## Functional Options Pattern
- Defaults defined in `defaults()` function: headless=true, 1920x1080, 30s timeout, bridge=true
- Location: `internal/engine/option.go`
- CLI integration: `baseOpts(cmd)` in `cmd/scout/helpers.go` combines headless/sandbox/browser/stealth options from Cobra flags
## Error Handling
- Always prefix with `"scout:"` followed by the subsystem
- Examples: `"scout: challenge: detect: %w"`, `"scout: fingerprint store: mkdir: %w"`, `"scout: workspace: resolve path: %w"`, `"scout: write file: %w"`, `"scout: read password: %w"`
- Use `%w` verb for wrapping (enables `errors.Is`/`errors.As`)
## Cleanup Functions
- `SetHeaders()`, `EvalOnNewDocument()` return cleanup functions
- `HijackRouter` uses `Run()` (starts goroutine) and `Stop()` (halts it)
- Common pattern: return `func() {}` (no-op) when no cleanup needed
## Struct Tags
## Logging
- Singleton pattern with `sync.Once`
- KSUID-based log file names for unique identification
- Environment-controlled: `SCOUT_LOGGER_ENABLED` env var
- Feature flag persistence in `~/.cache/scout/` via `internal/flags/`
- `LoggingWriter` wraps `io.Writer` to capture output while passing through, with 1MB max buffer
- Command executions (stdout/stderr capture)
- Session lifecycle events
- Error conditions with structured context
## Code Generation
- Re-exports all `internal/engine` types as type aliases: `type Browser = engine.Browser`
- Re-exports constants and constructor functions
## Environment Variables
- Feature flags: `SCOUT_STEALTH=true/1`, `SCOUT_BRIDGE=false`, `SCOUT_TRACE=1`
- Auth: `SCOUT_PASSPHRASE`, `SCOUT_AGENT_API_KEY`
- Logging: `SCOUT_LOGGER_ENABLED`
- Tracing: `OTEL_EXPORTER_OTLP_ENDPOINT`
- Pattern: always `SCOUT_` prefix for project-specific env vars
## Module Design
- Public API surface through `pkg/scout/` facade only
- `internal/engine/` is the core -- not directly importable by external consumers
- Constructor + functional options is the standard public API pattern
- `pkg/scout/scout.go` serves as the barrel file, re-exporting everything from `internal/engine`
- Sub-packages (`pkg/scout/mcp/`, `pkg/scout/plugin/`) have their own public APIs
- Build tags: `//go:build !windows` in `process_unix.go`, implicit windows in `process_windows.go`
- Taskfile platform conditionals: `platforms: [ windows ]` vs `platforms: [ linux, darwin ]`
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Internal engine core is never imported directly by consumers; only `pkg/scout` is public
- Functional options pattern (`New(opts ...Option)`) for all configuration
- Plugin system uses subprocess JSON-RPC 2.0 for extensibility without coupling
- Session management with process-level tracking via gops
- Lazy browser initialization in MCP and agent servers
## Layers
- Purpose: Cobra-based CLI with 50+ subcommands exposing all Scout features
- Location: `cmd/scout/`
- Contains: One file per command group (e.g., `scrape.go`, `crawl.go`, `gather.go`, `hijack.go`, `repl.go`)
- Depends on: `pkg/scout`, `grpc/server`, `pkg/scout/mcp`, `pkg/scout/scraper`, `internal/engine/session`, `internal/flags`, `internal/logger`, `internal/tracing`
- Used by: End users, shell scripts
- Key files: `cmd/scout/scout.go` (root command + `main()`), `cmd/scout/helpers.go` (shared `baseOpts()` helper)
- Purpose: Stable public API re-exporting internal engine types via type aliases
- Location: `pkg/scout/scout.go`
- Contains: Generated type aliases (`type Browser = engine.Browser`), function re-exports (`New`, `With*` options)
- Depends on: `internal/engine`, `internal/engine/browser`
- Used by: All entry points (CLI, gRPC, MCP, agent, plugins)
- Key detail: `pkg/scout/scout.go` is code-generated by `gen-facade-full.go` -- DO NOT EDIT manually
- Purpose: All browser automation logic -- navigation, extraction, forms, crawling, stealth, fingerprinting, challenges, screenshots, PDFs, research, etc.
- Location: `internal/engine/`
- Contains: ~100 non-test Go files. Key types: `Browser`, `Page`, `Element`, `Option`
- Depends on: `internal/engine/lib/` (internalized rod), `internal/engine/browser/`, `internal/engine/stealth/`, `internal/engine/fingerprint/`, `internal/engine/hijack/`, `internal/engine/llm/`, `internal/engine/session/`, `internal/engine/swarm/`
- Used by: `pkg/scout` facade only (no direct external imports)
- Purpose: Forked rod browser automation library (launcher, CDP protocol, input simulation, device emulation)
- Location: `internal/engine/lib/`
- Contains: `launcher/` (browser process management), `cdp/` (Chrome DevTools Protocol client), `proto/` (CDP protocol types), `input/` (keyboard/mouse), `devices/` (device emulation profiles), `utils/`, `js/` (injected scripts)
- Depends on: Nothing internal (leaf dependency)
- Used by: `internal/engine/` only
- Purpose: Remote browser control via gRPC with mTLS and device pairing
- Location: `grpc/`
- Contains: `proto/scout.proto` (service definition), `scoutpb/` (generated protobuf), `server/` (implementation)
- Depends on: `pkg/scout`, `internal/engine/swarm`, `internal/idle`, `pkg/scout/identity`
- Used by: CLI daemon commands (`cmd/scout/daemon.go`, `cmd/scout/server.go`)
- Key files: `grpc/server/server.go` (session management, all RPC handlers), `grpc/server/tls.go` (mTLS), `grpc/server/pairing.go` (device pairing)
- Purpose: Model Context Protocol server exposing 18 browser tools for AI assistants
- Location: `pkg/scout/mcp/`
- Contains: Tool definitions split by category: `tools_browser.go` (navigate, click, type, extract, eval, etc.), `tools_capture.go` (screenshot, snapshot, pdf), `tools_session.go` (session management), `tools_swarm.go` (distributed crawl), `tools_websocket.go` (WS monitoring), `resources.go` (3 MCP resources)
- Depends on: `pkg/scout`, `pkg/scout/plugin`, `internal/idle`, `internal/metrics`, `internal/tracing`, `github.com/modelcontextprotocol/go-sdk/mcp`
- Used by: CLI `cmd/scout/server.go` (stdio + SSE transport)
- Key pattern: Lazy browser init via `mcpState.ensureBrowser()`. All tools auto-instrumented with OpenTelemetry via `addTracedTool()`.
- Purpose: REST API for AI agent frameworks (OpenAI/Anthropic tool schemas)
- Location: `pkg/scout/agent/`
- Contains: `server.go` (HTTP server with rate limiting, CORS, optional API key auth), `provider.go` (9 tools as AI-callable functions), `openapi.yaml` (embedded OpenAPI 3.1.0 spec)
- Depends on: `pkg/scout`, `internal/idle`, `internal/metrics`, `x/time/rate`
- Used by: CLI `cmd/scout/agent.go`
- Endpoints: `GET /tools`, `POST /call`, `GET /health`, `GET /metrics`, `GET /openapi.yaml`
- Purpose: Subprocess-based extensibility via JSON-RPC 2.0 on stdin/stdout
- Location: `pkg/scout/plugin/`
- Contains: `manager.go` (discovery + lifecycle), `manifest.go` (plugin.json schema), `client.go` (JSON-RPC client), `protocol.go` (wire protocol), 8 capability proxies (`mode_proxy.go`, `tool_proxy.go`, `command_proxy.go`, `sink_proxy.go`, `auth_proxy.go`, `resource_proxy.go`, `middleware_proxy.go`, `event_proxy.go`), `extractor.go`
- Depends on: `pkg/scout/scraper` (Mode interface)
- Used by: `pkg/scout/mcp`, CLI root command registration
- Key pattern: Plugins discovered from `~/.scout/plugins/*/plugin.json` and `$SCOUT_PLUGIN_PATH`. Lazy process launch on first call.
- Purpose: Authenticated scraping for 20+ platforms (Slack, Discord, LinkedIn, etc.)
- Location: `pkg/scout/scraper/`
- Contains: `mode.go` (Mode interface), `scraper.go` (orchestrator), `auth/` (session management with AES-256-GCM encryption), `crypt/` (crypto utils), `modes/` (20 platform implementations)
- Depends on: Nothing in `pkg/scout` (standalone, uses its own auth)
- Used by: CLI `cmd/scout/scrape.go`, plugins
- Purpose: Declarative YAML/JSON workflow files for multi-step browser automation
- Location: `pkg/scout/strategy/`
- Contains: `strategy.go` (types + parser), executor, sinks
- Depends on: `pkg/scout`
- Used by: CLI `cmd/scout/strategy.go`
## Data Flow
- Browser sessions tracked in `~/.scout/sessions/<hash>/` with `scout.pid` and `job.json`
- Fingerprints persisted in `~/.scout/fingerprints/`
- Plugin state in `~/.scout/plugins/` with `lock.json` for checksums
- Feature flags in `~/.cache/scout/`
- Reports in `~/.scout/reports/`
- Upload OAuth tokens in `~/.scout/upload.json`
- Command logs as KSUID-named files via `internal/logger/`
## Key Abstractions
- Purpose: Wraps Chrome process lifecycle + CDP connection
- Location: `internal/engine/browser.go`
- Pattern: Functional options via `New(opts ...Option)`. Nil-safe `Close()`. Owns launcher, session tracking, VPN/fingerprint rotation, bridge server.
- Purpose: Single browser tab with navigation, extraction, evaluation, forms, screenshots
- Location: `internal/engine/page.go`, `internal/engine/page_eval.go`
- Pattern: Created via `Browser.NewPage(url)`. Wraps rod page. `WaitLoad()` must be called before extraction on external sites.
- Purpose: DOM element with click, type, extract, attribute access
- Location: `internal/engine/element.go`
- Pattern: Found via `Page.Element(selector)` or `Page.ElementByXPath(xpath)`. Struct tag extraction with `scout:"selector"` / `scout:"selector@attr"`.
- Purpose: Platform-specific scraping implementation
- Location: `pkg/scout/scraper/mode.go`
- Pattern: Interface with `Name()`, `Description()`, `AuthProvider()`, `Scrape(ctx, session, opts)`. Channel-based results. 20 implementations in `pkg/scout/scraper/modes/`.
- Purpose: Abstraction for AI model backends used by ExtractWithLLM
- Location: `internal/engine/llm/provider.go`
- Pattern: Interface with `Name()` + `Complete(ctx, systemPrompt, userPrompt)`. Implementations: Ollama, OpenAI-compatible, Anthropic.
- Purpose: Declares plugin capabilities and launch command
- Location: `pkg/scout/plugin/manifest.go`
- Pattern: JSON schema with `capabilities` array. 10 capability types: `scraper_mode`, `extractor`, `mcp_tool`, `cli_command`, `auth_provider`, `mcp_resource`, `mcp_resource_template`, `mcp_prompt`, `sink`, `middleware`, `events`.
- Purpose: Declarative multi-step workflow definition
- Location: `pkg/scout/strategy/strategy.go`
- Pattern: YAML/JSON with `browser`, `auth`, `steps[]`, `output` sections. `LoadFile()` + `Validate()` + `Execute()`.
## Entry Points
- Location: `cmd/scout/scout.go`
- Triggers: User runs `scout <command>` from terminal
- Responsibilities: Parse flags, route to command handler, manage gops agent, session cleanup, tracing init, logger capture
- Location: `grpc/server/server.go`
- Triggers: `scout daemon start` or `scout server`
- Responsibilities: Persistent browser sessions, mTLS auth, device pairing, event streaming, swarm coordination
- Location: `pkg/scout/mcp/server.go`
- Triggers: AI assistant connects via stdio or SSE
- Responsibilities: 18 browser tools, 3 resources, lazy browser lifecycle, idle timeout, OpenTelemetry tracing
- Location: `pkg/scout/agent/server.go`
- Triggers: `scout agent serve [--addr localhost:9000]`
- Responsibilities: REST API for AI frameworks, rate limiting, CORS, API key auth, OpenAPI spec serving
- Location: `pkg/scout/plugin/sdk/`
- Triggers: Plugin manager launches subprocess on first capability call
- Responsibilities: JSON-RPC 2.0 server on stdin/stdout, register handlers for modes/extractors/tools
## Error Handling
- All public methods return `error` as last value
- `Browser.Close()` and key methods are nil-safe and idempotent via `sync.Once`
- Session cleanup runs on every `main()` invocation to handle crashes
- gRPC server returns gRPC status codes (`codes.NotFound`, `codes.Internal`)
- MCP tools return error text in `mcp.TextContent` with `result.IsError = true`
- Plugin subprocess errors marshaled as JSON-RPC error responses
## Cross-Cutting Concerns
```mermaid
```
<!-- GSD:architecture-end -->

## Workflow

Use the Superpowers workflow for all development work:

1. **Brainstorm** (`/superpowers:brainstorm`) — explore intent, requirements, and design before any implementation. Required before creating features, components, or modifying behaviour.
2. **Spec** (`/superpowers:writing-plans`) — produce a written plan from the brainstorm output before touching code.
3. **Execute** (`/superpowers:executing-plans`) — implement against the spec with TDD; write tests first, then code.
4. **Verify** (`/superpowers:verification-before-completion`) — validate built features against acceptance criteria before marking work done.

For debugging, use `/superpowers:systematic-debugging`. For code review, use `/superpowers:requesting-code-review`.

Phase specs live in `docs/superpowers/specs/`. Consult them before starting any phase work.

# context-mode — MANDATORY routing rules

You have context-mode MCP tools available. These rules are NOT optional — they protect your context window from flooding. A single unrouted command can dump 56 KB into context and waste the entire session.

## BLOCKED commands — do NOT attempt these

### curl / wget — BLOCKED
Any Bash command containing `curl` or `wget` is intercepted and replaced with an error message. Do NOT retry.
Instead use:
- `ctx_fetch_and_index(url, source)` to fetch and index web pages
- `ctx_execute(language: "javascript", code: "const r = await fetch(...)")` to run HTTP calls in sandbox

### Inline HTTP — BLOCKED
Any Bash command containing `fetch('http`, `requests.get(`, `requests.post(`, `http.get(`, or `http.request(` is intercepted and replaced with an error message. Do NOT retry with Bash.
Instead use:
- `ctx_execute(language, code)` to run HTTP calls in sandbox — only stdout enters context

### WebFetch — BLOCKED
WebFetch calls are denied entirely. The URL is extracted and you are told to use `ctx_fetch_and_index` instead.
Instead use:
- `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` to query the indexed content

## REDIRECTED tools — use sandbox equivalents

### Bash (>20 lines output)
Bash is ONLY for: `git`, `mkdir`, `rm`, `mv`, `cd`, `ls`, `npm install`, `pip install`, and other short-output commands.
For everything else, use:
- `ctx_batch_execute(commands, queries)` — run multiple commands + search in ONE call
- `ctx_execute(language: "shell", code: "...")` — run in sandbox, only stdout enters context

### Read (for analysis)
If you are reading a file to **Edit** it → Read is correct (Edit needs content in context).
If you are reading to **analyze, explore, or summarize** → use `ctx_execute_file(path, language, code)` instead. Only your printed summary enters context. The raw file content stays in the sandbox.

### Grep (large results)
Grep results can flood context. Use `ctx_execute(language: "shell", code: "grep ...")` to run searches in sandbox. Only your printed summary enters context.

## Tool selection hierarchy

1. **GATHER**: `ctx_batch_execute(commands, queries)` — Primary tool. Runs all commands, auto-indexes output, returns search results. ONE call replaces 30+ individual calls.
2. **FOLLOW-UP**: `ctx_search(queries: ["q1", "q2", ...])` — Query indexed content. Pass ALL questions as array in ONE call.
3. **PROCESSING**: `ctx_execute(language, code)` | `ctx_execute_file(path, language, code)` — Sandbox execution. Only stdout enters context.
4. **WEB**: `ctx_fetch_and_index(url, source)` then `ctx_search(queries)` — Fetch, chunk, index, query. Raw HTML never enters context.
5. **INDEX**: `ctx_index(content, source)` — Store content in FTS5 knowledge base for later search.

## Subagent routing

When spawning subagents (Agent/Task tool), the routing block is automatically injected into their prompt. Bash-type subagents are upgraded to general-purpose so they have access to MCP tools. You do NOT need to manually instruct subagents about context-mode.

## Output constraints

- Keep responses under 500 words.
- Write artifacts (code, configs, PRDs) to FILES — never return them as inline text. Return only: file path + 1-line description.
- When indexing content, use descriptive source labels so others can `ctx_search(source: "label")` later.

## ctx commands

| Command | Action |
|---------|--------|
| `ctx stats` | Call the `ctx_stats` MCP tool and display the full output verbatim |
| `ctx doctor` | Call the `ctx_doctor` MCP tool, run the returned shell command, display as checklist |
| `ctx upgrade` | Call the `ctx_upgrade` MCP tool, run the returned shell command, display as checklist |
