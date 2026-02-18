# Known Issues

## Open Issues

~~### Race method does not return matched index~~ [RESOLVED]

- **Status:** Fixed — Race now uses `Matches()` to identify the winning selector index.

### gRPC server test coverage is low

- **Severity:** Low
- **Status:** Partially resolved
- **Description:** The `grpc/server/` package now has integration tests (`server_test.go`) but coverage is still low (~30%). The `TestErrorPaths` test hangs due to browser timeout behavior. Needs more targeted tests.
- **Workaround:** Manual testing with `scout client` REPL, `scout` CLI commands, or `.scripts/test-client-server.sh`.

## Resolved Issues

| Issue                                               | Resolution                                                                                                                                             | Date    |
|-----------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|---------|
| Taskfile contains inapplicable tasks                | Legacy template tasks (proto:generate, sqlc:generate, goreleaser) replaced with valid proto, grpc:server, grpc:client, grpc:workflow, grpc:build tasks | 2025    |
| CI build workflow installs unneeded system packages | `.github/workflows/build.yml` removed; CI uses reusable `inovacc/workflows`                                                                            | 2025    |
| Race method does not return matched index           | Fixed: uses `Matches()` on returned element to determine winning selector index                                                                        | 2026-02 |
| CLI commands fail against mTLS server               | Fixed: all CLI commands now use `resolveClient(cmd)` instead of `getClient(addr)` for proper mTLS                                                      | 2026-02 |
| Server sessions die after 30s (context deadline)    | Fixed: server passes `WithTimeout(0)` to disable rod's one-shot page timeout for long-lived sessions                                                   | 2026-02 |
