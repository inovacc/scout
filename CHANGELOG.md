# Changelog

All notable changes to Scout are documented here. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Security
- **Pairing now requires an out-of-band token (BREAKING).** `PairingServer.Pair` rejects the RPC with `codes.PermissionDenied` unless the client presents a matching `x-pairing-token` gRPC metadata value, compared in constant time (`crypto/subtle`). A server configured with an empty token rejects *all* pairing (fails closed). Closes the trust-on-first-use auto-enroll where any host able to reach the pairing port enrolled itself in the trust store and gained full remote browser control (including `eval`). `scout server` / `scout daemon` generates and prints a 160-bit base32 token when none is supplied via `--pairing-token` / `SCOUT_PAIRING_TOKEN`; `scout device pair` reads the same flag/env (or prompts) and sends it in metadata.
- **All remaining reachable CVEs cleared via dependency + toolchain bump.** `golang.org/x/net` v0.54.0 → v0.55.0 (GO-2026-5025 / 5027 / 5028 / 5029 / 5030), `golang.org/x/crypto` v0.51.0 → v0.52.0, `golang.org/x/sys` v0.44.0 → v0.45.0 (transitive), and `toolchain go1.26.4` (stdlib `net/textproto` GO-2026-5039 + `crypto/x509` GO-2026-5037). `govulncheck ./...` now reports **No vulnerabilities found** (was 7 reachable). Language version stays `go 1.26.0`.
- **Plugin subprocesses now receive a fail-closed environment allowlist.** `pkg/scout/plugin` previously scrubbed the child env with a substring *denylist* (`PASSPHRASE`/`TOKEN`/`SECRET`/…), which failed open — any secret whose name lacked a known fragment (`AWS_ACCESS_KEY_ID`, `ANTHROPIC_KEY`, `KUBECONFIG`, `SSH_AUTH_SOCK`, DSNs, …) leaked into untrusted plugins. The launcher now passes only a curated allowlist (process basics + `SCOUT_HOME` / `SCOUT_PLUGIN_PATH` / `SCOUT_CDP_ENDPOINT`); everything else is dropped. A new optional manifest `env` field lets a plugin explicitly opt in to extra variable names it legitimately needs.
- **gRPC session RPCs now enforce per-device ownership.** mTLS gated *who* could connect, but any trusted device could drive *another* device's session — `Eval`/`InjectJS` (arbitrary JS in a victim's authenticated browser), `Screenshot`/`ExportHAR`/`StreamHijack` (traffic/cookie/page exfiltration), or `DestroySession` (DoS) — by supplying its UUID. The creator's device ID (already recorded in `sessionPeer` at `CreateSession`) is now enforced at the single `getSession` chokepoint all 31 session RPCs route through: a mismatch returns `PermissionDenied`, an unknown ID returns `NotFound`. Enforcement applies only when both the owner and caller carry a real mTLS identity, so the insecure/loopback local daemon is unaffected.

### Removed
- **`github.com/ollama/ollama` SDK dependency dropped** — clears 8 unfixable reachable CVEs (GO-2025-3557 / 3558 / 3559 / 3582 / 3689 / 3695 / 3824 / 4251, all "Fixed in: N/A"). `govulncheck` reachable count drops 16 → 7. Also removes transitive `gin`, `mattn/go-sqlite3`, and `charmbracelet/bubbletea` from the build graph. `internal/engine/llm/ollama.go` and its test deleted; `OllamaProvider` / `NewOllamaProvider` / `WithOllama*` facade re-exports removed.

### Changed
- The `ollama` LLM provider now routes through Ollama's OpenAI-compatible `/v1` endpoint via `NewOpenAIProvider` — no behavior change for chat/completion. `scout ollama list` / `pull` / `status` are reimplemented directly on Ollama's native REST API (`GET /api/tags`, `POST /api/pull`) — identical UX, no SDK dependency.

### Fixed
- **AI-agent REST tools `click` / `type` / `extract_text` no longer error.** They passed bare JS expressions to `page.Eval`, which expects function form (`() => …`) and `.apply`s the value — producing `TypeError: (...).apply is not a function`. The handlers now emit proper arrow functions. (`pkg/scout/agent/agent.go`.)

## [1.0.4] - 2026-05-20

