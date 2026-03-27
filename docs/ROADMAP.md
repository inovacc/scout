# Project Roadmap

## Current Status

**Core library complete through Phase 58.** All phases delivered. See git history for details.

### Completed Phases (Summary)

| Phase | Feature | Status |
|-------|---------|--------|
| 1 | Core API (Browser, Page, Element, Eval) | Done |
| 2 | Advanced Features (screenshots, PDF, hijack, stealth, emulation) | Done |
| 3 | Scraping Toolkit (extract, forms, pagination, search, crawl) | Done |
| 4 | gRPC Service Layer (mTLS, pairing, 25+ RPCs) | Done |
| 5 | Unified CLI (50+ Cobra subcommands, daemon) | Done |
| 6–11 | Swagger, extensions, batch, map, markdown, multi-engine search | Done |
| 12 | Recipe System (extract, automate, validate, interactive, flow) | Done |
| 13 | WebFetch & WebSearch (multi-engine, GitHub extraction) | Done |
| 14 | LLM-Powered Extraction (6 providers, workspace, review pipeline) | Done |
| 15 | Async Job System (persistent state, cancellation) | Done |
| 16 | Custom JS Injection (helpers, templates, gRPC InjectJS) | Done |
| 17 | Bridge Extension (WebSocket, DOM, clipboard, tabs, recording) | Done |
| 17b | Bot Protection Bypass (challenge solver, CAPTCHA services) | Done |
| 18 | User Profiles (capture, encryption, merge, diff, gRPC RPCs) | Done |
| 19–20 | Credential Capture, Browser Auto-Detection | Done |
| 21 | Docker & Browser Manager (images, CI/CD, Helm, pkg/browser/) | Done |
| 22–23 | WebFetch/WebSearch (retry, redirects, multi-engine RRF) | Done |
| 24 | Rod Fork Patches (nil-guard, WaitSafe, zombie cleanup) | Done |
| 25 | Accessibility Snapshot (ARIA, iframes, LLM integration) | Done |
| 26 | MCP Server (41 tools, 3 resources, stdio + SSE transport) | Done |
| 27 | AutoFree & Request Blocking Presets | Done |
| 28 | Page Intelligence (framework, PWA, tech stack, render mode) | Done |
| 29 | Credential Capture & Replay | Done |
| 30 | Screen Recorder (CDP screencast, GIF export) | Done |
| 31 | Research Agent, window.__scout API, Forgeron fingerprints | Done |
| 32 | Bridge Form Auto-Fill & Download Management | Done |
| 33 | VPN Extension Integration (Surfshark, proxy rotation) | Done |
| 34 | Session Hijacking (real-time HTTP + WebSocket capture, gRPC streaming, CLI) | Done |
| 35 | Scraper Framework + Modes (Mode interface, Slack, Discord, Teams, Reddit) | Done |
| 36 | Scraper Modes Batch 2 (Gmail, Outlook, LinkedIn, Jira, Confluence) | Done |
| 37 | Scraper Modes Batch 3 (Twitter/X, YouTube, Notion, Google Drive, SharePoint) | Done |
| 38 | Scraper Modes Batch 4 (Amazon, Google Maps, Salesforce, Grafana, Cloud Consoles) | Done |
| 39 | Runbook rename, MCP SSE, test coverage, GoDoc examples | Done |
| 40 | Multi-tab orchestration (TabGroup), MCP expanded to 33 tools, ping/curl diagnostics | Done |
| 41 | Electron Support (runtime download, CDP connection, CLI flags) | Done |
| 42 | Command Logging (internal/flags, internal/logger, scout logger subcommand, PersistentPreRun capture) | Done |
| 43 | Launcher `browser.json` manifest (per-platform revisions, zip names, download hosts, auto-update via LAST_CHANGE) | Done |
| 44 | Session Reuse & Reset — `WithReusableSession()`, `WithTargetURL()`, domain-hash routing, `scout session reset [id|--all]`, orphan watchdog | Done |
| 45 | Site Health Checker — `scout test-site <url>` crawls site, detects broken links, console errors, JS exceptions, network failures; structured report (JSON/table) | Done |
| 46 | REPL Mode — `scout repl [url]` interactive local browser shell with 20 commands (navigate, eval, click, type, extract, screenshot, markdown, health, tabs, etc.) | Done |

