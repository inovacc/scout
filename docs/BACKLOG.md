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
| Benchmark suite for core operations | P3 | Medium | Page creation, extraction, pagination, snapshot throughput |
| ~~Fuzz testing for recipe parser~~ | ~~P3~~ | ~~Medium~~ | ~~Done (v0.28.0) — FuzzParse + FuzzResolveSelector with 12 seed corpus entries~~ |

### Platform & Compatibility

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ARM64 Linux browser download | P2 | Medium | Playwright host fallback for arm64; validate in CI |
| ~~Chrome protocol version tracking~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.28.0) — .scripts/rod-upstream-diff.sh with --check/--full modes~~ |
| ~~Headless=new migration~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0)~~ |

### Features

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ~~Proxy chain support~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.28.0) — WithProxyChain, ValidateProxyChain, ProxyChainDescription~~ |
| ~~HAR export~~ | ~~P2~~ | ~~Medium~~ | ~~Done (v0.27.0) — HijackRecorder with ExportHAR()~~ |
| ~~Cookie jar persistence~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0) — SaveCookiesToFile/LoadCookiesFromFile~~ |
| Multi-tab orchestration | P3 | Large | Coordinate actions across tabs with shared state and sync primitives |
| PDF form filling | P3 | Medium | Fill interactive PDF forms via browser rendering |
| ~~Visual regression testing~~ | ~~P3~~ | ~~Large~~ | ~~Done (v0.28.0) — VisualDiff with threshold, color tolerance, diff image overlay~~ |

### Infrastructure

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ~~MCP server SSE transport~~ | ~~P2~~ | ~~Medium~~ | ~~Done — ServeSSE() with --sse/--addr CLI flags~~ |
| ~~gRPC reflection + health service~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0) — health.NewServer() registered~~ |
| ~~CLI shell completions~~ | ~~P2~~ | ~~Quick~~ | ~~Done (v0.27.0) — scout completion bash/zsh/fish/powershell~~ |
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
