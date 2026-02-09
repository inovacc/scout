# Project Roadmap

## Current Status
**Overall Progress:** 85% Complete

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
- [ ] Increase core test coverage from 33.2% to 80%+
- [ ] Add tests for PDF generation, device emulation, DOM traversal
- [ ] Add tests for EvalResult type conversions (Float, Decode, JSON)

### Phase 5: Storage & JS Execution [PLANNED]
- [ ] Session storage get/set/clear
- [ ] Local storage get/set/clear
- [ ] Enhanced cookie management (filter, export, import)
- [ ] JS execution toolkit (run scripts to extract website info)
- [ ] Script injection and result collection patterns

### Phase 6: Distributed Crawling [PLANNED]
- [ ] Swarm mode: split crawl workloads across multiple browser instances
- [ ] Multi-IP support: assign different proxies per browser in the cluster
- [ ] Work distribution: BFS queue shared across workers
- [ ] Result aggregation: merge results from all workers
- [ ] Headless cluster configuration options

### Phase 7: Documentation & Release [NOT STARTED]
- [ ] Publish to GitHub with git remote
- [ ] Add LICENSE file
- [ ] Create initial git tag / release
- [ ] Add GoDoc examples for key functions
- [ ] Write integration test examples

## Test Coverage

**Current:** ~33% (core) + new feature tests  |  **Target:** 80%

| File | Coverage | Status |
|------|----------|--------|
| option.go | 100.0% | Complete |
| browser.go | 54.5% - 83.3% | Needs improvement |
| page.go | 0.0% - 80.0% | Many methods untested |
| element.go | 0.0% - 83.3% | Many methods untested |
| network.go | 0.0% - 100.0% | Accessor methods untested |
| eval.go | 0.0% - 66.7% | Float/JSON/Decode untested |
| extract.go | NEW | Tested |
| form.go | NEW | Tested |
| ratelimit.go | NEW | Tested |
| paginate.go | NEW | Tested |
| search.go | NEW | Tested |
| crawl.go | NEW | Tested |
