# Backlog

## Priority Levels

| Priority | Timeline |
|----------|----------|
| P1 | Next release |
| P2 | This quarter |
| P3 | Future |

## Open Items

| Priority | Item | Notes |
|----------|------|-------|
| P1 | ~~Web Search MCP tool improvements~~ | Done — `search_and_extract` already uses goroutines + WaitGroup for parallel fetch |
| P2 | ~~Step-by-Step Guide Generator~~ | Done — `pkg/scout/guide/` with Recorder, Step, Guide, RenderMarkdown |
| P2 | ~~Deprecate `pkg/scout/recipe/` package~~ | Done — removed 2026-03-16, consumers migrated to `runbook` directly |

## Completed Items (Archive)

<details>
<summary>Testing & Quality (all done)</summary>

| Item | Completed |
|------|-----------|
| Integration test suite for scraper modes | v0.28.0 — mock Mode/Session/AuthProvider, registry, progress, cancellation tests |
| Test coverage for gRPC streaming RPCs | v0.28.0 — StreamHijack, double-start/stop, invalid session tests |
| Benchmark suite for core operations | BenchmarkPageCreation, BenchmarkExtract, BenchmarkPagination, BenchmarkSnapshot |
| Fuzz testing for recipe parser | v0.28.0 — FuzzParse + FuzzResolveSelector with 12 seed corpus entries |
</details>

<details>
<summary>Platform & Compatibility (all done)</summary>

| Item | Completed |
|------|-----------|
| browser.json revision manifest | Phase 43/50 — embedded manifest with per-platform revisions, zip names, download hosts; LAST_CHANGE fallback |
| Chrome protocol version tracking | v0.28.0 — .scripts/rod-upstream-diff.sh with --check/--full modes |
| Headless=new migration | v0.27.0 |
</details>

<details>
<summary>Features (all done)</summary>

| Item | Completed |
|------|-----------|
| Proxy chain support | v0.28.0 — WithProxyChain, ValidateProxyChain, ProxyChainDescription |
| HAR export | v0.27.0 — HijackRecorder with ExportHAR() |
| Cookie jar persistence | v0.27.0 — SaveCookiesToFile/LoadCookiesFromFile |
| Multi-tab orchestration | TabGroup with Do/DoAll/DoParallel/Broadcast/Navigate/Wait/Collect |
| Auto-upload results to GDrive/OneDrive | scout upload auth/file/status with OAuth2 |
| Session reuse & clean reset | WithReusableSession(), WithTargetURL(), domain-hash routing, scout session reset |
| Orphan process detection (PID reuse) | v0.49.0 — gops agent + IsScoutProcess() + Page.WaitClose() |
| REPL mode | scout repl with 20 commands |
| Site health checker / test page | scout test-site with crawl, error detection, JSON/table report |
| Page gather | Phase 47 — scout gather one-shot page intelligence |
| PDF form filling | Phase 51 — PDFFormFields(), FillPDFForm(), CLI scout pdf-form |
| Visual regression testing | v0.28.0 — VisualDiff with threshold, color tolerance, diff image overlay |
</details>

<details>
<summary>Infrastructure (done items)</summary>

| Item | Completed |
|------|-----------|
| MCP server SSE transport | ServeSSE() with --sse/--addr CLI flags |
| gRPC reflection + health service | v0.27.0 — health.NewServer() registered |
| CLI shell completions | v0.27.0 — scout completion bash/zsh/fish/powershell |
| Rebrand rod references to scout | -rod to -scout, cache dir rod/ to scout/, error links updated |
| Plugin system | Phase 53 — subprocess JSON-RPC 2.0, manager, proxies, Go SDK, CLI |
| OpenTelemetry tracing | Phase 54 — internal/tracing/, MCPToolSpan, ScraperSpan, addTracedTool wrapper |
| Guide generator | Phase 56 — `pkg/scout/guide/` Recorder for step-by-step how-to docs |
| Session startup cleanup | Phase 57 — `CleanStaleSessions()` removes dead/orphaned sessions on start |
| Session dir restructure | Phase 57 — `<hash>/{scout.pid, job.json, data/}` separates metadata from browser profile |
| Job tracking | Phase 55 — `job.json` session job metadata (type, status, progress, steps) |
</details>

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
