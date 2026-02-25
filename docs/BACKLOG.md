# Backlog

## Priority Levels

| Priority | Timeline      |
|----------|---------------|
| P1       | First month   |
| P2       | First quarter |
| P3       | Future        |

## Scraper Modes

Dedicated scraper modes for authenticated services. Each mode provides structured extraction of user data from web applications via headless browser automation. Requires valid user
credentials/session.

### ~~Slack Scraper Mode~~ [REMOVED]

- **Priority:** P1
- **Status:** Removed — `scraper/slack/` package deleted; project focuses on generic auth framework
- **Effort:** N/A

### Teams Scraper Mode

- **Priority:** P2
- **Description:** Scrape Microsoft Teams via browser automation. Extract chats, channel messages, meeting history, shared files, and team/channel structure. Handle Microsoft SSO auth flow.
- **Scope:** Team/channel list, chat messages, meeting notes, shared files metadata, wiki pages.
- **Effort:** Large

### Discord Scraper Mode

- **Priority:** P2
- **Description:** Scrape Discord servers via browser automation. Extract messages, channels, threads, pins, server member lists, and roles.
- **Scope:** Server list, channel messages (with threads), member directory, roles, pins, file attachments.
- **Effort:** Large

### Gmail Scraper Mode

- **Priority:** P2
- **Description:** Scrape Gmail via browser automation. Extract emails, labels, attachments metadata, and contacts. Handle Google auth flow with 2FA support.
- **Scope:** Inbox/label listing, email content (subject, body, headers), attachment download, contact list, label management.
- **Effort:** Large

### Outlook Scraper Mode

- **Priority:** P2
- **Description:** Scrape Outlook Web via browser automation. Extract emails, folders, calendar events, and contacts. Handle Microsoft SSO auth.
- **Scope:** Folder listing, email content, calendar events, contact list, attachment metadata.
- **Effort:** Large

### LinkedIn Scraper Mode

- **Priority:** P2
- **Description:** Scrape LinkedIn profiles, posts, job listings, and company pages. Handle LinkedIn auth and anti-bot measures.
- **Scope:** Profile data, connections, posts/articles, job search results, company pages, messaging.
- **Effort:** Large

### Twitter/X Scraper Mode

- **Priority:** P3
- **Description:** Scrape X/Twitter via browser automation. Extract tweets, profiles, followers, trends, and search results.
- **Scope:** Timeline, user profiles, tweet threads, search results, trending topics, bookmarks.
- **Effort:** Large

### Reddit Scraper Mode

- **Priority:** P3
- **Description:** Scrape Reddit via browser automation. Extract posts, comments, subreddit metadata, and user profiles.
- **Scope:** Subreddit feeds, post content with comments, user profiles, search results, saved posts.
- **Effort:** Medium

### YouTube Scraper Mode

- **Priority:** P3
- **Description:** Scrape YouTube via browser automation. Extract video metadata, comments, channel info, and playlist data.
- **Scope:** Video metadata (title, description, stats), comments, channel pages, playlists, search results.
- **Effort:** Medium

### Jira Scraper Mode

- **Priority:** P2
- **Description:** Scrape Jira via browser automation. Extract issues, boards, sprints, comments, and attachments. Handle Atlassian auth.
- **Scope:** Issue listing with filters, issue details (comments, attachments, history), board/sprint views, dashboards.
- **Effort:** Large

### Confluence Scraper Mode

- **Priority:** P2
- **Description:** Scrape Confluence via browser automation. Extract pages, spaces, comments, and attachments. Handle Atlassian auth.
- **Scope:** Space listing, page content with hierarchy, comments, attachments, search results.
- **Effort:** Large

### Notion Scraper Mode

- **Priority:** P3
- **Description:** Scrape Notion via browser automation. Extract pages, databases, blocks, and comments.
- **Scope:** Workspace pages, database views, page content (blocks), comments, shared pages.
- **Effort:** Medium

### ~~GitHub Scraper Mode~~ [SUPERSEDED → Phase 23]

- **Priority:** P1
- **Status:** Superseded by Phase 23 (WebFetch & WebSearch — GitHub Data Extraction). The new design provides a comprehensive GitHub extraction toolkit (`pkg/scout/github.go`) plus general-purpose `WebFetch`/`WebSearch` tools, inspired by Claude Code's `Fetch()` and `WebSearch()` mechanisms.
- **Effort:** Large

### Google Drive Scraper Mode

- **Priority:** P3
- **Description:** Scrape Google Drive via browser automation. Extract file listings, metadata, sharing info, and folder structure. Handle Google auth.
- **Scope:** File/folder tree, metadata (owner, sharing, dates), recent activity, shared drives.
- **Effort:** Medium

