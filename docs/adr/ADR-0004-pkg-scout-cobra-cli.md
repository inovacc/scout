# ADR-0004: Move Library to pkg/scout and Create Unified Cobra CLI

## Status

Accepted (2026-02)

## Context

The scout library originally lived in the root package (`package scout`). As the project grew, multiple command binaries were added under `cmd/`:

- `cmd/server/` - gRPC server
- `cmd/client/` - Interactive CLI client
- `cmd/example-workflow/` - Streaming demo
- `cmd/slack-assist/` - Slack session capture

This caused two problems:

1. **Go toolchain conflict**: Having `package scout` in the root directory prevented creating `cmd/scout/main.go` because Go forbids `package main` alongside `package scout` in the same directory.
2. **Fragmented UX**: Four separate binaries made installation and discovery harder for users. Each binary had its own flags and patterns.

## Decision

1. **Move the core library to `pkg/scout/`** so the import path becomes `github.com/inovacc/scout/pkg/scout`. The root package becomes a deprecation stub.
2. **Create a single Cobra CLI at `cmd/scout/`** that absorbs all separate binaries into subcommands (`scout server`, `scout client`, `scout slack`, etc.).
3. **Add a background gRPC daemon** for session persistence across CLI invocations, with auto-start and PID file management in `~/.scout/`.
4. **Clean break** with no backward-compatibility re-exports at the root package (project is pre-v1).

## Consequences

### Positive

- Single `scout` binary with discoverable subcommands via `--help`
- Session persistence: create a browser session once, use it across multiple commands
- Daemon auto-starts on first use, no manual server management needed
- Library import path is explicit: `pkg/scout` clearly separates library from binary
- Internal CLI package (`cmd/scout/internal/cli/`) prevents external import of CLI internals

### Negative

- Breaking change for all consumers of the root import path (requires `s/scout/pkg\/scout/` in imports)
- All 18 examples needed import path updates
- Platform-specific daemon code needed (`daemon_unix.go`, `daemon_windows.go`)
- Slightly more complex development setup (daemon process management)

### Neutral

- The `github.com/spf13/cobra` dependency is isolated to `cmd/scout/` and does not affect library consumers
- File-based session tracking (`~/.scout/sessions/`) was chosen over a `ListSessions` gRPC RPC to keep the proto surface minimal
