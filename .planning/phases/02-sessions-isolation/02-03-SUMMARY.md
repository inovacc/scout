---
phase: 02-sessions-isolation
plan: 03
subsystem: session
tags: [uuid, session-isolation, browser, options]

requires:
  - phase: 02-sessions-isolation
    provides: session tracking infrastructure (SessionDataDir, FindReusableSession)

provides:
  - UUID v7 session ID generation in New() — every call is isolated by default
  - WithReuseSession() as the canonical explicit opt-in for session persistence
  - WithReusableSession() deprecated alias (removal 2026-05-01)

affects: [02-sessions-isolation, cmd/scout, examples, plugins]

tech-stack:
  added: [github.com/google/uuid (already in go.mod v1.6.0)]
  patterns:
    - "Session IDs are random UUID v7, never derived from URL or browser name"
    - "Session reuse requires explicit WithReuseSession() — no implicit sharing"

key-files:
  created: []
  modified:
    - internal/engine/option.go
    - internal/engine/browser.go

key-decisions:
  - "UUID v7 chosen over v4 for time-ordered IDs compatible with scout session list ordering"
  - "WithReusableSession() kept as alias (not removed) to avoid breaking existing callers"
  - "Pre-existing build errors in lib/launcher and session/process_windows.go are out of scope"

patterns-established:
  - "WithReuseSession(): canonical name for session persistence opt-in"
  - "New() always creates a fresh session unless reusableSession flag is set"

requirements-completed: [SESS-03]

duration: 10min
completed: 2026-03-29
---

# Phase 02 Plan 03: UUID Session ID Isolation Summary

**UUID v7 replaces deterministic SessionHash in New(), eliminating implicit session reuse; WithReuseSession() added as the only explicit persistence opt-in**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-29T18:30:00Z
- **Completed:** 2026-03-29T18:40:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Every `New()` call now generates a unique UUID v7 session ID — two calls against the same URL no longer share state
- `WithReuseSession()` added as canonical opt-in for session persistence per D-01/D-02/D-03
- `WithReusableSession()` updated to delegate to `WithReuseSession()` with Deprecated comment and 2026-05-01 removal date
- The implicit reuse path (checking if `scout.pid` existed for a deterministic hash) is completely removed

## Task Commits

1. **Task 1: Add WithReuseSession() and deprecate WithReusableSession()** - `814051f` (feat)
2. **Task 2: Replace SessionHash with UUID v7 in New()** - `6bb96ac` (feat)

## Files Created/Modified

- `internal/engine/option.go` - Added `WithReuseSession()`, made `WithReusableSession()` a deprecated alias
- `internal/engine/browser.go` - Replaced `SessionHash` block with `uuid.Must(uuid.NewV7()).String()`, added uuid import

## Decisions Made

- UUID v7 chosen (not v4) because time-ordered IDs keep `scout session list` sorted naturally
- `WithReusableSession()` retained as alias rather than removed — breaking change avoidance for existing examples and plugins
- Pre-existing build errors in `internal/engine/lib/launcher/browser.go` and `internal/engine/session/process_windows.go` are out of scope (not caused by this plan)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Pre-existing build errors exist in `internal/engine/lib/launcher` and `internal/engine/session/process_windows.go` (unrelated to this plan's changes). These are deferred per scope boundary rules.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Session ID isolation is now enforced at the `New()` level
- `WithReuseSession()` is the single supported reuse path, ready for cleanup of call sites in Phase 02 remaining plans
- Existing callers of `WithReusableSession()` still compile without changes

---
*Phase: 02-sessions-isolation*
*Completed: 2026-03-29*
