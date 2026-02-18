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

### GitHub Scraper Mode

- **Priority:** P3
- **Description:** Scrape GitHub via browser automation for data beyond API limits. Extract repo content, issues, PRs, discussions, wikis, and actions.
- **Scope:** Repo metadata, issue/PR threads, discussions, wiki pages, actions logs, user profiles.
- **Effort:** Medium

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

### LLM-Powered Extraction

- **Priority:** P2
- **Description:** AI-powered data extraction using LLM providers. Send page content (as markdown) plus a natural language prompt to an LLM and get structured data back. Supports multiple providers
  via interface: OpenAI, Anthropic, Ollama (local). Optional JSON schema validation on responses.
- **Scope:** `pkg/scout/llm.go` with `ExtractWithLLM()` function and `LLMProvider` interface. Built-in providers for OpenAI, Anthropic, Ollama. CLI
  `scout extract-ai --url=<url> --prompt="..." [--provider=ollama] [--schema=file.json]`.
- **Effort:** Large
- **Dependencies:** Depends on HTML-to-Markdown engine (Phase 10) for page content preparation. HTTP clients for LLM APIs.

### Async Job System

- **Priority:** P3
- **Description:** Job manager for long-running batch and crawl operations. Provides job IDs, status polling, cancellation, and persistent state. Enables running large crawls/batches in the background
  with progress tracking.
- **Scope:** `pkg/scout/jobs.go` with job lifecycle management. Persistent state in `~/.scout/jobs/`. CLI `scout jobs list/status/cancel/wait`.
- **Effort:** Medium
- **Dependencies:** Integrates with batch scraper and crawl commands.

### Screen Recorder

- **Priority:** P3
- **Description:** Capture browser sessions as video using Chrome DevTools Protocol `Page.startScreencast`. Record page interactions as WebM, GIF, or PNG frame sequences. Complement the existing
  `NetworkRecorder` (HAR) with synchronized video evidence. Pure-Go WebM encoding with optional ffmpeg fallback for MP4.
- **Scope:** `ScreenRecorder` type in `pkg/scout/screenrecord.go` with functional options (`WithFrameRate`, `WithQuality`, `WithMaxDuration`, `WithFormat`). Start/Stop/Pause/Resume lifecycle. Export
  as WebM (primary), GIF (short clips), or PNG sequence. gRPC RPCs for remote control. CLI `scout record start/stop/export` commands. Combined HAR+video forensic bundles.
- **Effort:** Large
- **Dependencies:** CDP `Page.screencastFrame` events, go-rod's underlying protocol access. Pure-Go WebM/VP8 encoder or vendored library. Optional ffmpeg detection for MP4.

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

### gRPC Server Test Coverage

- **Priority:** P2
- **Description:** The `grpc/server/` package has 0% test coverage. All 25+ RPCs (CreateSession, Navigate, Click, Type, Screenshot, etc.) are untested. Consider integration tests against a local
  httptest server with a real browser.
- **Effort:** Large

### GoDoc Examples

- **Priority:** P2
- **Description:** Add `Example*` test functions for key API entry points: New, Browser.NewPage, Page.Element, Page.Eval, Page.Hijack, Element.Click, Element.Input, NetworkRecorder, KeyPress.
- **Effort:** Medium

### ~~Remove Legacy Taskfile Tasks~~ [DONE]

- **Priority:** P3
- **Status:** Complete — removed `proto:generate`, `sqlc:generate`, `generate`, `build:dev`, `build:prod`, `run`, `release`, `release:snapshot`, `release:check`. Added `lint:fix`, `slack-assist` to
  `grpc:build`.
- **Effort:** Small

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
