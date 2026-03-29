---
phase: 03-cli-consolidation
plan: "04"
subsystem: cli
tags: [cobra, grpc, cli-grouping, screenshot]

requires:
  - phase: 03-cli-consolidation
    provides: CLI restructure context (plans 01-03)

provides:
  - grpcCmd parent command grouping 17 daemon commands under scout grpc
  - standalone scout screenshot at root (no daemon required)
  - scout grpc screenshot and scout grpc pdf for daemon-based capture

affects: [03-05]

tech-stack:
  added: []
  patterns: [grpc subcommand grouping via grpcCmd parent, standalone root commands promoted from mcp subgroup]

key-files:
  created:
    - cmd/scout/grpc_group.go
  modified:
    - cmd/scout/interact.go
    - cmd/scout/inspect.go
    - cmd/scout/navigate.go
    - cmd/scout/network.go
    - cmd/scout/storage.go
    - cmd/scout/window.go
    - cmd/scout/screenshot.go
    - cmd/scout/mcp_screenshot.go
    - cmd/scout/scout.go

key-decisions:
  - "grpcCmd parent registered in scout.go init() so it appears before plugin commands"
  - "rootScreenshotCmd reuses mcpScreenshotCmd.RunE to avoid code duplication"
  - "mcp screenshot kept as-is for backwards compat within mcp group; root screenshot is the new canonical path"

patterns-established:
  - "grpcCmd pattern: daemon commands added via grpcCmd.AddCommand() not rootCmd.AddCommand()"
  - "Standalone promotion: use separate var + shared RunE to add command at both mcp and root without duplication"

requirements-completed: [CLI-05, CLI-07]

duration: 12min
completed: 2026-03-29
---

# Phase 03 Plan 04: gRPC Command Grouping and Screenshot Consolidation Summary

**17 bare gRPC daemon commands grouped under `scout grpc` subcommand; standalone `scout screenshot <url>` promoted to root**

## Performance

- **Duration:** 12 min
- **Started:** 2026-03-29T20:20:00Z
- **Completed:** 2026-03-29T20:32:00Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments

- Created `grpcCmd` parent and moved all 17 daemon commands (click, type, select, hover, focus, clear, key, title, url, text, attr, eval, html, navigate, back, forward, reload, cookie, header, block, storage, window) under `scout grpc`
- Moved gRPC-based screenshot/pdf under `scout grpc screenshot` and `scout grpc pdf`
- Promoted standalone browser screenshot to root as `scout screenshot <url>` (no daemon required)

## Task Commits

1. **Task 1: Create grpcCmd parent and move all gRPC daemon commands under it** - `f15b060` (feat)
2. **Task 2: Consolidate screenshot — gRPC version under grpc, standalone at root** - `4f80f4c` (feat)

## Files Created/Modified

- `cmd/scout/grpc_group.go` - New file: grpcCmd parent command definition
- `cmd/scout/interact.go` - rootCmd.AddCommand -> grpcCmd.AddCommand for click/type/select/hover/focus/clear/key
- `cmd/scout/inspect.go` - rootCmd.AddCommand -> grpcCmd.AddCommand for title/url/text/attr/eval/html
- `cmd/scout/navigate.go` - rootCmd.AddCommand -> grpcCmd.AddCommand for navigate/back/forward/reload
- `cmd/scout/network.go` - rootCmd.AddCommand -> grpcCmd.AddCommand for cookie/header/block
- `cmd/scout/storage.go` - rootCmd.AddCommand -> grpcCmd.AddCommand for storage
- `cmd/scout/window.go` - rootCmd.AddCommand -> grpcCmd.AddCommand for window
- `cmd/scout/screenshot.go` - rootCmd.AddCommand -> grpcCmd.AddCommand for screenshot/pdf
- `cmd/scout/mcp_screenshot.go` - Added rootScreenshotCmd at root; reuses mcpScreenshotCmd.RunE
- `cmd/scout/scout.go` - Added rootCmd.AddCommand(grpcCmd) in init()

## Decisions Made

- `grpcCmd` registered in `scout.go` `init()` so it is always present before plugin commands run
- `rootScreenshotCmd.RunE = mcpScreenshotCmd.RunE` avoids code duplication while providing two registration paths
- `mcp screenshot` kept as-is (no removal) — breaking changes within mcp group are not needed since standalone is now canonical

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- CLI-05 and CLI-07 satisfied
- `scout --help` output is significantly shorter (17 fewer root-level commands)
- Ready for phase 03-05

---
*Phase: 03-cli-consolidation*
*Completed: 2026-03-29*

## Self-Check: PASSED

- grpc_group.go: FOUND
- 03-04-SUMMARY.md: FOUND
- Commits f15b060, 4f80f4c: FOUND
- grpcCmd.AddCommand in all 7 daemon files: FOUND
- rootCmd.AddCommand(grpcCmd) in scout.go: FOUND
- No leaked daemon commands at root: CLEAN
