# Project Roadmap

## Current Status

**Core library complete through Phase 42.** All 42 phases delivered. See git history for details.

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
| 26 | MCP Server (33 tools, 3 resources, stdio + SSE transport) | Done |
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
| 43 | Launcher `browser.json` manifest (per-platform revisions, zip names, download hosts, auto-update via LAST_CHANGE) | Planned |
| 44 | Session Reuse & Reset — `WithReusableSession()`, `WithTargetURL()`, domain-hash routing, `scout session reset [id|--all]`, orphan watchdog | Done |
| 45 | Site Health Checker — `scout test-site <url>` crawls site, detects broken links, console errors, JS exceptions, network failures; structured report (JSON/table) | Done |
| 46 | REPL Mode — `scout repl [url]` interactive local browser shell with 20 commands (navigate, eval, click, type, extract, screenshot, markdown, health, tabs, etc.) | Done |

### Next Phase

| Phase | Feature | Status |
|-------|---------|--------|
| 47 | Page Gather — `scout gather <url>` one-shot page intelligence: DOM state, HAR, links, screenshots, cookies, metadata, console log, frameworks, accessibility snapshot | Done |
| 48 | Cloud Upload — `scout upload` with OAuth2 auth for Google Drive and OneDrive; `scout upload auth`, `scout upload file`, `scout upload status`; config persisted to `~/.scout/upload.json` | Done |

### Remaining Work

See [BACKLOG.md](BACKLOG.md) for future work.
