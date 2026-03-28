# Milestones

## v0.1.0 - Core API [COMPLETE]

**Goal:** Functional browser automation library with essential features.

- [x] Browser creation with functional options
- [x] Page navigation, content access, element finding
- [x] Element interaction (click, input, select, hover)
- [x] JavaScript evaluation with typed results
- [x] Screenshots (viewport, full-page, PNG/JPEG)
- [x] PDF generation
- [x] Network control (headers, cookies, hijacking, URL blocking)
- [x] Stealth mode, incognito mode, device emulation
- [x] DOM tree traversal and element-scoped queries
- [x] Wait conditions for page and elements
- [x] CI pipeline (test, lint, vulncheck on GitHub Actions)
- **Test Coverage:** 33.2% (option 100%, browser ~60%, page/element/network partial)

## v0.2.0 - Scraping Toolkit [COMPLETE]

**Goal:** Full-featured web scraping, search, and interaction toolkit.

- [x] Struct-tag extraction engine (`scout:"selector"` / `scout:"selector@attr"`)
- [x] Table extraction (headers + rows, map format)
- [x] Page metadata extraction (title, description, OG, Twitter, JSON-LD)
- [x] Convenience extractors (ExtractText, ExtractTexts, ExtractLinks, ExtractAttribute)
- [x] Form detection, filling (map + struct tags), CSRF token, submit
- [x] Multi-step form wizard
- [x] Rate limiting with token bucket and retry/backoff
- [x] Pagination: click-next, URL-pattern, infinite-scroll, load-more (generics)
- [x] Search engine integration (Google, Bing, DuckDuckGo SERP parsing)
- [x] BFS web crawling with depth/page limits, domain filtering
- [x] Sitemap.xml parser
- [x] Tests for all new features
- **New dependency:** `golang.org/x/time/rate`

## v0.1.5 - Window Control & Storage [COMPLETE]

**Goal:** Browser state management and window control.

- [x] Window control: minimize, maximize, fullscreen, restore
- [x] Get/set window bounds (position, dimensions)
- [x] localStorage get/set/clear
- [x] sessionStorage get/set/clear
- [x] Save/load full session state (URL, cookies, storage)
- [x] Per-OS launch options (`option_unix.go`, `option_windows.go`)
- [x] Tests for window control and storage features

## v0.3.0 - HAR Recording & gRPC Remote Control [COMPLETE]

**Goal:** Network traffic recording and remote browser control via gRPC.

- [x] HAR 1.2 network recording via CDP events (`recorder.go`)
- [x] `NetworkRecorder` with functional options (`WithCaptureBody`, `WithCreatorName`)
- [x] Page-level keyboard input (`KeyPress`, `KeyType`)
- [x] gRPC service definition with 25+ RPCs (`grpc/proto/scout.proto`)
- [x] Multi-session gRPC server with event streaming (`grpc/server/`)
- [x] Server binary with reflection and graceful shutdown (`cmd/server/`)
- [x] Interactive CLI client with event streaming (`cmd/client/`)
- [x] Bidirectional streaming example workflow (`cmd/example-workflow/`)
- [x] HAR recorder example (`examples/advanced/har-recorder/`)
- [x] Tests for NetworkRecorder and keyboard input
- **New dependencies:** `google.golang.org/grpc`, `google.golang.org/protobuf`, `github.com/google/uuid`

## v0.4.0 - Scraper Modes & Encrypted Sessions [COMPLETE]

**Goal:** Pluggable scraper framework with encrypted session persistence.

- [x] Base scraper types (Credentials, Progress, AuthError, RateLimitError)
- [x] AES-256-GCM + Argon2id encryption utilities (`scraper/crypto.go`)
- [x] Slack scraper mode: browser auth, API client, channels, messages, threads, files, users, search
- [x] Encrypted session capture and persistence (`scraper/slack/session.go`)
- [x] CLI for Slack session management (capture, load, decrypt)
- **Test Coverage:** scraper 84.3% (Slack mode removed)

## v0.5.0 - Unified CLI [COMPLETE]

**Goal:** Single Cobra CLI binary replacing all separate command binaries.

