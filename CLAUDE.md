# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scout is a Go library (`github.com/inovacc/scout/pkg/scout`) that wraps [go-rod](https://github.com/go-rod/rod) to provide a simplified, Go-idiomatic API for headless browser automation, web scraping,
search, and crawling. The core library is in `pkg/scout/`. A gRPC service layer (`grpc/`) provides remote browser control. A unified Cobra CLI (`cmd/scout/`) exposes all features as commands with a
background daemon for session persistence.

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
task generate:stealth  # Regenerate stealth anti-detection JS assets (requires Node.js/npx)
task grpc:server       # Run the gRPC server via scout CLI
task grpc:client       # Run the interactive CLI client via scout CLI
```

Run a single test:

```bash
go test -v -run TestName ./...
```

Tests require a Chromium-based browser available on the system. `newTestBrowser` calls `t.Skipf` if the browser is unavailable, so tests skip gracefully in environments without a browser.

### Browser Support

Scout supports multiple Chromium-based browsers via `BrowserType`:

- `BrowserChrome` (default) — rod auto-detect + auto-download
- `BrowserBrave` — auto-detects locally, auto-downloads from GitHub releases if not installed
- `BrowserEdge` — auto-detects locally, error includes download URL if not installed

Use `WithBrowser(BrowserBrave)` or CLI `--browser=brave`. `WithExecPath()` takes precedence if both are set. Firefox is not supported (CDP removed in Firefox 141, June 2025). Downloaded browsers are cached in `~/.scout/browsers/`. Use `scout browser list` to see detected and downloaded browsers.

### Chrome Extension Loading

Load unpacked Chrome extensions via `WithExtension(paths...)`. This sets `--load-extension` and `--disable-extensions-except` launch flags automatically.

```go
b, _ := scout.New(scout.WithExtension("/path/to/ext1", "/path/to/ext2"))
```

Download extensions from the Chrome Web Store via `DownloadExtension(id)`, which fetches CRX2/CRX3 files, unpacks to `~/.scout/extensions/<id>/`, and reads `manifest.json` for metadata. Load downloaded extensions by ID via `WithExtensionByID(ids...)`.

```go
info, _ := scout.DownloadExtension("cjpalhdlnbpafiamejdnhcphjbkeiagm") // uBlock Origin
b, _ := scout.New(scout.WithExtensionByID("cjpalhdlnbpafiamejdnhcphjbkeiagm"))
```

CLI commands:

- `scout extension load --path=<dir> [--url=<url>]` — interactive dev workflow (non-headless, blocks until Ctrl+C)
- `scout extension test --path=<dir> [--screenshot=out.png]` — headless testing, lists loaded extensions
- `scout extension list` — list downloaded extensions and browser-loaded extensions
- `scout extension download <id>` — download + unpack from Chrome Web Store to `~/.scout/extensions/`
- `scout extension remove <id>` — delete a locally downloaded extension

### Docker

```bash
docker build -t scout:latest .                    # Full image (Chromium + scout CLI)
docker build -f Dockerfile.slim -t scout:slim .   # CLI-only (distroless, ~15MB)
docker run --rm --shm-size=2g scout:latest version
docker compose up -d                              # gRPC server with tmpfs /dev/shm
```

- Full image: `debian:bookworm-slim` + Chromium + fonts + `dumb-init`, non-root user `scout`, ports 9551/9552
- Slim image: `gcr.io/distroless/static-debian12:nonroot`, CLI/gRPC client only
- `scout browser download brave` — pre-populate Brave in containers

## Architecture

```
scout/
├── pkg/scout/          # Core library (package scout)
├── pkg/stealth/        # Anti-bot-detection stealth (extract-stealth-evasions + custom ExtraJS)
├── pkg/identity/       # Device identity, Luhn check digits, trust
├── pkg/discovery/      # mDNS service discovery
├── pkg/scout/recipe/   # Recipe system (extract + automate + analyze/generate)
├── pkg/scout/mcp/      # MCP server (Model Context Protocol via stdio)
├── extensions/         # Embedded Chrome extensions (scout-bridge)
├── cmd/scout/          # Unified Cobra CLI binary (package main)
├── grpc/               # gRPC service layer
│   ├── proto/          # Protobuf definitions
│   ├── scoutpb/        # Generated Go code
│   └── server/         # gRPC server + mTLS + pairing
├── scraper/            # Scraper framework + auth
├── examples/           # 18 runnable examples
└── docs/               # Documentation, ADRs, roadmap
```

Library code is in `pkg/scout/` (flat, single-package). Import as `github.com/inovacc/scout/pkg/scout`. The gRPC layer is in `grpc/`. The unified CLI is at `cmd/scout/`. Additional packages:
`pkg/stealth/` (internalized go-rod/stealth), `pkg/identity/` (Syncthing-style device identity with Luhn check digits), `pkg/discovery/` (mDNS service discovery).

### Core Types (rod wrappers)

| Type                            | Wraps                   | File                    |
|---------------------------------|-------------------------|-------------------------|
| `Browser`                       | `*rod.Browser`          | `pkg/scout/browser.go`  |
| `Page`                          | `*rod.Page`             | `pkg/scout/page.go`     |
| `Element`                       | `*rod.Element`          | `pkg/scout/element.go`  |
| `EvalResult`                    | JS eval results         | `pkg/scout/eval.go`     |
| `HijackRouter`, `HijackContext` | rod hijack types        | `pkg/scout/network.go`  |
| `WindowState`, `WindowBounds`   | Window state control    | `pkg/scout/window.go`   |
| `NetworkRecorder`               | HAR 1.2 traffic capture | `pkg/scout/recorder.go` |

### HAR Recording Types

| Type                                                  | Purpose                                                   | File          |
|-------------------------------------------------------|-----------------------------------------------------------|---------------|
| `NetworkRecorder`                                     | Captures HTTP traffic via CDP, exports HAR                | `recorder.go` |
| `HARLog`, `HAREntry`, `HARRequest`, `HARResponse`     | HAR 1.2 data model                                        | `recorder.go` |
| `HARHeader`, `HARContent`, `HARTimings`, `HARCreator` | HAR sub-types                                             | `recorder.go` |
| `RecorderOption`                                      | Functional options (`WithCaptureBody`, `WithCreatorName`) | `recorder.go` |

### Scraping Toolkit Types

| Type                                  | Purpose                             | File           |
|---------------------------------------|-------------------------------------|----------------|
| `TableData`, `MetaData`               | Extraction results                  | `extract.go`   |
| `Form`, `FormField`, `FormWizard`     | Form interaction                    | `form.go`      |
| `RateLimiter`                         | Rate limiting + retry               | `ratelimit.go` |
| `PaginateByClick/URL/Scroll/LoadMore` | Generic pagination                  | `paginate.go`  |
| `SearchResults`, `SearchResult`       | SERP parsing                        | `search.go`    |
| `CrawlResult`, `SitemapURL`           | Web crawling                        | `crawl.go`     |
| `MapOption`                           | URL map/link discovery options      | `map.go`       |
| `MarkdownOption`                      | HTML-to-Markdown conversion options | `markdown.go`  |
| `storageAPI`, `SessionState`          | Web storage & session persistence   | `storage.go`   |
| `SwaggerSpec`, `SwaggerPath`, etc.    | OpenAPI/Swagger spec extraction     | `swagger.go`   |
| `ExtensionInfo`                       | Chrome extension metadata + path    | `extension.go` |
| `DownloadBrave`, `ListDownloadedBrowsers` | Browser auto-download + cache   | `browser_download.go` |
| `WebFetchResult`, `WebFetchOption`        | URL content extraction + cache  | `webfetch.go`         |
| `WebSearchResult`, `WebSearchOption`      | Search + fetch pipeline         | `websearch.go`        |
| `BlockAds`, `BlockTrackers`, `BlockFonts`, `BlockImages` | URL blocking preset pattern slices | `option.go` |
| `FrameworkInfo`                           | Detected frontend framework (name, version, SPA flag) | `detect.go` |
| `TechStack`                               | Technology stack detection (CSS, build tool, CMS, analytics, CDN) | `detect.go` |
| `RenderMode`, `RenderInfo`                | Rendering mode classification (CSR/SSR/SSG/ISR) | `detect.go` |
| `GitHubRepo`, `GitHubIssue`, `GitHubPR`, `GitHubUser`, `GitHubRelease` | GitHub data extraction | `github.go` |
| `ChallengeType`, `ChallengeInfo`          | Bot protection challenge detection (9 types) | `challenge.go` |
| `SnapshotOption`                          | Accessibility tree snapshot options  | `snapshot.go`  |
| `CapturedCredentials`, `BrowserInfo`      | Credential capture & replay          | `capture.go`   |
| `UserProfile`, `ProfileDiff`              | Portable browser identity + diff     | `profile.go`   |
| `AsyncJob`, `AsyncJobManager`             | Persistent async job lifecycle       | `jobs.go`       |
| `WebMCPTool`, `WebMCPToolResult`          | Web-native MCP tool discovery + call | `webmcp.go`    |
| `PWAInfo`, `WebAppManifest`               | Progressive Web App detection        | `detect.go`    |
| `AutoFreeConfig`                          | Browser recycling configuration      | `autofree.go`  |
| `ValidationResult`, `ValidationError`     | Recipe dry-run validation results    | `recipe/validate.go` |
| `LLMValidation`, `ValidateWithLLM`       | LLM-based recipe validation prompts  | `recipe/validate.go` |
| `FlowStep`, `FormInfo`, `DetectFlow`, `GenerateFlowRecipe` | Multi-page flow detection and recipe generation | `recipe/flow.go` |
| `InjectHelper`, `InjectAllHelpers`        | Built-in JS extraction helper injection | `helpers.go`      |
| `HelperTableExtract`, `HelperInfiniteScroll`, `HelperShadowQuery`, `HelperWaitForSelector`, `HelperClickAll` | Bundled JS helper constants | `helpers.go` |
| `ScriptTemplate`, `RenderTemplate`, `InjectTemplate`, `BuiltinTemplates` | Parameterized JS script templates | `templates.go` |
| `SelectorScore`                           | Selector resilience scoring result   | `recipe/score.go`    |
| `InteractiveCreate`                       | Interactive recipe creation wizard   | `recipe/interactive.go` |
| `BridgeServer`                            | WebSocket server for bridge comms    | `bridge_ws.go`       |
| `BridgeMessage`                           | Bridge WebSocket message type        | `bridge_ws.go`       |
| `BridgeEvent`                             | Bridge event from browser→Go        | `bridge_events.go`   |
| `BridgeRecorder`, `RecordedStep`, `RecordedRecipe` | Interaction recording for recipe generation | `bridge_record.go` |
| `ResolveExtensions`, `ResolveExtensionsWithBase` | Profile extension ID→path resolution | `profile.go`    |

### MCP Server Types

| Type                                          | Purpose                                        | File                |
|-----------------------------------------------|-------------------------------------------------|---------------------|
| `ServerConfig`                                | MCP server configuration (headless, stealth)    | `mcp/server.go`     |
| `mcpState`                                    | Lazy browser/page management with mutex         | `mcp/server.go`     |

### Sitemap Extract Types

| Type                                          | Purpose                                        | File                |
|-----------------------------------------------|-------------------------------------------------|---------------------|
| `SitemapPage`                                 | Per-page DOM + Markdown extraction result       | `sitemap.go`        |
| `SitemapResult`                               | Full sitemap extraction output                  | `sitemap.go`        |
| `SitemapOption`                               | Functional options for `SitemapExtract()`        | `sitemap.go`        |

### LLM Extraction Types

| Type                                          | Purpose                                        | File                |
|-----------------------------------------------|-------------------------------------------------|---------------------|
| `LLMProvider`                                 | Interface for LLM backends (Complete + Name)    | `llm.go`            |
| `LLMOption`, `LLMJobResult`                   | Functional options and pipeline result          | `llm.go`, `llm_review.go` |
| `OllamaProvider`                              | Ollama local LLM provider                       | `llm_ollama.go`     |
| `OpenAIProvider`                              | OpenAI-compatible provider (OpenAI, OpenRouter, DeepSeek, Gemini) | `llm_openai.go` |
| `AnthropicProvider`                           | Anthropic Messages API provider                  | `llm_anthropic.go`  |
| `LLMWorkspace`, `LLMSession`, `LLMJob`       | Filesystem-based session/job persistence         | `llm_workspace.go`  |
| `JobStatus`, `SessionIndex`, `JobIndex`       | Workspace state tracking types                   | `llm_workspace.go`  |

### gRPC Service Layer

```
grpc/
  proto/scout.proto        # Protocol buffer definitions (ScoutService)
  scoutpb/                 # Generated Go code (committed for consumer convenience)
  server/
    server.go              # gRPC service implementation (ScoutServer)
    tls.go                 # mTLS certificate generation and TLS config
    pairing.go             # Syncthing-style device pairing handshake
    display.go             # Server instance table view with peer tracking
    platform_linux.go      # Linux session defaults (auto --no-sandbox)
    platform_windows.go    # Windows session defaults (no-op)
    platform_other.go      # Darwin/other session defaults (no-op)