### Security
- H1: `IsScoutProcess` matches binary basename exactly (`scout` / `scout.exe`) instead of `strings.Contains(_, "scout")`. Prior check allowed any binary with "scout" in the name to spoof orphan-cleanup.
- H2: Per-OS `ProcessStartToken` (Linux `/proc/<pid>/stat` field 22, Windows `GetProcessTimes` CreationTime) recorded in `SessionInfo.BrowserStartToken`. Verified before kill in `CleanOrphans`, `Reset`, `CleanStaleSessions` to defeat PID-reuse races.
- H3: `scout.pid` and `job.json` use atomic write (CreateTemp + fsync + Rename). Crash mid-write no longer destroys reusable session metadata.
- H4: Session directories are rejected when not owned by the current user (Unix `st_uid`, Windows owner SID). Defeats local-coresident pre-planting attacks at the predictable hash path.
- M1: Session dirs `0o700`, `scout.pid` / `job.json` `0o600`. Chrome's cookie SQLite under `data/` is no longer world-readable.
- M2: `os.TempDir()` fallback removed. Fails closed instead of leaking Chrome profiles + OAuth tokens to `/tmp`.
- M3: Cross-process mkdir-based mutex on `<session>/.lock` with stale-holder detection. Prevents double-claim of reusable sessions.
- M4: `golang.org/x/net/publicsuffix` replaces the hand-curated two-part-TLD map in `RootDomain`. Closes silent cross-domain session collisions for entries like `gov.uk`, `ac.uk`.

### Added
- `internal/engine/scouthome` — centralized resolver for the per-user state root. Honors `SCOUT_HOME` env var, then platform-conventional default.
- `SCOUT_HOME` environment variable, applied consistently across all persisted directories (sessions, browsers, plugins, fingerprints, extensions, reports, upload OAuth tokens, electron cache).
- `docs/quality/SESSION_HARDENING.md` — full audit doc with severity tiers and remediation status.
- 24 new tests across 5 files: atomic-write, exact-match scout binary detection (16 cases), `ProcessStartToken` stability + PID rejection, `verifyProcess` (empty-token degrade, mismatched-token reject, matching-token accept), ownership rejection, lock acquire/release/stale-reclaim, scouthome resolution paths.

### Changed
- **State root path migrated**: Windows `%LOCALAPPDATA%\Scout`, macOS `~/Library/Application Support/Scout`, Linux `$XDG_DATA_HOME/scout` (or `~/.local/share/scout`). Back-compat: legacy `~/.scout` with content takes precedence when the new path is absent.
- Daemon-created sessions (`scout session create`) are reusable by default; `Reusable: true` persists in `scout.pid`. Previously sessions were written as ephemeral and removed on `Browser.Close`.
- Reusable sessions are NEVER auto-cleaned by `CleanStaleSessions` or `CleanOrphans`, regardless of process liveness (H6 contract). Only manual `scout session reset/destroy` removes them.
- `RemoveAll` retry budget switched to exponential backoff: 50 ms × 2^N (was flat 500 ms × 5).
- `Reset()` skips the post-kill 500 ms sleep when the recorded browser PID is already dead.
- `ResetAll()` logs per-session failures via `slog.Warn`.
- Orphan watchdog goroutine wraps `CleanOrphans` in deferred `recover()` and logs on panic.
- `List()` emits `slog.Warn` when `ReadInfo` fails for reasons other than `os.ErrNotExist`.
- `scout browser list` header shows the actual resolved cache path instead of the hardcoded `~/.scout/browsers/`.

### Fixed
- Windows `ProcessAlive` always returned false — `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` lacks `SYNCHRONIZE` access required by `WaitForSingleObject`; every call returned `WAIT_FAILED` ("Access is denied"). `CleanOrphans`, `Reset`, and `CleanStaleSessions` silently skipped their browser-kill step on Windows. Pre-existing `TestProcessAlive` and `TestCleanStaleSessions` failures on `main` now pass.

### Tests
- 78 passing in `internal/engine/session/`, 0 failing
- Plugin registry tests updated to set `SCOUT_HOME` alongside `HOME`/`USERPROFILE`

## [1.0.3] - 2026-04 (stabilization milestone)

### Added
- Session lifecycle and isolation test scaffolds
- `scout extract` parent command grouping `table`, `meta`, `extract-ai`
- `scout grpc` parent command grouping 17 daemon subcommands
- `scout grpc screenshot` + standalone `scout screenshot`
- `removeRetries` / `removeRetryWait` constants for Windows file-lock retry
- `tls.Dialer` real implementation (was TODO)
- Phase artifacts archived to `.planning-archive/`

### Changed
- `engine.New()` now generates UUID v7 session IDs (replaces `SessionHash`)
- `Browser.Close()` consolidated to single cleanup path
- `CleanOrphans` does full session-dir removal; `DeviceIDFromCert` callers updated
- Browser isolation: `--system-browser` opt-in required for system-installed browsers (no silent fallback)
- MCP help text tool count corrected: 33 → 18

### Removed
- Rod fallback for launching browsers — explicit error when no cached browser available
- `cmd/scout/credentials.go` (merged into `auth`)
- `scout websearch` (merged into `search`); `search_engines` subcommands
- Standalone `scout markdown` (grouped under `extract`)

### Workflow
- Migrated from GSD to Superpowers (brainstorm → spec → execute → verify)

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
