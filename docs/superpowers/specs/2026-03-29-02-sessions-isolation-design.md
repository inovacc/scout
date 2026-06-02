# Phase 02: Sessions & Isolation — Design

**Date:** 2026-03-29
**Status:** In-progress
**Requirements:** SESS-01, SESS-02, SESS-03, SESS-04, SESS-05, SESS-06, ISOL-01, ISOL-02, ISOL-03, ISOL-04
**Depends on:** Phase 1

---

## Design / Approach / Components

Five plans address session lifecycle and browser isolation independently:

### Plan 02-01: Session Cleanup (SESS-01, SESS-02, SESS-05)
Consolidate `Browser.Close()` to a single cleanup path: `launcher.Cleanup()` then `os.RemoveAll(parentDir)`. Eliminate the `ResetSession()` overlap that caused 500ms latency and partial cleanup. `CleanStaleSessions` upgraded to remove orphaned Chrome data directories (not just PID files).

### Plan 02-02: Windows Process Detection + Retry Budget (SESS-04, SESS-06)
Replace `OpenProcess` false-positive pattern with `WaitForSingleObject(handle, 0)` for zombie detection. Add a retry budget for Windows file lock release to outlast Chrome handle teardown. Completed — self-check PASSED.

### Plan 02-03: UUID Session ID Isolation (SESS-03)
Replace deterministic hash session IDs with UUID v7 per `New()` call. `WithReuseSession()` added as canonical opt-in for session persistence. `WithReusableSession()` deprecated with 2026-05-01 removal date. Completed — accomplishments confirmed in summary.

### Plan 02-04: Single Cleanup Path in Browser.Close() (SESS-01, SESS-05)
Formal plan for the close-path consolidation (companion to 02-01 research). Ensures no double-cleanup race: `launcher.Cleanup` followed by `RemoveAll(parent)` in one code path with no redundant `ResetSession` call.

### Plan 02-05: Rod Fallback Elimination (ISOL-01, ISOL-02, ISOL-03, ISOL-04)
Remove the silent rod fallback that could download to `~/.cache/rod/` or probe system-installed browsers. `BestCached()` made strict: returns hard error if `~/.scout/browsers/` cache is empty (auto-download Chrome for Testing). `--system-browser` flag is the sole opt-in path to system browsers. Completed — accomplishments confirmed in summary.

---

## Rationale & Decisions

| Old Approach | New Approach | Impact |
|---|---|---|
| Deterministic hash session IDs | UUID v7 per `New()` | Eliminates implicit reuse bug (SESS-03) |
| `OpenProcess` for zombie detection | `WaitForSingleObject(h, 0)` | Correct zombie detection on Windows (SESS-04) |
| Double cleanup: `launcher.Cleanup + ResetSession` | Single path: `launcher.Cleanup + RemoveAll(parent)` | Removes 500ms latency, fixes SESS-05 |
| Rod fallback silent download | Hard error if `BestCached()` fails | Makes isolation boundary explicit (ISOL-02) |

**Session reuse policy:** Two sessions against the same URL never share cookies, localStorage, or session state by default. `WithReuseSession()` is the only opt-in. This was the root cause of the original deterministic hash bug.

**Windows-specific:** `WaitForSingleObject(WAIT_OBJECT_0 == 0)` correctly identifies a terminated process; `OpenProcess` returned handles to zombie processes, causing false "still alive" reports that blocked cleanup.

**Research sources (high confidence):** Windows `WaitForSingleObject` semantics, UUID v7 (`github.com/google/uuid`), rod launcher internals, `os.RemoveAll` behavior on non-empty directories.

---

## Constraints & Assumptions

- Phase 1 must be complete before Phase 2 execution (panic sites converted, recover() boundaries in place).
- Windows `ProcessAlive` test requires starting a real subprocess, capturing its PID, waiting for exit, then asserting `ProcessAlive(pid) == false`. Works cross-platform.
- `ISOL-04`: `BestCached()` auto-downloads Chrome for Testing when cache is empty — this is the correct behavior, not an error. The hard error is only when `BestCached()` cannot resolve any browser.
- `WithReusableSession()` removal date: 2026-05-01 (logged in BACKLOG).
- No mocking — all session tests use real browser instances or real subprocess PIDs.

**Known pitfalls documented in research:**
- `WaitForSingleObject` return value semantics (`WAIT_OBJECT_0 = 0`, not non-zero)
- `launcher.Cleanup()` removes the inner data dir, not the parent — `RemoveAll(parent)` is needed additionally
- UUID v7 import path: `github.com/google/uuid` (not a fork)
- `CleanStaleSessions` race with reusable sessions — addressed by UUID-based naming
- `WithReuseSession` vs `WithReusableSession` naming — canonical is `WithReuseSession`

---

## Testing & Acceptance

**Success Criteria:**
1. Running the same URL twice in separate `Browser` instances never shares cookies, localStorage, or session state.
2. After `Browser.Close()`, no orphaned Chrome processes or data directories remain under `~/.scout/sessions/`.
3. On Windows, `ProcessAlive` correctly reports a terminated process as dead (no zombie false-positives).
4. Default `New()` call never opens a system-installed browser.
5. `--system-browser` is the sole opt-in path to system browsers.

**Wave 0 test gaps identified in research:**
- `internal/engine/session/session_lifecycle_test.go` — covers SESS-01, SESS-02, SESS-04, SESS-05, SESS-06
- `internal/engine/browser_isolation_test.go` — covers ISOL-01, ISOL-02, ISOL-03
- `internal/engine/session_id_test.go` — covers SESS-03

**No VERIFICATION.md file present** — Plans 02-02, 02-03, 02-05 summaries show PASSED; Plans 02-01 and 02-04 status unconfirmed.

---

## Review Notes

Plans 02-02 (Windows process detection), 02-03 (UUID isolation), and 02-05 (rod fallback removal) self-checks PASSED. Auto-fixed: minor issues on 02-02. No deviations from plan intent on completed plans.
