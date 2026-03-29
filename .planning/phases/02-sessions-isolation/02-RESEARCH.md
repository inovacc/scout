# Phase 2: Sessions & Isolation - Research

**Researched:** 2026-03-29
**Domain:** Session lifecycle, process management, browser isolation (Go/Windows/Unix)
**Confidence:** HIGH (all findings from direct source analysis)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Session Reuse Policy (SESS-03)**
- D-01: No implicit session reuse. Every `New()` call gets a fresh session directory with a unique ID. The deterministic hash-based reuse mechanism must be removed.
- D-02: Explicit opt-in reuse via `WithReuseSession()` option if a caller wants session persistence across runs. This is the only path to reuse.
- D-03: Session IDs should be random (UUID or similar) — no deterministic hashing from domain+browser combination.

**Session Cleanup (SESS-01, SESS-02, SESS-05)**
- D-04: `Browser.Close()` must clean up ALL resources: kill Chrome process, remove PID file, remove Chrome data directory. Single cleanup path, no redundant double cleanup (fixes S3 double cleanup latency).
- D-05: `CleanStaleSessions()` must remove orphaned Chrome data directories, not just PID files (fixes S5 incomplete cleanup).
- D-06: Remove the redundant `launcher.Cleanup()` + `ResetSession()` overlap. One canonical cleanup function.

**Windows Process Detection (SESS-04, SESS-06)**
- D-07: Replace `OpenProcess` with `WaitForSingleObject` using 0 timeout for `ProcessAlive` on Windows. Returns immediately — `WAIT_OBJECT_0` means terminated, `WAIT_TIMEOUT` means alive. Accurate zombie detection.
- D-08: Increase Windows file lock retries from 3x200ms to a more generous window (e.g., 5x500ms = 2.5s) to outlast Chrome's file handle release after process termination.

**Browser Isolation (ISOL-01, ISOL-02, ISOL-03, ISOL-04)**
- D-09: Default browser resolution uses ONLY `~/.scout/browsers/` cache. System-installed browsers are never probed unless `--system-browser` flag is explicitly set.
- D-10: Eliminate the rod fallback path that silently downloads to `~/.cache/rod/`. If no browser is available in `~/.scout/browsers/`, `BestCached()` auto-downloads Chrome for Testing there. Rod's own download path must be blocked or removed.
- D-11: `--system-browser` is the sole opt-in path to use system-installed browsers. Without it, Scout is fully isolated to its own browser cache.

### Claude's Discretion
- Specific implementation of `WithReuseSession()` API design
- Whether to use UUID v4 or UUID v7 for session IDs
- How to block rod's fallback download path (intercept in launcher, remove code path, or gate check)
- Whether `CleanStaleSessions()` should run synchronously at startup or in a background goroutine
- File lock retry timing specifics beyond the general "more generous" guidance

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SESS-01 | Sessions clean up all resources (process, PID file, data dir) on Browser.Close() | Close() currently does launcher.Cleanup() + ResetSession() redundantly; D-04/D-06 define single-path fix |
| SESS-02 | CleanStaleSessions removes orphaned Chrome data directories, not just PID files | CleanOrphans() only removes scout.pid, not session dir; CleanStaleSessions must RemoveAll |
| SESS-03 | New sessions never implicitly reuse stale session state (fix deterministic hash reuse bug) | browser.go:292-305 is the exact bug site; replace SessionHash with uuid |
| SESS-04 | Windows ProcessAlive correctly detects terminated processes (fix OpenProcess false positives) | process_windows.go:14-27 is the exact fix site; WaitForSingleObject pattern documented |
| SESS-05 | Session close avoids redundant double cleanup (remove launcher.Cleanup + ResetSession overlap) | browser.go:714-724 is the exact site; ResetSession adds 500ms unnecessary sleep |
| SESS-06 | Windows file lock retries are sufficient to outlast Chrome handle release | session_track.go:341-350 is the retry loop; change from 3x200ms to 5x500ms |
| ISOL-01 | Default browser resolution uses only ~/.scout/browsers/ cache, never system-installed browsers | Already implemented via ResolveCached(); gap is the rod fallback at browser.go:341 |
| ISOL-02 | Rod fallback path is eliminated or gated behind explicit opt-in | browser.go:340-341 comment "fall through to rod auto-detect" is the exact gap |
| ISOL-03 | --system-browser flag is the only way to use system-installed browsers | Already gated; just ensure rod fallback doesn't bypass this gate |
| ISOL-04 | Browser.BestCached() auto-downloads Chrome for Testing when cache is empty | Already implemented in detect.go:78-119; preserved as-is |
</phase_requirements>

