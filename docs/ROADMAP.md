# Project Roadmap

## Current Status

**Overall Progress:** 91% Complete

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

### Phase 4: Testing & Quality [COMPLETE]

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
- [x] **Generic auth framework** (`scraper/auth/`) — Provider interface, Registry, BrowserAuth flow, BrowserCapture (capture all data before close), OAuth2 PKCE server, Electron CDP connection,
  encrypted session persistence
- [x] **Encryption utilities** (`scraper/crypto.go`) — EncryptData/DecryptData with passphrase-based key derivation
- [x] **Generic auth CLI** (`cmd/scout/internal/cli/auth.go`) — `scout auth login/capture/status/logout/providers`
- ~~[x] **Slack mode** — removed in favor of generic auth framework~~
- [ ] **Teams mode** (P2) — Microsoft SSO, chat/channel messages, meeting history, shared files
- [ ] **Discord mode** (P2) — server/channel messages, threads, member lists, roles, pins
- [ ] **Gmail mode** (P2) — Google auth + 2FA, email content, labels, attachments, contacts
- [ ] **Outlook mode** (P2) — Microsoft SSO, emails, folders, calendar events, contacts
- [ ] **LinkedIn mode** (P2) — profile data, posts, jobs, connections, company pages
- [ ] **Jira/Confluence modes** (P2) — Atlassian auth, issues, boards, pages, spaces
- [ ] **Social/productivity modes** (P3) — Twitter, Reddit, YouTube, Notion, GitHub, etc.
- [ ] **E-commerce modes** (P3) — Amazon, Google Maps
- [ ] **Cloud/monitoring modes** (P3) — AWS/GCP/Azure consoles, Grafana, Datadog

### Phase 8: Unified CLI [COMPLETE]

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

### Phase 9: ~~Firecrawl Integration~~ [REMOVED]

- ~~Firecrawl client removed — project focuses on native browser-based scraping~~

### Phase 10: Native HTML-to-Markdown Engine [COMPLETE]

- [x] Pure Go HTML→Markdown converter in `pkg/scout/markdown.go`
- [x] `page.Markdown()` — convert full page HTML to clean markdown
- [x] `page.MarkdownContent()` — main content only (readability heuristics)
- [x] Support: headings, links, images, lists, tables, code blocks, bold/italic, blockquotes
- [x] Mozilla Readability-like content scoring to strip nav/footer/sidebar/ads
- [x] Functional options: `WithMainContentOnly()`, `WithIncludeImages()`, `WithIncludeLinks()`
- [x] CLI: `scout markdown --url=<url> [--main-only]`
- [x] Tests with fixture HTML pages covering all markdown element types

### Browser Support

| Browser         | Status      | Auto-download | Notes                                                                                     |
|-----------------|-------------|---------------|-------------------------------------------------------------------------------------------|
| Chrome/Chromium | ✅ Default   | ✅ rod built-in | rod auto-detect + auto-download                                                          |
| Brave           | ✅ Supported | ✅ GitHub releases | `WithBrowser(BrowserBrave)` — auto-downloads from `brave/brave-browser` releases if not installed |
| Microsoft Edge  | ✅ Supported | ❌ Installer only | `WithBrowser(BrowserEdge)` — error message includes download URL                         |
| Firefox         | ❌ Blocked   | N/A           | CDP removed in Firefox 141 (June 2025). Requires WebDriver BiDi maturity in Go ecosystem. |

#### Pre-Existing Browser Detection [PLANNED]

Automatically detect all Chromium-based browsers installed on the system (Chrome, Brave, Edge, Vivaldi, Opera, Arc, Chromium) and add them to the available browser list. Currently Scout only checks known paths per `BrowserType`; this feature does a full system scan.

- [ ] **System scanner** (`pkg/scout/browser_detect.go`) — scan common install paths per OS (Program Files, /Applications, /usr/bin, snap, flatpak)
- [ ] **Registry scan (Windows)** — read `HKLM\SOFTWARE\Clients\StartMenuInternet` and `App Paths` for browser executables
- [ ] **Version extraction** — run `browser --version` or parse manifest/plist to get version string
- [ ] **`DetectBrowsers() []BrowserInfo`** — return all found browsers with name, path, version, type
- [ ] **`scout browser list --detect`** — enhanced list showing all system browsers, not just Scout-managed ones
- [ ] **`WithAutoDetect()` option** — pick best available browser automatically (prefer Chrome > Brave > Edge > Chromium)
- [ ] **Tests** — mock filesystem paths, registry stubs, version parsing

### Phase 11: Batch Scraper [COMPLETE]