- [x] Move core library to `pkg/scout/` (import: `github.com/inovacc/scout/pkg/scout`)
- [x] Cobra CLI scaffold with persistent flags, daemon management, session tracking
- [x] Port `cmd/server/` and `cmd/client/` into `scout server` / `scout client`
- [x] Browser control commands via gRPC (navigate, click, type, screenshot, etc.)
- [x] Standalone scraping commands (search, crawl, table, meta, form)
- [x] Port `cmd/slack-assist/` into `scout slack capture/load/decrypt`
- [x] Remove old separate binaries
- [x] Update documentation (README, CLAUDE.md, ROADMAP)
- **New dependency:** `github.com/spf13/cobra`
- **Test Coverage:** pkg/scout 76.7% | scraper 84.3%

## v0.6.0 - ~~Firecrawl Integration~~ [REMOVED]

- ~~Firecrawl client removed — project focuses on native browser-based scraping~~

## v0.7.0 - Markdown, URL Map, Identity & mTLS [COMPLETE]

**Goal:** Native HTML-to-Markdown, URL discovery, device identity, and mTLS.

- [x] Pure Go HTML-to-Markdown converter with readability scoring (`markdown.go`, `readability.go`)
- [x] `page.Markdown()` and `page.MarkdownContent()` methods
- [x] CLI: `scout markdown --url=<url> [--main-only]`
- [x] URL Map / Link Discovery (`map.go`) combining sitemap + BFS link harvesting
- [x] CLI: `scout map <url> [--search=term] [--limit=N]`
- [x] Internalized `go-rod/stealth` into `internal/engine/stealth/`
- [x] Multi-browser support: Brave, Edge auto-detection
- [x] Chrome extension loading via `WithExtension()`
- [x] Syncthing-style device identity (`pkg/identity/`)
- [x] mTLS authentication (`grpc/server/tls.go`)
- [x] Device pairing handshake (`grpc/server/pairing.go`)
- [x] mDNS peer discovery (`pkg/discovery/`)
- [x] Platform-specific session defaults (`grpc/server/platform_*.go`)
- [x] Batch scraper (`pkg/scout/batch.go`)
- [x] Multi-engine search (`cmd/scout/search_engines.go`)
- [x] Recipe system (`pkg/scout/recipe/`)
- [x] CLI introspection: `scout aicontext`, `scout cmdtree`
- [x] mTLS fix for all CLI commands
- [x] Server session timeout fix (disable rod per-page timeout)
- [x] Tagged v0.7.0, v0.7.1, v0.7.2
- **Coverage:** pkg/scout 75.0% | pkg/identity 81.1% | scraper 84.3%

## v0.7.4 - Extension Download & CRX Support [COMPLETE]

**Goal:** Download Chrome extensions from Web Store, CRX2/CRX3 unpacking, persistent extension storage.

- [x] `DownloadExtension(id)` — download CRX from Chrome Web Store, unpack to `~/.scout/extensions/<id>/`
- [x] CRX3 format parsing (magic + version + protobuf header + ZIP)
- [x] CRX2 format parsing (magic + version + pubkey + sig + ZIP)
- [x] HTTP timeout (60s) for CRX downloads
- [x] `ListLocalExtensions()`, `RemoveExtension(id)`, `ExtensionDir()`
- [x] `WithExtensionByID(ids...)` option to load downloaded extensions by ID
- [x] Extension ID resolution in `New()` before browser launch
- [x] CLI: `scout extension download <id>`, `scout extension remove <id>`
- [x] Updated `scout extension list` to show `~/.scout/extensions/` entries
- [x] Zip-slip protection in CRX extraction
- [x] Unit tests for CRX2/CRX3 unpacking, manifest parsing, listing, removal
- **Coverage:** pkg/scout 75.0%

## v0.7.5 - LLM-Powered Extraction [COMPLETE]

**Goal:** AI-powered data extraction with multi-provider LLM support and review pipeline.

