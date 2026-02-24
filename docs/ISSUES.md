# Known Issues

## Open Issues

~~### Race method does not return matched index~~ [RESOLVED]

- **Status:** Fixed — Race now uses `Matches()` to identify the winning selector index.

~~### gRPC server test coverage below target~~ [RESOLVED]

- **Severity:** Low
- **Status:** Fixed — Coverage raised from 67.7% to 80.6% with Interactive commands, pairing, TLS, mapKey, truncate, GetLocalIPs tests.

~~### Rod fork: segfault on disconnected page (upstream #1103)~~ [RESOLVED]

- **Severity:** High
- **Status:** Fixed — `PageDisconnectedError` nil-guard added in `pkg/rod/page_eval.go` (Phase 24).

~~### Rod fork: context not propagated in page operations (upstream #1179)~~ [RESOLVED]

- **Severity:** Medium
- **Status:** Fixed — `Info()`, `Activate()`, `TriggerFavicon()` now use `p.browser.Context(p.ctx)` instead of `p.browser.ctx` (commit 61fb628).
- **Description:** Page's context is not passed through to internal operations in `pkg/rod/page.go` (line ~851). Causes operations to ignore cancellation.
- **Workaround:** Use timeouts at the Scout wrapper level.

~~### WaitStable panic on "Execution context was destroyed" (upstream #1157)~~ [RESOLVED]

- **Severity:** Medium
- **Status:** Fixed — `WaitSafe()` method provides panic recovery wrapping `WaitStable` (commit bd53ea6).
- **Description:** `WaitStable` can panic when page navigation destroys the execution context during stability check. Affects SPA navigation scenarios.
- **Workaround:** Avoid `WaitStable` during navigation; use `WaitLoad` + manual delay.

~~### Zombie Chrome processes after Browser.Close (upstream #865)~~ [RESOLVED]

- **Severity:** Medium
- **Status:** Fixed — launcher reference retained in Browser struct; `launcher.Kill()` on Close() walks process tree (commit 61fb628).
- **Description:** `Browser.Close()` may leave orphan Chrome child processes (GPU, renderer, utility). Over time, zombie processes accumulate in daemon mode.
- **Workaround:** Manual `pkill -f chrome` after extended sessions.

### ~~Window maximize leaves blank space~~ [RESOLVED]

- **Severity:** Medium
- **Status:** Fixed — `setWindowState()` now clears `EmulationDeviceMetricsOverride` after maximize/fullscreen so Chrome uses actual window dimensions instead of the initial 1920x1080 viewport pin.

## Resolved Issues

| Issue                                               | Resolution                                                                                                                                             | Date    |
|-----------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|---------|
| Taskfile contains inapplicable tasks                | Legacy template tasks (proto:generate, sqlc:generate, goreleaser) replaced with valid proto, grpc:server, grpc:client, grpc:workflow, grpc:build tasks | 2025    |
| CI build workflow installs unneeded system packages | `.github/workflows/build.yml` removed; CI uses reusable `inovacc/workflows`                                                                            | 2025    |
| Race method does not return matched index           | Fixed: uses `Matches()` on returned element to determine winning selector index                                                                        | 2026-02 |
| CLI commands fail against mTLS server               | Fixed: all CLI commands now use `resolveClient(cmd)` instead of `getClient(addr)` for proper mTLS                                                      | 2026-02 |
| Server sessions die after 30s (context deadline)    | Fixed: server passes `WithTimeout(0)` to disable rod's one-shot page timeout for long-lived sessions                                                   | 2026-02 |
| gRPC server test coverage below target              | Fixed: coverage raised from 67.7% to 80.6% with Interactive, pairing, TLS, mapKey, truncate, GetLocalIPs tests                                        | 2026-02 |
| Rod fork: segfault on disconnected page (#1103)     | Fixed: `PageDisconnectedError` nil-guard in `pkg/rod/page_eval.go` (Phase 24)                                                                         | 2026-02 |
| Rod fork: context not propagated in page operations (#1179) | Fixed: `Info()`, `Activate()`, `TriggerFavicon()` now use `p.browser.Context(p.ctx)` instead of `p.browser.ctx` (commit 61fb628)              | 2026-02 |
| Zombie Chrome processes after Browser.Close (#865) | Fixed: launcher reference retained in Browser struct; `launcher.Kill()` on Close() walks process tree (commit 61fb628)                              | 2026-02 |
| WaitStable panic on "Execution context was destroyed" (#1157) | Fixed: `WaitSafe()` method provides panic recovery wrapping `WaitStable` (commit bd53ea6)                                                | 2026-02 |
