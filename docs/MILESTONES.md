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

## v0.3.0 - Storage & JS Execution [PLANNED]
**Goal:** Complete browser state management and JS execution toolkit.

- [ ] Session storage get/set/clear
- [ ] Local storage get/set/clear
- [ ] Enhanced cookie management (filter, export, import)
- [ ] JS execution patterns for website info extraction
- [ ] 80%+ test coverage

## v0.4.0 - Distributed Crawling [PLANNED]
**Goal:** Swarm-mode crawling across multiple browser instances.

- [ ] Browser cluster / pool management
- [ ] Multi-proxy swarm distribution
- [ ] Shared work queue for BFS crawling
- [ ] Result aggregation

## v0.5.0 - Documentation & Release [PLANNED]
**Goal:** Comprehensive documentation and published release.

- [ ] LICENSE file
- [ ] GoDoc examples for Browser, Page, Element, EvalResult, and new features
- [ ] Integration test examples (login flow, form submission, scraping)
- [ ] Published to GitHub with initial release tag
