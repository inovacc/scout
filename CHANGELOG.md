# Changelog

All notable changes to Scout are documented here. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed
- **CI: Docker + release publishing repaired (Go 1.26).** `docker/Dockerfile` (+`.slim`) and the standalone release workflow built with Go 1.25 while `go.mod` requires 1.26, so the v1.1.1 *Docker Publish* job and one of two duplicate *Release* workflows failed with `go.mod requires go >= 1.26.0`. Bumped the Docker builder image and release `go-version` to 1.26 and removed the failing duplicate `release.yaml` (the reusable lib-mode release; the self-contained goreleaser `release.yml` is retained). The `Test` quality-check workflow now also runs on `main` (was `branches-ignore: [main]`, so `main` was never gated). Because `main` had never been CI-gated, the first run exposed two further issues: the org reusable workflow's `golangci-lint` (built with Go 1.25) cannot load a Go-1.26 module under `GOTOOLCHAIN=local`; and `go test ./...` is not CI-safe (many browser tests `Fatal` rather than `t.Skip` without Chromium). The workflow is now self-contained with `GOTOOLCHAIN=auto` and gates **build** (blocking) and runs `go test -short` + a coverage report plus lint + vulncheck (all advisory). Making `-short` reliably green so the test gate can block — a few un-gated browser E2E tests (`TestE2ETouchGestures`, `TestSessionHijackerWithAutoAttach`; `TestWithRemoteCDP` gated here) plus a shared-browser-fixture isolation bug — is tracked as the Tests/Coverage GA item.

## [1.1.1] - 2026-06-04

