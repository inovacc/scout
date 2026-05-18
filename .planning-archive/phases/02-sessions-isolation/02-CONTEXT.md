# Phase 2: Sessions & Isolation - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix session lifecycle bugs so sessions open cleanly, close cleanly, never leak processes, and never touch system browsers without explicit opt-in. 10 requirements: SESS-01 through SESS-06, ISOL-01 through ISOL-04.

</domain>

<decisions>
## Implementation Decisions

### Session Reuse Policy (SESS-03)
- **D-01:** No implicit session reuse. Every `New()` call gets a fresh session directory with a unique ID. The deterministic hash-based reuse mechanism must be removed.
- **D-02:** Explicit opt-in reuse via `WithReuseSession()` option if a caller wants session persistence across runs. This is the only path to reuse.
- **D-03:** Session IDs should be random (UUID or similar) — no deterministic hashing from domain+browser combination.

### Session Cleanup (SESS-01, SESS-02, SESS-05)
- **D-04:** `Browser.Close()` must clean up ALL resources: kill Chrome process, remove PID file, remove Chrome data directory. Single cleanup path, no redundant double cleanup (fixes S3 double cleanup latency).
- **D-05:** `CleanStaleSessions()` must remove orphaned Chrome data directories, not just PID files (fixes S5 incomplete cleanup).
- **D-06:** Remove the redundant `launcher.Cleanup()` + `ResetSession()` overlap. One canonical cleanup function.

### Windows Process Detection (SESS-04, SESS-06)
- **D-07:** Replace `OpenProcess` with `WaitForSingleObject` using 0 timeout for `ProcessAlive` on Windows. Returns immediately — `WAIT_OBJECT_0` means terminated, `WAIT_TIMEOUT` means alive. Accurate zombie detection.
- **D-08:** Increase Windows file lock retries from 3x200ms to a more generous window (e.g., 5x500ms = 2.5s) to outlast Chrome's file handle release after process termination.

### Browser Isolation (ISOL-01, ISOL-02, ISOL-03, ISOL-04)
- **D-09:** Default browser resolution uses ONLY `~/.scout/browsers/` cache. System-installed browsers are never probed unless `--system-browser` flag is explicitly set.
- **D-10:** Eliminate the rod fallback path that silently downloads to `~/.cache/rod/`. If no browser is available in `~/.scout/browsers/`, `BestCached()` auto-downloads Chrome for Testing there. Rod's own download path must be blocked or removed.
- **D-11:** `--system-browser` is the sole opt-in path to use system-installed browsers. Without it, Scout is fully isolated to its own browser cache.

### Claude's Discretion
- Specific implementation of `WithReuseSession()` API design
- Whether to use UUID v4 or UUID v7 for session IDs
- How to block rod's fallback download path (intercept in launcher, remove code path, or gate check)
- Whether `CleanStaleSessions()` should run synchronously at startup or in a background goroutine
- File lock retry timing specifics beyond the general "more generous" guidance

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Session Lifecycle Research
- `.planning/research/sessions-isolation.md` — Detailed bug analysis with file paths, line numbers, and root causes for all 6 session issues

### Codebase Architecture
- `.planning/codebase/ARCHITECTURE.md` — Layer boundaries and session management location
- `.planning/codebase/CONCERNS.md` — Tech debt items related to sessions

### Phase 1 Context
- `.planning/phases/01-safety-net-dead-code/01-CONTEXT.md` — recover() boundaries now protect session code from crashing processes

### Key Source Files
- `internal/engine/session/` — Session tracking, cleanup, process detection
- `internal/engine/session/process_windows.go` — Windows ProcessAlive (to be fixed)
- `internal/engine/session/process_unix.go` — Unix ProcessAlive (reference)
- `internal/engine/session/session_track.go` — CleanStaleSessions, CleanOrphans, ResetSession
- `internal/engine/browser.go` — Browser.Close(), session creation, deterministic hash
- `internal/engine/browser/` — BestCached(), browser detection, download management
- `internal/engine/lib/launcher/` — Rod launcher, browser process management

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/engine/session/session_track.go` — existing `CleanStaleSessions()`, `ResetSession()`, `CleanOrphans()` to be fixed
- `internal/engine/session/process_windows.go` — `ProcessAlive` to be rewritten with WaitForSingleObject
- `google/gops` agent — already integrated for process discovery
- `BestCached()` in `internal/engine/browser/` — already downloads Chrome for Testing

### Established Patterns
- Session directories: `~/.scout/sessions/<hash>/{scout.pid, job.json, data/}`
- Nil-safe Close() — maintain this pattern
- Error wrapping: `fmt.Errorf("scout: session: %w", err)`

### Integration Points
- `Browser.Close()` in `internal/engine/browser.go` — main cleanup entry point
- `main()` in `cmd/scout/scout.go` — calls `CleanStaleSessions()` on startup
- `WithReuseSession()` — new option to add in `option.go`

</code_context>

<specifics>
## Specific Ideas

No specific requirements — correctness fixes with clear targets from research.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 02-sessions-isolation*
*Context gathered: 2026-03-29*
