# Known Issues

## Open Issues

~~### Race method does not return matched index~~ [RESOLVED]

- **Status:** Fixed — Race now uses `Matches()` to identify the winning selector index.

~~### gRPC server test coverage below target~~ [RESOLVED]

- **Severity:** Low
- **Status:** Fixed — Coverage raised from 67.7% to 80.6% with Interactive commands, pairing, TLS, mapKey, truncate, GetLocalIPs tests.

### Rod fork: segfault on disconnected page (upstream #1103)

- **Severity:** High
- **Status:** Open — patch planned for Phase 24
- **Description:** `getJSCtxID()` in `pkg/rod/page_eval.go` can segfault when page/connection is nil (e.g., browser disconnected mid-operation). Upstream rod has no fix. Patch: nil-guard returning `ErrDisconnected`.
- **Workaround:** Wrap rod calls with `recover()` at the Scout wrapper level.

### Rod fork: context not propagated in page operations (upstream #1179)

- **Severity:** Medium
- **Status:** Open — patch planned for Phase 24
- **Description:** Page's context is not passed through to internal operations in `pkg/rod/page.go` (line ~851). Causes operations to ignore cancellation.
- **Workaround:** Use timeouts at the Scout wrapper level.

### WaitStable panic on "Execution context was destroyed" (upstream #1157)

- **Severity:** Medium
- **Status:** Open — fix planned for Phase 24
- **Description:** `WaitStable` can panic when page navigation destroys the execution context during stability check. Affects SPA navigation scenarios.
- **Workaround:** Avoid `WaitStable` during navigation; use `WaitLoad` + manual delay.

### Zombie Chrome processes after Browser.Close (upstream #865)

- **Severity:** Medium
- **Status:** Open — fix planned for Phase 24
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