- [x] `BatchScrape(urls []string, fn func(*Page, string) error, ...BatchOption)` in `pkg/scout/batch.go`
- [x] Concurrent page pool with configurable parallelism (`WithBatchConcurrency(n)`)
- [x] Per-URL result collection with error isolation (one failure doesn't abort batch)
- [x] Progress callback (`WithBatchProgress(func(done, total int))`)
- [x] Rate limiting integration (`WithBatchRateLimit(rl *RateLimiter)`)
- [x] CLI: `scout batch --urls=u1,u2 --urls-file=file.txt [--concurrency=5] [--format=json]`

### Phase 12: URL Map / Link Discovery [COMPLETE]

- [x] `Map(url string, ...MapOption) ([]string, error)` in `pkg/scout/map.go`
- [x] Lightweight link-only crawl — collect URLs without full page extraction
- [x] Combine sitemap.xml parsing + on-page link harvesting
- [x] Filters: `WithMapSubdomains()`, `WithMapIncludePaths(...)`, `WithMapExcludePaths(...)`, `WithMapSearch(term)`
- [x] `WithMapLimit(n)` to cap discovered URLs
- [x] CLI: `scout map <url> [--search=term] [--include-subdomains] [--limit=100]`
- [x] Tests: link dedup, subdomain filtering, search filtering, sitemap integration

### Phase 12b: Recipe System [COMPLETE]

- [x] Declarative recipe JSON format with two types: `extract` and `automate`
- [x] Recipe types and JSON parsing (`pkg/scout/recipe/recipe.go`)
- [x] Extraction recipe executor with container/field selectors, pagination (`pkg/scout/recipe/extract.go`)
- [x] Automation recipe executor with sequential action steps (`pkg/scout/recipe/automate.go`)
- [x] Named selectors (`selectors` map) with `$name` references resolved at parse time — enables selector reuse across fields/steps
- [x] CLI: `scout recipe run --file=recipe.json`, `scout recipe validate --file=recipe.json`
- [x] Unit tests for recipe parsing (`pkg/scout/recipe/recipe_test.go`)

### Phase 12c: Recipe Creator — AI-Assisted Recipe Generation [IN PROGRESS]

Automatically analyze a target website and generate a ready-to-run recipe JSON file. Scout navigates the site, inspects the DOM structure, identifies interactive elements and data patterns, and produces an `extract` or `automate` recipe. Optionally uses an LLM to resolve ambiguous selectors, name fields semantically, and plan multi-step automation flows.

#### Site Analysis Engine

- [x] **`AnalyzeSite(ctx, browser, url, ...AnalyzeOption) (*SiteAnalysis, error)`** (`pkg/scout/recipe/analyze.go`) — navigate to URL, inspect DOM, classify page type
- [x] **Page classification** — detect page type: listing, form, article, table, unknown — via heuristic scoring
- [x] **Container detection** — find repeated DOM structures (article, card, item, product, result, li, tr, row) via count + field scoring
- [x] **Field discovery** — JS-based child inspection: headings→title, a[href]→link, img[src]→image, price patterns, time/date elements
- [ ] **Selector generation** — produce robust CSS selectors: prefer `[data-*]`, `[role]`, semantic tags over brittle `.class-hash` chains; validate uniqueness
- [ ] **Selector resilience scoring** — score selectors by stability heuristics (attribute-based > class-based > nth-child); warn on fragile selectors
- [x] **Pagination detection** — identify next-page buttons via `a[rel="next"]`, `.next`, `[aria-label*="next"]`, `.pagination a:last-child`; URL pattern detection
- [x] **Form detection** — find `<form>` elements via `DetectForms()`, map input fields (name, type, selector, placeholder, required)
- [x] **Interactive element mapping** — detect buttons (non-submit), tabs (`[role="tab"]`), toggles (`[data-toggle]`)
- [x] **SiteAnalysis type** — `URL`, `PageType`, `Containers`, `Forms`, `Pagination`, `Interactables`, `Metadata`

#### Recipe Generation (Rule-Based)

- [x] **`GenerateRecipe(analysis *SiteAnalysis, ...GenerateOption) (*Recipe, error)`** (`pkg/scout/recipe/generate.go`)
- [x] **Extract recipe generation** — from top-ranked container + fields, build `items.container`, `items.fields` map, detect `@attr` for links/images
- [x] **Automate recipe generation** — from form + interactable analysis, build sequential steps (navigate → type per field → click submit)
- [x] **Pagination wiring** — attach detected click pagination to recipe (strategy, next_selector, max_pages)
- [x] **WaitFor inference** — set `wait_for` to the container selector
- [x] **Output defaults** — set format to `json`, name derived from page title or domain
- [x] **`WithGenerateType("extract"|"automate")` option** — force recipe type instead of auto-detect
- [x] **`WithGenerateFields(fields ...string)` option** — only include specified fields in extraction recipe
- [x] **`WithGenerateMaxPages(n)` option** — set pagination max pages

#### AI-Assisted Generation (Optional LLM Enhancement)

- [ ] **`WithAI(provider LLMProvider)` option** — enable LLM-assisted recipe generation (reuses Phase 14 LLM provider interface)
- [ ] **Semantic field naming** — send container HTML sample to LLM, ask for meaningful field names ("price", "title", "rating") instead of generic ("text_1", "link_2")
- [ ] **Selector refinement** — LLM suggests more stable selectors when rule-based ones are fragile (class-hash dependent)
- [ ] **Automation planning** — given a goal description (`WithGoal("login and export CSV")`), LLM plans the step sequence: which fields to fill, buttons to click, waits to add
- [ ] **Multi-page flow detection** — LLM analyzes page transitions (login → dashboard → settings) and generates multi-step automate recipe
- [ ] **Validation prompts** — after generation, LLM reviews the recipe for completeness and suggests missing steps or error handling
- [ ] **Prompt templates** — structured prompts with page HTML context, selector candidates, and recipe schema as system prompt; user goal as user prompt
- [ ] **Fallback** — if LLM unavailable or errors, fall back to rule-based generation silently

#### Recipe Validation & Testing

- [ ] **Dry-run mode** — `ValidateRecipe(browser, recipe) (*ValidationResult, error)` navigates to URL, checks all selectors resolve, reports missing fields
- [ ] **Selector health check** — for each selector in recipe, verify it matches expected count of elements
- [ ] **Sample extraction** — run recipe on first page only, return sample items for user review before full run
- [ ] **Auto-fix suggestions** — when selectors fail, re-analyze page and suggest updated selectors

#### CLI Commands

- [x] `scout recipe create <url> [--type=extract|automate] [--output=recipe.json]` — analyze site + generate recipe
- [ ] `scout recipe create <url> --ai [--goal="scrape all products"] [--provider=ollama]` — AI-assisted generation
- [ ] `scout recipe create <url> --interactive` — step-by-step guided creation: show candidates, let user pick containers/fields
- [ ] `scout recipe test --file=recipe.json` — dry-run validation with sample output
- [ ] `scout recipe fix --file=recipe.json` — re-analyze site, update broken selectors in existing recipe

#### Testing

- [x] Container detection tests (product listing with 4 items)
- [x] Pagination detection tests (click-next with `.next` selector)
- [x] Form detection tests (login form with username/password)
- [x] Article detection tests (og:type article classification)
- [x] Metadata extraction tests (title, description from meta tags)
- [x] Generate extract recipe tests (container + fields + validate + run end-to-end)
- [x] Generate automate recipe tests (form → steps with navigate + type + click)
- [x] Force type tests (WithGenerateType override)
- [ ] AI integration tests with mock LLM provider
- [ ] CLI integration tests for `recipe create` and `recipe test`

### Multi-Engine Search [COMPLETE]

- [x] Engine-specific search subcommands (`cmd/scout/search_engines.go`)
- [x] Engines: Google, Bing, DuckDuckGo (web + news + images), Wikipedia, Google Scholar, Google News
- [x] Structured output (JSON/text), pagination support
- [x] CLI: `scout search --engine=google --query="..."` or shorthand `scout search:google "query"`

### Phase 13: Swagger/OpenAPI Extraction [COMPLETE]

- [x] **Swagger/OpenAPI detection** (`pkg/scout/swagger.go`) — auto-detect Swagger UI 3+, ReDoc, page title heuristics, inline spec from JS context
- [x] **Spec extraction** — fetch and parse OpenAPI 3.x and Swagger 2.0 specifications
- [x] **Data model** — `SwaggerSpec`, `SwaggerInfo`, `SwaggerPath`, `SwaggerServer`, `SwaggerParam`, `SwaggerSecurity` types
- [x] **URL resolution** — handle relative/absolute spec URLs, inline specs from Swagger UI store
- [x] **Schema parsing** — extract `components/schemas` (OpenAPI 3.x) and `definitions` (Swagger 2.0)
- [x] **Security definitions** — extract `securitySchemes` / `securityDefinitions`
- [x] **Functional options** — `WithSwaggerEndpointsOnly()`, `WithSwaggerRaw()`
- [x] **Browser/Page methods** — `Browser.ExtractSwagger(url, ...)` and `Page.ExtractSwagger(...)`
- [x] **CLI command** — `scout swagger <url> [--endpoints-only] [--raw] [--format=json|text] [--output=file]`
- [x] **Tests** — detection (UI 3+, 2.0, ReDoc, non-swagger), extraction, endpoints-only, schema/security parsing, JSON marshaling

### Phase 14: LLM-Powered Extraction [COMPLETE]

- [x] `ExtractWithLLM(prompt string, ...LLMOption) (string, error)` on `*Page` in `pkg/scout/llm.go`
- [x] `ExtractWithLLMJSON(prompt string, target any, ...LLMOption) error` for typed extraction
- [x] Provider interface: `LLMProvider` with `Name() string` + `Complete(ctx, systemPrompt, userPrompt) (string, error)`
- [x] Built-in providers: Ollama (`llm_ollama.go`), OpenAI-compatible (`llm_openai.go` — covers OpenAI, OpenRouter, DeepSeek, Gemini), Anthropic (`llm_anthropic.go`)
- [x] Pipeline: page.Markdown()/MarkdownContent() → build prompt → provider.Complete() → optional JSON schema validation
- [x] LLM Review pipeline (`llm_review.go`): `ExtractWithLLMReview()` — extract with LLM1, review with LLM2
- [x] Workspace persistence (`llm_workspace.go`): filesystem session/job tracking with `sessions.json`, `jobs/jobs.json`, `jobs/<uuid>/` structure
- [x] Functional options: `WithLLMProvider`, `WithLLMModel`, `WithLLMTemperature`, `WithLLMMaxTokens`, `WithLLMSchema`, `WithLLMSystemPrompt`, `WithLLMTimeout`, `WithLLMMainContent`, `WithLLMReview`, `WithLLMReviewModel`, `WithLLMReviewPrompt`, `WithLLMWorkspace`, `WithLLMSessionID`, `WithLLMMetadata`
- [x] CLI: `scout extract-ai --url=<url> --prompt="..." [--provider=ollama] [--model=...] [--schema=file.json] [--review] [--review-provider=...] [--workspace=dir]`
- [x] CLI: `scout ollama list/pull/status`, `scout ai-job list/show/session list/session create/session use`
- [x] Tests: 40+ tests covering mock providers, prompt construction, schema validation, workspace lifecycle, review pipeline, OpenAI/Anthropic httptest servers

### Phase 15: Async Job System [PLANNED]

- [ ] Job manager in `pkg/scout/jobs.go` for long-running crawl/batch operations
- [ ] Job lifecycle: create → running → completed/failed/cancelled
- [ ] Job ID generation, status polling, cancellation
- [ ] Persistent job state in `~/.scout/jobs/` (JSON files)
- [ ] CLI: `scout jobs list`, `scout jobs status <id>`, `scout jobs cancel <id>`, `scout jobs wait <id>`
- [ ] Integration with batch scraper and crawl commands

### Phase 16: Custom JS & Extension Injection [PLANNED]

Pre-inject custom JavaScript files and Chrome extensions into browser sessions to enhance communication, data extraction, and page instrumentation before any page scripts run.

- [ ] **JS injection API** (`pkg/scout/inject.go`) — `WithInjectJS(paths ...string)` option to load JS files at browser launch
- [ ] **Per-page injection** — use `EvalOnNewDocument()` to inject scripts before page load on every navigation
- [ ] **Script bundle loading** — load multiple JS files from a directory (`WithInjectDir(dir)`)
- [ ] **Built-in extraction helpers** — bundled JS utilities for common extraction patterns (table scraping, infinite scroll detection, shadow DOM traversal, MutationObserver wrappers)
- [ ] **Communication bridge** — JS↔Go message passing via `window.__scout.send(msg)` / `window.__scout.on(event, fn)` using CDP `Runtime.bindingCalled`
- [ ] **Extension auto-loading** — extend `WithExtension()` to support pre-configured extension bundles (ad blockers, consent auto-clickers, custom data extractors)
- [ ] **Extension marketplace** — `~/.scout/extensions/` directory for persistent extension storage, `scout extension install <url|name>`
- [ ] **Session-scoped injection** — gRPC `InjectJS` RPC to inject scripts into running sessions dynamically
- [ ] **Script templates** — parameterized JS templates with Go `text/template` syntax for reusable injection patterns
- [ ] **CLI commands**:
  - `scout inject --file=helper.js` — inject JS into current session
  - `scout inject --dir=scripts/` — inject all JS from directory
  - `scout session create --inject=helper.js,bridge.js` — inject at session creation
  - `scout session create --extension=~/.scout/extensions/adblocker` — load extension bundle
- [ ] Tests: injection ordering, multi-file loading, communication bridge, extension bundle loading

### Phase 17: Scout Bridge Extension — Bidirectional Browser Control [IN PROGRESS]

A built-in Chrome extension (`extensions/scout-bridge/`) that establishes a persistent bidirectional communication channel between the Scout Go backend and the browser runtime. Unlike CDP-only control (which operates from outside the browser), the bridge extension runs *inside* the browser context with full access to Chrome Extension APIs, enabling capabilities that CDP alone cannot provide.

#### Core: Communication Channel

- [x] **Extension scaffold** (`extensions/scout-bridge/`) — Manifest V3 Chrome extension with service worker and content script, embedded via `extensions/extensions.go` using `embed.FS` and written to temp dir at startup
- [ ] **WebSocket transport** (`extensions/scout-bridge/ws.go` + `background.js`) — Extension service worker connects to a local WebSocket server embedded in Scout's gRPC daemon; auto-reconnect with exponential backoff
- [ ] **Message protocol** — JSON-RPC 2.0 over WebSocket: `{method, params, id}` request/response + `{method, params}` notifications; message types: `command` (Go→browser), `event` (browser→Go), `query` (Go→browser with response)
- [ ] **Go WebSocket server** (`pkg/scout/bridge/server.go`) — Embedded in the gRPC daemon, accepts extension connections, routes messages to/from Scout sessions; multiplexes multiple tabs/pages
- [ ] **Session binding** — Extension auto-discovers which Scout session owns the browser via a launch flag or cookie; messages are routed to the correct `*scout.Page`
- [ ] **Heartbeat & health** — Periodic ping/pong between extension and server; connection status exposed via `scout bridge status`

#### Browser→Go: Event Streaming

- [ ] **DOM mutation observer** — Content script watches for DOM changes (element added/removed/modified) and streams structured events to Go: `{type: "mutation", selector, action, html}`
- [ ] **User interaction capture** — Record clicks, keystrokes, form inputs, scrolls, selections as structured events; replay-friendly format compatible with recipe system
- [ ] **Navigation events** — `beforeunload`, `hashchange`, `popstate`, SPA route changes (MutationObserver on `<title>` and URL), `pushState`/`replaceState` interception
- [ ] **Network observer** — `chrome.webRequest` API for request/response headers, timing, status codes; complements CDP HAR recording with extension-level visibility (service worker requests, extension requests)
- [ ] **Console & error forwarding** — Capture `console.log/warn/error`, uncaught exceptions, CSP violations; forward to Go with source location and stack traces
- [ ] **Storage change events** — Monitor `localStorage`, `sessionStorage`, `IndexedDB`, `cookie` changes in real-time; stream deltas to Go
- [ ] **Tab lifecycle events** — Tab created, activated, closed, moved, attached, detached; window focus/blur; complements CDP target events

#### Go→Browser: Remote Commands

- [ ] **DOM manipulation** — Insert/remove/modify elements, set attributes, change styles from Go without CDP `Runtime.evaluate`; extension content script executes with page privileges
- [ ] **Form auto-fill** — Extension-native form filling using `chrome.autofill` and content script input simulation; handles shadow DOM, web components, and cross-origin iframes that CDP cannot reach
- [ ] **Clipboard access** — Read/write clipboard via `chrome.clipboard` or `navigator.clipboard` from Go; CDP has no clipboard API
- [ ] **Download management** — `chrome.downloads` API: trigger, monitor, cancel, open downloads from Go; get download progress events
- [ ] **Notification control** — `chrome.notifications` API: create/clear browser notifications from Go; capture notification click events
- [ ] **Tab management** — Create, close, reload, move, pin/unpin, mute/unmute, duplicate tabs from Go via `chrome.tabs`
- [ ] **Bookmark & history access** — Read/write bookmarks and browsing history via `chrome.bookmarks` and `chrome.history`
- [ ] **Cookie management (enhanced)** — `chrome.cookies` API for cross-domain cookie access with full partition key support; superior to CDP cookie methods for SameSite/Partitioned cookies
- [ ] **Permission requests** — Trigger permission prompts (geolocation, camera, notifications) from Go and capture user responses

#### Content Script Toolkit

- [ ] **`window.__scout` API** — Global namespace injected by content script: `__scout.send(event)`, `__scout.on(command, handler)`, `__scout.query(method, params)` (returns Promise), `__scout.state` (shared state object)
- [ ] **Shadow DOM traversal** — Content script utility to pierce shadow roots and interact with web component internals; `__scout.shadowQuery(hostSelector, innerSelector)`
- [ ] **Cross-frame messaging** — Content script in each frame; `__scout.frame(selector).send(msg)` for cross-iframe communication without CDP frame targeting
- [ ] **Anti-detection evasion** — Extension-based stealth patches (navigator, WebGL, canvas) that are harder to detect than CDP-injected scripts because they run in the extension's isolated world
- [ ] **Page function injection** — `__scout.expose(name, fn)` to register Go-backed functions callable from page JavaScript; bidirectional RPC

#### Library Integration

- [x] **`WithBridge()` option** (`pkg/scout/option.go`) — Enable bridge extension auto-loading; writes extension to temp dir and loads via `WithExtension()`
- [ ] **`Bridge` type** (`pkg/scout/bridge.go`) — `Browser.Bridge()` returns the bridge instance; `bridge.Send(method, params)`, `bridge.On(event, handler)`, `bridge.Query(method, params) (result, error)`
- [ ] **Event subscriptions** — `bridge.OnMutation(selector, fn)`, `bridge.OnNavigation(fn)`, `bridge.OnConsole(fn)`, `bridge.OnNetwork(fn)`, `bridge.OnInteraction(fn)`
- [ ] **Command methods** — `bridge.InsertElement(html, parent)`, `bridge.SetClipboard(text)`, `bridge.Download(url)`, `bridge.CreateTab(url)`, `bridge.GetHistory(query)`
- [ ] **Fallback to CDP** — When bridge is unavailable (headless, no extension), methods gracefully degrade to CDP equivalents where possible; `bridge.Available() bool`

#### gRPC Integration

- [ ] **Bridge RPCs** in `grpc/proto/scout.proto` — `EnableBridge`, `BridgeSend`, `BridgeQuery`, `StreamBridgeEvents`
- [ ] **Event multiplexing** — Bridge events merged into the existing `StreamEvents` RPC alongside CDP events; tagged with `source: "bridge"` or `source: "cdp"`

#### CLI Commands

- [ ] `scout bridge status` — Show bridge connection status, extension version, connected tabs
- [ ] `scout bridge send <method> [params-json]` — Send command to browser via bridge
- [ ] `scout bridge listen [--events=mutation,navigation,console]` — Stream bridge events to stdout
- [ ] `scout bridge record` — Record all user interactions as a recipe-compatible action sequence
- [ ] `scout session create --bridge` — Create session with bridge extension enabled

#### Testing

- [ ] WebSocket server unit tests (connect, disconnect, reconnect, message routing)
- [ ] Message protocol tests (JSON-RPC serialization, error handling, timeout)
- [ ] Integration tests with real extension loaded via `WithExtension()`
- [ ] Content script tests (DOM mutation detection, shadow DOM traversal, cross-frame messaging)
- [ ] Fallback behavior tests (bridge unavailable → CDP degradation)
- [ ] Example: `examples/advanced/bridge-extension/`

### Phase 17a: Stealth Mode — Anti-Bot-Detection [IN PROGRESS]

Comprehensive stealth system that prevents headless browser detection across multiple vectors. Combines Chrome launch flags, JS injection (via `extract-stealth-evasions` + custom evasions), and fingerprint spoofing.

#### Core Evasions (extract-stealth-evasions v2.7.3)

- [x] `navigator.webdriver` — hidden/false
- [x] `chrome.runtime` — present and truthy
- [x] `Permissions` API — query overrides
- [x] `WebGLRenderingContext` — parameter interception
- [x] `hardwareConcurrency` — spoofed
- [x] `languages` / `plugins` / `mimeTypes` — populated
- [x] `iframe.contentWindow` — patched

#### Extra Evasions (custom `ExtraJS`)

- [x] **Canvas fingerprint noise** — subtle random noise on `toDataURL` and `getImageData` (unique per page)
- [x] **AudioContext fingerprint noise** — micro-gain noise on oscillator output
- [x] **WebGL vendor/renderer spoofing** — reports "Intel Inc." / "Intel Iris OpenGL Engine"
- [x] **`navigator.connection` spoofing** — effectiveType "4g", downlink 10, rtt 50
- [x] **`Notification.permission`** — returns "default"

#### Chrome Launch Flags

- [x] **`disable-blink-features=AutomationControlled`** — removes `navigator.webdriver=true` at Chrome level

#### Bot Detection Integration Tests

- [x] **`TestBotDetection_NoStealth`** — visits bot-detection sites without stealth, confirms bot IS detected (validates test sites work)
- [x] **`TestBotDetection_WithStealth`** — visits same sites with stealth, confirms bot is NOT detected
- [x] **Test sites**: bot.sannysoft.com, arh.antoinevastel.com/bots/areyouheadless, infosimples/detect-headless, pixelscan.net, seleniumbase/brotector, demo.fingerprint.com/playground

#### Planned

- [ ] **TLS/JA3 fingerprint rotation** — vary TLS fingerprint signatures via Chrome flags
- [ ] **Font fingerprint spoofing** — spoof `document.fonts` enumeration
- [ ] **Screen resolution consistency** — ensure `screen.width/height` match viewport settings
- [ ] **WebRTC leak prevention** — disable or spoof WebRTC local IP leak
- [ ] **Battery API spoofing** — consistent battery status across pages
- [ ] **More test sites** — creepjs.com, overpoweredjs.com, nobotspls.com, datadome.co/browserscan

### Phase 17b: AI-Powered Bot Protection Bypass [PLANNED]

Use LLM vision and the Scout Bridge extension to detect and solve Cloudflare challenges, CAPTCHAs, and other bot protection mechanisms automatically. The bridge extension (now enabled by default) provides the in-browser instrumentation needed for real-time challenge detection and interaction.

#### Challenge Detection

- [ ] **Challenge detector** (`pkg/scout/challenge.go`) — detect Cloudflare "Just a moment...", hCaptcha, reCAPTCHA, Turnstile, DataDome, PerimeterX, Akamai Bot Manager by page title, DOM markers, and URL patterns
- [ ] **Challenge type enum** — `ChallengeCloudflare`, `ChallengeHCaptcha`, `ChallengeRecaptcha`, `ChallengeTurnstile`, `ChallengeDataDome`, `ChallengeUnknown`
- [ ] **Auto-detect on navigation** — `WithAutoBypass()` option to automatically detect and attempt to solve challenges after every `Navigate()` / `NewPage()`
- [ ] **Bridge integration** — use `window.__scout` content script to detect challenge iframes, mutation-observe challenge DOM changes, and report challenge state back to Go

#### Cloudflare Bypass Strategies

- [ ] **Wait-based bypass** — Cloudflare JS challenge often resolves after a few seconds; detect and wait with exponential backoff up to timeout
- [ ] **Turnstile solver** — use LLM vision (`ExtractWithLLM` + screenshot) to identify Turnstile checkbox position, click via CDP
- [ ] **Cookie persistence** — after solving a challenge, capture `cf_clearance` and related cookies; persist via User Profile (Phase 18) for reuse across sessions
- [ ] **Browser fingerprint consistency** — ensure stealth mode + bridge extension produce consistent fingerprints that pass Cloudflare's TLS/JA3/HTTP2 checks
- [ ] **TLS fingerprint rotation** — configure Chrome launch flags to vary TLS fingerprint signatures

#### CAPTCHA Solving

- [ ] **Screenshot-based solving** — take screenshot of CAPTCHA region, send to LLM vision provider (GPT-4o, Claude, Gemini) for answer extraction
- [ ] **hCaptcha image classification** — LLM vision identifies correct images from the grid
- [ ] **reCAPTCHA v2 click** — detect and click the "I'm not a robot" checkbox; if image challenge appears, use LLM vision
- [ ] **Audio CAPTCHA fallback** — download audio challenge, transcribe with Whisper/LLM, submit text answer
- [ ] **Third-party solver integration** — `WithCAPTCHASolver(solver)` interface for 2Captcha, Anti-Captcha, CapSolver services as fallback

#### API & Options

- [ ] **`BypassChallenge(page *Page, ...BypassOption) error`** — attempt to solve the current challenge on the page
- [ ] **`WithBypassTimeout(d time.Duration)`** — max time to spend on challenge solving (default: 30s)
- [ ] **`WithBypassLLM(provider LLMProvider)`** — LLM provider for vision-based CAPTCHA solving
- [ ] **`WithBypassRetries(n int)`** — max retry attempts per challenge
- [ ] **`WithAutoBypass()`** — enable automatic challenge detection and solving on every navigation
- [ ] **`WithBypassCallback(fn func(ChallengeType))`** — notification when a challenge is detected/solved
- [ ] **`NavigateWithBypass(url string) error`** — convenience method: navigate + auto-bypass if challenged

#### CLI Commands

- [ ] `scout navigate <url> --bypass` — navigate with auto-bypass enabled
- [ ] `scout challenge detect` — check if current page has a bot challenge
- [ ] `scout challenge solve [--provider=openai] [--timeout=30s]` — attempt to solve current challenge
- [ ] `scout batch --urls=... --bypass` — batch scraping with auto-bypass per URL

#### Testing

- [ ] Challenge detection tests (mock Cloudflare/hCaptcha/reCAPTCHA HTML pages)
- [ ] Wait-based bypass tests (JS challenge that resolves after delay)
- [ ] Cookie persistence tests (solve → capture cookies → new session → verify bypass)
- [ ] LLM vision mock tests (screenshot → mock LLM response → verify click coordinates)

### Phase 18: User Profile — Portable Browser Identity [PLANNED]

A self-contained profile file (`.scoutprofile`) that captures everything needed to launch a browser that looks and behaves like a returning user. Profiles are portable, versionable, and can be shared across machines. On `New()` or `scout session create --profile=<file>`, Scout reads the profile, configures the browser, and hydrates all stored state — no manual setup required.

#### Profile File Format

- [ ] **`.scoutprofile` format** (`pkg/scout/profile.go`) — single JSON (or encrypted JSON) file containing all browser identity data
- [ ] **Schema versioning** — `{"version": 1, ...}` header for forward-compatible migration
- [ ] **Profile sections**:
  ```go
  type UserProfile struct {
      Version     int                `json:"version"`
      Name        string             `json:"name"`                   // human label ("work", "shopping-br")
      CreatedAt   time.Time          `json:"created_at"`
      UpdatedAt   time.Time          `json:"updated_at"`
      Browser     ProfileBrowser     `json:"browser"`                // browser type, exec path, window size
      Identity    ProfileIdentity    `json:"identity"`               // user-agent, language, timezone, locale, geolocation
      Cookies     []ProfileCookie    `json:"cookies"`                // all cookies with domain, path, expiry, SameSite
      Storage     ProfileStorage     `json:"storage"`                // per-origin localStorage + sessionStorage
      Headers     map[string]string  `json:"headers,omitempty"`      // extra headers to inject
      Extensions  []string           `json:"extensions,omitempty"`   // extension IDs (resolved via ~/.scout/extensions/)
      LaunchFlags map[string]string  `json:"launch_flags,omitempty"` // custom Chrome flags
      Proxy       string             `json:"proxy,omitempty"`        // proxy URL
      Notes       string             `json:"notes,omitempty"`        // freeform annotation
  }
  ```

#### Profile Capture (Browser → File)

- [ ] **`CaptureProfile(page *Page, ...ProfileOption) (*UserProfile, error)`** — snapshot the current browser state into a profile
- [ ] **Cookie capture** — dump all cookies across domains via CDP `Network.getAllCookies`
- [ ] **Storage capture** — enumerate origins, read localStorage + sessionStorage via JS eval per origin
- [ ] **Identity capture** — read current user-agent, language, timezone, viewport, geolocation from browser
- [ ] **Extension capture** — list loaded extension IDs that exist in `~/.scout/extensions/`
- [ ] **Save to file** — `profile.Save(path string) error` writes `.scoutprofile` JSON
- [ ] **Encrypted save** — `profile.SaveEncrypted(path, passphrase string) error` using existing `scraper/crypto.go` AES-256-GCM + Argon2id

#### Profile Load (File → Browser)

- [ ] **`WithProfile(path string)` option** — load profile at browser creation, configure all settings before launch
- [ ] **`WithProfileData(p *UserProfile)` option** — load from in-memory struct
- [ ] **Browser config** — apply browser type, window size, proxy, launch flags, extensions from profile
- [ ] **Identity injection** — set user-agent, accept-language, timezone override, geolocation override via CDP
- [ ] **Cookie hydration** — set all cookies via CDP `Network.setCookies` after page creation
- [ ] **Storage hydration** — navigate to each origin, inject localStorage + sessionStorage via JS eval
- [ ] **Header injection** — apply custom headers via `SetHeaders()`
- [ ] **Extension resolution** — resolve extension IDs to local paths via `extensionPathByID()`, warn if missing

#### Profile Management

- [ ] **`LoadProfile(path string) (*UserProfile, error)`** — read and parse `.scoutprofile` file
- [ ] **`LoadProfileEncrypted(path, passphrase string) (*UserProfile, error)`** — decrypt and parse
- [ ] **`MergeProfiles(base, overlay *UserProfile) *UserProfile`** — merge two profiles (overlay wins on conflict)
- [ ] **`DiffProfiles(a, b *UserProfile) ProfileDiff`** — compare two profiles, list changes
- [ ] **Profile validation** — `profile.Validate() error` checks required fields, cookie format, extension availability

#### CLI Commands

- [ ] `scout profile capture [--output=my.scoutprofile] [--encrypt]` — capture current session state to file
- [ ] `scout profile load <file.scoutprofile>` — create session from profile
- [ ] `scout profile show <file.scoutprofile>` — display profile summary (name, cookies count, origins, extensions)
- [ ] `scout profile merge <base> <overlay> [--output=merged.scoutprofile]` — merge two profiles
- [ ] `scout profile diff <a> <b>` — show differences between profiles
- [ ] `scout session create --profile=<file>` — create new session with profile applied

#### gRPC Integration

- [ ] **`CreateSession` extension** — accept optional profile payload in session creation request
- [ ] **`CaptureProfile` RPC** — capture running session state as profile, return serialized bytes
- [ ] **`LoadProfile` RPC** — apply profile to existing session

#### Testing

- [ ] Profile round-trip tests (capture → save → load → verify all fields match)
- [ ] Encrypted profile tests (save encrypted → load with correct/wrong passphrase)
- [ ] Cookie hydration tests (set cookies → capture → new browser → load → verify cookies present)
- [ ] Storage hydration tests (set localStorage → capture → new browser → load → verify storage present)
- [ ] Identity injection tests (user-agent, timezone, language preserved across capture/load)
- [ ] Merge and diff tests
- [ ] CLI integration tests

### Phase 19: Screen Recorder [PLANNED]

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

### Phase 20: Swarm — Distributed Processing [PLANNED]

Swarm distributes work units across multiple Scout instances (local or remote via gRPC), collects partial results, and merges them into a unified output. Each node processes a slice of the workload independently with its own browser, proxy, and identity.

- [ ] **Swarm coordinator** (`pkg/scout/swarm/coordinator.go`) — central dispatcher that splits work, assigns to workers, collects results
- [ ] **Work unit model** — `WorkUnit{ID, Type, Payload}` with types: URL batch, search query, recipe, crawl subtree, custom
- [ ] **Worker interface** — `Worker{Process(ctx, unit) (Result, error)}` implemented by local browser pool and remote gRPC peers
- [ ] **Local worker pool** (`pkg/scout/swarm/local.go`) — N browser instances on the same machine, concurrency-limited
- [ ] **Remote worker** (`pkg/scout/swarm/remote.go`) — proxy to a paired gRPC Scout server via mTLS, uses existing device identity
- [ ] **Work distribution strategies** — round-robin, least-loaded, hash-based (consistent URL→worker mapping for cache affinity)
- [ ] **Result merger** (`pkg/scout/swarm/merge.go`) — collect partial results, dedup, sort, merge into unified output (JSON, CSV, HAR bundle)
- [ ] **Fault tolerance** — retry failed units on different workers, dead worker detection via heartbeat, partial result recovery
- [ ] **Multi-IP support** — assign different proxies per worker for IP rotation (`WithSwarmProxies([]string)`)
- [ ] **Crawl distribution** — split BFS frontier across workers, shared visited-set via coordinator, merge link graphs
- [ ] **Batch distribution** — split URL list into chunks, fan-out to workers, fan-in results preserving input order
- [ ] **Recipe distribution** — run same recipe on different URL sets across workers, merge extracted items
- [ ] **Search distribution** — fan-out same query to multiple engines in parallel, merge and rank-fuse results
- [ ] **Progress & monitoring** — real-time progress aggregation across all workers, event stream to coordinator display
- [ ] **mDNS auto-discovery** — discover available Scout peers on LAN via existing `pkg/discovery/`, auto-add as workers
- [ ] **CLI commands**:
  - `scout swarm start [--workers=N] [--remote=addr1,addr2]` — start coordinator with local + remote workers
  - `scout swarm status` — show worker pool, active units, progress
  - `scout swarm run --recipe=file.json [--split-by=url]` — distribute recipe execution
  - `scout swarm crawl <url> [--workers=N]` — distributed crawl
  - `scout swarm batch --urls-file=file.txt [--workers=N]` — distributed batch
- [ ] **gRPC extensions** — `AssignWork`, `ReportResult`, `Heartbeat` RPCs in `grpc/proto/scout.proto`
- [ ] Tests: local pool, remote worker mock, distribution strategies, merge logic, fault tolerance

### Phase 21: Device Identity, mTLS & Discovery [COMPLETE]

- [x] **Device identity** (`pkg/identity/`) — Syncthing-style device IDs with Ed25519 keys, Luhn check digits
- [x] **mTLS authentication** (`grpc/server/tls.go`) — auto-generated certificates, mutual TLS for gRPC
- [x] **Device pairing** (`grpc/server/pairing.go`) — handshake protocol for mTLS certificate exchange
- [x] **mDNS discovery** (`pkg/discovery/`) — LAN service advertisement and peer discovery via zeroconf
- [x] **Platform session defaults** (`grpc/server/platform_*.go`) — auto `--no-sandbox` on Linux containers
- [x] **Server instance display** (`grpc/server/display.go`) — table view with peer tracking
- [x] **DevTools option** — `WithDevTools()` for browser DevTools panel
- [x] **CLI device commands** (`cmd/scout/internal/cli/device.go`) — `scout device pair/list/trust`

### Phase 21b: Docker Images — Container Deployment [IN PROGRESS]

Provide pre-built Docker images for running Scout CLI and gRPC server in containers. Supports headless browser automation in CI/CD pipelines, Kubernetes jobs, and serverless environments.

#### Docker Images

- [x] **`Dockerfile`** — Multi-stage build: Go 1.25 builder + `debian:bookworm-slim` runtime with Chromium, fonts (CJK, emoji, Noto), `dumb-init`, non-root `scout` user
- [x] **Base image** — `debian:bookworm-slim` with Chromium, fonts (CJK, emoji), and `dumb-init`
- [x] **Minimal image** — `gcr.io/distroless/static-debian12:nonroot` variant for scout CLI (no browser, gRPC client only) as `Dockerfile.slim`
- [x] **Image variants**:
  - `scout:latest` / `scout:<version>` — full image with Chromium + scout CLI
  - `scout:slim` — CLI-only (no browser), for gRPC client usage
- [x] **Docker Compose** — `docker-compose.yml` with scout-server service, healthcheck, tmpfs `/dev/shm`, volume for state
- [x] **`.dockerignore`** — excludes `.git`, `bin/`, `output/`, `examples/`, `.scripts/`, `.claude/`
- [x] **Environment variables** — `SCOUT_HEADLESS=true`, `SCOUT_NO_SANDBOX=true`
- [x] **`scout browser download` command** — download browsers in containers (`scout browser download brave`)
- [x] **Auto `--no-sandbox`** — existing `platform_linux.go` handles this for gRPC sessions

#### CI/CD Integration

- [ ] **GitHub Actions workflow** — build and push images to GHCR on tag
- [ ] **Multi-arch builds** — `linux/amd64` and `linux/arm64` via `docker buildx`
- [ ] **Image scanning** — Trivy vulnerability scan in CI
- [ ] **Size optimization** — target < 500MB for full image, < 50MB for slim

#### Kubernetes Support

- [ ] **Helm chart** (`deploy/helm/scout/`) — deploy scout gRPC server as a Kubernetes Deployment/Service
- [ ] **Job template** — example Kubernetes Job for one-shot crawl/scrape tasks
- [ ] **Resource limits** — recommended CPU/memory limits for browser containers (2 CPU, 2Gi RAM default)
- [ ] **Shared memory** — `/dev/shm` volume mount for Chrome (required to avoid crashes in containers)

#### Testing

- [ ] Container build smoke test (build + run basic crawl)
- [ ] gRPC server healthcheck in container
- [ ] Bridge extension loads in container
- [ ] Example: `examples/docker/` with docker-compose setup

### Phase 21c: Scout-Browser — Portable Browser Repository [PLANNED]

Extract browser download, patching, and management into a dedicated `inovacc/scout-browser` repository. This decouples browser lifecycle from the core library, enables independent versioning, and allows the community to contribute browser-specific fixes without touching the main codebase.

#### Repository Structure (`inovacc/scout-browser`)

- [ ] **`browser.go`** — core types: `BrowserType`, `BrowserInfo`, `BrowserRelease`, `Platform`
- [ ] **`download.go`** — download engine: fetch releases from GitHub/CDN, verify checksums, extract archives
- [ ] **`brave.go`** — Brave browser: GitHub releases API, asset name mapping, binary path resolution
- [ ] **`chrome.go`** — Chromium: integrate/wrap rod's launcher download logic, or use Chrome for Testing JSON API
- [ ] **`edge.go`** — Edge: parse `edgeupdates.microsoft.com` API, download installers (Windows MSI, macOS PKG, Linux DEB/RPM), auto-install where possible
- [ ] **`patch.go`** — browser patching: apply fixes to downloaded browsers (disable update checks, telemetry, first-run dialogs, default browser prompts)
- [ ] **`cache.go`** — cache management: `~/.scout/browsers/` directory, version tracking, cleanup of old versions, disk usage reporting
- [ ] **`release.go`** — GitHub release publishing: CI pipeline to download, patch, repackage, and publish fixed browser zips to `inovacc/scout-browser` releases
- [ ] **`verify.go`** — integrity verification: SHA-256 checksums, optional GPG signature validation

#### Browser Patching Pipeline

- [ ] **Disable auto-update** — remove/neuter update mechanisms (Brave: `BraveUpdate`, Chrome: `GoogleUpdate`, Edge: `MicrosoftEdgeUpdate`)
- [ ] **Disable telemetry** — patch preferences/policies to disable usage stats, crash reports, safe browsing callouts
- [ ] **Disable first-run** — skip first-run wizards, welcome tabs, default browser prompts
- [ ] **Hardened defaults** — set privacy-friendly defaults (no search suggestions, no URL predictions, no Safe Browsing network requests)
- [ ] **Extension pre-loading** — bundle Scout Bridge extension into patched browser distributions
- [ ] **CI pipeline** — GitHub Actions workflow: download latest releases → apply patches → run smoke tests → publish to `inovacc/scout-browser` releases with checksums

#### Integration with Scout

- [ ] **`go get github.com/inovacc/scout-browser`** — import as a Go module dependency
- [ ] **`scoutbrowser.Download(ctx, BrowserBrave)` API** — replaces inline `DownloadBrave()` in `pkg/scout/`
- [ ] **`scoutbrowser.Resolve(ctx, BrowserType)` API** — local lookup → cached download → fresh download fallback chain
- [ ] **`scoutbrowser.List()` API** — list all cached browsers with versions
- [ ] **`scoutbrowser.Patch(browserDir)` API** — apply patches to a browser installation
- [ ] **`scoutbrowser.Clean(keepLatest int)` API** — remove old cached versions, keep N latest
- [ ] **Migrate `pkg/scout/browser_download.go`** — move download logic to scout-browser, keep thin wrapper in scout core
- [ ] **CLI: `scout browser download [brave|chrome|edge]`** — download + patch a browser
- [ ] **CLI: `scout browser list`** — show cached and system browsers (already implemented)
- [ ] **CLI: `scout browser clean [--keep=2]`** — remove old cached versions
- [ ] **CLI: `scout browser patch <path>`** — apply patches to an existing browser installation

#### Testing

- [ ] Download + extract tests with httptest mock servers
- [ ] Patch application tests (verify preferences/policies modified correctly)
- [ ] Cache management tests (download, list, clean)
- [ ] Cross-platform binary resolution tests
- [ ] CI smoke test: download → patch → launch → navigate → screenshot → close

### Phase 22: Documentation & Release [IN PROGRESS]

- [x] Publish to GitHub with git remote
- [x] Create initial git tags (v0.1.3, v0.1.4, v0.1.5)
- [x] Add LICENSE file
- [x] Add GoDoc examples for key functions (20 `Example*` functions in `example_test.go`)
- [ ] Write integration test examples

### Phase 23: WebFetch & WebSearch — GitHub Data Extraction [IN PROGRESS]

A high-level web intelligence toolkit inspired by Claude Code's `WebFetch()` and `WebSearch()` tools. Provides URL fetching with automatic content extraction (HTML→Markdown), web searching with result aggregation, and a dedicated GitHub data extraction pipeline. Built on top of Scout's existing crawl, search, markdown, and extract engines.

#### Sub-phase 23a: WebFetch — URL Content Extraction

Fetch any URL and return clean, structured content (markdown, metadata, links). Combines navigation + readability + markdown conversion into a single call.

- [x] **`WebFetch` type** (`pkg/scout/webfetch.go`) — `Browser.WebFetch(url string, ...WebFetchOption) (*WebFetchResult, error)`
- [x] **WebFetchResult** — `{URL, Title, Markdown, HTML, Meta MetaData, Links []string, StatusCode int, FetchedAt time.Time}`
- [x] **Content modes** — `WithFetchMode("markdown"|"html"|"text"|"links"|"meta"|"full")` to control what gets extracted
- [x] **Main content extraction** — Reuse `MarkdownContent()` readability scoring to strip nav/ads/footer, return only article body
- [ ] **Prompt-based extraction** — `WithFetchPrompt(prompt string)` that pipes markdown through an LLM provider (Phase 14 dependency) for targeted extraction
- [x] **Caching** — `WithFetchCache(ttl time.Duration)` for in-memory content cache keyed by URL; avoid re-fetching within TTL
- [ ] **Follow redirects** — Track and report redirect chain in result
- [ ] **Error resilience** — Retry on network errors with exponential backoff (reuse `RateLimiter`), graceful timeout
- [x] **Batch fetch** — `Browser.WebFetchBatch(urls []string, ...WebFetchOption) []*WebFetchResult` with concurrent fetching
- [ ] **CLI** — `scout fetch <url> [--mode=markdown] [--main-only] [--cache=5m] [--output=file]`
- [x] **Tests** — 13 tests: all modes, caching, cache expiry, batch, error isolation, link dedup (88%+ coverage)

#### Sub-phase 23b: WebSearch — Search + Fetch Pipeline

Search the web and optionally fetch top results, returning structured search results with optional full content. Combines SERP parsing with WebFetch for a research-grade pipeline.

- [x] **`WebSearch` type** (`pkg/scout/websearch.go`) — `Browser.WebSearch(query string, ...WebSearchOption) (*WebSearchResult, error)`
- [x] **WebSearchResult** — `{Query, Engine, Results []WebSearchItem}` with `WebSearchItem{Title, URL, Snippet, Position, Content *WebFetchResult}`
- [x] **Fetch top N results** — `WithWebSearchFetch("markdown")` + `WithWebSearchMaxFetch(n)` to auto-fetch and extract content from top N results
- [x] **Concurrency control** — `WithWebSearchConcurrency(n)` for parallel fetch with semaphore pattern
- [x] **Cache passthrough** — `WithWebSearchCache(ttl)` passes through to WebFetch cache layer
- [x] **CLI** — `scout websearch "query" [--engine=google] [--fetch=markdown] [--max-fetch=5] [--max-pages=1] [--main-only]`
- [x] **Tests** — 7 tests: NoFetch, WithFetch, MainContent, MaxFetch, Cache, FetchErrorIsolation, OptionDefaults
- [ ] **Multi-engine aggregation** — `WithSearchEngines(Google, Bing, DuckDuckGo)` to run same query across engines, merge and deduplicate by URL
- [ ] **Rank fusion** — Reciprocal Rank Fusion (RRF) scoring when merging multi-engine results
- [ ] **Domain filtering** — `WithSearchDomain("github.com")`, `WithSearchExcludeDomain("pinterest.com")`
- [ ] **Time filtering** — `WithSearchRecent(duration)` for time-bounded searches

#### Sub-phase 23c: GitHub Data Extraction

Dedicated GitHub extraction toolkit using WebFetch + WebSearch + Scout's existing crawl/extract infrastructure. Provides structured access to GitHub repos, issues, PRs, code, discussions, and user profiles without API rate limits.

- [ ] **`GitHubExtractor` type** (`pkg/scout/github.go`) — high-level GitHub data extraction API
- [ ] **Repository info** — `ExtractRepo(owner, repo string) (*GitHubRepo, error)` — name, description, stars, forks, language, topics, license, README (as markdown)
- [ ] **Issue extraction** — `ExtractIssues(owner, repo string, ...GitHubOption) ([]GitHubIssue, error)` — title, body, labels, assignees, comments, state, timeline
- [ ] **PR extraction** — `ExtractPRs(owner, repo string, ...GitHubOption) ([]GitHubPR, error)` — title, body, diff stats, review comments, CI status, merge state
- [ ] **Code search** — `SearchCode(query string, ...GitHubOption) ([]GitHubCodeResult, error)` — file path, repo, matched lines, context
- [ ] **Discussion extraction** — `ExtractDiscussions(owner, repo string) ([]GitHubDiscussion, error)` — title, body, category, answers, comments
- [ ] **File/tree browsing** — `ExtractTree(owner, repo, path string) (*GitHubTree, error)` — directory listing, file content as markdown
- [ ] **User/org profiles** — `ExtractUser(username string) (*GitHubUser, error)` — bio, repos, contributions, pinned items
- [ ] **Release notes** — `ExtractReleases(owner, repo string) ([]GitHubRelease, error)` — tag, body, assets, date
- [ ] **GitHub search** — `SearchRepos(query string) ([]GitHubRepo, error)`, `SearchIssues(query string) ([]GitHubIssue, error)`
- [ ] **Pagination** — All list methods support `WithGitHubMaxPages(n)`, automatic next-page navigation
- [ ] **Rate limiting** — Built-in rate limiter for polite scraping (reuse `RateLimiter`)
- [ ] **Struct tags for extraction** — Use `scout:"selector"` tags for GitHub page element mapping
- [ ] **Data types**:
  ```go
  type GitHubRepo struct {
      Owner, Name, Description, Language, License string
      Stars, Forks, Watchers, OpenIssues          int
      Topics                                       []string
      README                                       string // markdown
      DefaultBranch                                string
  }
  type GitHubIssue struct {
      Number int
      Title, Body, State, Author string
      Labels   []string
      Comments []GitHubComment
      CreatedAt, UpdatedAt time.Time
  }
  type GitHubPR struct {
      Number int
      Title, Body, State, Author, BaseBranch, HeadBranch string
      Additions, Deletions, ChangedFiles int
      Labels   []string
      Reviews  []GitHubReview
      Comments []GitHubComment
      Merged   bool
      MergedAt *time.Time
  }
  ```
- [ ] **CLI commands**:
  - `scout github repo <owner/repo>` — extract repository info + README
  - `scout github issues <owner/repo> [--state=open] [--labels=bug] [--max-pages=5]`
  - `scout github prs <owner/repo> [--state=open] [--max-pages=5]`
  - `scout github code <query> [--repo=owner/repo]`
  - `scout github user <username>`
  - `scout github releases <owner/repo>`
  - `scout github tree <owner/repo> [path]`
- [ ] **Tests** — mock GitHub HTML pages in httptest, extraction accuracy, pagination, rate limiting

#### Sub-phase 23d: Research Agent Pipeline

Orchestrate WebSearch + WebFetch + GitHub extraction into automated research workflows.

- [ ] **`Research` type** (`pkg/scout/research.go`) — `Browser.Research(query string, ...ResearchOption) (*ResearchResult, error)`
- [ ] **Multi-source research** — search → fetch top results → extract structured data → merge into report
- [ ] **GitHub-focused research** — `WithResearchGitHub(owner, repo)` to include repo issues, PRs, discussions in research context
- [ ] **Output formats** — markdown report, JSON structured data, combined summary
- [ ] **CLI** — `scout research "query" [--github=owner/repo] [--depth=shallow|deep] [--format=markdown]`

### Phase 24: Rod Fork Patches — Stability Fixes [PLANNED]

Apply confirmed upstream bug fixes to Scout's internal rod fork (`pkg/rod/`). These are the first local modifications to the fork. See [ADR 007](adr/007-rod-ecosystem-analysis.md) for full analysis.

#### Fork-Level Patches (modify `pkg/rod/`)

- [ ] **Nil-guard on disconnected page** (rod #1103) — Guard `getJSCtxID()` in `page_eval.go` against nil page/connection, return `ErrDisconnected` instead of segfault
- [ ] **Context propagation** (rod #1179) — Pass page's context through to internal operations in `page.go`
- [ ] **Page context in Info/Activate/TriggerFavicon** (rod #1206) — Use `p.browser.Context(p.ctx)` instead of `p.browser.ctx` in 3 methods
- [ ] **Update `.dep-track.json`** — Record all local modifications with issue references

#### Wrapper-Level Fixes (modify `pkg/scout/`)

- [ ] **WaitStable panic recovery** (rod #1157) — Wrap `WaitLoad`/`WaitStable` with panic recovery + retry on "Execution context was destroyed"
- [ ] **WaitSafe method** (rod #1224) — Add `Page.WaitSafe(timeout)` combining `WaitLoad` + timeout guard without `WaitRequestIdle` conflict
- [ ] **Zombie process cleanup** (rod #865) — On `Browser.Close()`, walk Chrome process tree and kill orphan child processes
- [ ] **Hijack regexp validation** (rod #982) — Pre-validate pattern with `regexp.Compile()` before passing to rod's `Add()`

#### Testing

- [ ] Tests for nil-guard (simulate disconnected page, verify error not panic)
- [ ] Tests for WaitSafe (timeout behavior, panic recovery)
- [ ] Tests for zombie cleanup (verify no orphan processes after Close)
- [ ] Tests for hijack pattern validation (invalid regexp → clear error)

### Phase 25: Accessibility Snapshot — ARIA Tree for LLM Automation [PLANNED]

Port the accessibility snapshot system from [go-rod/rod-mcp](https://github.com/go-rod/rod-mcp) for LLM-driven element addressing. The snapshot produces a YAML-like ARIA tree with `[ref=s{gen}e{id}]` markers that LLMs can reference to interact with page elements.

- [ ] **Snapshot types** (`pkg/scout/snapshot.go`) — `SnapshotNode{Role, Name, Description, Value, Ref, Children}`, `SnapshotResult{YAML string, NodeMap map[string]*SnapshotNode}`
- [ ] **JS engine** (`pkg/scout/snapshot_js.go`) — Embedded ~1500-line JS from rod-mcp `types/js/snapshotter.js` for ARIA tree building with role extraction, name computation, ref marker injection
- [ ] **`Page.Snapshot(...SnapshotOption)` method** — Execute snapshot JS, parse result, return structured tree
- [ ] **Iframe traversal** — Recursively snapshot cross-origin iframes (rod-mcp pattern: `page.Frames()` → inject JS per frame → merge trees)
- [ ] **Ref-based element resolution** — `Page.ElementByRef(ref string) (*Element, error)` to find elements by snapshot ref marker
- [ ] **LLM integration** — Feed snapshot YAML as context to `ExtractWithLLM()` for element-aware extraction
- [ ] **CLI** — `scout snapshot [--format=yaml|json]` to dump current page accessibility tree
- [ ] **Tests** — snapshot generation, ref resolution, iframe merging, LLM context construction

### Phase 26: MCP Transport — Model Context Protocol Server [PLANNED]

Expose Scout's browser automation capabilities as MCP tools via stdio transport, enabling LLMs (Claude, GPT, etc.) to drive browser sessions through the standard Model Context Protocol. Uses the official `modelcontextprotocol/go-sdk` (not the unofficial `mark3labs/mcp-go` used by rod-mcp).

- [ ] **MCP server** (`cmd/scout/mcp.go`) — `scout mcp` command that starts MCP stdio transport
- [ ] **Tool definitions** — Map Scout capabilities to MCP tools:
  - `navigate`, `click`, `type`, `screenshot`, `snapshot` (accessibility)
  - `extract`, `search`, `fetch`, `eval`, `pdf`
  - `session_create`, `session_list`, `session_destroy`
- [ ] **Resource definitions** — Expose page state as MCP resources:
  - `scout://page/markdown` — current page as markdown
  - `scout://page/snapshot` — accessibility tree
  - `scout://page/screenshot` — base64 screenshot
- [ ] **Session management** — MCP server manages a single browser session, tools operate on current page
- [ ] **Accessibility snapshot integration** — `snapshot` tool returns YAML tree for LLM context (Phase 25 dependency)
- [ ] **CLI** — `scout mcp [--headless] [--stealth]` starts MCP server on stdio
- [ ] **Tests** — in-memory MCP transport tests using `mcp.NewInMemoryTransports()`, tool execution, resource reads

### Phase 27: Browser Recycling & Request Blocking [IN PROGRESS]

Production hardening features from [go-rod/bartender](https://github.com/go-rod/bartender) analysis. AutoFree prevents memory leaks in long-running daemon sessions. Request blocking provides efficient resource filtering.

#### AutoFree — Periodic Browser Recycling

- [ ] **`WithAutoFree(interval time.Duration)` option** — Periodically restart browser process to prevent memory leaks in daemon mode
- [ ] **Session preservation** — Save session state (cookies, storage, URL) before recycle, restore after
- [ ] **Graceful recycle** — Wait for in-flight operations to complete before restart
- [ ] **Integration with gRPC daemon** — Daemon sessions auto-recycle after configurable interval (default: 1 hour)

#### Request Blocking [COMPLETE]

- [x] **`WithBlockPatterns(patterns ...string)` option** — Block matching URL patterns on every new page via `SetBlockedURLs()`
- [x] **Built-in block lists** — `BlockAds`, `BlockTrackers`, `BlockFonts`, `BlockImages` preset pattern groups
- [x] **Per-page blocking** — `Page.Block(patterns ...string)` convenience method for page-level request filtering
- [x] **Tests** — `TestWithBlockPatterns`, `TestBlockPresetVariables`, `TestWithBlockPatternsMultiplePresets`, `TestPageBlock`

#### Testing

- [ ] AutoFree lifecycle tests (recycle interval, session preservation)

## Test Coverage

**Current:** pkg/scout 75.7% | pkg/identity 81.1% | scraper 84.3% | **Total: 54.9%** | **Target:** 80%

| Package          | Coverage | Status                   |
|------------------|----------|--------------------------|
| pkg/scout        | 75.7%    | Below target             |
| pkg/scout/recipe | 81.5%    | ✅ Target met             |
| pkg/identity     | 81.1%    | ✅ Target met             |
| scraper          | 84.3%    | ✅ Complete               |
| grpc/server      | 80.6%    | ✅ Target met             |
| pkg/stealth      | 50.0%    | Tests added              |
| extensions       | 0.0%     | No tests (embed wrapper) |
| pkg/discovery    | 0.0%     | No tests                 |
| scraper/auth     | 0.0%     | No tests                 |
