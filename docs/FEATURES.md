# Feature Requests

## Completed Features

### Browser Automation Core

- **Status:** Completed
- **Description:** Full browser lifecycle, page navigation, element interaction, JS evaluation, screenshots, PDF, network control, stealth mode, device emulation, DOM traversal.

### Scraping Toolkit

- **Status:** Completed
- **Description:** Struct-tag extraction engine, table/metadata extraction, form detection and filling, rate limiting with retry, pagination (click/URL/scroll/load-more), search engine integration (
  Google/Bing/DDG), BFS crawling with sitemap parser.

### Window Control & Session Management

- **Status:** Completed
- **Description:** Window state control (minimize, maximize, fullscreen, restore), window bounds get/set, localStorage/sessionStorage access, save/load full session state (URL, cookies, storage).
  Implemented in `window.go` and `storage.go`.

### HAR Network Recording

- **Status:** Completed
- **Description:** Capture HTTP traffic via Chrome DevTools Protocol events, export as HAR 1.2 format. `NetworkRecorder` with functional options for body capture toggle and creator metadata.
  Implemented in `recorder.go`.

### Keyboard Input

- **Status:** Completed
- **Description:** Page-level keyboard control with `KeyPress(key)` for single keys and `KeyType(keys...)` for sequences. Uses rod `input.Key` constants. Added to `page.go`.

### gRPC Remote Control

- **Status:** Completed
- **Description:** Multi-session browser control via gRPC with 25+ RPCs covering session lifecycle, navigation, element interaction, query, capture, forensic recording, and event streaming. Includes
  bidirectional interactive streaming. Implemented in `grpc/server/`.

### Scraper Framework & Generic Auth

- **Status:** Completed
- **Description:** Pluggable scraper framework with encrypted session persistence (AES-256-GCM + Argon2id). Generic auth framework with Provider interface, browser capture, OAuth2 PKCE, Electron CDP.
  Implemented in `scraper/` and `scraper/auth/`.

### Unified CLI

- **Status:** Completed
- **Description:** Single Cobra CLI binary (`cmd/scout/`) replacing all separate binaries. Background gRPC daemon for session persistence. 40+ subcommands covering session management, navigation,
  interaction, inspection, capture, scraping, and Slack session management. File-based session tracking in `~/.scout/`.

### Retry with Backoff (Core Methods)

- **Priority:** P3
- **Status:** Completed (via RateLimiter)
- **Description:** Built-in retry logic for transient navigation and element-finding failures, with configurable backoff strategy. Implemented as `RateLimiter.Do()` and `Page.NavigateWithRetry()`.

### ~~Firecrawl Integration~~ [REMOVED]

- **Status:** Removed — `firecrawl/` package deleted in favor of native browser-based scraping

### HTML-to-Markdown Engine

- **Status:** Completed
- **Description:** Pure Go HTML-to-Markdown converter using `golang.org/x/net/html`. Readability scoring to extract main content (article/nav/footer scoring, class/ID pattern matching, link density
  penalty). Supports headings, links, images, lists, tables, code blocks, bold/italic, blockquotes. `page.Markdown()` and `page.MarkdownContent()` methods. CLI `scout markdown`. Implemented in
  `markdown.go` and `readability.go`.

### URL Map / Link Discovery

- **Status:** Completed
- **Description:** Lightweight URL-only discovery combining sitemap.xml parsing with on-page BFS link harvesting. Filters for subdomains, path patterns, search terms. Limit cap on discovered URLs. CLI
  `scout map <url>`. Implemented in `map.go`.

### Multi-Browser Support & Auto-Download

- **Status:** Completed
- **Description:** Auto-detection for Brave and Microsoft Edge browsers on Windows, macOS, and Linux. `WithBrowser(BrowserBrave)` or `--browser=brave`. Platform-specific path resolution in
  `browser_path_*.go`. Brave auto-downloads from GitHub releases if not installed locally (`browser_download.go`). Edge provides download URL in error message. Downloaded browsers cached in `~/.scout/browsers/`. CLI `scout browser list` shows detected and downloaded browsers.

### Chrome Extension Loading

- **Status:** Completed
- **Description:** Load unpacked Chrome extensions via `WithExtension(paths...)`. Sets `--load-extension` and `--disable-extensions-except` flags. CLI `scout extension load/test/list`.

### Device Identity & mTLS

- **Status:** Completed
- **Description:** Syncthing-style device IDs with Ed25519 keys and Luhn check digits (`pkg/identity/`). Mutual TLS authentication for gRPC connections (`grpc/server/tls.go`). Device pairing handshake
  for certificate exchange (`grpc/server/pairing.go`). mDNS peer discovery (`pkg/discovery/`). CLI `scout device pair/list/trust`.