### Security
- **Self-update now verifies integrity — closes a supply-chain RCE.** `scout update` previously downloaded the release binary and swapped it over the running executable with **no** checksum or signature check. It now fetches the release `checksums.txt`, verifies the download's SHA256 **before any bytes touch the executable path**, enforces HTTPS (rejecting downgrade redirects), bounds the read, and **fails closed** when a release lacks a checksums file. (`cmd/scout/update.go`.)
- **Auto-spawned daemon no longer inherits ambient secrets.** `scout daemon` self-spawns `scout server` with a curated environment allowlist instead of the full parent env, so the long-lived daemon no longer carries `SCOUT_PASSPHRASE` / `SCOUT_VAULT_PASSPHRASE` / `SCOUT_AGENT_API_KEY` / OAuth tokens it never consumes. (`cmd/scout/daemon.go`.)
- **flow: tightened secret redaction.** `Referer`/`Origin` header values (which can carry OAuth tokens in their URL query) are now URL-redacted before reaching the LLM digest, and `sanitizeSpec` is now **default-deny** — any non-structural request-header value is parameterized to `${secret.*}`, so a secret in a custom-named header can no longer survive into the shareable `flow.yaml`. (`pkg/scout/flow/analyze.go`.)
- **Sitemap parsing is now SSRF-gated and bomb-bounded.** `Browser.ParseSitemap` fetched `sitemap.xml` (and every nested `<loc>` in a sitemap index) with a bare `http.Get` — no SSRF policy, no body limit, no recursion bound. A hostile sitemap index could (a) redirect a child fetch at `169.254.169.254`/loopback (SSRF), (b) return a multi-GB body to exhaust memory, or (c) self-reference to recurse forever (amplification). It now applies the `urlpolicy` SSRF gate (block-by-default; `SCOUT_ALLOW_LOCAL_TARGETS` to permit) to **every** node, caps the body read at 50 MiB, and bounds recursion with a depth limit (5), a shared visited-set, and a 100k total-URL ceiling. A `http.Client{Timeout}` replaces the no-timeout default client. (`internal/engine/crawl.go`.)
- **MCP `crawl` / `sitemap` tools now SSRF-gate their start URL.** Every other URL-ingress MCP tool (`navigate`, `gather`, …) ran `state.checkURL` before navigating, but `crawl` and `sitemap` skipped it — letting an MCP client steer them at `169.254.169.254`/loopback. Both now apply the same block-by-default gate ahead of `ensureBrowser`. (`pkg/scout/mcp/tools_crawl.go`, `tools_sitemap.go`.)
- **gRPC `CreateSession` rejects path-traversal in HAR/hijack output paths.** The `HarOut` / `HijackOut` request fields were joined under the session directory and written verbatim, so a trusted gRPC client could supply an absolute path or a `..`-traversing name and write the HAR/hijack artifact to an arbitrary location on the daemon host. `CreateSession` now validates both up front — a bare filename is required (no path separators, no `..`, not absolute) — returning `InvalidArgument` before any session is created; the destroy/flush sites re-validate the persisted `monitors.json` path defensively and fall back to the default. (`grpc/server/server_session.go`.)
- **Remote-response reads are now size-bounded (memory-DoS hardening).** Eight `io.ReadAll`/streaming-decode sites that consumed a remote HTTP response with no limit now use the canonical `io.ReadAll(io.LimitReader(body, max+1))` + overflow-check pattern, so a hostile or misconfigured server cannot exhaust memory: browser-archive download (1 GiB), CRX extension download (64 MiB), plugin-registry index (8 MiB, also switched from streaming decode to bounded read+`Unmarshal`), flow replay response (16 MiB), WebMCP responses ×2 (8 MiB), and the Anthropic/OpenAI LLM responses (32 MiB). (`internal/engine/browser/download.go`, `pkg/scout/browser/download.go`, `internal/engine/extension.go`, `pkg/scout/flow/runtime.go`, `internal/engine/webmcp.go`, `pkg/scout/plugin/registry/registry.go`, `internal/engine/llm/{anthropic,openai}.go`.)
- **Self-update refuses downgrades.** `scout update`'s `isNewer` accepted any remote tag that merely *differed* from the local version, so a rolled-back or spoofed older release tag would install as an "update" — re-introducing already-patched vulnerabilities. The check is now semver-aware (`golang.org/x/mod/semver`): only a strictly-newer tag updates; equal/older is refused. `SCOUT_ALLOW_DOWNGRADE=1` re-enables deliberate rollback. (`cmd/scout/update.go`.)
- **Archive extraction is panic- and bomb-hardened.** The cpio (RPM payload) parser sliced with crafted `namesize`/`filesize` header fields, so a malformed `.rpm` could panic the process (slice out of range — `namesize=0` sliced `data[110:109]`); the deb `ar` reader did `make([]byte, size)` from an unvalidated size (negative → panic, huge → memory DoS); and the zip/tar/rpm extractors had no cap on uncompressed size or entry count (decompression bomb). New shared limits (`pkg/scout/archive/limits.go`) bound each axis: cpio name + per-entry size + gunzip payload (512 MiB), a 2 GiB total + 50k entry-count cap across zip/tar/rpm, and a 1 GiB deb member cap. (Zip-slip path traversal was already covered by `pathSlipCheck`/`symlinkTargetWithinDest`.) (`pkg/scout/archive/{rpm,zip,tar,deb}.go`.)
- **Swarm coordinator bounds its URL frontier.** `Coordinator.Enqueue` grew the dedup seen-set (and thus the crawl queue) with no limit, so a sprawling or hostile site could exhaust the daemon's memory. A new `SwarmConfig.MaxURLs` cap (default 10k) stops admitting new URLs once reached; `NewCoordinator` clamps a non-positive value to a fail-closed 100k default so the cap applies even on the gRPC daemon path where the config is built directly. (`internal/engine/swarm/{types,coordinator}.go`.)
- **Agent SSE `start` event escapes the tool name.** The streaming `/call` endpoint wrote the caller-supplied tool name into the `start` SSE event without escaping (the sibling `error`/`result` events already escaped their payloads), so a name containing quotes/newlines could break the JSON or inject extra SSE frames. It is now wrapped with `escapeJSON`. (The agent HTTP server is deprecated.) (`pkg/scout/agent/server.go`.)
- **Hijack response-body capture is size-bounded.** Opt-in body capture (`Hijack.LoadResponse` with `loadBody`) did an unbounded `io.ReadAll` of the server-controlled response; it now caps at 64 MiB, truncating past the cap and flagging it with an `X-Scout-Body-Truncated` header rather than failing the capture. (`internal/engine/hijack.go`.)
- **SSRF policy enforces the scheme gate even under `AllowLocal`.** `urlpolicy.Policy.Check` short-circuited to "allow" the instant `AllowLocal` (`SCOUT_ALLOW_LOCAL_TARGETS`) was set, which also disabled the http(s)-only scheme check — so a `file://` / `gopher://` / `data:` URL was permitted once an operator opted into local targets. The scheme gate now runs **before** the `AllowLocal` short-circuit, so only internal *HTTP(S)* targets are ever allowed. (`pkg/scout/urlpolicy/policy.go`.)
- **Windows version probe escapes the browser path; Chrome-for-Testing metadata read is bounded.** `probeBrowserVersion` / `probeBrowserVersionPlatform` interpolated the browser path into a single-quoted PowerShell `-Command` literal unescaped, so a path containing a `'` could break out and inject PowerShell — the path is now escaped (`psQuote`, doubling single quotes). Separately, `download_chromium.go`'s version-API JSON was decoded with an unbounded `json.NewDecoder`; it now reads through an 8 MiB `io.LimitReader` first. (`internal/engine/browser/{detect_version_windows,download_chromium}.go`, `pkg/scout/browser/detect_windows.go`.)

### Fixed
- `Browser.HandleAuth` is now nil-safe: on an uninitialized or closed browser it returns an error-reporting function instead of panicking with a nil-pointer dereference (matches the nil-safe contract for key `Browser` methods).

## [1.1.0] - 2026-06-03

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
