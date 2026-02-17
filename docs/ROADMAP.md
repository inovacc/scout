# Project Roadmap

## Current Status
**Overall Progress:** 90% Complete

## Phases

### Phase 1: Core API [COMPLETE]
- [x] Browser lifecycle management (New, Close, Pages, Version)
- [x] Functional options pattern for configuration (13 options)
- [x] Page navigation (Navigate, NavigateBack, NavigateForward, Reload)
- [x] Page content access (URL, Title, HTML)
- [x] Element finding by CSS selector, XPath, JS, text regex, coordinates
- [x] Element interaction (Click, DoubleClick, RightClick, Hover, Tap, Input, Select)
- [x] Element state inspection (Text, HTML, Attribute, Property, Visible, Interactable)
- [x] JavaScript evaluation with typed results (EvalResult)
- [x] Nil-safe and idempotent Close
- [x] Consistent error wrapping with `scout:` prefix

### Phase 2: Advanced Features [COMPLETE]
- [x] Screenshots: viewport, full-page, scroll, PNG/JPEG with quality control
- [x] PDF generation with configurable options (margins, scale, headers/footers)
- [x] Network interception via HijackRouter with glob-pattern matching
- [x] Cookie management (Set, Get, Clear)
- [x] Extra header injection with cleanup functions
- [x] URL blocking with wildcard patterns
- [x] HTTP basic authentication
- [x] Stealth mode via go-rod/stealth
- [x] Device emulation and viewport control
- [x] DOM tree traversal (Parent, Parents, Next, Previous, ShadowRoot, Frame)
- [x] Element-scoped child queries (Element, Elements, ElementByXPath, ElementByText)
- [x] Wait conditions (Load, Stable, DOMStable, Idle, RequestIdle, Visible, Interactable)
- [x] Dialog handling, element race, page activation

### Phase 3: Scraping Toolkit [COMPLETE]
- [x] **Extraction Engine** (`extract.go`) — struct-tag extraction, table/list/meta parsing, convenience text/attribute extractors
- [x] **Form Interaction** (`form.go`) — form detection, fill by map/struct, CSRF token, submit, multi-step wizard
- [x] **Rate Limiting** (`ratelimit.go`) — token bucket rate limiter, retry with exponential backoff, NavigateWithRetry
- [x] **Pagination** (`paginate.go`) — click-next, URL-pattern, infinite-scroll, load-more with generics
- [x] **Search Engine Integration** (`search.go`) — Google/Bing/DuckDuckGo SERP parsing
- [x] **Crawling** (`crawl.go`) — BFS crawl with depth/page limits, domain filtering, sitemap parser

### Phase 4: Testing & Quality [IN PROGRESS]
- [x] Test infrastructure (httptest server, newTestBrowser helper)
- [x] Browser lifecycle tests
- [x] Basic page navigation and content tests
- [x] Element click, input, attribute, visibility tests
- [x] Network header, cookie, and hijack tests
- [x] Extraction engine tests (struct, table, meta, convenience methods)
- [x] Form interaction tests (detect, fill, submit, CSRF, wizard)
- [x] Rate limiter tests (wait, retry, concurrency)
- [x] Pagination tests (URL-pattern, click, dedup, options)
- [x] Search parser tests (Google, Bing, DDG, URL cleaning)
- [x] Crawl tests (BFS, max pages, handler stop, sitemap, URL normalization)
- [x] Window control tests (minimize, maximize, fullscreen, restore, bounds)
- [x] Storage and session tests (localStorage, sessionStorage, save/load state)
- [x] NetworkRecorder tests (capture entries, export HAR, body toggle, Stop idempotency, Clear)
- [x] Keyboard input tests (KeyPress, KeyType)
- [x] EvalResult type conversion tests (String, Int, Float, Bool, IsNull, JSON, Decode)
- [x] Page method tests (NavigateForward, ScrollScreenshot, PDF, ElementByJS, ElementByText, Search, etc.)
- [x] Element method tests (DoubleClick, RightClick, Hover, Tap, Type, Press, DOM traversal, etc.)
- [x] Increase core test coverage from 69.9% to 80%+ (achieved 80.1%)

