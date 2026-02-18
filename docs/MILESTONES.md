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
- [x] Internalized `go-rod/stealth` into `pkg/stealth/`
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

## v0.7.4 - Extension Download & CRX Support [IN PROGRESS]

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
- **Coverage:** pkg/scout 75.3%

## v0.8.0 - Screen Recorder [PLANNED]

**Goal:** Capture browser sessions as video for forensic evidence.

- [ ] ScreenRecorder type using CDP `Page.startScreencast`
- [ ] WebM/GIF/PNG export formats
- [ ] gRPC RPCs and CLI commands (`scout record start/stop/export`)
- [ ] Combined HAR+video forensic bundles

## v0.9.0 - Distributed Crawling [PLANNED]

**Goal:** Swarm-mode crawling across multiple browser instances.

- [ ] Browser cluster / pool management
- [ ] Multi-proxy swarm distribution
- [ ] Shared work queue for BFS crawling
- [ ] Result aggregation

## v1.0.0 - Documentation & Release [IN PROGRESS]

**Goal:** Comprehensive documentation and stable release.

- [x] Published to GitHub with git remote
- [x] Git tags (v0.1.3, v0.1.4, v0.1.5)
- [x] LICENSE file
- [ ] GoDoc examples for Browser, Page, Element, EvalResult, and new features
- [ ] Integration test examples (login flow, form submission, scraping)
- [ ] 80%+ test coverage (pkg/scout: 75.3% — regressed with new features)
- **Coverage:** pkg/scout 75.3% | pkg/identity 81.1% | scraper 84.3%