```

| Type                   | Purpose                                                   | File                     |
|------------------------|-----------------------------------------------------------|--------------------------|
| `ScoutServer`          | Multi-session gRPC service                                | `grpc/server/server.go`  |
| `ScoutService` (proto) | 25+ RPCs: session, nav, interact, capture, record, stream | `grpc/proto/scout.proto` |

### Scraper Framework

```
scraper/
  scraper.go              # Base types: Credentials, Progress, AuthError, RateLimitError, ExportJSON
  crypto.go               # AES-256-GCM + Argon2id: EncryptData, DecryptData
  auth/                   # Generic browser auth framework + encrypted session persistence
```

| Type                          | Purpose                           | File                 |
|-------------------------------|-----------------------------------|----------------------|
| `Credentials`, `Progress`     | Base scraper types                | `scraper/scraper.go` |
| `AuthError`, `RateLimitError` | Typed error conditions            | `scraper/scraper.go` |
| `EncryptData`, `DecryptData`  | AES-256-GCM + Argon2id encryption | `scraper/crypto.go`  |

### Unified CLI (`cmd/scout/`)

Single binary `scout` with Cobra subcommands (package main). Communicates with a background gRPC daemon for session persistence.

```
cmd/scout/
├── root.go                 # Entry point (main()) + root command + persistent flags (--addr, --session, --output, --format)
├── daemon.go               # Auto-start gRPC daemon, getClient(), resolveSession()
├── daemon_unix.go          # Unix process detach (Setsid)
├── daemon_windows.go       # Windows process detach (CREATE_NEW_PROCESS_GROUP)
├── helpers.go              # writeOutput(), readPassphrase(), truncate(), baseOpts(), stealthOpts()
├── version.go              # scout version
├── session.go              # scout session create/destroy/list/use
├── server.go               # scout server (gRPC server)
├── client.go               # scout client (interactive REPL)
├── navigate.go             # scout navigate/back/forward/reload
├── screenshot.go           # scout screenshot/pdf
├── har.go                  # scout har start/stop/export
├── interact.go             # scout click/type/select/hover/focus/clear/key
├── inspect.go              # scout title/url/text/attr/eval/html
├── window.go               # scout window get/min/max/full/restore
├── storage.go              # scout storage get/set/list/clear
├── network.go              # scout cookie/header/block
├── search.go               # scout search (standalone)
├── search_engines.go       # scout search:google/bing/duckduckgo/wikipedia (multi-engine)
├── batch.go                # scout batch --urls=... [--concurrency=N]
├── crawl.go                # scout crawl (standalone)
├── extract.go              # scout table/meta (standalone)
├── form.go                 # scout form detect/fill/submit (standalone)
├── fetch.go                # scout fetch <url> [--mode=markdown] [--main-only]
├── markdown.go             # scout markdown --url=<url> [--main-only]
├── map.go                  # scout map <url> [--search=term] [--limit=N]
├── recipe.go               # scout recipe run/validate/create/flow
├── swagger.go              # scout swagger <url> (detect + extract OpenAPI/Swagger specs)
├── websearch.go            # scout websearch "query" [--engine=google] [--fetch=markdown]
├── extension.go            # scout extension load/test/list/download/remove
├── sitemap.go              # scout sitemap extract
├── browser.go              # scout browser list
├── bridge.go               # scout bridge status/send/listen/events/ws-send/query/click/type/dom/tabs/clipboard/record
├── llm.go                  # scout extract-ai, scout ollama, scout ai-job
├── auth.go                 # scout auth login/capture/status/logout/providers
├── device.go               # scout device pair/list/trust
├── aicontext.go            # scout aicontext [--json]
├── credentials.go          # scout credentials capture/replay/show
├── challenge.go            # scout challenge detect
├── detect.go               # scout detect <url> [--framework] [--pwa] [--tech] [--render] [--json]
├── github.go               # scout github repo/issues/prs/user/releases/tree
├── mcp.go                  # scout mcp [--headless] [--stealth]
├── inject.go               # scout inject <url> --code/--file/--dir
├── jobs.go                 # scout jobs list/status/cancel
├── webmcp.go               # scout webmcp discover/call
├── profile.go              # scout profile capture/load/show/merge/diff/session-capture/session-load
├── snapshot.go             # scout snapshot [--format=yaml|json] [--iframes] [--llm]
└── cmdtree.go              # scout cmdtree [--json]
```

Daemon state: `~/.scout/daemon.pid`, `~/.scout/current-session`, `~/.scout/sessions/`

### Examples

`examples/` contains 18 standalone runnable programs:

- `examples/simple/` — 8 examples: navigation, screenshots, extraction, JS eval, forms, cookies
- `examples/advanced/` — 10 examples: search, pagination, crawling, rate limiting, hijacking, stealth, PDF, HAR recording
- Each is a separate `package main` importing `github.com/inovacc/scout/pkg/scout`
- Build individually: `cd examples/simple/basic-navigation && go build .`

**Functional options pattern** for configuration: `New(opts ...Option)` with `With*()` functions in `option.go`. Each feature area has its own options (`ExtractOption`, `SearchOption`,
`PaginateOption`, `CrawlOption`, `RateLimitOption`, `SwaggerOption`). Defaults: headless=true, 1920x1080, 30s timeout.

**Escape hatches**: `RodPage()` and `RodElement()` expose the underlying rod instances when the wrapper API is insufficient.

## Conventions

- **WaitLoad**: `NewPage()` does not wait for DOM load. Call `page.WaitLoad()` before `Extract()`, `ExtractMeta()`, `PDF()`, etc. when targeting external sites.
- **extractAll[T] (pagination)**: Finds first `scout:` tag match, walks to `parentElement`, extracts remaining fields within that parent. All struct fields must be resolvable within the parent of the
  first field's match.
- **Error wrapping**: All errors use `fmt.Errorf("scout: action: %w", err)` — consistent `scout:` prefix.
- **Nil-safety**: `Browser.Close()` and key methods are nil-safe and idempotent. Methods guard with `if b == nil || b.browser == nil`.
- **Cleanup patterns**: `SetHeaders()` and `EvalOnNewDocument()` return cleanup functions. `HijackRouter` has `Run()` (blocking, use in goroutine) and `Stop()`.
- **Struct tags**: `scout:"selector"` or `scout:"selector@attr"` for extraction; `form:"field_name"` for form filling.
- **Generics**: Pagination functions use type parameters (`PaginateByClick[T]`) — package-level functions because Go methods can't have type params.
- **Nolint**: `Element.Interactable()` uses `//nolint:nilerr`; `RateLimiter.calculateBackoff` uses `//nolint:gosec` for jitter rand.
- **Platform-specific options**: `WithXvfb()` lives in `option_unix.go` (`//go:build !windows`). The `xvfb`/`xvfbArgs` fields compile on all platforms but the option function is only available on
  Unix.