| 47 | Page Gather — `scout gather <url>` one-shot page intelligence: DOM state, HAR, links, screenshots, cookies, metadata, console log, frameworks, accessibility snapshot | Done |
| 48 | Cloud Upload — `scout upload` with OAuth2 auth for Google Drive and OneDrive; `scout upload auth`, `scout upload file`, `scout upload status`; config persisted to `~/.scout/upload.json` | Done |
| 49 | Internal Migration — Move `pkg/scout/` to `internal/engine/` with public facade, extract domain sub-packages (detect, fingerprint, hijack, llm, vpn, session), merge rod internals into engine | Done |
| 49.5 | Process Management — `google/gops` agent for scout process discovery, `IsScoutProcess()` for reliable orphan detection (no PID reuse), `Page.WaitClose()` for browser window close detection, synchronous session dir cleanup, platform-specific process files (`_windows.go`, `_linux.go`) | Done |
| 51 | PDF Form Filling — `Page.PDFFormFields()` detects fillable fields, `Page.FillPDFForm(fields)` fills them via browser rendering; CLI: `scout pdf-form fields`, `scout pdf-form fill` | Done |
| 52 | Test Coverage — browser package 25→34%, session package 47→78%, ARM64 removal, Chromium revision update | Done |
| 53 | Plugin System — subprocess-based extensibility via JSON-RPC 2.0, plugin discovery (`~/.scout/plugins/`, `$SCOUT_PLUGIN_PATH`), scraper mode/extractor/MCP tool proxies, Go SDK for plugin authors, CLI `scout plugin list/install/remove/run` | Done |
| 54 | OpenTelemetry Tracing & Plugin URL Install — `internal/tracing/` package with `Init()`, `MCPToolSpan()`, `ScraperSpan()`; all 33 MCP tools auto-instrumented via `addTracedTool()` wrapper; scraper CLI instrumented; `scout plugin install <url>` downloads and extracts plugin archives; test coverage improvements for browser, MCP, and plugin packages | Done |
| 55 | MCP Reliability & New Tools — fix screenshot/navigate timeouts (`WithTimeout(0)`, best-effort `WaitLoad`), fix session reset (close page before browser, 500ms delay), enhanced snapshot tool (`maxDepth`/iframes/filter), `search_and_extract` MCP tool, `scout connect` CLI command, resolve all 28 golangci-lint issues | Done |
| 56 | Guide Generator & MCP Coverage — `pkg/scout/guide/` Recorder for step-by-step guides, `search_and_extract` parallel fetch, recipe→runbook deprecation aliases, MCP test coverage expansion | Done |
| 57 | Session Lifecycle — `CleanStaleSessions()` on startup removes non-reusable/orphaned sessions, session dir restructured to `<hash>/{scout.pid, job.json, data/}` separating metadata from browser profile, `DataDir()` API, Windows file lock retries (3×200ms), `job.json` tracking for session jobs | Done |
| 58 | Swarm Mode & Reports — `internal/engine/swarm/` distributed crawling (coordinator, worker, domain-partitioned queue), `scout swarm start <url>`, report system (`~/.scout/reports/{uuidv7}.txt`) with AI-consumable markdown format, gather/crawl/health report types, `scout report list/show/delete`, default browser `BestCached()` fallback, deprecated recipe package removed, 700+ tests added across all scraper modes | Done |

### Phase 59 — Plugin System v2: Full Extension Framework [DONE]

**Goal:** Evolve the plugin system from 3 capabilities (`scraper_mode`, `extractor`, `mcp_tool`) into a comprehensive extension framework with 8 capability types. Plugins become first-class Scout citizens that can hook into browser lifecycle, provide auth strategies, expose MCP resources/prompts, react to events, and ship results to external sinks.