### Phase 5: Storage & Session Management [COMPLETE]
- [x] Session storage get/set/clear (`storage.go`)
- [x] Local storage get/set/clear (`storage.go`)
- [x] Save/load full session state (URL, cookies, storage) (`storage.go`)
- [x] Window control: minimize, maximize, fullscreen, restore, bounds (`window.go`)

### Phase 6: HAR Recording & gRPC Remote Control [COMPLETE]
- [x] **HAR Network Recording** (`recorder.go`) — capture HTTP traffic via CDP events, export HAR 1.2 format
- [x] **Keyboard Input** (`page.go`) — `KeyPress(key)` and `KeyType(keys...)` for page-level keyboard control
- [x] **gRPC Service Layer** (`grpc/`) — protobuf service definition, multi-session server with 25+ RPCs
- [x] **gRPC Server Binary** (`cmd/server/`) — standalone gRPC server with reflection and graceful shutdown
- [x] **Interactive CLI Client** (`cmd/client/`) — command-driven browser control with event streaming
- [x] **Example Workflow** (`cmd/example-workflow/`) — bidirectional streaming demo

### Phase 7: Scraper Modes [IN PROGRESS]
- [x] **Scraper mode architecture** (`scraper/`) — base types (Credentials, Progress, AuthError, RateLimitError), ExportJSON, ProgressFunc callback
- [x] **Generic auth framework** (`scraper/auth/`) — Provider interface, Registry, BrowserAuth flow, BrowserCapture (capture all data before close), OAuth2 PKCE server, Electron CDP connection, encrypted session persistence
- [x] **Slack mode** (`scraper/slack/`) — workspace auth (token + browser), channel listing, message history with threads, file listing, user directory, search, channel export
- [x] **Slack auth provider** (`scraper/slack/provider.go`) — SlackProvider implements auth.Provider for generic framework
- [x] **Slack session capture** (`scraper/slack/session.go`) — CaptureFromPage, encrypted save/load (AES-256-GCM + Argon2id)
- [x] **Encryption utilities** (`scraper/crypto.go`) — EncryptData/DecryptData with passphrase-based key derivation
- [x] **Slack Assist CLI** (`cmd/slack-assist/`) — capture, load, decrypt subcommands for browser-assisted credential management
- [x] **Generic auth CLI** (`cmd/scout/internal/cli/auth.go`) — `scout auth login/capture/status/logout/providers`
- [ ] **Teams mode** (P2) — Microsoft SSO, chat/channel messages, meeting history, shared files
- [ ] **Discord mode** (P2) — server/channel messages, threads, member lists, roles, pins
- [ ] **Gmail mode** (P2) — Google auth + 2FA, email content, labels, attachments, contacts
- [ ] **Outlook mode** (P2) — Microsoft SSO, emails, folders, calendar events, contacts
- [ ] **LinkedIn mode** (P2) — profile data, posts, jobs, connections, company pages
- [ ] **Jira/Confluence modes** (P2) — Atlassian auth, issues, boards, pages, spaces
- [ ] **Social/productivity modes** (P3) — Twitter, Reddit, YouTube, Notion, GitHub, etc.
- [ ] **E-commerce modes** (P3) — Amazon, Google Maps
- [ ] **Cloud/monitoring modes** (P3) — AWS/GCP/Azure consoles, Grafana, Datadog