- **Window state transitions**: Chrome requires restoring to `normal` before changing between non-normal states. `setWindowState()` handles this automatically.
- **NetworkRecorder**: Attach to a page, records all HTTP traffic via CDP events. `Stop()` is nil-safe and idempotent. `ExportHAR()` produces HAR 1.2 JSON. `Clear()` resets entries.
- **Page keyboard methods**: `KeyPress(key)` and `KeyType(keys...)` operate at the page level (not element-scoped). Used by the gRPC server for `PressKey` RPC.
- **HTML-to-Markdown**: `convertHTMLToMarkdown()` is a pure function testable without browser. `Page.Markdown()` wraps it with page HTML. `Page.MarkdownContent()` applies readability scoring first via
  `WithMainContentOnly()`.
- **Readability scoring**: `extractMainContent()` uses tag-based scoring (article +20, nav -25), class/ID pattern matching, link density penalty. Returns highest-scoring DOM node.
- **URL Map**: `Browser.Map()` combines sitemap.xml parsing + BFS on-page link harvesting. Reuses `visitedSet`, `normalizeURL`, `resolveLink` from crawl.go.
- **Platform session defaults**: `grpc/server/platform_*.go` uses build constraints to apply OS-specific defaults in `CreateSession` (e.g., `--no-sandbox` on Linux). Follows the same pattern as
  `daemon_unix.go`/`daemon_windows.go`.
