---
phase: 02-sessions-isolation
plan: 05
subsystem: browser
tags: [browser-isolation, rod, chrome-for-testing, session, launcher]

requires:
  - phase: 02-sessions-isolation
    provides: browser cache management via BestCached() auto-download

provides:
  - Explicit error from launchLocal() when no cached browser is available
  - Rod silent fallback path removed; ~/.cache/rod/ no longer written on failure

affects: [any phase that exercises browser launch without pre-cached browsers]

tech-stack:
  added: []
  patterns: ["launchLocal returns explicit error rather than falling through to rod's built-in download"]

key-files:
  created: []
  modified:
    - internal/engine/browser.go

key-decisions:
  - "Rod fallback removed: BestCached() failure now returns explicit fmt.Errorf; no silent download to ~/.cache/rod/"
  - "ISOL-04 preserved: BestCached() internally auto-downloads Chrome for Testing when cache is empty"

patterns-established:
  - "Isolation boundary: default browser resolution uses only ~/.scout/browsers/ cache; system browsers require explicit opt-in"

requirements-completed: [ISOL-01, ISOL-02, ISOL-03, ISOL-04]

duration: 5min
completed: 2026-03-29
---

# Phase 02 Plan 05: Remove Rod Fallback Summary

**Explicit error from launchLocal() replaces silent rod auto-download fallback; ~/.cache/rod/ write path eliminated**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-29T18:40:00Z
- **Completed:** 2026-03-29T18:45:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Removed the silent "fall through to rod auto-detect/download" path in `launchLocal()` default branch
- `BestCached()` failure now returns `fmt.Errorf("scout: no browser available in cache; run 'scout browser download'...")` instead of silently downloading to `~/.cache/rod/`
- `BestCached()` auto-download behavior (ISOL-04) preserved — error path is only reached if the download itself fails
- System browser access remains gated behind `o.systemBrowser` (ISOL-03 unchanged)

## Task Commits

1. **Task 1: Remove rod fallback; return explicit error when no browser available** - `e907b16` (fix)

## Files Created/Modified

- `internal/engine/browser.go` - Replaced silent rod fallback with explicit error in `launchLocal()` default branch

## Decisions Made

None - followed plan as specified.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- ISOL-01 through ISOL-04 all satisfied; browser isolation boundary is now explicit and auditable
- No known blockers for remaining phase work

---
*Phase: 02-sessions-isolation*
*Completed: 2026-03-29*
