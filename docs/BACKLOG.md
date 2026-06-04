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
| P1 | **DEPRECATION**: remove `scout agent serve` + `pkg/scout/agent/` (REST AI ingress) | Marker added 2026-05-24; removal **2026-07-23** (60-day window per CLAUDE.md). Superseded by MCP server. Per plugin-first OKR T2: two AI ingresses = forever drift. Migration: `scout plugin install --host all`. |
| P3 | iOS Safari via ios-webkit-debug-proxy | Extend Phase 73 mobile to iOS |
| P3 | Claude Code marketplace submission | Submit plugin to official Anthropic marketplace |
| P2 | **DEPRECATION** (removal after 2026-07-02): remove `UserProfile.Cookies/Storage/Headers` (`internal/engine/profile.go`). Superseded by `pkg/scout/vault`. Migrate callers of `CaptureProfile`/`ApplyProfile` to read browser-bound secrets from the vault, then drop the fields and their capture/apply branches. | Fields marked deprecated 2026-06-02. |
| P3 | **ERR-01**: finish the `scout:` error-prefix migration across the remaining ~750 non-`scout:` `fmt.Errorf` call sites. STRUCT-03 (2026-06-03) covered the highest-impact paths (`cmd/scout/update.go`, `pkg/scout/browser/`, `pkg/scout/runbook/`, `pkg/scout/identity/identity.go`). The remainder already carry a subsystem prefix (e.g. `pkg/scout/scraper/modes/*` ×292, plus `plugin`/`archive`/`strategy`/`tools`, and `pkg/scout/identity/trust.go`) and are deferred to v2 for the outer `scout:` wrap. | Mechanical prefix-only change; no behavior impact. Convention per CLAUDE.md error-wrapping. |

### SSRF-V2 — deeper SSRF coverage (v1 shipped 2026-06-03)

v1 (`pkg/scout/urlpolicy`) validates only the top-level target URL pre-navigation
at MCP + agent ingress. Out of scope, tracked here:
- Redirect-to-internal: re-validate the landed URL or disable redirect-following for guarded navigations.
- In-page subresources (fetch/XHR/img to internal IPs): enforce via CDP request interception (reuse the hijack/block infra).
- Crawler-discovered links: apply the policy to each URL swarm_crawl/gather fetches, not just the seed.
- DNS rebinding: pin the resolved IP through to navigation, or intercept at CDP.

### STEALTH-V2 — behavioral evasion + network fingerprint (static fingerprints shipped)

Live probe 2026-06-04 (`docs/quality/STEALTH-MATURITY-2026-06-04.md`) confirmed Scout's stealth
defeats the **static-fingerprint** detectors that block most scrapers — scrapfly's automation
detector returned "Not Automated, 0 detected signals", and `navigator.webdriver` / plugins /
WebGL vendor (Intel, not SwiftShader) / UA / languages all read human-like. Two frontier gaps
remain, both **P3** (only relevant against the most advanced anti-bot stacks):
- **Humanize input layer** — synthesize human-like mouse paths (bezier / variable speed), typing
  cadence, and scroll jitter so *behavioral* classifiers (bot.incolumitas `behavioralClassificationScore`,
  reCAPTCHA v3, DataDome) don't flag no-interaction runs. Scout masks *what the browser is*, not
  *how a human drives it*. Suggest an optional `WithHumanize()` wired through `Element.Click`/`Input`
  and page scroll.
- **UA + network-fingerprint currency** — keep the spoofed UA's Chrome major version current (the
  probe showed `Chrome/114`, dated) and document/close the **TLS JA3/JA4 + HTTP/2** vector: the real
  Chrome-for-Testing build + host OS still show through at the network layer, which JS-only evasion
  doesn't address. Also consider `WithoutBridge()` as the stealth-mode default (content-script
  injection is itself a signal).

### HARDENING-V2 — residual findings from the 2026-06-04 adversarial audit

The audit's confirmed CRITICAL (self-update had no integrity verification → RCE) and the
daemon-env secret leak were FIXED 2026-06-04. The following finder-only (un-verified) items
remain; fix opportunistically:
- **P2 — flow secret leak** (`pkg/scout/flow/analyze.go`): `Referer`/`Origin` header values bypass
  LLM-digest redaction (ship OAuth tokens to the remote LLM); `sanitizeSpec` uses a header-name
  *denylist*, so a secret in a non-standard-named header survives into the shareable `flow.yaml`.
  Make both **default-deny** (parameterize every non-structural header) + add hygiene tests.
- **P3 — unbounded `io.ReadAll`/decoder on network bodies** (DoS): `internal/engine/extension.go`
  (CRX), `internal/engine/browser/download.go` (browser archive), `internal/engine/browser/download_chromium.go:297`
  (metadata json), `internal/engine/crawl.go` `ParseSitemap` (also unbounded recursion + should apply
  the SSRF urlpolicy to followed `loc` URLs). Wrap with `io.LimitReader` (mirror `cmd/scout/plugin.go:214`).
