---
phase: 03-cli-consolidation
plan: "05"
subsystem: internal/engine, cmd/scout/mcp
tags: [cleanup, tls, mcp, technical-debt]
dependency_graph:
  requires: []
  provides: [CLEAN-07, MCP-01]
  affects: [internal/engine/lib/cdp, cmd/scout/mcp]
tech_stack:
  added: []
  patterns: [tls.Dialer Go stdlib]
key_files:
  created: []
  modified:
    - internal/engine/lib/cdp/utils.go
    - internal/engine/lib/launcher/launcher.go
    - internal/engine/lib/launcher/manager.go
    - internal/engine/lib/launcher/flags/flags.go
    - internal/engine/page_rod.go
    - internal/engine/input.go
    - cmd/scout/mcp.go
decisions:
  - "Remove hardcoded tool count from MCP help text — replaced with descriptive label to avoid staleness"
metrics:
  duration: ~10min
  completed: 2026-03-29
  tasks_completed: 2
  files_modified: 7
---

# Phase 03 Plan 05: tls.Dialer Fix and MCP Tool Count Correction Summary

**One-liner:** Replaced Go 1.15-era tls.Dial workaround with tls.Dialer{}.DialContext; corrected MCP help text from stale "33 tools" to accurate "18 built-in browser automation tools".

## Tasks Completed

| # | Name | Commit | Files |
|---|------|--------|-------|
| 1 | Replace tls.Dialer TODO + annotate stale TODOs | 0b9a7dc | 6 files |
| 2 | Fix MCP server description tool count 33 → 18 | b23f392 | cmd/scout/mcp.go |

## What Was Built

**Task 1:** Replaced the `tlsDialer.DialContext` implementation that called `tls.Dial` (a Go 1.15 workaround) with `(&tls.Dialer{}).DialContext(ctx, network, address)` using the proper Go stdlib API. Also annotated five additional stale TODO comments across the rod-fork internals with precise explanations (upstream limitations, empirical timing rationale, deferred low-value item).

**Task 2:** Updated `cmd/scout/mcp.go` Long description — removed the stale list of 33 tools in seven categories (Content/Network/Forms/Analysis/Inspection were all migrated to plugins) and replaced with accurate grouping of the 18 built-in tools: Browser (11), Capture (5), WebSocket (3).

## Deviations from Plan

**1. [Rule 1 - Bug] MCP tool count fix applied to cmd/scout/mcp.go, not pkg/scout/mcp/server.go**
- **Found during:** Task 2
- **Issue:** The plan referenced `pkg/scout/mcp/server.go` but the "33 tools" string was in `cmd/scout/mcp.go` (the CLI Long help text). `server.go` never contained the string.
- **Fix:** Applied the change to the correct file `cmd/scout/mcp.go`.
- **Files modified:** cmd/scout/mcp.go
- **Commit:** b23f392

## Known Stubs

None.

## Self-Check: PASSED

- `internal/engine/lib/cdp/utils.go` — exists, contains `tls.Dialer`
- `cmd/scout/mcp.go` — exists, contains "18 built-in browser automation tools"
- Commit `0b9a7dc` — verified in git log
- Commit `b23f392` — verified in git log
- `go build ./internal/...` — OK
- `go build ./pkg/...` — OK
