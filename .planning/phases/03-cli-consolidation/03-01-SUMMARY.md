---
phase: "03"
plan: "01"
subsystem: cmd/scout
tags: [cli, auth, credentials, cleanup]
dependency_graph:
  requires: []
  provides: [CLI-02]
  affects: [cmd/scout/auth.go]
tech_stack:
  added: []
  patterns: [cobra subcommands, functional options]
key_files:
  created: []
  modified:
    - cmd/scout/auth.go
  deleted:
    - cmd/scout/credentials.go
decisions:
  - "No deprecated alias for credentials — breaking change accepted per D-02"
  - "Plaintext path uses scout.CaptureCredentials; encrypted path uses scraper/auth.BrowserCapture"
metrics:
  duration: "5m"
  completed: "2026-03-29"
  tasks: 2
  files: 2
---

# Phase 3 Plan 01: Merge credentials into auth Summary

**One-liner:** Deleted credentials.go and expanded auth.go with replay/show subcommands and --plaintext flag on capture.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Confirm recipe.go absent (no-op) | — | none |
| 2 | Merge credentials into auth, delete credentials.go | a85215c | cmd/scout/auth.go, cmd/scout/credentials.go (deleted) |

## What Was Done

- `cmd/scout/credentials.go` deleted; `credentialsCmd` no longer registered (was self-registered via `init()`).
- `auth capture` gains `--plaintext` (unencrypted JSON output), `--on-close`, and `--persist` flags.
- `auth replay` subcommand added — restores cookies/localStorage/sessionStorage from a plaintext JSON file.
- `auth show` subcommand added — prints credential file contents to stdout.
- `auth` subcommand list now: login, capture, replay, show, status, logout, providers.
- Build passes: `go build ./cmd/scout/`.

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

- `cmd/scout/credentials.go`: confirmed deleted
- `cmd/scout/auth.go`: contains replay/show subcommands and --plaintext flag
- Commit a85215c: confirmed in git log
