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
| Integration test suite for scraper modes | P2 | Large | httptest-based tests for each scraper mode with mock HTML fixtures |
| Test coverage for gRPC streaming RPCs | P2 | Medium | StreamHijack, ScreenRecord, and other streaming RPCs |
| Benchmark suite for core operations | P3 | Medium | Page creation, extraction, pagination, snapshot throughput |
| Fuzz testing for recipe parser | P3 | Medium | Go native fuzzing for YAML recipe parsing edge cases |

### Platform & Compatibility

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| ARM64 Linux browser download | P2 | Medium | Playwright host fallback for arm64; validate in CI |
| Chrome protocol version tracking | P2 | Medium | Script to diff upstream rod CDP bindings and flag breaking changes |
| Headless=new migration | P2 | Quick | Default to `--headless=new` (Chrome 112+), deprecate old headless |

### Features

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| Proxy chain support | P2 | Medium | Route through multiple proxies (SOCKS5 → HTTP) for layered anonymity |
| HAR export | P2 | Medium | Export session hijack captures to HAR 1.2 format |
| Cookie jar persistence | P2 | Quick | Save/load cookies to file for session resumption across runs |
| Multi-tab orchestration | P3 | Large | Coordinate actions across tabs with shared state and sync primitives |
| PDF form filling | P3 | Medium | Fill interactive PDF forms via browser rendering |
| Visual regression testing | P3 | Large | Screenshot comparison with diff threshold for page change detection |

### Infrastructure

| Item | Priority | Effort | Scope |
|------|----------|--------|-------|
| MCP server SSE transport | P2 | Medium | Add HTTP+SSE transport alongside stdio for remote MCP access |
| gRPC reflection + health service | P2 | Quick | Enable gRPC server reflection and standard health checks |
| CLI shell completions | P2 | Quick | Generate bash/zsh/fish completions for scout CLI |
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
