# Known Issues

## Open Issues

### Race method does not return matched index
- **Severity:** Medium
- **Status:** Open
- **Description:** `Page.Race()` always returns `-1` as the match index (page.go:518), even though the method signature promises to return the index of the matched selector. The rod `race.Do()` API does not directly expose which selector won.
- **Workaround:** None. Callers cannot determine which selector matched.

### gRPC server lacks test coverage
- **Severity:** Medium
- **Status:** Open
- **Description:** The `grpc/server/` package has no tests. All RPCs are exercised only manually via the CLI client or example workflow. Regressions in API mapping (e.g., wrong method names, incorrect parameter types) would not be caught by CI.
- **Workaround:** Manual testing with `scout client` REPL or `scout` CLI commands.

## Resolved Issues

| Issue | Resolution | Date |
|-------|------------|------|
| Taskfile contains inapplicable tasks | Legacy template tasks (proto:generate, sqlc:generate, goreleaser) replaced with valid proto, grpc:server, grpc:client, grpc:workflow, grpc:build tasks | 2025 |
| CI build workflow installs unneeded system packages | `.github/workflows/build.yml` removed; CI uses reusable `inovacc/workflows` | 2025 |