### Internalized Stealth

- **Status:** Completed
- **Description:** `go-rod/stealth` forked and internalized into `internal/engine/stealth/`. Removes external dependency while maintaining anti-bot-detection capabilities.

### Platform-Specific Server Defaults

- **Status:** Completed
- **Description:** Build-constraint-based platform defaults for gRPC server sessions. Auto-applies `--no-sandbox` on Linux (containers/WSL). Windows and macOS get no extra defaults. Implemented in
  `grpc/server/platform_*.go`.

### Multi-Engine Search

- **Status:** Completed
- **Description:** Engine-specific search subcommands with registry pattern. Supports Google, Bing, DuckDuckGo (web, news, images), Wikipedia, Google Scholar, Google News. Structured JSON/text output with pagination. CLI `scout search --engine=google --query="..."` or shorthand `scout search:google "query"`. Implemented in `cmd/scout/search_engines.go`.

### Batch Scraper

- **Status:** Completed
- **Description:** Concurrent batch scraping of multiple URLs with page pool, error isolation, and progress reporting. `BatchScrape()` function with configurable concurrency, per-URL error collection, progress callback, rate limiter integration. CLI `scout batch --urls=... [--concurrency=5]`. Implemented in `pkg/scout/batch.go`.

### Recipe System

- **Status:** Completed
- **Description:** Declarative JSON recipe format for extraction and automation. Two recipe types: `extract` (data scraping with selectors and pagination) and `automate` (sequential action playbooks). CLI `scout recipe run --file=recipe.json`, `scout recipe validate --file=recipe.json`. Implemented in `pkg/scout/recipe/`.

### CLI Introspection Commands

- **Status:** Completed
- **Description:** Built-in `scout aicontext` generates AI context document with categorized commands, structure, and examples. `scout cmdtree` visualizes the full command tree with flags. Both support `--json` output. Implemented in `cmd/scout/aicontext.go` and `cmd/scout/cmdtree.go`.

### Chrome Extension Download & Management

- **Status:** Completed
- **Description:** Download Chrome extensions from the Web Store by ID, unpack CRX2/CRX3 files, and store persistently in `~/.scout/extensions/`. `DownloadExtension(id)` fetches and unpacks with zip-slip protection. `ListLocalExtensions()` and `RemoveExtension(id)` for management. `WithExtensionByID(ids...)` option loads downloaded extensions by ID at browser launch. CLI `scout extension download/remove/list`. Implemented in `pkg/scout/extension.go`.

### Scout Bridge Extension

- **Status:** Completed
- **Description:** Built-in Manifest V3 Chrome extension for bidirectional Go↔browser communication. Extension files in `extensions/scout-bridge/`, embedded via `extensions/extensions.go` using `embed.FS`, auto-loaded by default (disable with `WithoutBridge()`). WebSocket transport, event streaming, DOM observation, clipboard, tab management, recording. Implemented in `internal/engine/bridge.go` and `extensions/`.

### LLM-Powered Extraction

- **Status:** Completed
- **Description:** AI-powered data extraction using LLM providers. Send page content (as markdown) to an LLM with a natural language prompt, get structured data back. Pluggable `LLMProvider` interface with 6 built-in providers: Ollama (local), OpenAI, Anthropic, OpenRouter, DeepSeek, Gemini. Optional JSON schema validation. LLM review pipeline (`ExtractWithLLMReview()`) sends extraction output to a second LLM for quality review. Workspace persistence tracks sessions and jobs in a filesystem directory with `sessions.json`, `jobs/jobs.json`, and per-job `jobs/<uuid>/` folders. CLI `scout extract-ai`, `scout ollama list/pull/status`, `scout ai-job list/show/session`. Implemented in `pkg/scout/llm.go`, `llm_ollama.go`, `llm_openai.go`, `llm_anthropic.go`, `llm_review.go`, `llm_workspace.go`, `cmd/scout/llm.go`.

### Sitemap Extract

- **Status:** Completed
- **Description:** Crawl an entire site and extract DOM JSON + Markdown for every page using the bridge extension. `Browser.SitemapExtract()` performs BFS crawl reusing a single page + bridge across navigations. Functional options for depth, max pages, delay, DOM depth, CSS selector scoping, main-only markdown, skip JSON/Markdown, and output directory. Per-page output files (`dom.json`, `dom.md`) plus `index.json` and `index.md`. CLI `scout sitemap extract <url>`. Implemented in `pkg/scout/sitemap.go`, `cmd/scout/sitemap.go`.

### Stealth Mode — Anti-Bot-Detection

