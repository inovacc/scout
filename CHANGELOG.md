# Changelog

All notable changes to Scout are documented here. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.2] - 2026-03-28

### Added
- Agent server: Bearer token auth via `--api-key` flag and `SCOUT_AGENT_API_KEY` env var
- `docs/openapi.yaml`: OpenAPI 3.1.0 spec for all agent HTTP endpoints
- `deploy/helm/scout/values.schema.json`: JSON Schema for Helm chart values validation
- `scout plugin check-updates`: check installed plugins against registry for available updates
- Plugin update throttling: `ShouldCheck()`/`MarkChecked()` for daily auto-check
- Benchmark suite: 11 benchmarks for HAR recorder, agent provider, and metrics
- `examples/README.md`: gallery of 18 examples + 8 cookbook recipes

### Changed
- npm package bumped to v1.0.1
- MILESTONES.md updated with v1.0.1 entry

## [1.0.1] - 2026-03-28

### Security
- Upgrade `go-sdk` v1.3.1 → v1.4.1 (fixes cross-site tool execution, JSON null CVEs)
- Upgrade `ollama` v0.16.2 → v0.18.3 (fixes 15 CVEs: resource exhaustion, GZIP DoS)
- Agent server: request body limit (1 MB), read/write/idle timeouts
- npm `install.js`: SHA256 checksum verification, redirect depth limit

### Added
- CORS middleware with origin echo and OPTIONS preflight
- Token bucket rate limiter (100 rps default, `--rate-limit` flag)
- `WebSocketOpsTotal` metric counter for ws_listen/ws_send/ws_connections
- Grafana dashboard template: `deploy/grafana/scout-dashboard.json` (15 panels)
- `docs/API.md`: reference for 18 MCP tools, 7 HTTP endpoints, 9 agent tools
- E2E test suite: 10 browser scenarios
- Plugin validation CI workflow
- README.md refresh with v1.0.0 features

### Changed
- Agent server coverage: 41.7% → 91.4%
- Plugin system coverage: 36.2% → 84.4%
- MCP metrics: snapshot→ExtractionsTotal, pdf→ScreenshotsTotal, swarm→NavigationsTotal
- Fix Build Plugins CI workflow (release/ path)

## [1.0.0] - 2026-03-28

### Added
- **Claude Code Plugin**: manifest, `.mcp.json`, 6 skills, 3 agents, SessionStart hook
- **Mobile browser automation**: ADB integration, `WithMobile()`, touch gestures (`Touch`, `Swipe`, `PinchZoom`)
- **WebSocket HAR recording**: `_webSocketMessages` extension, `ExportWebSocketHAR()`
- **Agent HTTP server**: `scout agent serve` with 6 endpoints for AI frameworks
- **Cloud deployment**: Helm chart with HPA/PVC, `scout cloud deploy/status/scale/uninstall`
- **Prometheus metrics**: `internal/metrics/` with JSON and Prometheus handlers
- **GoReleaser**: cross-platform binaries (linux/darwin/windows × amd64/arm64)
- **npm package**: `@inovacc/scout-browser` with auto-download binary
- `process_unix.go` (`//go:build !windows`) for macOS cross-compilation

### Changed
- Public facade regenerated with mobile types
- Lint fixes: errcheck, forbidigo, modernize (SplitSeq, CutPrefix)
