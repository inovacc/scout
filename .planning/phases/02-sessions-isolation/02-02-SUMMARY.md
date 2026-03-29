---
phase: 02-sessions-isolation
plan: "02"
subsystem: session
tags: [windows, process-detection, file-locks, cleanup]
dependency_graph:
  requires: []
  provides: [accurate-windows-zombie-detection, retry-budget-constants]
  affects: [CleanStaleSessions, Reset]
tech_stack:
  added: []
  patterns: [WaitForSingleObject for process liveness, named constants for retry budget]
key_files:
  created: []
  modified:
    - internal/engine/session/process_windows.go
    - internal/engine/session/session_track.go
decisions:
  - "Use WaitForSingleObject(h, 0) with WAIT_TIMEOUT check instead of GetExitCodeProcess — correctly handles zombie processes"
  - "Cast windows.WAIT_TIMEOUT (syscall.Errno) to uint32 for comparison with WaitForSingleObject return value"
  - "removeRetries=5, removeRetryWait=500ms chosen per SESS-06 spec; real-world validation deferred to Phase 2 testing"
metrics:
  duration: "~10 minutes"
  completed: "2026-03-29"
  tasks_completed: 2
  files_modified: 2
---

# Phase 02 Plan 02: Windows Process Detection + Retry Budget Summary

**One-liner:** WaitForSingleObject replaces GetExitCodeProcess for zombie-safe Windows process detection; retry budget raised to 5x500ms via named constants.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Rewrite ProcessAlive with WaitForSingleObject | 7c1e897 | internal/engine/session/process_windows.go |
| 2 | Add removeRetries/removeRetryWait constants | b756b0c | internal/engine/session/session_track.go |

## What Was Done

**Task 1 — process_windows.go:**
- Replaced `GetExitCodeProcess` approach with `WaitForSingleObject(h, 0)` (zero timeout = non-blocking poll)
- `WAIT_TIMEOUT` (258) = process still running; `WAIT_OBJECT_0` (0) = exited; `WAIT_FAILED` = error/invalid
- Switched import from `syscall` to `golang.org/x/sys/windows` for named constants
- Used `PROCESS_QUERY_LIMITED_INFORMATION` (0x1000) — requires fewer OS privileges than the previous 0x0400
- Added inline comment explaining WAIT_TIMEOUT semantics
- Deviation: cast `windows.WAIT_TIMEOUT` (`syscall.Errno`) to `uint32` to match `WaitForSingleObject` return type — compiler rejected direct comparison

**Task 2 — session_track.go:**
- Added `removeRetries = 5` and `removeRetryWait = 500 * time.Millisecond` constants after imports
- Updated `Reset()`: `for range 3` + hardcoded `time.Sleep(500ms)` → `for range removeRetries` + `time.Sleep(removeRetryWait)`
- Updated `CleanStaleSessions()`: `for range 3` + `time.Sleep(200ms)` → `for range removeRetries` + `time.Sleep(removeRetryWait)`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Type mismatch in WaitForSingleObject result comparison**
- **Found during:** Task 1 verification
- **Issue:** `windows.WAIT_TIMEOUT` is `syscall.Errno` but `WaitForSingleObject` returns `uint32` — direct comparison rejected by compiler
- **Fix:** Cast to `uint32(windows.WAIT_TIMEOUT)` for the comparison
- **Files modified:** internal/engine/session/process_windows.go
- **Commit:** 7c1e897

## Known Stubs

None.

## Self-Check: PASSED

- `internal/engine/session/process_windows.go` — exists, contains `WaitForSingleObject`
- `internal/engine/session/session_track.go` — exists, contains `removeRetries` and `removeRetryWait`
- Commit 7c1e897 — verified in git log
- Commit b756b0c — verified in git log
- `GOOS=windows go build ./internal/engine/session/` — exits 0
- `go build ./internal/engine/session/...` — exits 0