**Status:** All 6 sub-phases complete. 8 capability types implemented. 12 plugin binaries ship under `plugins/`.

#### 59a — Browser Middleware (`browser_middleware`)

Plugins intercept and modify the page lifecycle at defined hook points. Inspired by thimble's 4-stage hook lifecycle and interceptor chain pattern.

**New RPC methods:**
- `middleware/before_navigate` — called before `Page.Navigate()`, can modify URL or block
- `middleware/after_load` — called after `Page.WaitLoad()`, can inject JS or modify DOM
- `middleware/before_extract` — called before extraction, can transform selectors
- `middleware/on_error` — called on page errors (network, JS), can retry or skip

**Manifest:**
```json
{
  "capabilities": ["browser_middleware"],
  "middleware": {
    "hooks": ["before_navigate", "after_load", "on_error"],
    "priority": 50
  }
}
```

**Design:**
- Middleware chain ordered by `priority` (0=first, 100=last)
- Each hook receives context (URL, page state, error) and returns action (`allow`, `block`, `modify`, `retry`)
- Core engine calls `MiddlewareChain.Execute(hook, ctx)` at each lifecycle point
- SDK: `server.RegisterMiddleware("before_navigate", handler)`
- New `MiddlewareProxy` in plugin manager dispatches to all registered middleware plugins

**Internal changes:**
- Add `MiddlewareChain` to `internal/engine/` with `Register()` and `Execute()` methods
- Inject chain into `Page.Navigate()`, `Page.WaitLoad()`, extraction paths
- `plugin.Manager` aggregates middleware from all discovered plugins

#### 59b — Auth Providers (`auth_provider`)

Plugins define custom authentication strategies. Currently `auth.Provider` is a Go interface requiring compiled-in implementations — this externalizes it via RPC.

**New RPC methods:**
- `auth/login_url` — returns the URL to start auth flow
- `auth/detect` — given page state (URL, cookies, localStorage keys), returns bool
- `auth/capture` — given page state, returns session data (cookies, tokens, storage)
- `auth/validate` — given saved session, returns validity + expiry info
- `auth/refresh` — given expired session, returns refreshed session (optional)

**Manifest:**
```json
{
  "capabilities": ["auth_provider"],
  "auth": {
    "name": "custom-saml",
    "description": "SAML SSO via corporate IdP",
    "login_url": "https://idp.example.com/sso",
    "session_fields": ["cookies", "tokens", "localStorage"]
  }
}
```

**Design:**
- `AuthProxy` implements `auth.Provider` interface, forwards to plugin via RPC
- Plugin receives serialized page state (not a live page handle) for security isolation
- Core `BrowserAuth()` flow unchanged — polls `detect`, calls `capture` on success
- SDK: `server.RegisterAuth(handler)` with `AuthHandler` interface
- Plugins can support OAuth2/PKCE via `auth/oauth_callback` if they need a local HTTP server (core provides `auth.OAuthServer` as a shared utility)

**Internal changes:**
- New `AuthProxy` in `pkg/scout/plugin/` implementing `auth.Provider`
- `auth.Register()` accepts both built-in and plugin providers
- Plugin manager auto-registers discovered auth providers

#### 59c — MCP Resources & Prompts (`mcp_resource`, `mcp_prompt`)

Expand MCP plugin support beyond tools. Currently Scout has 3 hardcoded resources and 0 prompts. Plugins can provide both.

**New RPC methods:**
- `resource/read` — given URI, returns resource content
- `resource/list` — returns available resources (for dynamic resource sets)
- `prompt/get` — given prompt name + arguments, returns message array
- `prompt/list` — returns available prompts

**Manifest:**
```json
{
  "capabilities": ["mcp_resource", "mcp_prompt"],
  "resources": [
    {"uri": "scout-plugin://reports/latest", "name": "Latest Report", "mimeType": "text/markdown"}
  ],
  "resource_templates": [
    {"uriTemplate": "scout-plugin://cache/{key}", "name": "Cache Entry"}
  ],
  "prompts": [
    {
      "name": "analyze_page",
      "description": "Analyze current page for accessibility issues",
      "arguments": [{"name": "severity", "required": false}]
    }
  ]
}
```