- **LLM Provider interface**: `LLMProvider` has just `Name()` + `Complete(ctx, system, user)`. `OpenAIProvider` covers OpenAI, OpenRouter, DeepSeek, Gemini via configurable base URL. `AnthropicProvider` uses the Messages API. All use `net/http` directly (no SDK deps except Ollama).
- **LLM Review pipeline**: `ExtractWithLLMReview()` extracts with LLM1, optionally reviews with LLM2. `WithLLMReview(provider)` enables the second pass. Results persisted to workspace via `WithLLMWorkspace(ws)`.
- **LLM Workspace**: Filesystem-based job tracking at `<path>/sessions.json`, `<path>/jobs/jobs.json`, `<path>/jobs/<uuid>/job.json`. Extract and review output written to `extract.md` and `review.md` alongside job metadata.
- **Framework detection**: `Page.DetectFrameworks()` returns all detected frontend frameworks via JS global/DOM inspection. `Page.DetectFramework()` returns the primary one (meta-frameworks like Next.js/Nuxt take precedence over React/Vue). Detects: React, Vue, Angular, AngularJS, Svelte, Next.js, Nuxt, SvelteKit, Remix, Gatsby, Astro, Ember, Backbone, jQuery. `FrameworkInfo` has `Name`, `Version`, `SPA` fields.
- **Remote CDP**: `WithRemoteCDP(endpoint)` connects to an existing Chrome DevTools Protocol endpoint (e.g. `"ws://127.0.0.1:9222"`) instead of launching a local browser. Skips the entire launcher; most launch-related options are ignored. Use for managed browser services (BrightData, Browserless) or remote Chrome instances.
- **Request blocking presets**: `WithBlockPatterns(BlockAds...)` sets URL patterns blocked on every `NewPage()` via `SetBlockedURLs()`. Presets: `BlockAds`, `BlockTrackers`, `BlockFonts`, `BlockImages`. `Page.Block(patterns...)` for ad-hoc per-page blocking.
- **Named recipe selectors**: Recipe JSON supports a `selectors` map with `$name` references in fields and steps. References are resolved at parse time. The `+` sibling prefix and `@attr` suffix are preserved through resolution. Unknown `$refs` produce a clear error.
- **Bridge extension default**: The Scout Bridge extension is enabled by default (`bridge: true` in `defaults()`). Extension files are embedded via `extensions/extensions.go` using `embed.FS` and written to a temp dir at startup. Disable with `WithoutBridge()` or `SCOUT_BRIDGE=false`.
- **SitemapExtract**: `Browser.SitemapExtract()` combines BFS crawl with bridge DOM/Markdown extraction. Reuses a single page + bridge across navigations. Outputs per-page `dom.json`/`dom.md` files plus `index.json`/`index.md` when `WithSitemapOutputDir()` is set.
- **gRPC default port**: The daemon and server default to port `9551` (not the standard gRPC `50051`) to avoid conflicts.
- **CLI baseOpts pattern**: `baseOpts(cmd)` in `helpers.go` combines `WithHeadless`, `WithNoSandbox`, `browserOpt`, and `stealthOpts` into a reusable `[]scout.Option` slice. All CLI commands use `scout.New(baseOpts(cmd)...)` or `scout.New(append(baseOpts(cmd), extraOpt)...)`.
- **Stealth mode**: `WithStealth()` or `SCOUT_STEALTH=true/1` enables anti-bot-detection. CLI: `--stealth` persistent flag. In `browser.go`, adds `disable-blink-features=AutomationControlled` launch flag. In `NewPage()`, creates pages via `stealth.Page()` which injects JS + ExtraJS.
- **Stealth asset generation**: `task generate:stealth` (requires Node.js/npx) regenerates `pkg/stealth/assets.go` from `extract-stealth-evasions@latest`. Not part of `go generate`.
- **Accessibility snapshot**: `Page.Snapshot()` and `Page.SnapshotWithOptions()` inject JS that walks the DOM, computes ARIA roles/names, and produces YAML-like indented output with `[ref=s{gen}e{id}]` markers. `Page.ElementByRef(ref)` finds elements by `data-scout-ref` attribute. JS lives in `snapshot_script.go` (not `snapshot_js.go` — `_js.go` suffix triggers GOOS=js build constraint). `WithSnapshotIframes()` enables recursive iframe traversal. `SnapshotWithLLM()` feeds snapshot YAML to an LLM for element analysis. CLI: `scout snapshot [--format=yaml|json] [--iframes] [--llm]`.
- **Challenge detection**: `Page.DetectChallenges()` evaluates JS to detect 9 bot protection types: Cloudflare, Turnstile, reCAPTCHA v2/v3, hCaptcha, DataDome, PerimeterX, Akamai, AWS WAF. Returns `[]ChallengeInfo` with `Type`, `Confidence`, `Details`. `Page.HasChallenge()` is a quick boolean check.
- **MCP server**: `pkg/scout/mcp/` exposes Scout as MCP server via stdio. `mcpState` lazily initializes browser+page with mutex protection. 15 tools (navigate, click, type, screenshot, snapshot, extract, eval, back, forward, wait, search, fetch, pdf, session_list, session_reset) and 3 resources (markdown, url, title). In-memory transport tests via `mcp.NewInMemoryTransports()`. Logger writes to stderr (stdout = MCP JSON-RPC).
- **Credential capture**: `CaptureCredentials(ctx, url, opts)` opens a headed browser, waits for Ctrl+C via `signal.NotifyContext`, then captures cookies, localStorage, sessionStorage, user agent, browser version. `SaveCredentials`/`LoadCredentials` serialize to JSON. `ToSessionState()` converts to `SessionState` for `Page.LoadSession()`. CLI: `scout credentials capture/replay/show`.
- **Tech stack detection**: `Page.DetectTechStack()` detects CSS frameworks (Bootstrap, Tailwind, etc.), build tools (Webpack, Vite, etc.), CMS (WordPress, etc.), analytics (Google Analytics, etc.), and CDN (Cloudflare, etc.) via JS DOM inspection. Returns `TechStack` struct.
- **Render mode detection**: `Page.DetectRenderMode()` classifies pages as CSR/SSR/SSG/ISR via framework-specific heuristics (Next.js data props, Nuxt payload, Gatsby static query, etc.). Returns `RenderInfo` with `Mode`, `Confidence`, `Details`.
- **GitHub extraction**: `Browser.GitHubRepo()`, `Browser.GitHubIssues()`, `Browser.GitHubPRs()`, `Browser.GitHubUser()`, `Browser.GitHubReleases()`, `Browser.GitHubTree()` scrape GitHub pages via browser automation. Return typed structs (`GitHubRepo`, `GitHubIssue`, `GitHubPR`, `GitHubUser`, `GitHubRelease`). CLI: `scout github repo/issues/prs/user/releases/tree`.
- **Custom JS injection**: `WithInjectJS(paths...)`, `WithInjectDir(dir)`, `WithInjectCode(code...)` read JS files and inject via `EvalOnNewDocument` before page scripts on every `NewPage()`. CLI: `scout inject <url> --code/--file/--dir`.
- **User profile encryption**: `SaveProfileEncrypted(path, passphrase)` and `LoadProfileEncrypted(path, passphrase)` use AES-256-GCM + Argon2id (via `scraper/crypto.go`). `MergeProfiles(base, overlay)` merges two profiles (overlay wins). `DiffProfiles(a, b)` returns `ProfileDiff` with added/removed/changed fields. `Validate()` checks required fields and cookie/storage formats.
- **WebMCP discovery**: `Page.DiscoverWebMCPTools()` scans for MCP tools via `<meta name="mcp-server">`, `<meta name="mcp-tools">`, `<link rel="mcp">`, `<script type="application/mcp+json">`, and `/.well-known/mcp`. `Page.CallWebMCPTool(name, params)` invokes via JSON-RPC 2.0 or `window.__mcp_tools[name]()`. CLI: `scout webmcp discover/call`.
- **Async job system**: `AsyncJobManager` with persistent JSON state in `~/.scout/jobs/`. Job lifecycle: create → running → completed/failed/cancelled. `RegisterCancel(id, fn)` for cancellation callbacks. CLI: `scout jobs list/status/cancel`.
- **Smart wait**: `WaitFrameworkReady()` detects the page framework and waits for framework-specific readiness (React hydration, Angular NgZone, Vue nextTick, etc.) with 5s timeout fallback.
- **Browser AutoFree**: `WithAutoFree(interval)` starts a background goroutine that periodically recycles the browser process to prevent memory leaks. `recycleBrowser()` saves page URLs and cookies, closes browser, re-launches with same options, restores state. `WithAutoFreeCallback(fn)` for recycle notifications. Goroutine stops when `Browser.Close()` is called.
- **WebSearch multi-engine**: `WithSearchEngines("google", "bing", "duckduckgo")` runs the same query across multiple engines. Results merged via Reciprocal Rank Fusion (k=60). `WithSearchDomain()` appends `site:` filter, `WithSearchExcludeDomain()` appends `-site:` filters.
- **WebFetch retry**: `WithFetchRetries(n)` and `WithFetchRetryDelay(d)` add retry logic around page navigation. `RedirectChain []string` in `WebFetchResult` tracks redirect hops.
- **Recipe validation**: `recipe.ValidateRecipe(browser, recipe)` navigates to URL, checks all selectors resolve, returns `ValidationResult` with errors and sample item count. `SelectorHealthCheck(page, selectors)` returns per-selector match counts. CLI: `scout recipe test --file=recipe.json`.
- **Selector resilience scoring**: `ScoreSelector(selector)` returns `SelectorScore` with stability rating (attribute-based > class-based > nth-child). `ScoreRecipeSelectors(recipe)` scores all selectors in a recipe. Higher scores = more stable selectors.
- **Interactive recipe creation**: `InteractiveCreate(ctx, browser, url)` provides a step-by-step guided wizard for building recipes. Shows container candidates, lets user pick fields. CLI: `scout recipe create <url> --interactive` or `scout recipe create -i`.
- **Bridge WebSocket**: `BridgeServer` manages WebSocket connections between Go and the bridge extension. `WithBridgePort(port)` configures the WS port. `BridgeMessage` for request/response, `BridgeEvent` for browser-to-Go event streaming (mutations, interactions, navigation). CLI: `scout bridge events`, `scout bridge ws-send`.
- **Profile gRPC RPCs**: `CaptureProfile` RPC captures running session state as a portable profile. `LoadProfile` RPC applies a profile to an existing session. CLI: `scout profile session-capture`, `scout profile session-load`.
- **Docker CI/CD**: `.github/workflows/docker.yml` builds and pushes images to GHCR on tag. Multi-arch (`linux/amd64`, `linux/arm64`) via `docker buildx`. Trivy vulnerability scanning. Helm chart at `deploy/helm/scout/`.
- **Built-in extraction helpers**: `InjectHelper(page, helper)` and `InjectAllHelpers(page)` inject bundled JS utilities. Constants: `HelperTableExtract` (table→JSON), `HelperInfiniteScroll` (scroll detection), `HelperShadowQuery` (shadow DOM), `HelperWaitForSelector` (wait), `HelperClickAll` (batch click). Injected via `EvalOnNewDocument`.
- **Script templates**: `ScriptTemplate` wraps Go `text/template` for parameterized JS. `RenderTemplate(name, params)` renders, `InjectTemplate(page, name, params)` renders + injects. `BuiltinTemplates` includes `extract-list`, `fill-form`, `scroll-and-collect`.
- **Bridge DOM commands**: `QueryDOM`, `ClickElement`, `TypeText`, `InsertHTML`, `RemoveElement`, `ModifyAttribute` bridge commands for DOM manipulation from Go. `ObserveDOM` for MutationObserver streaming. CLI: `scout bridge query/click/type/dom`.
- **Bridge clipboard/tabs**: `GetClipboard`/`SetClipboard` for clipboard access, `ListTabs`/`CloseTab` for tab management. `ConsoleMessages` for console capture/forwarding. CLI: `scout bridge tabs/clipboard`.
- **Bridge interaction recording**: `BridgeRecorder` captures user interactions (clicks, typing, navigation) as `RecordedStep` events and exports as `RecordedRecipe` compatible with the recipe system. CLI: `scout bridge record [--output=recipe.json]`.
- **Recipe flow detection**: `DetectFlow(ctx, browser, url)` analyzes multi-page transitions (login → dashboard → settings) via `FlowStep` and `FormInfo` types. `GenerateFlowRecipe(steps)` produces multi-step automate recipes. `ValidateWithLLM(provider, recipe)` reviews recipes for completeness via `LLMValidation`. CLI: `scout recipe flow <url>`.
- **Profile extension resolution**: `ResolveExtensions(profile)` and `ResolveExtensionsWithBase(profile, basePath)` resolve extension IDs in a profile to local filesystem paths via `extensionPathByID()`, warning on missing extensions. `scout session create --profile=<file>` applies full profile at session creation.
- **gRPC InjectJS RPC**: `InjectJS` RPC injects JavaScript into running sessions dynamically. Session-scoped, persists across navigations via `EvalOnNewDocument`.