- **Status:** Completed
- **Description:** Comprehensive stealth system combining Chrome launch flags (`disable-blink-features=AutomationControlled`), core JS injection from `extract-stealth-evasions` (navigator.webdriver, chrome.runtime, Permissions, WebGL, plugins, etc.), and custom `ExtraJS` evasions (canvas/audio fingerprint noise, WebGL vendor spoofing, navigator.connection, Notification.permission). Enabled via `WithStealth()` option, `--stealth` CLI flag, or `SCOUT_STEALTH=true`. Integration tests against real bot-detection sites. Implemented in `internal/engine/stealth/` and `internal/engine/browser.go`.

### WebFetch — URL Content Extraction

- **Status:** Completed
- **Description:** Fetch any URL and return clean, structured content. `Browser.WebFetch(url, ...WebFetchOption)` returns markdown, metadata, links, and optionally raw HTML. Content modes: full, markdown, html, text, links, meta. Main content extraction via readability scoring. In-memory caching with TTL. Batch fetching with `WebFetchBatch()`. CLI `scout fetch <url> [--mode=...] [--main-only]`. Implemented in `pkg/scout/webfetch.go`, `cmd/scout/fetch.go`.

### Recipe Creator — Site Analysis & Generation

- **Status:** Completed
- **Description:** Automatically analyze a target website and generate a ready-to-run recipe JSON file. `AnalyzeSite()` navigates, inspects DOM, classifies page type (listing/form/article/table), detects containers, fields, forms, pagination, and interactable elements. `GenerateRecipe()` produces extract or automate recipes from analysis. CLI `scout recipe create <url>`. Implemented in `pkg/scout/recipe/analyze.go`, `pkg/scout/recipe/generate.go`.

### MCP Server (34 Tools)

- **Status:** Completed
- **Description:** Model Context Protocol server exposing Scout browser automation as 37 LLM-callable tools across 8 categories: Browser (navigate, click, type, back, forward, wait, screenshot, snapshot, extract, eval, open), Content (markdown, table, meta, pdf, search, fetch, search_and_extract), Network (cookie, header, block, ping, curl), Forms (form_detect, form_fill, form_submit), Analysis (crawl, detect), Inspection (storage, hijack, har, swagger), Session (session_list, session_reset). 3 resources: scout://page/markdown, scout://page/url, scout://page/title. Supports stdio and HTTP+SSE transport. CLI `scout mcp`, `scout mcp --install`, `scout mcp screenshot`, `scout mcp open`. Implemented in `pkg/scout/mcp/`.

### Multi-Tab Orchestration

- **Status:** Completed
- **Description:** TabGroup struct for coordinating actions across multiple browser pages. `Do()` for sequential, `DoAll()`/`DoParallel()` for concurrent execution, `Broadcast()` for same action on all tabs, `Navigate()`, `Wait()`, `Collect()` for data gathering. Implemented in `pkg/scout/tabgroup.go`.

### Process Management & Browser Close Detection

- **Status:** Completed
- **Description:** gops agent registration for process discovery, `IsScoutProcess()` for reliable orphan detection immune to PID reuse, `Page.WaitClose()` for detecting browser window close via CDP `TargetTargetDestroyed` event, synchronous session directory cleanup, platform-specific process files. `mcp open` now exits cleanly when the user closes the browser window.

### Screen Recorder

- **Status:** Completed
- **Description:** Capture browser sessions as video using CDP `Page.startScreencast`. GIF export. Implemented in `internal/engine/recorder.go`.

### PDF Form Filling

- **Status:** Completed
- **Description:** Detect fillable PDF form fields via `Page.PDFFormFields()` and fill them via browser rendering with `Page.FillPDFForm(fields)`. CLI: `scout pdf-form fields`, `scout pdf-form fill`. Implemented in `internal/engine/pdf_form.go` and `cmd/scout/pdf.go`.

### Knowledge Base Builder

- **Status:** Completed
- **Description:** Crawl a site and build a structured knowledge base with DOM, markdown, links, tech stack, and page metadata. `Browser.Knowledge(url, opts...)` with depth, concurrency, and timeout controls. CLI: `scout knowledge <url>`. Implemented in `internal/engine/knowledge.go`.

### Plugin System

- **Status:** Completed
- **Description:** Subprocess-based plugin extensibility via JSON-RPC 2.0. Plugins are separate executables discovered from `~/.scout/plugins/` and `$SCOUT_PLUGIN_PATH`. Supports three capability types: scraper modes, extractors, and MCP tools. Go SDK for plugin authors in `pkg/scout/plugin/sdk/`. CLI: `scout plugin list/install/remove/run`. Supports URL-based install with archive extraction. Lazy process launch with graceful shutdown.

### OpenTelemetry Tracing