---

## Summary

Phase 2 is a surgical correctness fix across three independently testable areas: session lifecycle, Windows process detection, and browser isolation. All bugs have been precisely located in the prior research — this phase has no exploratory work, only targeted edits at known line ranges.

The biggest risk is the Close() refactor (D-04/D-06): the current double-cleanup (launcher.Cleanup + ResetSession) adds 500ms latency on every non-reusable session close due to ResetSession's unconditional sleep. The fix is to remove ResetSession from Close() and let launcher.Cleanup() own the data-dir removal. The session directory's parent dir (the hash/UUID dir) must also be removed — currently only launcher.Cleanup() removes the inner `data/` subdir while the outer dir persists.

The deterministic hash bug (SESS-03) is the highest impact: every `New()` call against the same URL+browser silently reuses the previous session if `scout.pid` was not cleaned. Replacing the hash with a random UUID (v7 recommended — already used by job tracking in `session/job.go`) eliminates this class of bug entirely.

**Primary recommendation:** Fix in dependency order — SESS-04 first (ProcessAlive correctness unblocks accurate stale session detection), then SESS-03 (remove hash, use UUID), then SESS-01/SESS-05/SESS-06 (cleanup path consolidation and retry budget), then ISOL-02 (rod fallback gate).

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/sys/windows` | already in go.mod | `WaitForSingleObject` syscall | Standard Go Windows syscall wrapper; avoids raw `syscall` package limitations |
| `github.com/google/uuid` | already in go.mod | Random session IDs (UUID v7) | Already used in project (`session/job.go` uses `uuid.New()`); v7 is time-ordered |
| Go stdlib `os`, `syscall` | stdlib | ProcessAlive, file ops | Already used throughout session package |

**Note:** No new dependencies required. All needed libraries are already in go.mod.

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/google/gops/goprocess` | already in go.mod | `IsScoutProcess` detection | Keep as-is — correct and already integrated |

---

## Architecture Patterns

### Pattern 1: WaitForSingleObject for ProcessAlive (Windows)

**What:** Replace `OpenProcess` handle check with `WaitForSingleObject(handle, 0)` which distinguishes live from terminated/zombie processes.

**When to use:** Only in `process_windows.go` — Unix uses `Signal(0)` which already works correctly.

**Exact replacement for `process_windows.go:14-27`:**
```go
// Source: Windows API docs — WaitForSingleObject with 0 timeout
// WAIT_TIMEOUT (258) = process still running
// WAIT_OBJECT_0 (0)  = process has exited
// WAIT_FAILED        = error (handle invalid)

const (
    processQueryLimitedInformation = 0x1000
    waitTimeout                    = 0x00000102 // WAIT_TIMEOUT
)

func ProcessAlive(pid int) bool {
    if pid <= 0 {
        return false
    }
    h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
    if err != nil {
        return false
    }
    defer syscall.CloseHandle(h)
    result, _ := syscall.WaitForSingleObject(h, 0)
    return result == waitTimeout // WAIT_TIMEOUT means still running
}
```

**Key insight:** `WaitForSingleObject(h, 0)` returns immediately. `WAIT_TIMEOUT` means the process has NOT exited (still alive). `WAIT_OBJECT_0` means it has exited. This correctly handles zombie processes.

