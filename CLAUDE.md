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
- **Session directory**: `~/.scout/sessions/<hash>/{scout.pid, job.json, data/}`. Metadata (`scout.pid`, `job.json`) at hash level; `data/` is the Chrome user-data-dir. `SessionDir(id)` returns hash dir, `SessionDataDir(id)` returns `data/` subdir.
- **Session startup cleanup**: `CleanStaleSessions()` runs in `main()` on every invocation. Removes non-reusable sessions unconditionally, dead reusable sessions, and orphaned dirs without `scout.pid`. Windows file lock retries (3×200ms).
- **Session reset**: `ResetSession(id)` and `ResetAllSessions()` in `session_track.go`. CLI: `scout session reset [id]`, `scout session reset --all`. Kills browser process and removes session dir.
- **Job tracking**: `job.json` in session dir tracks job type, status (pending/running/completed/failed), progress, steps, timestamps. API: `NewJob()`, `WriteJob()`, `StartJob()`, `CompleteJob()`, `FailJob()`, `AddJobStep()`.
- **Health check**: `Browser.HealthCheck(url, opts...)` crawls site detecting broken links, console errors, JS exceptions, network failures. CLI: `scout test-site <url> [--depth N] [--concurrency N] [--click] [--json] [--timeout 30s]`.
- **REPL mode**: `scout repl [url]` standalone local browser shell with 20 commands (navigate, eval, click, type, extract, screenshot, markdown, cookies, tabs, health, etc.). No daemon required.
- **Page gather**: `Browser.Gather(url, opts...)` one-shot page intelligence collector. Returns DOM, HAR, links, screenshots, cookies, metadata, console log, frameworks, accessibility snapshot. CLI: `scout gather <url>` with `--html`, `--har`, `--screenshot`, `--links`, etc.
- **Cloud upload**: `Uploader` with OAuth2 for Google Drive and OneDrive. CLI: `scout upload auth --sink gdrive`, `scout upload file <path>`, `scout upload status`. Config in `~/.scout/upload.json`.
- **gops agent**: `github.com/google/gops/agent` started in `main()` with `ShutdownCleanup: true`. Makes every scout process discoverable. `IsScoutProcess(pid)` in `session/process_gops.go` uses `goprocess.Find()` to confirm a PID is a scout Go binary (avoids PID reuse false positives).
- **Browser close detection**: `Page.WaitClose()` returns a channel closed when the page target is destroyed (CDP `TargetTargetDestroyed`). Used by `mcp open` to exit when user closes browser window. `Launcher.Exit()` exposes process-exit channel. `Browser.Done()` delegates to launcher.
- **Session cleanup**: `launcher.Cleanup()` called synchronously (not `go`) for non-reusable sessions, ensuring session dir is removed before process exits. `EnrichSessionInfo()` populates `Exec` and `BuildVersion` from gops metadata.
- **Process platform files**: `process_windows.go` and `process_linux.go` in `internal/engine/session/` — each contains platform-specific `ProcessAlive` + shared gops-based `IsScoutProcess`/`ScoutProcessInfo`.
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
- **Claude Code plugin**: `.claude-plugin/plugin.json` + `.mcp.json` + 6 skills + 3 agents. Test: `claude --plugin-dir .`. Skills: `/scout:scrape`, `/scout:screenshot`, `/scout:test-site`, `/scout:gather`, `/scout:crawl`, `/scout:monitor`.

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