**Design:**
- `ResourceProxy` calls `resource/read` for each declared resource/template
- `PromptProxy` calls `prompt/get` for each declared prompt
- Resources namespaced: `scout-plugin://{plugin-name}/{resource-path}`
- Plugin manager registers all resources/prompts on MCP server via `server.AddResource()` and `server.AddPrompt()`
- SDK: `server.RegisterResource(uri, handler)`, `server.RegisterPrompt(name, handler)`

**Internal changes:**
- New `ResourceProxy`, `PromptProxy` in `pkg/scout/plugin/`
- `Manager.RegisterMCPResources(server)` and `Manager.RegisterMCPPrompts(server)` alongside existing `RegisterMCPTools()`

#### 59d — Event Hooks (`event_hook`)

Plugins subscribe to browser events and react in real-time. Bridges the existing `BridgeEvent` and `HijackEvent` systems to plugins.

**New RPC methods:**
- `event/subscribe` — declare interest in event types (returns immediately)
- Plugin receives events as **notifications** (no response needed): `event/emit`
- `event/action` — plugin can request actions in response (optional): navigate, screenshot, inject JS

**Manifest:**
```json
{
  "capabilities": ["event_hook"],
  "events": {
    "subscribe": ["dom.mutation", "navigation", "console.log", "network.request", "network.response", "ws.received"],
    "actions": ["inject_js", "screenshot"]
  }
}
```

**Design:**
- On plugin load, manager subscribes to declared event types on `BridgeServer.Subscribe()` and `SessionHijacker.Events()`
- Events forwarded to plugin as JSON-RPC notifications: `{method: "event/emit", params: {type, data, timestamp}}`
- Plugin can optionally send `event/action` requests back (e.g., take screenshot on specific DOM mutation)
- Actions are sandboxed — only pre-declared actions in manifest are allowed
- Event delivery is best-effort (buffered channel, dropped on overflow)
- SDK: `server.OnEvent(eventType, handler)` with `EventHandler` interface

**Internal changes:**
- New `EventProxy` in `pkg/scout/plugin/` subscribes to bridge/hijack events
- `Manager.ConnectEvents(bridge, hijacker)` wires event sources to plugin subscribers
- Rate limiting per plugin (configurable, default 100 events/sec)

#### 59e — Transport / Output Sinks (`output_sink`)

Plugins define custom destinations for scraper results, reports, and gathered data. Instead of stdout/JSON/file, results flow to S3, databases, webhooks, message queues, etc.

**New RPC methods:**
- `sink/init` — initialize connection (receives config from CLI flags or env)
- `sink/write` — send a batch of results
- `sink/flush` — ensure all buffered data is written
- `sink/close` — graceful shutdown

**Manifest:**
```json
{
  "capabilities": ["output_sink"],
  "sinks": [
    {
      "name": "s3",
      "description": "Write results to S3 bucket",
      "config_schema": {
        "type": "object",
        "properties": {
          "bucket": {"type": "string"},
          "prefix": {"type": "string"},
          "region": {"type": "string"}
        },
        "required": ["bucket"]
      }
    }
  ]
}
```

**Design:**
- `SinkProxy` implements a new `OutputSink` interface: `Init(config)`, `Write(results)`, `Flush()`, `Close()`
- CLI: `scout scrape --mode slack --sink s3 --sink-config bucket=my-data,prefix=slack/`
- Multiple sinks can be active simultaneously (fan-out)
- Built-in sinks (stdout, json-file, report) remain as defaults
- SDK: `server.RegisterSink(name, handler)` with `SinkHandler` interface
- Sink receives `[]scraper.Result` batches (same format as scrape output)