## Testing

- `pkg/scout/testutil_test.go` provides `newTestServer()` and `newTestBrowser(t)` (headless, no-sandbox, auto-cleanup).
- Route registration: test files call `registerTestRoutes(fn)` in `init()` to add httptest routes. The `newTestServer()` function collects all registered routes.
- Core routes: `/`, `/page2`, `/json`, `/echo-headers`, `/set-cookie`, `/redirect`, `/slow`
- Extract routes: `/extract`, `/table`, `/meta`, `/links`, `/nested`, `/products-list`
- Form routes: `/form`, `/form-csrf`, `/wizard-step1`, `/wizard-step2`, `/submit`
- Paginate routes: `/products-page{1,2,3}`, `/api/products`, `/infinite`, `/load-more`
- Search routes: `/serp-google`, `/serp-google-page2`, `/serp-bing`, `/serp-ddg`
- Crawl routes: `/crawl-start`, `/crawl-page{1,2,3}`, `/sitemap.xml`
- Map routes: `/map-start`, `/map-page1`, `/map-page1-sub`, `/map-page2`, `/map-page3`
- Markdown routes: `/markdown`
- Swagger routes: `/swagger/`, `/swagger/spec`, `/swagger/v2`, `/swagger/v2/spec`, `/redoc/`, `/not-swagger`
- Recorder routes: `/recorder-page`, `/recorder-asset`, `/recorder-api`
- WebFetch routes: `/webfetch`, `/webfetch-minimal`
- Detect routes: `/detect-react`, `/detect-nextjs`, `/detect-vue`, `/detect-angular`, `/detect-svelte`, `/detect-jquery`, `/detect-none`, `/detect-gatsby`, `/detect-astro`
- Tech stack routes: `/detect-tech-wordpress`, `/detect-tech-react-vite`, `/detect-tech-plain`
- Render mode routes: `/detect-render-csr`, `/detect-render-ssr`, `/detect-render-ssg`, `/detect-render-nextjs-ssp`, `/detect-render-plain`
- PWA routes: `/detect-pwa-full`, `/detect-pwa-none`, `/detect-pwa-manifest-only`, `/pwa-manifest.json`
- GitHub routes: `/github-repo`, `/github-issues`, `/github-prs`, `/github-user`, `/github-releases`
- Snapshot routes: `/snapshot-basic`, `/snapshot-form`, `/snapshot-nested`, `/snapshot-hidden`
- Challenge routes: `/challenge-cloudflare`, `/challenge-turnstile`, `/challenge-recaptcha-v2`, `/challenge-hcaptcha`, `/challenge-datadome`, `/challenge-none`, `/challenge-multi`
- WebMCP routes: `/webmcp-meta`, `/webmcp-script`, `/webmcp-js`, `/webmcp-none`, `/mcp-api`, `/mcp-tools.json`, `/.well-known/mcp`
- Bridge test routes: WebSocket server unit tests, message routing, event streaming, DOM manipulation commands (QueryDOM, ClickElement, TypeText, InsertHTML, RemoveElement, ModifyAttribute), clipboard, tab management, console forwarding, interaction recording (BridgeRecorder)
- Snapshot test routes: `/snapshot-basic`, `/snapshot-form`, `/snapshot-nested`, `/snapshot-hidden`, iframe traversal tests, LLM integration tests
- Helper/template test routes: extraction helper injection, script template rendering + injection
- Recipe CLI integration test routes: `recipe create` and `recipe test` end-to-end CLI tests
- Inject test routes: uses inline httptest servers for JS injection verification
- Profile tests: uses `t.TempDir()` for encrypted save/load, merge, diff, validation
- Async job tests: uses `t.TempDir()` for job manager persistence
- Stability tests: `TestWaitSafe_NilPage`, `TestWaitSafe_Normal`, `TestHijack_InvalidRegexp` in `stability_test.go`
- Bot detection tests: external sites (bot.sannysoft.com, arh.antoinevastel.com, pixelscan.net, brotector, fingerprint.com) — skipped with `-short`
- Window tests: no routes needed — window control operates on the browser window itself
- Tests use `t.Skipf` when browser is unavailable — they will not fail in headless CI without Chrome, they skip.
- No mocking framework; tests run against a real headless browser and local HTTP test server.

