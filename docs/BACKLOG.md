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
| ~~browser.json revision manifest~~ | ~~P1~~ | ~~Medium~~ | ~~Done (Phase 43/50) — `browser.json` embedded manifest with per-platform revisions, zip names, download hosts; `LAST_CHANGE` fallback~~ |
| ~~Chrome protocol version tracking~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.28.0) — .scripts/rod-upstream-diff.sh with --check/--full modes~~ |
| ~~Headless=new migration~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0)~~ |

### Features

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ~~Proxy chain support~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.28.0) — WithProxyChain, ValidateProxyChain, ProxyChainDescription~~ |
| ~~HAR export~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.27.0) — HijackRecorder with ExportHAR()~~ |
| ~~Cookie jar persistence~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0) — SaveCookiesToFile/LoadCookiesFromFile~~ |
| ~~Multi-tab orchestration~~ | ~~P3~~ | ~~Large~~ | ~~Done — TabGroup with Do/DoAll/DoParallel/Broadcast/Navigate/Wait/Collect~~ |
| ~~Auto-upload results to GDrive/OneDrive~~ | ~~P2~~ | ~~Medium~~ | ~~Done — `scout upload auth/file/status` with OAuth2 for Google Drive and OneDrive~~ |
| ~~Session reuse & clean reset~~ | ~~P1~~ | ~~Medium~~ | ~~Done — `WithReusableSession()`, `WithTargetURL()`, domain-hash routing, `scout session reset [id\|--all]`~~ |
| ~~Orphan process detection (PID reuse)~~ | ~~P1~~ | ~~Medium~~ | ~~Done (v0.49.0) — gops agent + `IsScoutProcess()` replaces `ProcessAlive` for scout PIDs; `Page.WaitClose()` detects browser window close via CDP; synchronous session dir cleanup~~ |
| ~~REPL mode~~ | ~~P2~~ | ~~Medium~~ | ~~Done — `scout repl [url]` interactive local browser shell with 20 commands~~ |
| ~~Site health checker / test page~~ | ~~P1~~ | ~~Large~~ | ~~Done — `scout test-site <url>` with crawl, console/JS/network error detection, JSON/table report~~ |
| ~~Page gather~~ | ~~P1~~ | ~~Medium~~ | ~~Done (Phase 47) — `scout gather <url>` one-shot page intelligence~~ |
| ~~PDF form filling~~ | ~~P3~~ | ~~Medium~~ | ~~Done (Phase 51) — `PDFFormFields()`, `FillPDFForm()`, CLI `scout pdf-form fields/fill`~~ |
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
