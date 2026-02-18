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

### Multi-Browser Support

- **Status:** Completed
- **Description:** Auto-detection for Brave and Microsoft Edge browsers on Windows, macOS, and Linux. `WithBrowser(BrowserBrave)` or `--browser=brave`. Platform-specific path resolution in
  `browser_path_*.go`.

### Chrome Extension Loading

- **Status:** Completed
- **Description:** Load unpacked Chrome extensions via `WithExtension(paths...)`. Sets `--load-extension` and `--disable-extensions-except` flags. CLI `scout extension load/test/list`.

### Device Identity & mTLS

- **Status:** Completed
- **Description:** Syncthing-style device IDs with Ed25519 keys and Luhn check digits (`pkg/identity/`). Mutual TLS authentication for gRPC connections (`grpc/server/tls.go`). Device pairing handshake
  for certificate exchange (`grpc/server/pairing.go`). mDNS peer discovery (`pkg/discovery/`). CLI `scout device pair/list/trust`.

### Internalized Stealth

- **Status:** Completed
- **Description:** `go-rod/stealth` forked and internalized into `pkg/stealth/`. Removes external dependency while maintaining anti-bot-detection capabilities.

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

### Scout Bridge Extension (Partial)

- **Status:** In Progress
- **Description:** Built-in Manifest V3 Chrome extension for bidirectional Go↔browser communication. Extension scaffold embedded in `bridge_assets.go`, auto-loaded via `WithBridge()` option. Full WebSocket transport, event streaming, and remote command capabilities planned. Partially implemented in `pkg/scout/bridge.go` and `pkg/scout/bridge_assets.go`.

### LLM-Powered Extraction

- **Status:** Completed
- **Description:** AI-powered data extraction using LLM providers. Send page content (as markdown) to an LLM with a natural language prompt, get structured data back. Pluggable `LLMProvider` interface with 6 built-in providers: Ollama (local), OpenAI, Anthropic, OpenRouter, DeepSeek, Gemini. Optional JSON schema validation. LLM review pipeline (`ExtractWithLLMReview()`) sends extraction output to a second LLM for quality review. Workspace persistence tracks sessions and jobs in a filesystem directory with `sessions.json`, `jobs/jobs.json`, and per-job `jobs/<uuid>/` folders. CLI `scout extract-ai`, `scout ollama list/pull/status`, `scout ai-job list/show/session`. Implemented in `pkg/scout/llm.go`, `llm_ollama.go`, `llm_openai.go`, `llm_anthropic.go`, `llm_review.go`, `llm_workspace.go`, `cmd/scout/llm.go`.

## Proposed Features

### Screen Recorder

- **Priority:** P2
- **Status:** Proposed
- **Description:** Capture browser sessions as video using CDP `Page.startScreencast`. Record page interactions as WebM, GIF, or PNG frame sequences. Combined HAR+video forensic bundles.

### Distributed Crawling (Swarm Mode)

- **Priority:** P2
- **Status:** Proposed
- **Description:** Split crawl workloads across multiple browser instances running on different IPs/proxies. Browser cluster management, shared BFS queue, result aggregation, headless swarm
  configuration.

### Context Support

- **Priority:** P2
- **Status:** Proposed
- **Description:** Accept `context.Context` on methods for cancellation and deadline propagation, instead of relying solely on rod's timeout mechanism.

### Connection to Existing Browser

- **Priority:** P2
- **Status:** Proposed
- **Description:** Add an option to connect to an already-running browser via WebSocket URL (rod supports `ControlURL`), useful for debugging and reusing browser sessions.

### Page Pool

- **Priority:** P3
- **Status:** Proposed
- **Description:** Page pooling for concurrent scraping workloads, managing a fixed number of pages and recycling them across tasks.
