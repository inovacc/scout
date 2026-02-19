# Known Issues

## Open Issues

~~### Race method does not return matched index~~ [RESOLVED]

- **Status:** Fixed — Race now uses `Matches()` to identify the winning selector index.

### gRPC server test coverage below target

- **Severity:** Low
- **Status:** Partially resolved
- **Description:** The `grpc/server/` package has integration tests (`server_test.go`) with 67.7% coverage. Still below 80% target. Needs more targeted tests for individual RPCs and error paths.
- **Workaround:** Manual testing with `scout client` REPL, `scout` CLI commands, or `.scripts/test-client-server.sh`.

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