- [x] `LLMProvider` interface with `Name()` + `Complete(ctx, system, user) (string, error)`
- [x] Ollama provider (`llm_ollama.go`) — local LLM via `github.com/ollama/ollama/api`
- [x] OpenAI-compatible provider (`llm_openai.go`) — covers OpenAI, OpenRouter, DeepSeek, Gemini
- [x] Anthropic provider (`llm_anthropic.go`) — Messages API
- [x] `ExtractWithLLM()` and `ExtractWithLLMJSON()` on `*Page`
- [x] LLM Review pipeline: `ExtractWithLLMReview()` — extract with LLM1, review with LLM2
- [x] Workspace persistence: filesystem session/job tracking (`llm_workspace.go`)
- [x] CLI: `scout extract-ai`, `scout ollama list/pull/status`, `scout ai-job list/show/session`
- [x] 40+ tests with mock providers, httptest servers, workspace lifecycle
- **New dependency:** `github.com/ollama/ollama`
- **Coverage:** pkg/scout 75.7%

## v0.28.0 - Scraper Framework & Coverage [COMPLETE]

**Goal:** Pluggable scraper modes, proxy chain, visual diff, fuzz testing.

- [x] 19 scraper modes (Slack, Teams, Discord, Reddit, Gmail, Outlook, LinkedIn, Jira, Confluence, Twitter/X, YouTube, Notion, Google Drive, SharePoint, Salesforce, Amazon, Google Maps, Cloud Consoles, Grafana/Datadog)
- [x] Proxy chain support (WithProxyChain, ValidateProxyChain)
- [x] Visual regression testing (VisualDiff with threshold)
- [x] Fuzz testing for recipe/runbook parser
- [x] gRPC streaming test coverage raised to 80.6%
- **Coverage:** pkg/scout/scraper 91.7% | pkg/scout/recipes 91.7% | pkg/scout/identity 81.1%

## v0.49.0 - Internal Migration & Process Management [COMPLETE]

**Goal:** Move core to `internal/engine/`, add gops-based process management.

- [x] Migrate `pkg/scout/` core to `internal/engine/` with public facade
- [x] Extract domain sub-packages (detect, fingerprint, hijack, llm, vpn, session, stealth)
- [x] Internalize rod into `internal/engine/lib/`
- [x] gops agent for process discovery
- [x] `IsScoutProcess()` for reliable orphan detection
- [x] `Page.WaitClose()` for browser window close detection
- [x] Synchronous session directory cleanup
- [x] Platform-specific process files (`_windows.go`, `_linux.go`)
- [x] Browser manifest (`browser.json`) with per-platform download configuration
- [x] Session reuse (`WithReusableSession()`), REPL mode, health checker, page gather, cloud upload
- [x] PDF form filling (`PDFFormFields()`, `FillPDFForm()`)
- [x] Test coverage improvements for browser and session packages
- **Coverage:** internal/engine/browser 53.7% | internal/engine/llm 70.4% | internal/engine/session 78.9%

## v0.53.0 - Plugin System [COMPLETE]

**Goal:** Subprocess-based plugin extensibility for scraper modes, extractors, and MCP tools.

- [x] Plugin manifest (`plugin.json`) with validation and capability declaration
- [x] JSON-RPC 2.0 protocol (request/response/notification) over stdin/stdout
- [x] Subprocess client with lazy launch, health monitoring, graceful shutdown
- [x] Plugin manager with discovery from `~/.scout/plugins/` and `$SCOUT_PLUGIN_PATH`
- [x] `ModeProxy` implementing `scraper.Mode` via JSON-RPC forwarding
- [x] `ToolProxy` forwarding MCP tool calls to plugin subprocess
- [x] `extractorProxy` implementing `Extractor` interface via JSON-RPC
- [x] Go SDK for plugin authors (`pkg/scout/plugin/sdk/`) with `Server`, `RegisterMode/Extractor/Tool`, `Run()`
- [x] Example plugin (`pkg/scout/plugin/sdk/example_plugin/`)
- [x] CLI: `scout plugin list/install/remove/run`
- [x] Scraper CLI fallback to plugin manager for unknown modes
- [x] MCP server integration via `ServerConfig.PluginManager`
- [x] Unit tests for manifest, protocol, manager, mode proxy, tool proxy
- **Coverage:** pkg/scout/plugin 36.2%

## v0.54.0 - OpenTelemetry Tracing & Plugin URL Install [COMPLETE]

**Goal:** Full observability for MCP tools and scraper operations, plus URL-based plugin installation.