### SharePoint Scraper Mode

- **Priority:** P3
- **Description:** Scrape SharePoint via browser automation. Extract documents, lists, sites, and pages. Handle Microsoft SSO.
- **Scope:** Site listing, document libraries, list data, page content, site permissions.
- **Effort:** Large

### Salesforce Scraper Mode

- **Priority:** P3
- **Description:** Scrape Salesforce via browser automation. Extract leads, contacts, opportunities, and reports.
- **Scope:** Object listings (leads, contacts, accounts, opportunities), reports/dashboards, activity history.
- **Effort:** Large

### Amazon Product Scraper Mode

- **Priority:** P3
- **Description:** Scrape Amazon product pages. Extract product details, prices, reviews, rankings, and seller info.
- **Scope:** Product pages, search results, review pages, price history, seller profiles.
- **Effort:** Medium

### Google Maps Scraper Mode

- **Priority:** P3
- **Description:** Scrape Google Maps. Extract business listings, reviews, locations, and contact info.
- **Scope:** Business search results, place details, reviews, photos metadata, operating hours.
- **Effort:** Medium

### Cloud Console Scrapers (AWS/GCP/Azure)

- **Priority:** P3
- **Description:** Scrape cloud provider consoles for resource inventory and billing data not easily available via API.
- **Scope:** Resource listings, billing dashboards, cost explorer, service quotas, IAM summaries.
- **Effort:** Extra Large

### Grafana/Datadog Dashboard Scraper

- **Priority:** P3
- **Description:** Scrape monitoring dashboards for screenshots and data export. Handle auth flows.
- **Scope:** Dashboard screenshots, panel data extraction, alert history, metric queries.
- **Effort:** Medium

---

## Core Features

### ~~HTML-to-Markdown Engine~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `pkg/scout/markdown.go` + `readability.go` with `page.Markdown()`, `page.MarkdownContent()`, readability scoring, 17 pure-function tests + browser integration tests, CLI
  `scout markdown`
- **Effort:** Large

### ~~Multi-Engine Search Command~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `cmd/scout/search_engines.go` with engine registry (Google, Bing, DuckDuckGo, Wikipedia, Google Scholar, Google News), structured output, pagination
- **Effort:** Medium

### ~~Batch Scraper~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `pkg/scout/batch.go` with `BatchScrape()`, configurable concurrency, error isolation, progress callback, rate limiter integration, CLI `scout batch`
- **Effort:** Medium

### ~~URL Map / Link Discovery~~ [DONE]

- **Priority:** P2
- **Status:** Complete — `pkg/scout/map.go` with `Map()` function, sitemap + BFS link harvesting, path/subdomain/search filters, CLI `scout map`
- **Effort:** Medium

### WebFetch & WebSearch — GitHub Data Extraction (Phase 23)

- **Priority:** P1
- **Description:** High-level web intelligence toolkit inspired by Claude Code's `WebFetch()` and `WebSearch()` tools. Four sub-phases: (a) `WebFetch` — single-call URL→Markdown extraction with caching and batch support, (b) `WebSearch` — multi-engine search with auto-fetch and rank fusion, (c) `GitHubExtractor` — dedicated GitHub repo/issue/PR/code/discussion extraction without API rate limits, (d) Research Agent — orchestrated multi-source research workflows.
- **Scope:** `pkg/scout/webfetch.go`, `pkg/scout/websearch.go`, `pkg/scout/github.go`, `pkg/scout/research.go`. CLI: `scout fetch`, `scout websearch`, `scout github repo/issues/prs/code/user/releases/tree`, `scout research`.
- **Effort:** Extra Large
- **Dependencies:** HTML-to-Markdown engine (done), search engine integration (done), crawl (done), batch scraper (done). Optional: LLM-Powered Extraction (Phase 14) for prompt-based content extraction.

### ~~LLM-Powered Extraction~~ [DONE]

- **Priority:** P2
- **Status:** Complete — `pkg/scout/llm.go`, `llm_ollama.go`, `llm_openai.go`, `llm_anthropic.go`, `llm_review.go`, `llm_workspace.go` with `ExtractWithLLM()`, `ExtractWithLLMJSON()`, `ExtractWithLLMReview()`, workspace persistence, 6 providers (Ollama, OpenAI, Anthropic, OpenRouter, DeepSeek, Gemini), CLI `scout extract-ai`, `scout ollama`, `scout ai-job`
- **Effort:** Large

### Async Job System

- **Priority:** P3
- **Description:** Job manager for long-running batch and crawl operations. Provides job IDs, status polling, cancellation, and persistent state. Enables running large crawls/batches in the background
  with progress tracking.
