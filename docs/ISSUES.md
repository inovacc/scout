# Known Issues

## Open Issues

No open issues at this time.

## Resolved Issues

| Issue | Resolution | Date |
|-------|------------|------|
| Taskfile contains inapplicable tasks | Legacy template tasks replaced with valid proto/grpc tasks | 2025 |
| CI build workflow installs unneeded system packages | `.github/workflows/build.yml` removed; CI uses reusable `inovacc/workflows` | 2025 |
| Race method does not return matched index | Fixed: uses `Matches()` to determine winning selector index | 2026-02 |
| CLI commands fail against mTLS server | Fixed: all CLI commands use `resolveClient(cmd)` for proper mTLS | 2026-02 |
| Server sessions die after 30s | Fixed: `WithTimeout(0)` disables rod one-shot page timeout | 2026-02 |
| gRPC server test coverage below target | Fixed: coverage raised from 67.7% to 80.6% | 2026-02 |
| Rod fork: segfault on disconnected page (#1103) | Fixed: nil-guard in `pkg/rod/page_eval.go` | 2026-02 |
| Rod fork: context not propagated (#1179) | Fixed: page ops use `p.browser.Context(p.ctx)` | 2026-02 |
| Zombie Chrome processes (#865) | Fixed: `launcher.Kill()` walks process tree on Close() | 2026-02 |
| WaitStable panic (#1157) | Fixed: `WaitSafe()` provides panic recovery | 2026-02 |
| Window maximize blank space | Fixed: clears DeviceMetricsOverride after maximize | 2026-02 |
| Windows browser detection opens GUI | Fixed: PowerShell `-WindowStyle Hidden` (v0.20.0) | 2026-02 |
| ParseVersion wrong for Brave | Fixed: regex returns first match (v0.20.0) | 2026-02 |
| Bridge extension opens visible browser in headless tests | Fixed: skip bridge loading when `--headless` active (old mode doesn't support extensions) | 2026-02 |
| Orphaned browser on terminal close (`mcp open`) | Fixed: gops agent + `Page.WaitClose()` CDP event detection + synchronous `launcher.Cleanup()` | 2026-03 |
| `CleanOrphans` false positives from PID reuse | Fixed: `IsScoutProcess()` via gops confirms PID is a scout Go process | 2026-03 |
| Session directory not cleaned on close | Fixed: `launcher.Cleanup()` made synchronous (was `go` goroutine, racing process exit) | 2026-03 |
| MCP `screenshot`/`navigate` timeout (`context deadline exceeded`) | Fixed: `WithTimeout(0)` disables rod 30s page timeout for MCP; `WaitLoad` made best-effort with 15s cap | 2026-03 |
| MCP session disconnect after `session_reset` | Fixed: close page before browser + 500ms delay for OS port/dir cleanup | 2026-03 |
| Sitemap extract drops CDP connection on Chrome for Testing | Fixed: `Bridge.ResetReady()` clears stale `available` flag before each navigation; Chrome for Testing kills WebSocket on stale binding access | 2026-03 |
| `scout browser list` leaks system browser paths | Fixed: default mode shows only `~/.scout/browsers/` cache; system scan moved behind `--detect` flag | 2026-03 |
| Cobra command errors silently dropped | Fixed: CLI now prints cobra errors to stderr | 2026-05-21 |
| Session dirs leaked when `New()` partially failed | Fixed: enforce cleanup of partial `New()` failures | 2026-05-21 |
| Reusable daemon sessions lost on daemon restart | Fixed (H6 follow-up): persistent reusable sessions survive daemon restart | 2026-05-21 |
| Open-ended reusable sessions accumulated forever | Fixed (Phase 77.2): persistent sessions now require `WithExpiration()`; `ExpiresAt` enforced via audit | 2026-05-21 |
| Advisory lock collided with `scout.pid` writes | Fixed (Phase 77.1): lock moved to sibling `scout.lock` (`LockFileEx` Windows, `flock` Unix) | 2026-05-21 |
| Cleanup stalled on AV / OneDrive / Search Indexer–held dirs | Fixed (Phase 77.4): single-shot `RemoveAll` fast path + `StartCleanupRetrier` (60 s for process lifetime) | 2026-05-21 |
| Stale Chrome lock blocked session reuse | Fixed (Phase 77.4): stale Chrome lock files reclaimed on session reuse | 2026-05-21 |
| `preWriteStubInfo` overwrote existing `scout.pid` | Fixed (Phase 77.4): `preWriteStubInfo` skips when `scout.pid` already exists | 2026-05-21 |
| `registerSession` lost canonical sessionID through plumbing (deferred L5) | Fixed (Phase 77.1): canonical sessionID plumbed through reuse + ephemeral branches | 2026-05-21 |
| CLI session tracker collided with engine session dirs | Fixed: CLI session tracker moved to (now removed) `active-sessions/`; canonical session dir is authoritative | 2026-05-21 |