### Phase 8: Unified CLI [IN PROGRESS]
- [x] Move core library to `pkg/scout/` (import: `github.com/inovacc/scout/pkg/scout`)
- [x] Cobra CLI scaffold with persistent flags, daemon management, session tracking
- [x] Port `cmd/server/` and `cmd/client/` into `scout server` / `scout client` subcommands
- [x] Browser control commands via gRPC: navigate, back, forward, reload, click, type, select, hover, focus, clear, key
- [x] Inspection commands: title, url, text, attr, eval, html
- [x] Capture commands: screenshot, pdf, har start/stop/export
- [x] Window and storage commands: window get/min/max/full/restore, storage get/set/list/clear
- [x] Network commands: cookie get/set/clear, header, block
- [x] Standalone scraping commands: search, crawl, table, meta, form detect/fill/submit
- [x] Port `cmd/slack-assist/` into `scout slack capture/load/decrypt`
- [x] Remove old `cmd/server/`, `cmd/client/`, `cmd/example-workflow/`, `cmd/slack-assist/`
- [x] Update documentation (README, CLAUDE.md)

### Phase 9: Firecrawl Integration [COMPLETE]
- [x] Pure HTTP Go client for Firecrawl v2 REST API (`firecrawl/`)
- [x] Scrape, crawl, search, map, batch scrape, and extract endpoints
- [x] Typed errors (`AuthError`, `RateLimitError`, `APIError`)
- [x] Generic `poll[T]()` for async job completion (crawl, batch)
- [x] Functional options pattern for all endpoints
- [x] CLI commands: `scout firecrawl scrape/crawl/search/map/batch/extract`
- [x] Unit tests with mock HTTP server (13 tests, 80%+ coverage)
- [x] Integration tests behind `//go:build integration` + `FIRECRAWL_API_KEY`

### Phase 10: Native HTML-to-Markdown Engine [COMPLETE]
- [x] Pure Go HTML→Markdown converter in `pkg/scout/markdown.go`
- [x] `page.Markdown()` — convert full page HTML to clean markdown
- [x] `page.MarkdownContent()` — main content only (readability heuristics)
- [x] Support: headings, links, images, lists, tables, code blocks, bold/italic, blockquotes
- [x] Mozilla Readability-like content scoring to strip nav/footer/sidebar/ads
- [x] Functional options: `WithMainContentOnly()`, `WithIncludeImages()`, `WithIncludeLinks()`
- [x] CLI: `scout markdown --url=<url> [--main-only]`
- [x] Tests with fixture HTML pages covering all markdown element types