- [x] `internal/tracing/` package: `Init()`, `Tracer()`, `Start()`, `MCPToolSpan()`, `ScraperSpan()`
- [x] No-op by default; enabled via `SCOUT_TRACE=1` or `OTEL_EXPORTER_OTLP_ENDPOINT`
- [x] All 33 MCP tools auto-instrumented via `addTracedTool()` wrapper
- [x] Scraper CLI instrumented with `ScraperSpan()` in `scout scrape run`
- [x] `tracing.Init()` wired into CLI bootstrap (`cmd/scout/scout.go`)
- [x] `scout plugin install <url>` downloads archives, extracts, finds `plugin.json`, installs
- [x] Test suite for tracing package (6 tests)
- [x] Extended browser and MCP test coverage
- **New dependencies:** `go.opentelemetry.io/otel` v1.41.0 and related packages

## v0.55.0 - Phase 55: MCP Enhancements & CDP Connect [COMPLETE]

**Goal:** New MCP tools, enhanced snapshot options, and CDP browser connection.

- [x] `search_and_extract` MCP tool: combined web search + browser-rendered content extraction in one call
- [x] `scout connect` CLI: connect to running browser via CDP endpoint for real-browser-profile automation (`cmd/scout/connect.go`)
- [x] Enhanced `snapshot` MCP tool: `maxDepth`, `iframes`, `filter` options for fine-grained accessibility tree control
- [x] MCP timeout fix: `WithTimeout(0)` disables rod 30s page timeout; `WaitLoad` best-effort with 15s cap
- [x] Session reset fix: close page before browser + 500ms delay for OS cleanup
- **Tag:** v0.49.0
- **Coverage:** 20.8% overall

## v0.56.0 - Phase 56–57: Guide Generator & Session Lifecycle [COMPLETE]

**Goal:** Step-by-step guide recording, session startup cleanup, and directory restructure.

- [x] `pkg/scout/guide/` — Recorder for capturing browser sessions as step-by-step guides with `RenderMarkdown()`
- [x] `CleanStaleSessions()` — startup cleanup removes non-reusable/orphaned session directories
- [x] Session dir restructured: `<hash>/{scout.pid, job.json, data/}` separates metadata from browser profile
- [x] `DataDir(id)` / `SessionDataDir(id)` API for browser user-data directory
- [x] `job.json` session tracking with type, status, progress, steps
- [x] Windows file lock retries (3×200ms) in `Reset()` and `CleanStaleSessions()`
- [x] Launcher cleanup made synchronous for non-reusable sessions
- [x] Recipe→runbook deprecation compat aliases
- [x] MCP test coverage expansion (ping, curl, redirect, browser tools)
- **Coverage:** internal/engine/session 83.1% | pkg/scout/guide 100% | internal/tracing 82.7% | Total 13.0%

## v0.58.0 - Phase 58: Swarm Mode, Reports & Coverage [COMPLETE]

**Goal:** Distributed crawling, AI-consumable reports, ManagedPagePool, massive test coverage expansion.

- [x] Distributed crawling: `internal/engine/swarm/` with Coordinator, Worker, DomainQueue
- [x] gRPC swarm transport: JoinSwarm, LeaveSwarm, FetchBatch, SubmitResults, SwarmStatus RPCs
- [x] CLI: `scout swarm start <url>`, `scout swarm join <addr>`, `scout swarm status`
- [x] Swarm proxy support: `--proxy` flag with round-robin assignment per worker
- [x] Report system: `~/.scout/reports/{uuidv7}.txt` with AI-consumable markdown format
- [x] Three report types: health_check, gather, crawl — each with tailored AI instructions
- [x] Report CLI: `scout report list/show/delete`, `scout report schedule <url> --every 1h`
- [x] `--report` flag on: test-site, gather, crawl, swarm start
- [x] MCP report tools: report_list, report_show, report_delete
- [x] MCP swarm tool: swarm_crawl (41 tools total)
- [x] `ManagedPagePool` for concurrent page scraping with acquire/release lifecycle
- [x] Default browser fallback via `BestCached()` — fixes "Failed to get debug url" on Windows
- [x] Deprecated `pkg/scout/recipe/` removed, consumers migrated to `runbook` directly
- [x] ADR-0008: Distributed Crawling design document
- [x] Scroll-capture example (`examples/simple/scroll-capture/`)
- [x] E2E gRPC swarm integration tests (5 tests)
- [x] ~850+ new tests: all 19 scraper modes, archive, fingerprint, plugin/sdk, hijack, vpn, detect, stealth, llm, report system
- [x] Fixed 8 browser-dependent tests to skip in -short mode
- [x] Taskfile test targets fixed (`./scraper/...` → `./internal/...`)
- [x] `task test:unit` passes with 0 failures
- **Coverage:** All 19 scraper modes tested | internal/engine/report 100% | 41 MCP tools