**Internal changes:**
- New `OutputSink` interface in `pkg/scout/scraper/`
- New `SinkProxy` in `pkg/scout/plugin/`
- `Manager.GetSink(name)` returns sink proxy
- Scraper runner fans out results to all active sinks

#### 59f — Scraper Modes to Plugins

Extract all 19 built-in scraper modes into standalone plugins using the expanded capability set (modes + auth providers + event hooks).

| Batch | Modes | Notes |
|-------|-------|-------|
| 1 — Communication | slack, discord, teams, reddit | Validates migration pattern |
| 2 — Email & Docs | gmail, outlook, linkedin, jira, confluence | Uses `auth_provider` capability |
| 3 — Content | twitter, youtube, notion, gdrive, sharepoint | Uses `event_hook` for API interception |
| 4 — Data & Enterprise | amazon, gmaps, salesforce, grafana, cloud | Multi-capability plugins |

**Migration per mode:**
1. Create plugin binary under `plugins/{mode}/main.go` using expanded SDK
2. Move `Mode` + `AuthProvider` into plugin (now both are RPC-capable)
3. Write `plugin.json` with `scraper_mode` + `auth_provider` capabilities
4. 30-day deprecation window; plugin takes precedence if installed
5. Ship pre-built binaries via GitHub Releases

---

**Implementation order:** 59c → 59b → 59e → 59a → 59d → 59f

| Sub-phase | Reason for order |
|-----------|-----------------|
| 59c (resources/prompts) | Simplest — extends existing MCP registration, no new engine hooks |
| 59b (auth providers) | Unblocks mode migration — modes need plugin auth to externalize |
| 59e (output sinks) | Independent of browser — pure data pipeline, easy to test |
| 59a (browser middleware) | Requires engine changes — hook points in Navigate/WaitLoad/Extract |
| 59d (event hooks) | Most complex — bridges two event systems, needs rate limiting |
| 59f (mode extraction) | Depends on 59b+59d — modes use auth and events |

**Protocol changes summary:**

| Capability | New RPC Methods | Manifest Fields | SDK Registration |
|-----------|----------------|-----------------|-----------------|
| `browser_middleware` | `middleware/*` (4) | `middleware.hooks`, `middleware.priority` | `RegisterMiddleware()` |
| `auth_provider` | `auth/*` (5) | `auth.name`, `auth.login_url`, `auth.session_fields` | `RegisterAuth()` |
| `mcp_resource` | `resource/*` (2) | `resources`, `resource_templates` | `RegisterResource()` |
| `mcp_prompt` | `prompt/*` (2) | `prompts` | `RegisterPrompt()` |
| `event_hook` | `event/*` (2) | `events.subscribe`, `events.actions` | `OnEvent()` |
| `output_sink` | `sink/*` (4) | `sinks[].name`, `sinks[].config_schema` | `RegisterSink()` |

**Success criteria:**
- All 8 capability types working with SDK + proxy + tests
- At least 1 reference plugin per new capability type
- Existing plugins unchanged (full backward compatibility)
- 19 scraper modes migrated to plugins
- Core binary size reduced significantly

### Phase 60 — TikTok Scraper Mode [DONE]

Add a `tiktok` scraper mode for video metadata, comments, profiles, and trending content. Follows the same pattern as Phases 35–38 (built-in first, migrated to plugin in 59f).

**Scope:**
- `pkg/scout/scraper/modes/tiktok/` — Mode + AuthProvider implementation
- Session hijacking for TikTok API interception (`/api/`, `/tiktok/item/`)
- Result types: `video`, `comment`, `profile`, `sound`, `hashtag`
- Auth: cookie-based (sessionid, tt_webid_v2) + localStorage tokens
- Rate limiting awareness (TikTok aggressive throttling)
- CLI: `scout scrape --mode tiktok --target <profile_url|hashtag>`

### Phase 61 — Strategy Files [DONE]

Declarative YAML/JSON strategy files that define multi-step browser automation workflows. A strategy file describes what to scrape, how to authenticate, which sinks to use, and orchestration logic — loaded by `scout strategy run -f strategy.yaml`.