### Phase 11: Batch Scraper [PLANNED]
- [ ] `BatchScrape(urls []string, fn func(*Page, string) error, ...BatchOption)` in `pkg/scout/batch.go`
- [ ] Concurrent page pool with configurable parallelism (`WithBatchConcurrency(n)`)
- [ ] Per-URL result collection with error isolation (one failure doesn't abort batch)
- [ ] Progress callback (`WithBatchProgress(func(done, total int))`)
- [ ] Rate limiting integration (`WithBatchRateLimit(rl *RateLimiter)`)
- [ ] CLI: `scout batch --urls=u1,u2 --urls-file=file.txt [--concurrency=5] [--format=json]`
- [ ] Tests: concurrency, error isolation, progress, rate limiting

### Phase 12: URL Map / Link Discovery [COMPLETE]
- [x] `Map(url string, ...MapOption) ([]string, error)` in `pkg/scout/map.go`
- [x] Lightweight link-only crawl — collect URLs without full page extraction
- [x] Combine sitemap.xml parsing + on-page link harvesting
- [x] Filters: `WithMapSubdomains()`, `WithMapIncludePaths(...)`, `WithMapExcludePaths(...)`, `WithMapSearch(term)`
- [x] `WithMapLimit(n)` to cap discovered URLs
- [x] CLI: `scout map <url> [--search=term] [--include-subdomains] [--limit=100]`
- [x] Tests: link dedup, subdomain filtering, search filtering, sitemap integration

### Phase 13: LLM-Powered Extraction [PLANNED]
- [ ] `ExtractWithLLM(page *Page, prompt string, ...LLMOption)` in `pkg/scout/llm.go`
- [ ] Provider interface: `LLMProvider` with `Complete(ctx, systemPrompt, userPrompt) (string, error)`
- [ ] Built-in providers: OpenAI, Anthropic, Ollama (local)
- [ ] Pipeline: render page → convert to markdown → send markdown + prompt to LLM → parse response
- [ ] Optional JSON schema validation on LLM response (`WithLLMSchema(schema)`)
- [ ] `WithLLMProvider(provider)`, `WithLLMModel(model)`, `WithLLMTemperature(t)`
- [ ] CLI: `scout extract-ai --url=<url> --prompt="..." [--provider=ollama] [--model=llama3] [--schema=file.json]`
- [ ] Tests: mock LLM provider, prompt construction, schema validation

### Phase 14: Async Job System [PLANNED]
- [ ] Job manager in `pkg/scout/jobs.go` for long-running crawl/batch operations
- [ ] Job lifecycle: create → running → completed/failed/cancelled
- [ ] Job ID generation, status polling, cancellation
- [ ] Persistent job state in `~/.scout/jobs/` (JSON files)
- [ ] CLI: `scout jobs list`, `scout jobs status <id>`, `scout jobs cancel <id>`, `scout jobs wait <id>`
- [ ] Integration with batch scraper and crawl commands

### Phase 15: Screen Recorder [PLANNED]
- [ ] **ScreenRecorder type** (`pkg/scout/screenrecord.go`) — capture page frames via CDP `Page.startScreencast`, assemble into video
- [ ] Functional options: `WithFrameRate(fps)`, `WithQuality(0-100)`, `WithMaxDuration(d)`, `WithFormat("webm"|"mp4")`
- [ ] Frame-by-frame capture using `Page.screencastFrame` CDP events, ACK-based flow control
- [ ] Export as WebM (VP8/VP9) using pure-Go encoder or as frame directory (PNG sequence)
- [ ] Optional MP4 export via ffmpeg subprocess (detected at runtime, graceful fallback)
- [ ] `Start()` / `Stop()` / `Pause()` / `Resume()` lifecycle, nil-safe and idempotent like NetworkRecorder
- [ ] GIF export for short recordings (e.g. bug reproduction clips)
- [ ] Combine with NetworkRecorder for synchronized HAR + video forensic bundles
- [ ] gRPC RPCs: `StartScreenRecording`, `StopScreenRecording`, `ExportRecording`
- [ ] CLI commands: `scout record start [--fps=N] [--quality=N]`, `scout record stop`, `scout record export [--format=webm|gif]`
- [ ] Example: `examples/advanced/screen-recorder/`
- [ ] Tests: start/stop lifecycle, frame capture, export formats, concurrent recording with HAR

### Phase 16: Distributed Crawling [PLANNED]
- [ ] Swarm mode: split crawl workloads across multiple browser instances
- [ ] Multi-IP support: assign different proxies per browser in the cluster
- [ ] Work distribution: BFS queue shared across workers
- [ ] Result aggregation: merge results from all workers
- [ ] Headless cluster configuration options

### Phase 17: Documentation & Release [IN PROGRESS]
- [x] Publish to GitHub with git remote
- [x] Create initial git tags (v0.1.3, v0.1.4, v0.1.5)
- [x] Add LICENSE file
- [ ] Add GoDoc examples for key functions
- [ ] Write integration test examples

## Test Coverage

**Current:** 33.3% (total project) | 69.9% (pkg/scout) | **Target:** 80%

| File | Coverage | Status |
|------|----------|--------|
| option.go | 100.0% | Complete |
| browser.go | ~60% | Needs improvement |
| page.go | ~65% | Improved — PDF, scroll, search, DOM, emulation tested |
| element.go | ~65% | Improved — click variants, input, traversal, state tested |
| network.go | ~50% | Accessor methods untested |
| eval.go | ~95% | Complete — String, Int, Float, Bool, IsNull, JSON, Decode |
| extract.go | Tested | Complete |
| form.go | Tested | Complete |
| ratelimit.go | Tested | Complete |
| paginate.go | Tested | Complete |
| search.go | Tested | Complete |
| crawl.go | Tested | Complete |
| window.go | Tested | Complete |
| storage.go | Tested | Complete |
| recorder.go | Tested | Complete |
