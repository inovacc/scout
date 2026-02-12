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
- [ ] Increase core test coverage from 69.9% to 80%+

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
- [x] **Slack mode** (`scraper/slack/`) — workspace auth (token + browser), channel listing, message history with threads, file listing, user directory, search, channel export
- [x] **Slack session capture** (`scraper/slack/session.go`) — CaptureFromPage, encrypted save/load (AES-256-GCM + Argon2id)
- [x] **Encryption utilities** (`scraper/crypto.go`) — EncryptData/DecryptData with passphrase-based key derivation
- [x] **Slack Assist CLI** (`cmd/slack-assist/`) — capture, load, decrypt subcommands for browser-assisted credential management
- [ ] **Teams mode** (P2) — Microsoft SSO, chat/channel messages, meeting history, shared files
- [ ] **Discord mode** (P2) — server/channel messages, threads, member lists, roles, pins
- [ ] **Gmail mode** (P2) — Google auth + 2FA, email content, labels, attachments, contacts
- [ ] **Outlook mode** (P2) — Microsoft SSO, emails, folders, calendar events, contacts
- [ ] **LinkedIn mode** (P2) — profile data, posts, jobs, connections, company pages
- [ ] **Jira/Confluence modes** (P2) — Atlassian auth, issues, boards, pages, spaces
- [ ] **Social/productivity modes** (P3) — Twitter, Reddit, YouTube, Notion, GitHub, etc.
- [ ] **E-commerce modes** (P3) — Amazon, Google Maps
- [ ] **Cloud/monitoring modes** (P3) — AWS/GCP/Azure consoles, Grafana, Datadog

### Phase 8: Distributed Crawling [PLANNED]
- [ ] Swarm mode: split crawl workloads across multiple browser instances
- [ ] Multi-IP support: assign different proxies per browser in the cluster
- [ ] Work distribution: BFS queue shared across workers
- [ ] Result aggregation: merge results from all workers
- [ ] Headless cluster configuration options

### Phase 9: Documentation & Release [IN PROGRESS]
- [x] Publish to GitHub with git remote
- [x] Create initial git tags (v0.1.3, v0.1.4, v0.1.5)
- [ ] Add LICENSE file
- [ ] Add GoDoc examples for key functions
- [ ] Write integration test examples

## Test Coverage

**Current:** 69.9% (core package)  |  **Target:** 80%

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
