---
phase: 03-cli-consolidation
plan: "03"
subsystem: cli
tags: [cobra, commands, extract, markdown, restructure]

requires: []
provides:
  - extract parent command grouping table/meta/ai subcommands
  - markdown top-level command removed
affects: [03-cli-consolidation]

tech-stack:
  added: []
  patterns:
    - "Extract subcommands grouped under a single parent cobra.Command"

key-files:
  created: []
  modified:
    - cmd/scout/extract.go
    - cmd/scout/llm.go
  deleted:
    - cmd/scout/markdown.go

key-decisions:
  - "No deprecated alias for removed markdown command — breaking changes accepted per project decisions"
  - "extractAICmd Use field changed from extract-ai to ai for natural sub-command naming"

patterns-established:
  - "Related extraction commands grouped under extract parent: scout extract table, scout extract meta, scout extract ai"

requirements-completed: [CLI-04, CLI-06]

duration: 15min
completed: 2026-03-29
---

# Phase 03 Plan 03: Remove markdown command, group extract subcommands under parent

**Deleted `markdown.go` (CLI-04) and moved `table`/`meta`/`extract-ai` under `scout extract` parent (CLI-06), reducing top-level command clutter**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-29T20:15:00Z
- **Completed:** 2026-03-29T20:30:00Z
- **Tasks:** 2
- **Files modified:** 2 (+ 1 deleted)

## Accomplishments
- Deleted `cmd/scout/markdown.go`; `scout fetch --mode=markdown` is the canonical path
- Created `extractCmd` parent in `extract.go` with `table` and `meta` as subcommands
- Moved `extractAICmd` (Use: `ai`) from top-level to under `extractCmd`; removed `rootCmd.AddCommand(extractAICmd)` from `llm.go`

## Task Commits

1. **Task 1: Delete markdown.go** - handled by parallel agent (markdown.go absent from HEAD)
2. **Task 2: Group table/meta/extract-ai under extract parent** - `1922328` (feat)

## Files Created/Modified
- `cmd/scout/extract.go` - Added `extractCmd` parent; changed `rootCmd.AddCommand(tableCmd, metaCmd)` to `extractCmd.AddCommand(tableCmd, metaCmd, extractAICmd)`
- `cmd/scout/llm.go` - Removed `rootCmd.AddCommand(extractAICmd)`; changed `extractAICmd.Use` from `"extract-ai"` to `"ai"`
- `cmd/scout/markdown.go` - DELETED

## Decisions Made
- No deprecated alias for `markdown` — breaking changes allowed per project decisions (D-09)
- `extract-ai` renamed to `ai` under the `extract` parent for clean `scout extract ai` UX

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `markdown.go` was already deleted and `auth.go` was already updated by another parallel agent before Task 1 ran; the build was clean, no rework needed.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- CLI-04 and CLI-06 satisfied
- `scout extract --help` lists: table, meta, ai
- `scout markdown` / `scout table` / `scout meta` / `scout extract-ai` all produce "unknown command"
- Ready for remaining Phase 03 plans

---
*Phase: 03-cli-consolidation*
*Completed: 2026-03-29*