**Strategy file format:**
```yaml
name: competitive-intel
version: "1.0"
browser:
  type: brave
  stealth: true
  headless: false

auth:
  provider: linkedin
  session: ~/.scout/sessions/linkedin.json
  capture_on_close: true    # uses CaptureOnClose if no session exists

steps:
  - name: scrape-profiles
    mode: linkedin
    targets:
      - https://linkedin.com/in/person1
      - https://linkedin.com/in/person2
    limit: 50

  - name: scrape-reviews
    mode: gmaps
    targets:
      - "restaurant name, city"
    limit: 100

output:
  sinks:
    - type: json-file
      path: ./results/
    - type: s3
      config:
        bucket: my-data
        prefix: intel/
  report: true
```

**Implementation:**
- `pkg/scout/strategy/` — parser, validator, executor
- `strategy.Strategy` struct with `Browser`, `Auth`, `Steps`, `Output` sections
- `strategy.Run(ctx, path)` — loads file, sets up browser/auth, executes steps sequentially
- Each step maps to an existing scraper mode or plugin mode
- Auth section integrates with `CaptureOnClose` for interactive credential capture
- Output section configures sinks (built-in or plugin)
- CLI: `scout strategy run -f <file>`, `scout strategy validate -f <file>`, `scout strategy init`
- Environment variable interpolation: `${ENV_VAR}` in strategy files
- Conditional steps: `when: { has_auth: true }` to skip steps based on state

### Phase 62 — API Middleware Proxy [DONE]

HTTP reverse proxy that sits between API consumers and legacy websites, exposing scraped data as REST/JSON endpoints. Turns any website into an API.

**Architecture:**
```
Client (curl, app) → Scout API Proxy → Browser Engine → Legacy Website
     GET /api/v1/search?q=...  →  navigate + extract  →  JSON response
```

**Implementation:**
- `pkg/scout/proxy/` — HTTP server, route registry, request→scrape pipeline
- `proxy.Server` wraps `http.Server` with browser pool (`ManagedPagePool`)
- Routes defined via config or strategy files:
  ```yaml
  routes:
    - path: /api/v1/products
      method: GET
      target: https://legacy-store.com/catalog
      extract:
        selector: ".product-card"
        fields:
          name: "h3"
          price: ".price"
          image: "img@src"
      cache_ttl: 5m

    - path: /api/v1/search
      method: GET
      target: https://legacy-store.com/search?q={{.query}}
      params: [query]
      extract:
        selector: ".result-item"
        fields:
          title: "a"
          url: "a@href"
  ```
- Browser pool reuses pages for performance; idle pages recycled
- Response caching with configurable TTL per route
- Rate limiting per client IP
- OpenAPI spec auto-generated from route definitions
- CLI: `scout proxy start -f routes.yaml [--port 8080]`, `scout proxy routes`
- Health endpoint: `GET /health` with browser pool status
- Webhook mode: `POST /api/v1/webhook` triggers scrape and POSTs result to callback URL

**Key features:**
- Template parameters: `{{.param}}` in target URLs from query string
- Pagination pass-through: `?page=N` maps to site pagination
- Auth session injection: routes reference named auth sessions
- Structured error responses with retry-after headers on rate limits
- Metrics endpoint for Prometheus/Grafana integration

### Phase 63 — CLI Command Plugin Capability [DONE]

**Goal:** Let any CLI command be expressible as a plugin. A new `cli_command` capability type allows plugins to provide or replace any Scout CLI command.

**Scope:**
- `CommandEntry` manifest type with args, flags, category, `replaces`, `requires_browser`
- `CommandProxy` synthesizes Cobra commands from plugin manifests at startup
- `command/execute` + `command/complete` RPC methods on SDK server
- `BrowserContext` provisioning (CDP endpoint forwarding) for `requires_browser: true` commands
- `Browser.CDPURL()` getter for CDP WebSocket URL
- `--builtin` escape hatch to force built-in command over plugin replacement
- SDK: `RegisterCommand()`, `ConnectBrowser()` helper