- **Scope:** `pkg/scout/jobs.go` with job lifecycle management. Persistent state in `~/.scout/jobs/`. CLI `scout jobs list/status/cancel/wait`.
- **Effort:** Medium
- **Dependencies:** Integrates with batch scraper and crawl commands.

### Scout Bridge Extension — Bidirectional Browser Control

- **Priority:** P2
- **Description:** Built-in Chrome Manifest V3 extension (`extensions/scout-bridge/`) that establishes a persistent WebSocket channel between the Scout Go backend and the browser runtime. Enables capabilities CDP alone cannot provide: DOM mutation streaming, user interaction capture, shadow DOM traversal, cross-frame messaging, clipboard access, download management, tab control, cookie management with full partition key support. Provides `window.__scout` content script API for page-level bidirectional RPC. Graceful fallback to CDP when extension is unavailable.
- **Scope:** Extension source in `extensions/scout-bridge/`, Go WebSocket server in `pkg/scout/bridge/`, `Bridge` type in `pkg/scout/bridge.go`, `WithBridge()` option, gRPC RPCs (`EnableBridge`, `BridgeSend`, `BridgeQuery`, `StreamBridgeEvents`), CLI `scout bridge status/send/listen/record`. Content script toolkit with `__scout.send()`, `__scout.on()`, `__scout.query()`, shadow DOM helpers, cross-frame messaging.
- **Effort:** Extra Large
- **Dependencies:** Existing `WithExtension()` infrastructure, gRPC daemon WebSocket embedding, Chrome Extension Manifest V3 APIs.

### ~~AI-Powered Bot Protection Bypass (Phase 17b)~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `ChallengeSolver`, `SolveFunc`, `SolverOption`, `CaptchaSolverService` interface, `TwoCaptchaService`, `CapSolverService`, `NavigateWithBypass()`, `WithAutoBypass()`, Cloudflare wait, Turnstile click, CAPTCHA LLM vision, cookie persistence. CLI `scout challenge detect/solve`.
- **Effort:** Large

### ~~Scout-Browser — Standalone Browser Module (Phase 21c)~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `pkg/browser/` standalone module with `Manager`, `Detect()`, `Download()`, `BrowserInfo`, platform-specific version detection (Windows registry, macOS plist, Linux desktop files), cache management. CLI `scout browser list/download`.
- **Effort:** Large

### ~~Docker Images — Container Deployment (Phase 21b)~~ [MOSTLY DONE]

- **Priority:** P1
- **Status:** Core + CI/CD done — `Dockerfile`, `Dockerfile.slim`, `.dockerignore`, `docker-compose.yml`, `scout browser download` command, GitHub Actions GHCR publishing (`.github/workflows/docker.yml`), multi-arch builds, Trivy scanning, Helm chart (`deploy/helm/scout/`). Remaining: Kubernetes job template, `examples/docker/`.
- **Effort:** Medium
- **Dependencies:** Unified CLI (done), gRPC server (done), platform detection (done).

### ~~Screen Recorder (Phase 30)~~ [DONE]

- **Priority:** P3
- **Status:** Complete — `ScreenRecorder` type with CDP screencast, `ScreenRecordOption` functional options, `ExportGIF()`, `ExportFrames()`, Start/Stop lifecycle, CLI `scout record start/stop/export`.
- **Effort:** Large

---

### ~~Test Coverage Gaps~~ [DONE]

- **Priority:** P1
- **Status:** Complete — pkg/scout coverage raised from 69.9% to 80.1%. Page and element methods now have extensive test coverage.
- **Effort:** Large

### ~~Element Method Test Coverage~~ [DONE]

- **Priority:** P1
- **Status:** Complete — DoubleClick, RightClick, Hover, Tap, Type, Press, SelectOptionByCSS, SetFiles, Focus, Blur, ScrollIntoView, Remove, SelectAllText, GetXPath, ContainsElement, Equal,
  CanvasToImage, BackgroundImage, Resource, Parents, Wait* all tested. Previous/ShadowRoot/Frame skip gracefully due to rod limitations.
- **Effort:** Large

### ~~EvalResult Type Conversion Tests~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `eval_test.go` covers String, Int, Float, Bool, IsNull, JSON, Decode with table-driven tests
- **Effort:** Small

### ~~Network Accessor Tests~~ [DONE]

- **Priority:** P2
- **Status:** Complete — HijackRequestAccessors, HijackLoadResponse, HijackSkip, HijackResponseFail, HandleAuth all tested
- **Effort:** Medium

### ~~Missing LICENSE File~~ [DONE]

- **Priority:** P1
- **Status:** Complete — BSD 3-Clause LICENSE file added
- **Effort:** Small

### ~~gRPC Server Test Coverage~~ [DONE]

