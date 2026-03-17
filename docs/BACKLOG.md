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
| P1 | 59e: Output sink plugin capability | Custom result destinations (S3, DB, webhook) via `sink/*` RPC |
| P2 | 59a: Browser middleware plugin capability | Hook points in Navigate/WaitLoad/Extract with priority chain |
| P2 | 59d: Event hook plugin capability | Bridge + hijack event forwarding to plugins with rate limiting |
| P2 | 59f: Scraper modes → plugins (4 batches, 19 modes) | Depends on 59b+59d — modes use auth and events |
| P2 | Plugin distribution via GitHub Releases | Ship pre-built plugin binaries per OS/arch |
| P1 | Phase 60: TikTok scraper mode | Video metadata, comments, profiles, trending; API interception |
| P2 | Phase 62: API middleware proxy | HTTP reverse proxy turning legacy sites into REST/JSON endpoints |
| P1 | Phase 65: Wave 2 — Content plugin migration | DEPRECATION: remove tools_content.go, pdf from tools_capture.go after +30 days |
| P2 | Phase 66: Wave 3 — Search plugin migration | DEPRECATION: remove tools_search.go after +30 days |
| P2 | Phase 67: Wave 4 — Network/Forms plugin migration | DEPRECATION: remove tools_network.go, tools_form.go, tools_inspect.go after +30 days |
| P2 | Phase 68: Wave 5 — Analysis/Guides plugin migration | DEPRECATION: remove tools_analysis.go, tools_swarm.go, tools_guide.go after +30 days |

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
| Swarm mode | Phase 58 — distributed crawling with coordinator, workers, gRPC transport, proxy support |
| Report system | Phase 58 — AI-consumable reports (`~/.scout/reports/`), 3 types, MCP tools, scheduling |
| ManagedPagePool | Phase 58 — concurrent page pool with acquire/release lifecycle |
| Recipe removal | Phase 58 — deprecated `pkg/scout/recipe/` deleted, consumers migrated to `runbook` |
| Default browser BestCached | Phase 58 — fixes "Failed to get debug url" by preferring cached browsers |
| Strategy files | Phase 61 — `pkg/scout/strategy/` YAML/JSON workflows with env expansion, validation, executor, 3 sinks, CLI |
| CLI command plugin capability | Phase 63 — `CommandProxy`, `command/execute` RPC, `BrowserContext` CDP forwarding |
| Auth provider plugin capability | Phase 59b — `AuthProxy` via JSON-RPC, SDK `RegisterAuth()`, `auth/detect/capture/validate` |
| MCP resources & prompts plugin capability | Phase 59c — `ResourceProxy`, `PromptProxy`, SDK `RegisterResource/RegisterPrompt` |
| Diagnostics plugin migration | Phase 64 Wave 1 — `scout-diag` (ping, curl) + `scout-reports` (report_list/show/delete) plugins |
| Browser isolation | Default `browser list` shows only cached; system scan behind `--detect` flag |
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
