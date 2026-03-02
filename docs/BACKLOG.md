# Backlog

## Priority Levels

| Priority | Timeline |
|----------|----------|
| P1 | Next release |
| P2 | This quarter |
| P3 | Future |

## Open Items

### Testing & Quality

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ~~Integration test suite for scraper modes~~ | ~~P2~~ | ~~Large~~ | ~~Done (v0.28.0) — mock Mode/Session/AuthProvider, registry, progress, cancellation tests~~ |
| ~~Test coverage for gRPC streaming RPCs~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.28.0) — StreamHijack, double-start/stop, invalid session tests~~ |
| ~~Benchmark suite for core operations~~ | ~~P3~~ | ~~Medium~~ | ~~Done — BenchmarkPageCreation, BenchmarkExtract, BenchmarkPagination, BenchmarkSnapshot~~ |
| ~~Fuzz testing for recipe parser~~ | ~~P3~~ | ~~Medium~~ | ~~Done (v0.28.0) — FuzzParse + FuzzResolveSelector with 12 seed corpus entries~~ |

### Platform & Compatibility

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ARM64 Linux browser download | P2 | Medium | Playwright host fallback for arm64; validate in CI |
| browser.json revision manifest | P1 | Medium | Replace hardcoded `RevisionDefault` with a `browser.json` config file in launcher; supports per-platform revisions, zip names, and download hosts; `LAST_CHANGE` fallback already added (ROADMAP Phase 43) |
| ~~Chrome protocol version tracking~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.28.0) — .scripts/rod-upstream-diff.sh with --check/--full modes~~ |
| ~~Headless=new migration~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0)~~ |

### Features

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ~~Proxy chain support~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.28.0) — WithProxyChain, ValidateProxyChain, ProxyChainDescription~~ |
| ~~HAR export~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.27.0) — HijackRecorder with ExportHAR()~~ |
| ~~Cookie jar persistence~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0) — SaveCookiesToFile/LoadCookiesFromFile~~ |
| ~~Multi-tab orchestration~~ | ~~P3~~ | ~~Large~~ | ~~Done — TabGroup with Do/DoAll/DoParallel/Broadcast/Navigate/Wait/Collect~~ |
| Auto-upload results to GDrive/OneDrive | P2 | Medium | Export scraper/runbook results directly to Google Drive or OneDrive via API; configurable output sink |
| Session reuse & clean reset | P1 | Medium | `WithReusableSession()` persists browser state across runs (essential for React/HMR flows); `scout session reset` clears all session data cleanly |
| REPL mode | P2 | Medium | Interactive browser REPL (`scout repl`) with live page context, tab-completion, history; supports eval, navigate, extract, screenshot inline |
| Site health checker / test page | P1 | Large | `scout test-site <url>` — crawls a site following all links, clicks interactive elements, detects broken links (404s), console errors, JS exceptions, network failures; generates a clear report with severity levels; essential for React/Vue/Angular dev workflows with hot-reload session reuse |
| PDF form filling | P3 | Medium | Fill interactive PDF forms via browser rendering |
| ~~Visual regression testing~~ | ~~P3~~ | ~~Large~~ | ~~Done (v0.28.0) — VisualDiff with threshold, color tolerance, diff image overlay~~ |

### Infrastructure

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ~~MCP server SSE transport~~ | ~~P2~~ | ~~Medium~~ | ~~Done — ServeSSE() with --sse/--addr CLI flags~~ |
| ~~gRPC reflection + health service~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0) — health.NewServer() registered~~ |
| ~~CLI shell completions~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0) — scout completion bash/zsh/fish/powershell~~ |
| ~~Rebrand rod references to scout~~ | ~~P2~~ | ~~Medium~~ | ~~Done — `-rod` → `-scout`, `DISABLE_ROD_FLAG` → `DISABLE_SCOUT_FLAG`, `rod-*` flags → `scout-*`, cache dir `rod/` → `scout/`, error links → github.com/inovacc/scout~~ |
| OpenTelemetry tracing | P3 | Large | Instrument core operations with span context propagation |
| Plugin system | P3 | XL | Dynamic loading of scraper modes and extractors |

## Completed Items (Archive)

<details>
<summary>Scraper Modes — Authenticated Services (all done)</summary>

| Mode | Completed |
|------|-----------|
| Slack | Phase 35 |
| Teams | Phase 35 |
| Discord | Phase 35 |
| Reddit | Phase 35 |
| Gmail | Phase 36 |
| Outlook | Phase 36 |
| LinkedIn | Phase 36 |
| Jira | Phase 36 |
| Confluence | Phase 36 |
| Twitter/X | Phase 37 |
| YouTube | Phase 37 |
| Notion | Phase 37 |
| Google Drive | Phase 37 |
| SharePoint | Phase 37 |
| Salesforce | Phase 38 |
| Amazon Products | Phase 38 |
| Google Maps | Phase 38 |
| Cloud Consoles | Phase 38 |
| Grafana/Datadog | Phase 38 |
</details>