- **Priority:** P2
- **Status:** Complete — Coverage raised from 67.7% to 80.6%. Added tests for Interactive commands (Type, PressKey, Eval, Scroll, Wait), CreateSession options, pairing (5 tests), TLS (2 tests), mapKey, truncate, GetLocalIPs.
- **Effort:** Medium

### ~~Window Maximize Blank Space Bug~~ [DONE]

- **Priority:** P1
- **Status:** Fixed — `setWindowState()` clears `EmulationClearDeviceMetricsOverride` after maximize/fullscreen
- **Effort:** Small

### ~~Robot/Bot Detection Framework (Phase 17b prerequisite)~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `pkg/scout/challenge.go` with `ChallengeType` enum (9 types), `ChallengeInfo` struct, `Page.DetectChallenges()`, `Page.DetectChallenge()`, `Page.HasChallenge()`, JS-based detection for Cloudflare, Turnstile, reCAPTCHA v2/v3, hCaptcha, DataDome, PerimeterX, Akamai, AWS WAF. CLI `scout challenge detect <url>`. 5 tests.
- **Effort:** Medium

### ~~Browser Type Selector in Roadmap UI~~ [DONE]

- **Priority:** P2
- **Status:** Complete — `scout browser list` shows detected + downloaded browsers, Brave auto-downloads from GitHub releases, Edge error includes download URL. Implemented in `browser_download.go` and `cmd/scout/browser.go`.
- **Effort:** Small

### ~~GoDoc Examples~~ [DONE]

- **Priority:** P2
- **Status:** Complete — 20 `Example*` functions in `example_test.go` covering New, NewPage, Element, Click, Input, Extract, Eval, Markdown, Hijack, Crawl, Map, Search, Screenshot, WaitLoad, RateLimiter, NetworkRecorder, WithBlockPatterns, Page.Block, WithRemoteCDP, KeyPress
- **Effort:** Medium

### ~~Remove Legacy Taskfile Tasks~~ [DONE]

- **Priority:** P3
- **Status:** Complete — removed `proto:generate`, `sqlc:generate`, `generate`, `build:dev`, `build:prod`, `run`, `release`, `release:snapshot`, `release:check`. Added `lint:fix`, `slack-assist` to
  `grpc:build`.
- **Effort:** Small

### ~~Rod Fork Stability Patches (Phase 24)~~ [DONE]

- **Priority:** P1
- **Status:** Complete — all fork-level and wrapper-level patches applied, context propagation verified correct, zombie cleanup improved, dep-track.json updated, full test coverage
- **Effort:** Medium

### ~~Accessibility Snapshot (Phase 25)~~ [DONE]

- **Priority:** P1
- **Status:** Complete — `Page.Snapshot()`, `Page.ElementByRef()`, iframe traversal via `WithSnapshotIframes()`, LLM integration via `SnapshotWithLLM()`, CLI `scout snapshot`, 9+ tests
- **Effort:** Large

### ~~MCP Transport (Phase 26)~~ [DONE]

- **Priority:** P1
- **Status:** Complete — 15 MCP tools (navigate, click, type, screenshot, snapshot, extract, eval, back, forward, wait, search, fetch, pdf, session_list, session_reset) + 3 resources, in-memory transport tests, `scout mcp` command
- **Effort:** Large

### Browser Recycling — AutoFree (Phase 27)

- **Priority:** P2
- **Description:** Periodic browser process restart to prevent memory leaks in long-running daemon sessions. From go-rod/bartender analysis. Save/restore session state across recycles. `WithAutoFree(interval)` option.
- **Effort:** Medium
- **Dependencies:** gRPC daemon session management

### ~~Request Blocking Presets (Phase 27)~~ [DONE]

- **Priority:** P2
- **Status:** Complete — `WithBlockPatterns(patterns...)` option, `BlockAds`/`BlockTrackers`/`BlockFonts`/`BlockImages` presets, `Page.Block()` convenience method, 4 tests
- **Effort:** Small

### ~~Named Recipe Selectors~~ [DONE]

- **Priority:** P2
- **Status:** Complete — `selectors` map in recipe JSON, `$name` references resolved at parse time with `+` prefix and `@attr` suffix preservation, 5 tests (parse + e2e browser)
- **Effort:** Small

### ~~Framework Detection~~ [DONE]

- **Priority:** P2
- **Status:** Complete — `Page.DetectFrameworks()` and `Page.DetectFramework()` in `detect.go`, 14 frameworks detected (React, Vue, Angular, Svelte, Next.js, Nuxt, SvelteKit, Remix, Gatsby, Astro, Ember, Backbone, AngularJS, jQuery), meta-framework precedence, SPA flag, 11 tests
- **Effort:** Small

### Page Intelligence — PWA, Render Mode & Tech Stack (Phase 28)