**ADR:** [docs/adr/007-mcp-plugin-migration.md](adr/007-mcp-plugin-migration.md)

### Phase 64–68 — MCP Tool Migration to Plugins [DONE]

**Goal:** Migrate 28 of 41 MCP tools from `pkg/scout/mcp/` into standalone plugin binaries using the `cli_command` + `mcp_tool` dual-capability pattern. Reduces core binary, enables independent release cycles.

| Phase | Wave | Plugin(s) | Tools | Risk |
|-------|------|-----------|-------|------|
| 64 | 1 — Diagnostics | `scout-diag`, `scout-reports` | ping, curl, report_list/show/delete | Low |
| 65 | 2 — Content | `scout-content` | markdown, table, meta, pdf | Medium |
| 66 | 3 — Search | `scout-search` | search, search_and_extract, fetch | Medium |
| 67 | 4 — Network/Forms | `scout-network`, `scout-forms` | cookie, header, block, storage, hijack, har, swagger, form_detect/fill/submit | Higher |
| 68 | 5 — Analysis/Guides | `scout-crawl`, `scout-guide` | crawl, detect, swarm_crawl, guide_start/step/finish | Highest |

Each wave follows 30-day deprecation: plugin released → warning on built-in → built-in removed.

**What stays built-in (13 tools):** navigate, click, type, extract, eval, back, forward, wait, screenshot, snapshot, session_list, session_reset, open.

---

### Phase 69 — Plugin Marketplace & Distribution [DONE]

**Goal:** First-class plugin discovery, installation, and updates.

**Delivered:**
- `scout plugin search <query>` — search GitHub-backed JSON index (`plugins/registry.json`)
- `scout plugin update [name|--all]` — update installed plugins to latest release
- Plugin version pinning in `~/.scout/plugins/lock.json` via `LockFile`
- SHA256 checksum verification (`registry.FileChecksum`, `VerifyChecksum`)
- `scout plugin install github:owner/plugin` shorthand for GitHub releases
- `pkg/scout/plugin/registry/` — registry client, lock file management

### Phase 70 — WebSocket Automation [DONE]

**Goal:** First-class WebSocket support beyond session hijacking.

**Delivered:**
- `Page.MonitorWebSockets(opts...)` — real-time WS message capture via JS interceptor
- `WithWSURLFilter(pattern)` / `WithWSCaptureAll()` options
- CLI: `scout ws listen <url> [--filter --timeout --json]`
- MCP tools: `ws_listen`, `ws_send`, `ws_connections`
- `WebSocketConnection`, `WebSocketMessage`, `WebSocketHandler` types exported via facade

### Phase 71 — AI Agent Integration [DONE]

**Goal:** Scout as a tool provider for AI agent frameworks.

**Delivered:**
- `pkg/scout/agent/` — `Provider` with 9 built-in tools (navigate, screenshot, extract_text, click, type_text, markdown, eval, page_url, page_title)
- `OpenAITools()` — schemas in OpenAI function calling format
- `AnthropicTools()` — schemas in Anthropic tool_use format
- `ToolSchemaJSON()` — JSON export for any framework
- `Call(ctx, name, args)` — tool execution with error wrapping

### Phase 72 — Visual Testing & Monitoring [DONE]

**Goal:** Continuous visual regression testing and site monitoring.

**Delivered:**
- `pkg/scout/monitor/` — `Monitor` with interval-based screenshot capture + diff
- `BaselineManager` — capture, load, list baselines (SHA256 checksums, PNG + metadata JSON)
- `Compare()` — pixel-level image diff with configurable threshold
- `ChangeHandler` callback on visual change detection
- Auto-baseline capture on first check

### Phase 72.5 — Deprecation Cleanup [DONE]

**Goal:** Remove deprecated built-in MCP tools replaced by plugins in Phases 64–68.