- **P3 — archive bombs / cpio panic** (`pkg/scout/archive/`): zip/tar/rpm extraction has no per-entry,
  total-size, or entry-count cap (zip bomb); `rpm.go:128` cpio newc parser slices buffers using
  unvalidated attacker hex fields (panic). Add limits + bounds checks.
- **P3 — PowerShell path interpolation** (`internal/engine/browser/detect_version_windows.go`,
  `pkg/scout/browser/detect_windows.go`): browser path interpolated unescaped into a single-quoted
  `-Command` string. Apply the existing `escapePowerShell` helper or pass the path as a separate argv.
- **P3 — SSE event injection** (`pkg/scout/agent/server.go:269`): unescaped `req.Name` in the `start`
  event. Low priority — `pkg/scout/agent` is already scheduled for removal (see P1 deprecation above).
- **P3 — `isNewer` permits downgrade** (`cmd/scout/update.go`): tag-inequality only; gate downgrades
  behind `--allow-downgrade` (semver compare).

## Completed Items (Archive)

<details>
<summary>Testing & Quality (all done)</summary>

| Item | Completed |
|------|-----------|
| Integration test suite for scraper modes | v0.28.0 — mock Mode/Session/AuthProvider, registry, progress, cancellation tests |
| Test coverage for gRPC streaming RPCs | v0.28.0 — StreamHijack, double-start/stop, invalid session tests |
| Benchmark suite for core operations | BenchmarkPageCreation, BenchmarkExtract, BenchmarkPagination, BenchmarkSnapshot |
| Fuzz testing for recipe parser | v0.28.0 — FuzzParse + FuzzResolveSelector with 12 seed corpus entries |
| **PHASE6-REMAINDER**: extract `swarm_crawl` + `ws_listen`/`ws_send`/`ws_connections` MCP handlers into `tools.SwarmCrawl`/`tools.WSListen`/`tools.WSSend`/`tools.WSConnections` verbs | 2026-06-03 — all four routed through `pkg/scout/tools/`; no inline browser logic remains in `tools_swarm.go`/`tools_websocket.go`. Closes the "no inline rod in MCP" criterion. |
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
| Output sink plugin capability | Phase 59e — `SinkProxy` via JSON-RPC `sink/init/write/flush/close`, SDK `RegisterSink()` |
| Browser middleware plugin capability | Phase 59a — `MiddlewareProxy`, `MiddlewareChain`, 4 hook points, SDK `RegisterMiddleware()` |
| Event hook plugin capability | Phase 59d — `EventProxy`, `EventDispatcher`, token bucket rate limiter, SDK `OnEvent()` |
| TikTok scraper mode | Phase 60 — auth provider, video extraction, @user/#hashtag target resolution, 9 tests |
| API middleware proxy | Phase 62 — `pkg/scout/proxy/` YAML routes, browser extraction, caching, CLI `scout proxy start` |
| Content plugin migration | Phase 65 Wave 2 — `scout-content` (markdown, table, meta, pdf) plugin |
| Search plugin migration | Phase 66 Wave 3 — `scout-search` (search, search_and_extract, fetch) plugin |
| Network/Forms plugin migration | Phase 67 Wave 4 — `scout-network` + `scout-forms` plugins |
| Analysis/Guides plugin migration | Phase 68 Wave 5 — `scout-crawl` + `scout-guide` plugins |
| Communication modes plugin | Phase 59f Batch 1 — `scout-comm` (slack, discord, teams, reddit) plugin |
| Email/Docs modes plugin | Phase 59f Batch 2 — `scout-email-docs` (gmail, outlook, linkedin, jira, confluence) plugin |
| Content/Social modes plugin | Phase 59f Batch 3 — `scout-content-social` (twitter, youtube, notion, gdrive, sharepoint) plugin |
| Enterprise modes plugin | Phase 59f Batch 4 — `scout-enterprise` (amazon, gmaps, salesforce, grafana, cloud) plugin |
| Plugin marketplace & registry | Phase 69 — `pkg/scout/plugin/registry/`, `scout plugin search/update`, lock file |
| WebSocket automation | Phase 70 — `Page.MonitorWebSockets()`, CLI `scout ws listen`, MCP ws_listen/ws_send/ws_connections |
| AI agent integration | Phase 71 — `pkg/scout/agent/` Provider with OpenAI/Anthropic tool schemas |
| Visual monitoring | Phase 72 — `pkg/scout/monitor/` baseline management, pixel diff, continuous monitoring |
| MCP deprecation cleanup | Phase 72.5 — removed 28 deprecated built-in MCP tools, core reduced to 18 tools |
| Mobile browser automation | Phase 73 — ADB, touch gestures (Touch/Swipe/PinchZoom), `scout mobile devices/connect` |
| WebSocket HAR recording | Phase 73.5 — `_webSocketMessages` extension, `ExportWebSocketHAR()`, WS event recording |
| Agent HTTP server | Phase 73.6 — `scout agent serve` REST API with 6 endpoints for LangChain/CrewAI |
| Claude Code plugin | Phase 73.7 — plugin manifest, 6 skills, 3 agents, MCP config, SessionStart hook |
| Plugin distribution | GoReleaser + GitHub Actions release workflow, auto-download in SessionStart hook |
| Cloud deployment | Phase 74 — Helm chart (HPA, PVC), `scout cloud` CLI, Prometheus metrics |
| Encoded session ID + binary scout.pid | Phase 77.1 (2026-05-21) — 12-char attr prefix + 24 `[A-Z]`, 432-byte `SCT1` binary record, `scout.lock` sibling |
| Persistent-session expiration | Phase 77.2 — `WithExpiration` required for reusable sessions; `ExpiresAt` stamped in reuse branch |
| Session audit tool | Phase 77.2 — `scout session audit` classifies live/orphaned/corrupt/expired/zombie and kills zombies |
| monitors.json sidecar | Phase 77.3 — per-session HAR/hijack/console/WS/blocks config; `scout session create --har --hijack --block` |
| Request blocking option | Phase 77.3 — `WithBlockRules(BlockRule{Pattern, Method})` via CDP URLPattern; aborts with `BlockedByClient` |
| AV-resilient cleanup retrier | Phase 77.4 — single-shot `RemoveAll` fast path; `StartCleanupRetrier` (60 s, process lifetime) for AV/OneDrive locks |
| MCP-host LLM provider | Phase 77.5 — `MCPSamplingProvider` routes completions via MCP host (no direct provider creds) |
| Cobra errors to stderr | Phase 77.5 — fixes silently dropped error output |
| Browser-test gating under `-short` | Phase 77.6 — `task test:unit` runs without Chromium; `task test:full` for the multi-minute browser suite |
| Toolchain bump | Phase 77.6 — Go 1.26.0, otel 1.43.0, grpc 1.81.1, ollama 0.24.0, x/crypto 0.51.0, x/net 0.54.0, x/oauth2 0.36.0 |
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