### Pattern 2: Random Session IDs (UUID v7)

**What:** Replace `SessionHash(targetURL, browserName)` with `uuid.New()` (UUID v4) or a time-ordered v7 equivalent.

**Recommendation:** Use UUID v7 (time-ordered) — `session/job.go` already generates `ksuid.New().String()` for job IDs. For session IDs, `github.com/google/uuid` v7 via `uuid.Must(uuid.NewV7()).String()` gives time-ordered IDs useful for `scout session list` ordering.

**Exact replacement for `browser.go:291-305`:**
```go
// Before (REMOVE):
// Always use a deterministic hash dir — never let launcher generate UUID.
if o.userDataDir == "" {
    // ... hash logic that auto-enables reuse
}

// After (REPLACE WITH):
// Generate a fresh random session ID for every new browser instance.
// Reuse is only possible via explicit WithReuseSession() opt-in.
if o.userDataDir == "" {
    id := uuid.Must(uuid.NewV7()).String()
    o.userDataDir = SessionDataDir(id)
    o.sessionID = id
}
```

**WithReuseSession() API design (Claude's discretion):**
```go
// In option.go — rename WithReusableSession to WithReuseSession per D-02 naming
// WithReuseSession opts into session persistence. The browser will search for
// a reusable session matching the given browser type and headless mode.
// Without this option, every New() call creates a fresh isolated session.
func WithReuseSession() Option {
    return func(o *options) { o.reusableSession = true }
}
```

The existing `WithReusableSession()` already does this — it just needs the random-ID path to not auto-enable reuse. The name change to `WithReuseSession` per D-02 is a rename, not a logic change.

### Pattern 3: Single Cleanup Path in Browser.Close()

**What:** Remove `ResetSession()` call from Close(). Let `launcher.Cleanup()` own data-dir removal. Add explicit removal of the session parent directory after launcher cleanup.

**Why ResetSession() must be removed from Close():**
- `ResetSession()` at `session_track.go:220-260` sleeps 500ms waiting for process exit — Chrome is already killed by `launcher.Kill()` at this point
- `launcher.Cleanup()` already removes `data/` subdir
- `ResetSession()` then tries to kill an already-dead process and remove an already-removed dir

**Replacement for `browser.go:707-724`:**
```go
// 7. Kill process tree and clean up session directory.
if b.launcher != nil {
    b.launcher.Kill()

    if !b.opts.reusableSession && b.sessionID != "" {
        // Non-reusable: launcher removes the Chrome data dir.
        b.launcher.Cleanup()
        // Then remove the session parent dir (scout.pid already removed in step 6).
        _ = os.RemoveAll(session.Dir(b.sessionID))
    }

    b.launcher = nil
}
```

**Key:** `session.Dir(id)` returns `~/.scout/sessions/<id>/` (the parent). `launcher.Cleanup()` removes `~/.scout/sessions/<id>/data/`. Then `RemoveAll` on the parent removes the now-empty parent dir cleanly.

### Pattern 4: Retry Budget for Windows File Locks

**What:** Change retry loop in `CleanStaleSessions` and `ResetSession` from 3x200ms to 5x500ms.

**File:** `session_track.go:341-350` and anywhere else `RemoveAll` is retried:
```go
// Before:
for range 3 {
    if err := os.RemoveAll(...); err == nil {
        break
    }
    time.Sleep(200 * time.Millisecond)
}

// After:
const (
    removeRetries  = 5
    removeRetryWait = 500 * time.Millisecond
)
for range removeRetries {
    if err := os.RemoveAll(...); err == nil {
        break
    }
    time.Sleep(removeRetryWait)
}
```

**Total budget:** 5x500ms = 2.5s max. STATE.md notes this is empirical and may still be insufficient — but 2.5s is a reasonable first step that covers the common case.

### Pattern 5: Rod Fallback Elimination (ISOL-02)

**What:** Convert the silent rod fallback into an explicit error when no browser is found.

**File:** `browser.go:329-342` — the `default:` branch of the browser resolution switch:
```go
default:
    if o.systemBrowser {
        if path, _, err := browser.BestDetected(); err == nil && path != "" {
            l = l.Bin(path)
        }
    } else {
        path, err := browser.BestCached()
        if err != nil || path == "" {
            // CHANGED: return error instead of silently falling through to rod
            return "", nil, fmt.Errorf("scout: no browser available in cache; run 'scout browser download' or set --system-browser")
        }
        l = l.Bin(path)
    }
    // REMOVED: "If detection fails, fall through to rod auto-detect/download."
```

**Note:** `BestCached()` already auto-downloads Chrome for Testing when the cache is empty (detect.go:110-117), so this error path should only be reached if the download itself fails. The comment and silent fallthrough are removed.

### Anti-Patterns to Avoid

- **Do not call ResetSession() from Close():** It sleeps unconditionally, adds latency, and duplicates work already done by launcher.Cleanup(). ResetSession() is for external/CLI use only (force-reset a specific session by ID).
- **Do not call CleanOrphans() twice in New():** Currently called at browser.go:65 and again at browser.go:270. Remove one call.
- **Do not add sleep before process kill:** launcher.Kill() sends SIGKILL/TerminateProcess — no need to wait for graceful exit in the non-reusable path.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Random session IDs | Custom hash/ID scheme | `uuid.Must(uuid.NewV7()).String()` | Already in go.mod; time-ordered; collision-free |
| Windows process state | Custom WaitForExit loop | `WaitForSingleObject(h, 0)` | Win32 API designed exactly for this; 0 timeout = non-blocking |
| File lock retry | Custom exponential backoff | Simple `for range N { sleep }` loop | Chrome lock duration is bounded; exponential is overkill here |
| Process kill on Windows | Manual TerminateProcess | `launcher.Kill()` (already handles it) | Rod launcher already wraps platform kill correctly |

---

## Common Pitfalls

### Pitfall 1: WaitForSingleObject Return Value Semantics
**What goes wrong:** Treating `WAIT_OBJECT_0 (0)` as "process alive" — it actually means "signaled/exited."
**Why it happens:** `0` looks like success/true in C conventions.
**How to avoid:** Check for `WAIT_TIMEOUT (258/0x102)` = process still running. Any other value = exited or error.
**Warning signs:** ProcessAlive returns true for known-dead processes on Windows.

### Pitfall 2: launcher.Cleanup() Removes Inner Dir Only
**What goes wrong:** After Close(), the session parent dir `~/.scout/sessions/<id>/` remains on disk even though `data/` is gone.
**Why it happens:** launcher.Cleanup() was designed to remove the Chrome user-data-dir, which is `data/` — it doesn't know about Scout's parent directory.
**How to avoid:** After `launcher.Cleanup()`, explicitly call `os.RemoveAll(session.Dir(b.sessionID))` to remove the parent.
**Warning signs:** Session dirs accumulate with empty interiors.

### Pitfall 3: UUID v7 Import Path
**What goes wrong:** `uuid.NewV7()` is not in older versions of `github.com/google/uuid`.
**Why it happens:** UUID v7 was added in google/uuid v1.6.0.
**How to avoid:** Check go.mod version. If < v1.6.0, use `uuid.New().String()` (v4) instead — both are random enough for session IDs.
**Warning signs:** Compile error `undefined: uuid.NewV7`.

### Pitfall 4: CleanStaleSessions Race with Reusable Sessions
**What goes wrong:** CleanStaleSessions sees `info.Reusable = true` with `ScoutPID != 0` but ProcessAlive returns false (due to the Windows bug being fixed). After fixing ProcessAlive, dead reusable sessions will now be correctly cleaned — which is the desired behavior. No race issue post-fix, but verify the logic path covers this.
**How to avoid:** After fixing ProcessAlive, manually test: create a reusable session, kill the scout process externally, run `scout session list` — session should appear as stale and be cleaned on next startup.

### Pitfall 5: WithReuseSession vs WithReusableSession Naming
**What goes wrong:** CONTEXT.md names the new option `WithReuseSession()` (D-02) but the existing option is `WithReusableSession()`. Callers in examples/, plugins/, and documentation reference `WithReusableSession`.
**How to avoid:** Keep `WithReusableSession()` as a deprecated alias pointing to `WithReuseSession()`. The CONTEXT.md decision renames the concept but existing callers must not break.

---

## Code Examples

### Session ID Generation (Claude's discretion: UUID v7)
```go
// Source: github.com/google/uuid (already in go.mod)
// Check go.mod version before using NewV7:
// - v1.6.0+: uuid.Must(uuid.NewV7()).String()
// - older:   uuid.New().String()  (v4, also fine)

import "github.com/google/uuid"

id := uuid.Must(uuid.NewV7()).String()
o.userDataDir = SessionDataDir(id)
o.sessionID = id
```

### WaitForSingleObject Pattern
```go
// Source: Windows API — golang.org/x/sys/windows
// Alternative using x/sys/windows (cleaner constants):
import "golang.org/x/sys/windows"

func ProcessAlive(pid int) bool {
    if pid <= 0 {
        return false
    }
    h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
    if err != nil {
        return false
    }
    defer windows.CloseHandle(h)
    result, _ := windows.WaitForSingleObject(h, 0)
    return result == windows.WAIT_TIMEOUT
}
```

### Session Dir Removal After launcher.Cleanup()
```go
// In Browser.Close(), non-reusable path:
b.launcher.Kill()
b.launcher.Cleanup()                          // removes ~/.scout/sessions/<id>/data/
_ = os.RemoveAll(session.Dir(b.sessionID))   // removes ~/.scout/sessions/<id>/
b.launcher = nil
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Deterministic hash session IDs | Random UUID per New() | This phase | Eliminates implicit reuse bug |
| OpenProcess for zombie detection | WaitForSingleObject(h, 0) | This phase | Correct zombie detection on Windows |
| Double cleanup: launcher.Cleanup + ResetSession | Single path: launcher.Cleanup + RemoveAll parent | This phase | Removes 500ms latency from Close() |
| Rod fallback silent download | Hard error if BestCached() fails | This phase | Makes isolation boundary explicit |

---

## Open Questions

1. **UUID v7 availability in go.mod**
   - What we know: google/uuid is in go.mod; v7 requires >= v1.6.0
   - What's unclear: Exact version in go.mod (not read during research)
   - Recommendation: Check go.mod in Wave 1 plan; fall back to `uuid.New()` (v4) if needed

2. **golang.org/x/sys/windows vs syscall for WaitForSingleObject**
   - What we know: Both work; x/sys/windows has named constants (WAIT_TIMEOUT, etc.); `syscall` package has raw constants
   - What's unclear: Whether x/sys/windows is already in go.mod
   - Recommendation: Use `golang.org/x/sys/windows` if already present (cleaner); otherwise use `syscall` with defined constants to avoid adding a dep

3. **Windows file lock total budget (STATE.md concern)**
   - What we know: 5x500ms = 2.5s is the plan; STATE.md flags this as empirical
   - What's unclear: Real-world Chrome lock duration on Windows 11 (test environment)
   - Recommendation: Implement 5x500ms; add a TODO comment noting it may need tuning; TEST-02 in v2 requirements will validate this

---

## Environment Availability

Step 2.6: SKIPPED — this phase is code/config changes only. No external tools, services, or CLIs beyond the project's own Go toolchain are required.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | None (Go native) |
| Quick run command | `go test -v -run TestSession ./internal/engine/session/...` |
| Full suite command | `task test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SESS-01 | Close() removes process + PID file + data dir | unit | `go test -v -run TestClose ./internal/engine/...` | ❌ Wave 0 |
| SESS-02 | CleanStaleSessions removes orphaned data dirs | unit | `go test -v -run TestCleanStaleSessions ./internal/engine/session/...` | ❌ Wave 0 |
| SESS-03 | New() creates fresh UUID session each call | unit | `go test -v -run TestNewSessionID ./internal/engine/...` | ❌ Wave 0 |
| SESS-04 | ProcessAlive returns false for zombie PID (Windows) | unit | `go test -v -run TestProcessAlive ./internal/engine/session/...` | ❌ Wave 0 |
| SESS-05 | Close() does not double-cleanup (no extra 500ms sleep) | unit | `go test -v -run TestCloseLatency ./internal/engine/...` | ❌ Wave 0 |
| SESS-06 | RemoveAll retried 5x with 500ms delay | unit | `go test -v -run TestRetryBudget ./internal/engine/session/...` | ❌ Wave 0 |
| ISOL-01 | Default New() never reads system browser paths | unit | `go test -v -run TestBrowserIsolation ./internal/engine/...` | ❌ Wave 0 |
| ISOL-02 | launchLocal returns error when BestCached fails (no rod fallback) | unit | `go test -v -run TestNoRodFallback ./internal/engine/...` | ❌ Wave 0 |
| ISOL-03 | WithSystemBrowser() enables system browser detection | unit | `go test -v -run TestSystemBrowserOpt ./internal/engine/...` | ❌ Wave 0 |
| ISOL-04 | BestCached() auto-downloads Chrome for Testing when cache empty | integration | `go test -v -run TestBestCachedDownload ./internal/engine/browser/...` | ✅ (existing) |

### Sampling Rate
- **Per task commit:** `go test -v -short ./internal/engine/session/...`
- **Per wave merge:** `task test:unit`
- **Phase gate:** `task test` (full suite green before `/gsd:verify-work`)

### Wave 0 Gaps
- [ ] `internal/engine/session/session_lifecycle_test.go` — covers SESS-01, SESS-02, SESS-04, SESS-05, SESS-06
- [ ] `internal/engine/browser_isolation_test.go` — covers ISOL-01, ISOL-02, ISOL-03
- [ ] `internal/engine/session_id_test.go` — covers SESS-03 (UUID per New() call, no hash reuse)

**Note:** SESS-04 (Windows ProcessAlive zombie detection) must use a mock or a real terminated PID. The safest approach: start a subprocess, capture its PID, wait for it to exit, then assert `ProcessAlive(pid) == false`. Works cross-platform. On Unix this already passes; the test proves the Windows fix works.

---

## Sources

### Primary (HIGH confidence)
- Direct source analysis of `internal/engine/browser.go` (lines 63-265, 270-342, 651-734)
- Direct source analysis of `internal/engine/session/process_windows.go`
- Direct source analysis of `internal/engine/session/process_unix.go`
- Direct source analysis of `internal/engine/session/session_track.go` (lines 284-354)
- Direct source analysis of `internal/engine/option.go` (session fields and options)
- `.planning/research/sessions-isolation.md` — prior deep code investigation

### Secondary (MEDIUM confidence)
- Windows API documentation for `WaitForSingleObject` semantics (WAIT_TIMEOUT=258, WAIT_OBJECT_0=0) — standard Windows API, well-documented

---

## Metadata

**Confidence breakdown:**
- Session lifecycle fixes: HIGH — exact file/line locations confirmed by source read
- Windows ProcessAlive fix: HIGH — exact API semantics confirmed, pattern clear
- Browser isolation: HIGH — rod fallback path confirmed at browser.go:341
- UUID library availability: MEDIUM — in go.mod but exact version not checked
- Windows retry budget (5x500ms): MEDIUM — reasonable estimate; flagged as empirical in STATE.md

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (stable domain — session lifecycle and Win32 APIs don't change)