**Delivered:**
- Removed 28 deprecated MCP tools (diag, reports, content, search, network, forms, inspect, analysis, guide)
- 9 tool implementation files and 8 test files deleted
- Core MCP server reduced to 18 built-in tools (navigate, click, type, extract, eval, back, forward, wait, screenshot, snapshot, pdf, session_list, session_reset, open, swarm_crawl, ws_listen, ws_send, ws_connections)
- All 28 tools remain available as standalone plugins

---

### Phase 73 — Mobile Browser Support [DONE]

**Goal:** Extend Scout to mobile browser automation.

**Delivered:**
- `WithMobile(cfg)` option with ADB device connection via CDP port forwarding
- `WithTouchEmulation()` for touch simulation on desktop
- `ListADBDevices()`, `SetupADBForward()`, `RemoveADBForward()` in `internal/engine/mobile_adb.go`
- Touch gestures on `Page`: `Touch()`, `Swipe()`, `PinchZoom()` via CDP `InputDispatchTouchEvent`
- CLI: `scout mobile devices [--json]`, `scout mobile connect [--device --port --url]`
- Public facade exports: `MobileConfig`, `TouchPoint`, `ADBDevice`, `WithMobile`, `WithTouchEmulation`

### Phase 73.5 — WebSocket HAR Recording [DONE]

**Goal:** Record WebSocket traffic in HAR format for replay and debugging.

**Delivered:**
- `HARWebSocketMessage`, `HARWebSocket` types in `internal/engine/hijack/har.go`
- `_webSocketMessages` custom HAR extension field (Chrome DevTools convention)
- `Recorder.Record()` now handles WSOpened, WSSent, WSReceived, WSClosed events
- `ExportHAR()` includes WebSocket entries alongside HTTP entries
- `ExportWebSocketHAR()` — WebSocket-only HAR export
- `WebSocketCount()`, `WebSocketMessageCount()` methods

### Phase 73.6 — Agent HTTP Server [DONE]

**Goal:** REST API for AI agent frameworks (LangChain, CrewAI, etc.).

**Delivered:**
- `pkg/scout/agent/server.go` — HTTP server wrapping Provider with 6 endpoints
- `GET /health` — server status and tool count
- `GET /tools`, `GET /tools/openai` — OpenAI function calling format
- `GET /tools/anthropic` — Anthropic tool_use format
- `GET /tools/schema` — full JSON schema
- `POST /call` — execute tool by name: `{"name": "navigate", "arguments": {"url": "..."}}`
- Idle timeout auto-shutdown, graceful shutdown via context
- CLI: `scout agent serve [--addr --headless --stealth --browser --idle-timeout]`
- CLI: `scout agent tools [--format openai|anthropic]`

### Phase 73.7 — Claude Code Plugin [DONE]

**Goal:** Package Scout as a Claude Code plugin for direct LLM integration.

**Delivered:**
- `.claude-plugin/plugin.json` — plugin manifest with metadata and keywords
- `.mcp.json` — MCP server config (scout mcp --headless --stealth)
- 6 skills: `/scout:scrape`, `/scout:screenshot`, `/scout:test-site`, `/scout:gather`, `/scout:crawl`, `/scout:monitor`
- 3 agents: `web-scraper`, `site-tester`, `browser-automation`
- SessionStart hook with `scripts/check-scout.sh` for binary verification
- Test locally: `claude --plugin-dir .`

---

### Phase 74 — Cloud Deployment [DONE]

**Goal:** Run Scout as a managed service.

**Delivered:**
- Helm chart at `deploy/helm/scout/` with HPA, PVC, multi-port service (agent/gRPC)
- CLI: `scout cloud deploy`, `scout cloud status`, `scout cloud scale <N>`, `scout cloud uninstall`
- `internal/metrics/` — zero-dependency Prometheus + JSON metrics (pages, navigations, screenshots, extractions, errors, tool calls)
- GoReleaser config (`.goreleaser.yaml`) + GitHub Actions release workflow
- Auto-download binary in plugin SessionStart hook

---

## Phase 75+ — Future

### Remaining Work

See [BACKLOG.md](BACKLOG.md) for future work.