## v1.0.0 - Claude Code Plugin, Mobile, Cloud & AI Agent Integration [COMPLETE]

**Goal:** Major release bringing Claude Code plugin packaging, mobile browser automation, cloud deployment, and AI agent HTTP API.

**Claude Code Plugin (Phase 73.7):**
- [x] `.claude-plugin/plugin.json` manifest with metadata and keywords
- [x] `.mcp.json` config — `scout mcp --headless --stealth` via stdio
- [x] 6 skills: `/scout:scrape`, `/scout:screenshot`, `/scout:test-site`, `/scout:gather`, `/scout:crawl`, `/scout:monitor`
- [x] 3 agents: `web-scraper`, `site-tester`, `browser-automation`
- [x] SessionStart hook with auto-download binary from GitHub Releases
- [x] `scripts/validate-plugin.sh` — comprehensive plugin validation (7 check categories)
- [x] CI workflow: `.github/workflows/plugin-validate.yml`

**Mobile Browser Automation (Phase 73):**
- [x] `WithMobile(MobileConfig{})` for ADB-connected Android Chrome
- [x] `WithTouchEmulation()` for touch simulation on desktop
- [x] Touch gestures: `Page.Touch()`, `Page.Swipe()`, `Page.PinchZoom()` via CDP
- [x] `ListADBDevices()`, `SetupADBForward()`, `RemoveADBForward()`
- [x] CLI: `scout mobile devices [--json]`, `scout mobile connect [--device --port --url]`

**WebSocket HAR Recording (Phase 73.5):**
- [x] `HARWebSocketMessage`, `HARWebSocket` types with `_webSocketMessages` extension
- [x] `Recorder.Record()` handles WSOpened/WSSent/WSReceived/WSClosed events
- [x] `ExportWebSocketHAR()` — WS-only export, `WebSocketCount()`, `WebSocketMessageCount()`

**Agent HTTP Server (Phase 73.6):**
- [x] `pkg/scout/agent/server.go` — REST API with 6 endpoints
- [x] `GET /health`, `GET /tools`, `GET /tools/openai`, `GET /tools/anthropic`, `GET /tools/schema`, `POST /call`
- [x] `GET /metrics` (Prometheus), `GET /metrics/json`
- [x] CLI: `scout agent serve [--addr --headless --stealth --idle-timeout]`

**Cloud Deployment (Phase 74):**
- [x] Helm chart: `deploy/helm/scout/` with HPA, PVC, multi-port service
- [x] CLI: `scout cloud deploy/status/scale/uninstall`
- [x] `internal/metrics/` — zero-dependency Prometheus + JSON metrics (7 counters)
- [x] Metrics wired into MCP server (navigate, screenshot, extract, tool calls, errors)

**Distribution:**
- [x] `.goreleaser.yaml` — cross-platform builds (linux/darwin/windows × amd64/arm64)
- [x] `.github/workflows/release.yml` — GoReleaser on `v*` tags
- [x] `npm/scout-browser/` (`@inovacc/scout-browser`) — npm package with auto-download binary
- [x] Fix `process_linux.go` → `process_unix.go` (`//go:build !windows`) for macOS cross-compilation

**Testing & Quality:**
- [x] 32+ new tests: metrics (6), agent server (14), mobile ADB (6), WS HAR (6)
- [x] Public facade regenerated with mobile types
- [x] Lint fixes: errcheck, forbidigo, modernize (SplitSeq, CutPrefix)
- **Coverage:** internal/metrics 100% | internal/engine/hijack 97.4%