## Session Hardening — deferred (phase B)

| Tag | Item | Rationale |
|-----|------|-----------|
| FOLLOW-UP | **Windows Job Object** (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`) — wrap every launched browser in a Windows Job Object so Chrome child processes are killed by the OS when the scout process exits via SIGKILL or console-close, eliminating the entire class of Windows orphan-browser leaks. | `p.Kill()` + retries are a best-effort workaround; a Job Object is the only true OS-level guarantee on Windows. |
| FOLLOW-UP | **Windows `CTRL_CLOSE_EVENT` / `CTRL_SHUTDOWN_EVENT` handler** — register a `SetConsoleCtrlHandler` callback so console-window close and system shutdown trigger the same graceful teardown as SIGINT/SIGTERM. | The current signal handler only catches SIGINT/SIGTERM; console-close bypasses it and leaves browsers alive. |
| FOLLOW-UP | **macOS/BSD `ProcessStartToken` / parent-PID cross-check** — add a `sysctl KERN_PROC` parent-PID comparison (or `libproc` start-time) to `verifyProcess` on Darwin so PID reuse is detected even when the process happens to be a Go binary (gops false-positive). | Current macOS path falls back to alive-only after start-token fails, which degrades PID-reuse protection to near-zero on a busy system. |
| FOLLOW-UP | **macOS/BSD `FindBrowsersUsingDataDir` returns nil** — implement `sysctl KERN_PROC_ALL` cmdline read to scan running processes for `--user-data-dir=<path>` on Darwin so `ReapSession` can kill a zombie whose `scout.pid` is corrupt or missing. | The current macOS implementation returns `nil, nil`, making a corrupt-pid zombie un-killable via the path-based fallback. |
| FOLLOW-UP | **Linux cmdline scan hardening** — fix `isBrowserCmdline` to handle (a) unquoted `--user-data-dir` values containing spaces (currently truncated at the first space by `/proc/<pid>/cmdline` NUL splitting) and (b) missing `/msedge` path suffix so Edge zombies are recognised. | Both cases cause `FindBrowsersUsingDataDir` to miss live orphan browsers, leaving them unkilled after a corrupt-pid reap. |
| FLOW v2 | WebSocket replay, multipart/file-upload, SSE/streaming bodies, query/json chain-injection (the analyzer currently emits header chains only), request-body reconstruction in the analyzer, a HAR reader for `--golden file.har`, a fully-automatic no-review analyze mode, and `scout flow export --go` (standalone Go module emitter). | `pkg/scout/flow` v1 covers REST/JSON + GraphQL with header chains, vault-sourced auth, and golden-`capture.json` verification; these extend protocol/coverage breadth. |