- **Status:** Completed
- **Description:** Full observability via OpenTelemetry. `internal/tracing/` package with `Init()`, `MCPToolSpan()`, `ScraperSpan()`, and `Start()` helpers. No-op by default; enabled via `SCOUT_TRACE=1` or `OTEL_EXPORTER_OTLP_ENDPOINT`. All 34 MCP tools auto-instrumented via `addTracedTool()` wrapper. Scraper CLI instrumented with `ScraperSpan()`. Supports stdout and OTLP exporters.

### `search_and_extract` MCP Tool

- **Status:** Completed
- **Description:** Combined web search and browser-rendered content extraction in a single MCP tool call. Searches the web, then navigates to the top result and extracts rendered content as markdown, eliminating the need for separate `search` + `navigate` + `markdown` tool calls. Implemented in `pkg/scout/mcp/`.

### `scout connect` CLI

- **Status:** Completed
- **Description:** Connect to a running browser instance via Chrome DevTools Protocol endpoint. Enables automation of real browser profiles with existing login sessions, cookies, and extensions. CLI: `scout connect <cdp-endpoint>`. Implemented in `cmd/scout/connect.go`.

### Enhanced `snapshot` MCP Tool

- **Status:** Completed
- **Description:** Fine-grained accessibility tree control with `maxDepth`, `iframes`, and `filter` options. `maxDepth` limits tree traversal depth, `iframes` includes cross-origin iframe content, and `filter` restricts output to matching node roles or names. Provides more targeted accessibility snapshots for LLM consumption.

### Step-by-Step Guide Generator

- **Status:** Completed
- **Description:** Record browser sessions into step-by-step how-to guides. `Recorder` captures actions as `Step` entries (action, selector, value, screenshot path), produces a `Guide` struct rendered to Markdown via `RenderMarkdown()`. Implemented in `pkg/scout/guide/`.

### Session Startup Cleanup

- **Status:** Completed
- **Description:** `CleanStaleSessions()` runs automatically on every `scout` invocation to remove leftover session directories. Removes non-reusable sessions unconditionally, dead reusable sessions, and orphaned directories without `scout.pid`. Kills orphaned browser processes. Retries removal 3× with 200ms delays for Windows file locks. Implemented in `internal/engine/session/session_track.go`, called from `cmd/scout/scout.go`.

### Session Directory Restructure

- **Status:** Completed
- **Description:** Session directories restructured from flat `<hash>/` (mixed browser data and metadata) to `<hash>/{scout.pid, job.json, data/}` separating session metadata from the Chrome user-data-dir. `DataDir(id)` returns the `data/` subdirectory path. `SessionDataDir(id)` exposed via engine and public facades. Implemented in `internal/engine/session/session_track.go`.

### Job Tracking

- **Status:** Completed
- **Description:** `job.json` metadata file tracks session jobs with type, status (pending/running/completed/failed), progress (current/total/message), steps, timestamps, and output. `NewJob()`, `WriteJob()`, `StartJob()`, `CompleteJob()`, `FailJob()`, `AddJobStep()`, `UpdateJobProgress()` API. Implemented in `internal/engine/session/job.go`.

### Distributed Crawling (Swarm Mode)

- **Status:** Completed
- **Description:** Split crawl workloads across multiple browser instances with different IPs/proxies. `internal/engine/swarm/` with Coordinator (domain-partitioned BFS queue, URL dedup, worker health monitoring), Worker (pull-based batch processing with real browser), and DomainQueue (per-domain rate limiting). gRPC transport: JoinSwarm, LeaveSwarm, FetchBatch, SubmitResults, SwarmStatus RPCs. CLI: `scout swarm start <url> --workers N --proxy list`, `scout swarm join <addr> --proxy url`. ADR-0008 design document.

### ManagedPagePool

- **Status:** Completed
- **Description:** Fixed-size pool of pre-created browser pages for concurrent scraping. Context-aware `Acquire()` blocks until a page is available or context cancelled. `Release()` resets page state via `about:blank` navigation. `Close()` drains and closes all pages. Implemented in `internal/engine/pagepool.go`.

### Report System

- **Status:** Completed
- **Description:** AI-consumable reports saved to `~/.scout/reports/{uuidv7}.txt` as structured markdown with metadata, findings, analysis instructions, and embedded JSON. Three report types: health_check, gather, crawl — each with tailored AI analysis prompts. `--report` flag on test-site, gather, crawl, swarm start. `scout report list/show/delete` CLI. `scout report schedule <url> --every 1h` for recurring health checks. MCP tools: report_list, report_show, report_delete. Implemented in `internal/engine/report.go`, `cmd/scout/report.go`, `cmd/scout/report_schedule.go`, `pkg/scout/mcp/tools_report.go`.

## Proposed Features

(No proposed features at this time. See [BACKLOG.md](BACKLOG.md) for future work.)