- **Priority:** P2
- **Description:** Extend framework detection into a comprehensive page intelligence system. PWA detection (service workers, manifest, installability). Rendering mode classification (CSR/SSR/SSG/ISR). Technology stack analysis (CSS frameworks, build tools, CMS, analytics, CDN). Smart framework-aware wait strategies (`WaitFrameworkReady`). CLI `scout detect`.
- **Scope:** `pkg/scout/detect.go` (extend), `pkg/scout/detect_pwa.go`, `pkg/scout/detect_tech.go`, `pkg/scout/wait_smart.go`. CLI: `scout detect <url> [--framework] [--pwa] [--tech] [--json]`.
- **Effort:** Large
- **Dependencies:** Framework detection (done)

### Remote CDP Endpoint Support

- **Priority:** P3
- **Description:** `WithRemoteCDP(endpoint)` option for connecting to managed browser services (BrightData, Browserless, etc.) and remote Chrome instances. From rod issue #1092.
- **Effort:** Small
- **Dependencies:** None

### ~~Forgeron Fingerprint Integration~~ [DONE]

- **Priority:** P3
- **Status:** Complete — Forgeron integrated for diverse browser fingerprint generation complementing stealth mode. Released in v0.21.0.
- **Effort:** Medium

### ~~Bridge Form Auto-Fill and Download Management~~ [DONE]

- **Priority:** P2
- **Status:** Complete — Bridge form auto-fill from profile data and download management (intercept/track/auto-rename) implemented in v0.22.0.
- **Effort:** Medium

### VPN Extension Integration — Phase 33

- **Priority:** P2
- **Description:** Surfshark proxy control via CDP for VPN extension integration. Enable per-session proxy rotation and geo-targeting through browser extension APIs.
- **Effort:** Large
- **Dependencies:** Chrome Extension Loading (done), Bridge Extension (done)

### Scraper Modes — Reddit, YouTube, Notion

- **Priority:** P2
- **Description:** Dedicated scraper modes for Reddit (posts, comments, subreddits), YouTube (video metadata, comments, channels), and Notion (pages, databases, blocks). Each provides structured extraction via headless browser automation.
- **Effort:** Large
- **Dependencies:** Generic auth framework (done), extraction engine (done)

### Research Agent Depth Configuration and Caching

- **Priority:** P3
- **Description:** Add configurable depth levels for the research agent (shallow/medium/deep), result caching with TTL, and incremental research that builds on previous results.
- **Effort:** Small
- **Dependencies:** Research agent (done)

### Bot Probe Insights → Stealth Evasion Improvements

- **Priority:** P3
- **Description:** Use bot detection probe results (`botdetect_probe_test.go`) to identify and fix failing stealth checks. Analyze probe reports across bare/stealth/stealth+fingerprint modes to prioritize evasion improvements in `pkg/stealth/`.
- **Effort:** Medium
- **Dependencies:** Bot detection probes (done), Stealth mode (done), Forgeron integration (done)

### Fingerprint Profile Persistence and Rotation

- **Priority:** P3
- **Description:** Persist generated Forgeron fingerprints to disk for reuse across sessions. Support fingerprint rotation strategies (per-session, per-domain, time-based) for long-running scraping.
- **Effort:** Small
- **Dependencies:** Forgeron integration (done), Profile system (done)

## Resolved Items