## Dependencies

### Core library (pkg/scout/)

- `github.com/go-rod/rod` — core browser automation via Chrome DevTools Protocol
- `github.com/ysmood/gson` — JSON number handling for `EvalResult`
- `golang.org/x/time/rate` — token bucket rate limiter for `RateLimiter`
- `golang.org/x/net/html` — HTML tokenizer/parser for markdown converter (indirect dep)
- `github.com/ollama/ollama` — Ollama Go client for local LLM inference
- `github.com/modelcontextprotocol/go-sdk/mcp` — Official MCP Go SDK for stdio transport

### Stealth (pkg/stealth/)

- Internalized fork of `go-rod/stealth` plus custom `ExtraJS` evasions — anti-bot-detection page creation (enabled via `WithStealth()` or `SCOUT_STEALTH=true`)
- Core JS from `extract-stealth-evasions` v2.7.3 (navigator.webdriver, chrome.runtime, Permissions, WebGL, plugins, etc.)
- Extra JS (`stealth_extra.go`): canvas/audio fingerprint noise, WebGL vendor spoofing, navigator.connection, Notification.permission
- Chrome launch flag: `disable-blink-features=AutomationControlled` set in `browser.go` when stealth enabled

### Identity & Discovery (pkg/identity/, pkg/discovery/)

- `golang.org/x/crypto` — Ed25519 key generation, certificate creation
- `github.com/grandcat/zeroconf` — mDNS service advertisement and discovery

### gRPC layer and CLI (grpc/ and cmd/scout/)

- `google.golang.org/grpc` — gRPC framework
- `google.golang.org/protobuf` — Protocol Buffers runtime
- `github.com/google/uuid` — session ID generation
- `github.com/spf13/cobra` — CLI framework

Note: The core library does NOT import gRPC or Cobra. Library-only consumers pull zero CLI/gRPC dependencies.

## CI

GitHub Actions (`.github/workflows/test.yml`) uses reusable `inovacc/workflows` — runs tests, lint, and vulnerability checks on push/PR to non-main branches.