| Item                             | Resolution                                                                                               | Date    |
|----------------------------------|----------------------------------------------------------------------------------------------------------|---------|
| Missing Git Remote               | Remote configured at `github.com/inovacc/scout.git`                                                      | 2025    |
| Taskfile Cleanup                 | Legacy template tasks replaced with valid proto/grpc tasks                                               | 2025    |
| Slack Scraper Mode               | Full implementation: API client, browser auth, encrypted session capture, CLI                            | 2026-02 |
| Remove Legacy Taskfile Tasks     | Removed all non-applicable tasks, added lint:fix and slack-assist build                                  | 2026-02 |
| EvalResult Type Conversion Tests | Full coverage: String, Int, Float, Bool, IsNull, JSON, Decode                                            | 2026-02 |
| Unified CLI                      | Single Cobra binary `cmd/scout/` replaces cmd/server, cmd/client, cmd/slack-assist, cmd/example-workflow | 2026-02 |
| Missing LICENSE File             | BSD 3-Clause LICENSE file added                                                                          | 2026-02 |
| Firecrawl Integration            | Pure HTTP Go client for Firecrawl v2 API with CLI commands                                               | 2026-02 |
| HTML-to-Markdown Engine          | Pure Go converter with readability scoring, `page.Markdown()`, CLI command                               | 2026-02 |
| URL Map / Link Discovery         | `Map()` with sitemap + BFS link harvesting, filters, CLI `scout map`                                     | 2026-02 |
| Test Coverage Gaps               | pkg/scout coverage raised from 69.9% to 80.1%                                                            | 2026-02 |
| Element Method Test Coverage     | Comprehensive element method tests added                                                                 | 2026-02 |
| Network Accessor Tests           | Hijack request/response accessor tests added                                                             | 2026-02 |
| Stealth Internalization          | `go-rod/stealth` internalized into `pkg/stealth/`                                                        | 2026-02 |
| Browser Auto-Detection           | Brave and Edge browser auto-detection via `WithBrowser()`                                                | 2026-02 |
| Chrome Extension Loading         | `WithExtension(paths...)` for unpacked extension loading                                                 | 2026-02 |
| Device Identity & mTLS           | Syncthing-style device IDs, mTLS auth, mDNS discovery                                                    | 2026-02 |
| Platform Session Defaults        | Auto `--no-sandbox` on Linux via build constraints                                                       | 2026-02 |
| Firecrawl Removal                | `firecrawl/` package removed — project focuses on native browser scraping                                | 2026-02 |
| Slack Removal                    | `scraper/slack/` package removed — replaced by generic auth framework                                    | 2026-02 |
| Multi-Engine Search Command      | Engine registry with Google, Bing, DDG, Wikipedia, Scholar, News in `search_engines.go`                  | 2026-02 |
| Batch Scraper                    | `BatchScrape()` with concurrency, error isolation, progress callback in `batch.go`                       | 2026-02 |
| Swagger/OpenAPI Extraction       | `pkg/scout/swagger.go` with Swagger UI 3+/2.0 detection, spec parsing, schema/security extraction, CLI `scout swagger` | 2026-02 |
| Chrome Extension Download        | `DownloadExtension(id)` with CRX2/CRX3 parsing, `~/.scout/extensions/` storage, `WithExtensionByID()`, CLI download/remove | 2026-02 |
| Scout Bridge Extension (partial) | `WithBridge()` option, embedded Manifest V3 extension in `extensions/scout-bridge/` via `embed.FS`, auto-load at startup | 2026-02 |
| LLM-Powered Extraction | `ExtractWithLLM()`, `ExtractWithLLMReview()`, workspace persistence, 6 providers (Ollama, OpenAI, Anthropic, OpenRouter, DeepSeek, Gemini), CLI `extract-ai`/`ollama`/`ai-job` | 2026-02 |
| Sitemap Extract | `SitemapExtract()` — BFS crawl + bridge DOM/Markdown extraction per page, output directory support, CLI `scout sitemap extract` | 2026-02 |
| Browser Type Selector | `scout browser list`, Brave auto-download from GitHub releases, Edge download URL in error, `browser_download.go` + `cmd/scout/browser.go` | 2026-02 |
| Docker Images (core) | `Dockerfile` (Chromium + scout), `Dockerfile.slim` (distroless), `.dockerignore`, `docker-compose.yml`, `scout browser download` command | 2026-02 |
| WebFetch implementation | `webfetch.go` with 6 content modes, caching, batch, 13 tests at 88%+ coverage, CLI `scout fetch` | 2026-02 |
| Recipe Creator | `analyze.go` + `generate.go` with site analysis, container/field detection, recipe generation, 11 tests at 81.5% coverage | 2026-02 |
| Stealth Mode Expansion | `stealth_extra.go` with 5 evasions, `disable-blink-features` launch flag, bot detection integration tests against 6 real sites | 2026-02 |
| Window Maximize Bug | Fixed: `setWindowState()` clears DeviceMetricsOverride after maximize/fullscreen | 2026-02 |
| WebSearch implementation | `websearch.go` with WebSearchResult/WebSearchItem types, 9 option functions, concurrent fetch, CLI `scout websearch`, 7 tests | 2026-02 |
| gRPC Server Test Coverage | Coverage raised from 67.7% to 80.6% with Interactive, pairing, TLS, mapKey, truncate, GetLocalIPs tests | 2026-02 |
| Rod Ecosystem Analysis | ADR 007 with analysis of wayang, bartender, rod-mcp, and 93 rod issues; patch plan for `pkg/rod/` | 2026-02 |
| Request Blocking Presets | `WithBlockPatterns()`, `BlockAds`/`BlockTrackers`/`BlockFonts`/`BlockImages` presets, `Page.Block()`, 4 tests | 2026-02 |
| Named Recipe Selectors | `selectors` map with `$name` references in recipe JSON, resolved at parse time, 5 tests | 2026-02 |
| Remote CDP Endpoint | `WithRemoteCDP(endpoint)` option for connecting to managed browser services, 2 tests | 2026-02 |
| GoDoc Examples | 20 `Example*` functions covering all major API entry points in `example_test.go` | 2026-02 |
| Framework Detection | `DetectFrameworks()` / `DetectFramework()` detecting 14 frameworks with version + SPA flag, 11 tests | 2026-02 |
| Rod Fork Patches (Phase 24) | Nil-guard on disconnected page, WaitSafe method, hijack regexp validation, 3 tests | 2026-02 |
| Accessibility Snapshot (Phase 25) | `Page.Snapshot()`, `Page.ElementByRef()`, snapshot JS engine, 9 tests | 2026-02 |
| MCP Transport (Phase 26) | MCP server with 10 tools + 3 resources via stdio, `scout mcp` command | 2026-02 |
| Bot Detection Framework | `DetectChallenges()` for 9 challenge types (Cloudflare, Turnstile, reCAPTCHA, hCaptcha, DataDome, etc.), CLI `scout challenge detect` | 2026-02 |
| Credential Capture (Phase 29) | `CaptureCredentials()`, `SaveCredentials()`, `LoadCredentials()`, CLI `scout credentials capture/replay/show` | 2026-02 |
| PWA Detection | `Page.DetectPWA()` with service worker, manifest, installability, HTTPS, push detection, 5 tests | 2026-02 |
| Tech Stack Detection | `Page.DetectTechStack()` with CSS/build/CMS/analytics/CDN detection, 4 tests | 2026-02 |
| Render Mode Detection | `Page.DetectRenderMode()` CSR/SSR/SSG/ISR classification, 6 tests | 2026-02 |
| CLI scout detect | Unified `scout detect <url>` with --framework/--pwa/--tech/--render/--json flags | 2026-02 |
| Custom JS Injection (Phase 16 core) | `WithInjectJS()`, `WithInjectDir()`, `WithInjectCode()`, CLI `scout inject`, 5 tests | 2026-02 |
| Profile Encryption | `SaveProfileEncrypted()`, `LoadProfileEncrypted()` with AES-256-GCM + Argon2id | 2026-02 |
| Profile Merge/Diff | `MergeProfiles()`, `DiffProfiles()`, `Validate()`, CLI `scout profile merge/diff`, 9 tests | 2026-02 |
| WebMCP Discovery (Phase 26b) | `DiscoverWebMCPTools()`, `CallWebMCPTool()`, meta/link/script/.well-known discovery, JSON-RPC + JS invocation, CLI, 10 tests | 2026-02 |
| Async Job System (Phase 15) | `AsyncJobManager` with persistent JSON state, lifecycle management, CLI `scout jobs list/status/cancel`, 7 tests | 2026-02 |
| Smart Wait Strategies | `WaitFrameworkReady()` with per-framework JS (React, Angular, Vue, Next.js, Nuxt, Svelte) | 2026-02 |
| Multi-engine Search (Phase 23b) | `WithSearchEngines()` with RRF scoring, `WithSearchDomain()`, `WithSearchExcludeDomain()`, 7 tests | 2026-02 |
| Recipe Validation (Phase 12c) | `ValidateRecipe()`, `SelectorHealthCheck()`, CLI `scout recipe test`, 5 tests | 2026-02 |
| WebFetch Retry + Redirects (Phase 23a) | `WithFetchRetries()`, `WithFetchRetryDelay()`, `RedirectChain` tracking, 4 tests | 2026-02 |
| Browser AutoFree (Phase 27) | `WithAutoFree(interval)` periodic recycling with session preservation, 4 tests | 2026-02 |
| Selector Resilience Scoring (Phase 12c) | `SelectorScore` type, `ScoreSelector()`, `ScoreRecipeSelectors()` for stability heuristics | 2026-02 |
| Bridge WebSocket Transport (Phase 17) | `BridgeServer`, `BridgeMessage`, `BridgeEvent` types, WS server, DOM mutation/interaction/navigation events | 2026-02 |
| Profile Hydration Tests (Phase 18) | Cookie hydration, storage hydration, identity injection tests passing end-to-end | 2026-02 |
| Profile gRPC RPCs (Phase 18) | `CaptureProfile`, `LoadProfile` RPCs for remote session profile management | 2026-02 |
| Docker CI/CD Publishing (Phase 21b) | GitHub Actions workflow, GHCR publishing, multi-arch builds, Trivy scanning, Helm chart | 2026-02 |
| GitHub Data Extraction CLI + Tests (Phase 23c) | `scout github repo/issues/prs/user/releases/tree` CLI commands, mock httptest pages | 2026-02 |
| Recipe Interactive Mode (Phase 12c) | `InteractiveCreate` wizard, `scout recipe create --interactive`, AI integration tests | 2026-02 |
| Built-in Extraction Helpers (Phase 16) | `InjectHelper`/`InjectAllHelpers` with 5 bundled JS helpers (table, scroll, shadow DOM, wait, click all) | 2026-02 |
| Script Templates (Phase 16) | `ScriptTemplate`, `RenderTemplate`, `InjectTemplate`, `BuiltinTemplates` (extract-list, fill-form, scroll-and-collect) | 2026-02 |
| Session-scoped gRPC Injection (Phase 16) | `InjectJS` RPC for dynamic JS injection into running sessions | 2026-02 |
| Bridge DOM Manipulation (Phase 17) | `QueryDOM`, `ClickElement`, `TypeText`, `InsertHTML`, `RemoveElement`, `ModifyAttribute`, `ObserveDOM` bridge commands | 2026-02 |
| Bridge Clipboard & Tab Management (Phase 17) | `GetClipboard`, `SetClipboard`, `ListTabs`, `CloseTab` bridge commands | 2026-02 |
| Bridge Console Forwarding (Phase 17) | `ConsoleMessages` bridge command for console capture/forwarding | 2026-02 |
| Recipe CLI Integration Tests (Phase 12c) | End-to-end CLI tests for `recipe create` and `recipe test`, selector `$ref` resolution tests | 2026-02 |
| Rod Fork Patches Complete (Phase 24) | Context propagation verified, page context in Info/Activate verified, zombie cleanup improved, dep-track.json updated, patch tests added | 2026-02 |
| MCP Additional Tools (Phase 26) | 5 new tools (search, fetch, pdf, session_list, session_reset), in-memory transport tests | 2026-02 |
| Accessibility Iframe/LLM (Phase 25) | `WithSnapshotIframes()` for iframe traversal, `SnapshotWithLLM()` for LLM integration, CLI `scout snapshot` | 2026-02 |
| Bridge Record Command (Phase 17) | `BridgeRecorder` with `RecordedStep`/`RecordedRecipe` types, `scout bridge record`, WS protocol tests, bridge-record example | 2026-02 |
| Recipe LLM Validation (Phase 12c) | `ValidateWithLLM()` with `LLMValidation` type for recipe completeness review | 2026-02 |
| Recipe Flow Detection (Phase 12c) | `DetectFlow()`, `GenerateFlowRecipe()` with `FlowStep`/`FormInfo` types, `scout recipe flow` CLI | 2026-02 |
| Profile Extension Resolution (Phase 18) | `ResolveExtensions()`, `ResolveExtensionsWithBase()` for extension ID→path resolution | 2026-02 |
| Profile CLI Integration (Phase 18) | `scout session create --profile`, CLI integration tests, Phase 18 marked COMPLETE | 2026-02 |
| AI-Powered Bot Protection Bypass (Phase 17b) | `ChallengeSolver`, `NavigateWithBypass()`, `WithAutoBypass()`, TwoCaptcha/CapSolver services, Cloudflare/Turnstile/CAPTCHA solving | 2026-02 |
| Scout-Browser Module (Phase 21c) | `pkg/browser/` with Manager, Detect, Download, BrowserInfo, platform-specific detection | 2026-02 |
| Screen Recorder (Phase 30) | `ScreenRecorder` with CDP screencast, `ExportGIF()`, `ExportFrames()`, CLI `scout record` | 2026-02 |
| Forgeron Fingerprint Integration | Forgeron library integrated for diverse browser fingerprint generation complementing stealth mode | 2026-02-25 |
| Scout-Browser Standalone Module (Phase 30) | `pkg/browser/` with Manager, Detect, Download, platform-specific version detection | 2026-02-25 |
| Bridge window.__scout API (Phase 31) | Content script bidirectional RPC with `__scout.send()`, `__scout.on()`, `__scout.query()`, CDP fallback | 2026-02-25 |
| Research Agent (Phase 31) | Orchestrated multi-source research workflows via `scout research` | 2026-02-25 |
| Bridge Form Auto-Fill and Download Management | Bridge form auto-fill from profile data and download management (intercept/track/auto-rename) in v0.22.0 | 2026-02-25 |
| VPN Provider API (Phase 33a) | Pluggable VPN interface with Surfshark integration, direct proxy support, auth handling | 2026-02-25 |
| VPN Extension Control (Phase 33b) | Chrome extension CDP manipulation, proxy settings, WebRTC leak prevention, connection tracking | 2026-02-25 |
| VPN Server Rotation (Phase 33c) | Per-page/interval rotation, country-based selection, CLI commands, split tunneling bypass list | 2026-02-25 |
| Stealth evasion fixes (5 new checks) | Added canvas/audio fingerprint noise, WebGL vendor spoofing, navigator.connection, Notification.permission | 2026-02-25 |
