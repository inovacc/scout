# Session Lifecycle & Browser Isolation - Deep Code Investigation

**Researched:** 2026-03-29
**Domain:** Session management, browser isolation, process lifecycle
**Confidence:** HIGH (direct source code analysis)

## Summary

Scout's session system uses a file-based tracker (`~/.scout/sessions/<hash>/`) with `scout.pid` metadata and `data/` Chrome user-data directories. Sessions are identified by deterministic SHA-256 hashes of `(root_domain + browser_name)`. The lifecycle spans: creation in `launchLocal()` -> registration in `registerSession()` -> cleanup in `Browser.Close()` or startup via `CleanStaleSessions()`. Browser isolation defaults to cache-only mode (`~/.scout/browsers/`) with system browser access gated behind `--system-browser` flag.

The architecture is solid but has several specific edge cases and potential leak vectors documented below.

---

## 1. Session Lifecycle

### 1.1 Session Creation Flow

**Entry point:** `internal/engine/browser.go:63` - `New()`

```
New() -> CleanOrphans() [line 65]
      -> launchLocal(o) [line 149]
         -> CleanOrphans() again [line 270] (double cleanup)
         -> resolve userDataDir [lines 273-305]
         -> launcher.Launch() [line 416]
      -> registerSession() [line 257/222]
      -> StartOrphanWatchdog() [line 262/224]
```

**Session ID resolution** (`browser.go:273-305`):
1. If `o.sessionID` is set explicitly -> use `SessionDataDir(o.sessionID)`
2. If `o.reusableSession` -> scan for matching session via `FindReusableSession()`
3. Otherwise -> generate deterministic hash: `SessionHash(targetURL, browserName)` -> use `SessionDataDir(hash)`

**Key observation (line 292-305):** When no explicit session ID or reuse is requested, Scout ALWAYS generates a deterministic hash. This means repeated runs against the same URL+browser combo share the same session directory. If `scout.pid` exists from a previous run, the code automatically sets `reusableSession = true` (line 303), which can cause **unintended session reuse**.

### 1.2 Session Registration

**File:** `internal/engine/browser.go:757-807` - `registerSession()`

Writes `scout.pid` to `~/.scout/sessions/<hash>/` containing:
- `ScoutPID`: current process PID (`os.Getpid()`)
- `BrowserPID`: from `launcher.PID()`
- `Reusable`, `Headless`, `Browser`, `DomainHash`, `Domain`
- `CreatedAt`, `LastUsed` timestamps
- `Exec`, `BuildVersion` from gops enrichment

**Two paths:**
1. **Existing session** (line 776-789): Updates `LastUsed`, `ScoutPID`, `BrowserPID` on existing `scout.pid`
2. **New session** (line 792-806): Creates new `SessionInfo` and writes it

### 1.3 Session Closure

**File:** `internal/engine/browser.go:651-734` - `Close()`

Idempotent via `sync.Once`. Steps:
1. Close `done` channel (stops autofree + orphan watchdog)
2. Stop bridge WebSocket server
3. Stop VPN/fingerprint rotators
4. Close CDP connection
5. **Session info update** (lines 693-705):
   - Reusable: clear PIDs, update `LastUsed`, keep `scout.pid`
   - Non-reusable: remove `scout.pid`
6. **Process/dir cleanup** (lines 708-724):
   - Reusable: skip cleanup
   - Non-reusable: `launcher.Kill()` + `launcher.Cleanup()` + `ResetSession()`

### 1.4 Startup Cleanup

**File:** `cmd/scout/scout.go:129` - called in `main()`:
```go
_, _ = session.CleanStaleSessions()
```

**File:** `internal/engine/session/session_track.go:292-354` - `CleanStaleSessions()`

For each directory in `~/.scout/sessions/`:
1. **No scout.pid** -> orphaned dir, `RemoveAll` immediately
2. **Reusable + live process** -> preserve (checks both ScoutPID and BrowserPID)
3. **Non-reusable OR dead reusable** -> kill browser if alive, then `RemoveAll` with 3x retry (200ms sleep for Windows file locks)

---

## 2. Session Leak Vectors

### 2.1 Double CleanOrphans on Every New()

`New()` calls `CleanOrphans()` at line 65, then `launchLocal()` calls it again at line 270. This is harmless but wasteful -- scans all session dirs twice per browser creation.

### 2.2 Deterministic Hash Causes Implicit Reuse

**Location:** `browser.go:292-305`

When `userDataDir` is empty and no explicit session requested, the code:
```go
hash := SessionHash(o.targetURL, browserName)
o.userDataDir = SessionDataDir(hash)
o.sessionID = hash
if _, err := ReadSessionInfo(hash); err == nil {
    o.reusableSession = true  // BUG: auto-enables reuse
}
```

If a previous session's `scout.pid` still exists (e.g., from a crash where cleanup didn't run), the NEW session silently becomes "reusable" and inherits the old Chrome profile data. This can leak cookies, local storage, and login state between independent runs.

### 2.3 CleanStaleSessions Race with New()

`CleanStaleSessions()` runs at startup in `main()`. If `New()` is called very quickly after (e.g., in a script), there's a TOCTOU race:
- `CleanStaleSessions` reads session dir, finds it stale, starts removal
- `New()` in same process writes new `scout.pid` to same hash dir
- Removal deletes the just-created `scout.pid`

This is unlikely in practice (single-threaded CLI) but possible in library usage.

### 2.4 Windows File Lock Retry Gaps

**`session_track.go:341-350`** and **`session_track.go:241-249`**:

Both `CleanStaleSessions` and `Reset` retry `RemoveAll` 3 times with 200ms or 500ms delays. Chrome on Windows can hold locks longer than 600ms-1500ms total. If removal fails silently (the error is swallowed in `CleanStaleSessions`), the stale session persists and may be auto-reused by the deterministic hash logic (leak 2.2).

### 2.5 ProcessAlive False Positives on Windows

**`process_windows.go:14-27`**:
```go
func ProcessAlive(pid int) bool {
    h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
    if err != nil {
        return false
    }
    _ = syscall.CloseHandle(h)
    return true
}
```

This checks if a process handle can be opened, but does NOT check if the process has exited. A zombie/terminated process whose handle hasn't been fully released will appear "alive". This can cause `CleanStaleSessions` to skip cleanup for actually-dead sessions.

**Contrast with Unix** (`process_unix.go:14-24`): Uses `Signal(0)` which correctly distinguishes live from dead processes.

### 2.6 gops Dependency for IsScoutProcess

**Both platform files:** `IsScoutProcess()` depends on `goprocess.Find()` from `google/gops`. This only works for Go binaries with gops agent enabled. If:
- The scout process crashed before gops registered
- The gops agent failed to start (line 122-124 in `scout.go` -- error is logged but not fatal)
- PID reuse assigned the old PID to a non-Go process

Then `IsScoutProcess` returns false, and `CleanOrphans` will kill the orphaned browser. This is actually correct behavior (fail-safe), but worth noting.

### 2.7 Orphan Watchdog Lifetime

**`browser.go:262`**: `StartOrphanWatchdog(DefaultOrphanCheckInterval, br.done)`

The watchdog goroutine runs every 2 minutes and dies when `br.done` is closed (in `Close()`). If `Close()` is never called (panic, SIGKILL, power loss), the watchdog dies with the process. The orphaned browser survives until the NEXT scout invocation calls `CleanStaleSessions()` in `main()`.

### 2.8 Non-Reusable Close Does Double Cleanup

**`browser.go:714-724`**:
```go
b.launcher.Kill()       // kills browser process tree
b.launcher.Cleanup()    // removes user-data-dir
if b.sessionID != "" {
    _ = ResetSession(b.sessionID)  // removes ENTIRE session dir
}
```

`launcher.Cleanup()` removes the data dir. Then `ResetSession()` tries to kill the browser (already dead) and remove the parent dir. The `Reset()` function sleeps 500ms waiting for process exit (line 235), adding unnecessary latency to `Close()` for non-reusable sessions.

---

## 3. Browser Isolation

### 3.1 Two-Mode Architecture

**Default (cache-only):** `--system-browser` is `false`
- `launchLocal()` line 321: calls `browser.ResolveCached()` -- only looks in `~/.scout/browsers/`
- `launchLocal()` line 337: calls `browser.BestCached()` -- same cache-only search
- If nothing cached: auto-downloads Chrome for Testing (`BestCached` line 110-117)

**System mode:** `--system-browser` is `true`
- `launchLocal()` line 319: calls `browser.Resolve()` -- tries system paths first, then downloads
- `launchLocal()` line 333: calls `browser.BestDetected()` -- scans `Program Files`, `LOCALAPPDATA`, etc.

### 3.2 Resolution Chain (Default/Cache-Only)

**`browser.ResolveCached()`** (`download.go:1025-1082`):
1. Check `installed.json` registry for browser name
2. Scan `~/.scout/browsers/<type>/<version>/` directories
3. If not found: download (Chrome for Testing, Chromium, Brave, or Edge)

**`browser.BestCached()`** (`detect.go:78-119`):
1. Check registry for: chrome, chromium, edge, brave (in order)
2. Scan disk: `~/.scout/browsers/{chrome,chromium,edge,brave}/<version>/<binary>`
3. If nothing found: auto-download Chrome for Testing (10-minute timeout)

### 3.3 System Browser Leak Points

**`download.go:651-716` - `downloadEdgeWindows()`**:
Even in cache-only mode, `DownloadEdge()` on Windows calls `downloadEdgeWindows()` which calls `lookupBrowser(Edge)` -- this scans system paths (`Program Files\Microsoft\Edge\...`). It then COPIES the system Edge into `~/.scout/browsers/edge/<version>/`. This is a design choice: the "cached" copy is derived from the system browser. If the system Edge updates, the cached copy becomes stale.

**`path_windows.go:35-37` - Chrome returns empty**:
```go
case Chrome:
    return "", nil // rod auto-detect
```
`lookupBrowser(Chrome)` on Windows returns empty string (no error). This means `browser.Resolve()` for Chrome falls through to the download path, never finding system Chrome. This is likely intentional (prefer Chrome for Testing over system Chrome).

### 3.4 Rod Fallback

**`browser.go:340-341`**:
```go
// If detection fails, fall through to rod auto-detect/download.
```

If BOTH `BestCached()` and `BestDetected()` fail (return error), the launcher falls through with no `Bin()` set. Rod's launcher then uses its own auto-detect/download logic (which downloads Chromium to `~/.cache/rod/` or platform equivalent). This bypasses Scout's isolation entirely.

### 3.5 Browser Registry (`installed.json`)

**`download.go:40-141`**: JSON file at `~/.scout/browsers/installed.json` tracks all downloaded browsers with name, version, binary path, platform, and install timestamp. `RegisterBrowser()` is called after every successful download. `LookupRegistryBrowser()` walks entries backwards (newest first) and verifies the binary still exists on disk.

---

## 4. Process Management

### 4.1 gops-Based Detection

**Both `process_windows.go` and `process_unix.go`** share identical `IsScoutProcess()` and `ScoutProcessInfo()`:

```go
func IsScoutProcess(pid int) bool {
    p, found, err := goprocess.Find(pid)
    if err != nil || !found { return false }
    return strings.Contains(strings.ToLower(p.Exec), "scout")
}
```

Uses gops to find Go processes, then string-matches "scout" in the executable name. This means:
- Non-Go browsers (Chrome, etc.) will never match -- correct
- A process named "scout-something-else" would match -- unlikely but possible
- Requires gops agent running in the target process

### 4.2 Platform Differences

| Aspect | Windows | Unix |
|--------|---------|------|
| `ProcessAlive` | `OpenProcess(QUERY_LIMITED_INFO)` | `Signal(0)` |
| Zombie detection | No (open handle succeeds) | Yes (signal returns error) |
| File lock retries | 3x 200ms in cleanup | Same code but locks less common |
| PID reuse risk | Higher (32-bit PIDs, faster reuse) | Lower (larger PID space on modern kernels) |

### 4.3 Orphan Cleanup Flow

**`CleanOrphans()`** (`session_track.go:189-218`):
1. List all sessions
2. Skip if ScoutPID or BrowserPID is 0
3. If `IsScoutProcess(ScoutPID)` -> owner alive, skip
4. If `ProcessAlive(BrowserPID)` -> orphaned browser, kill it
5. Remove `scout.pid`

**Gap:** Only removes `scout.pid`, not the session directory or Chrome data. Orphaned data dirs accumulate until `CleanStaleSessions()` runs at next startup.

---

## 5. Key Files Reference

| File | Lines | Purpose |
|------|-------|---------|
| `internal/engine/browser.go` | 63-265 | `New()`, `launchLocal()`, session creation |
| `internal/engine/browser.go` | 651-734 | `Close()`, session cleanup |
| `internal/engine/browser.go` | 757-807 | `registerSession()` |
| `internal/engine/session/session_track.go` | 60-69 | `Dir()`, `DataDir()` |
| `internal/engine/session/session_track.go` | 72-104 | `WriteInfo()`, `ReadInfo()` |
| `internal/engine/session/session_track.go` | 189-218 | `CleanOrphans()` |
| `internal/engine/session/session_track.go` | 284-354 | `CleanStaleSessions()` |
| `internal/engine/session/session_track.go` | 362-380 | `StartOrphanWatchdog()` |
| `internal/engine/session/session_track.go` | 458-474 | `Hash()` - deterministic session ID |
| `internal/engine/session/process_windows.go` | 14-27 | `ProcessAlive` Windows impl |
| `internal/engine/session/process_unix.go` | 14-25 | `ProcessAlive` Unix impl |
| `internal/engine/session/process_*.go` | 33-44 | `IsScoutProcess` (both platforms) |
| `internal/engine/session/job.go` | 1-207 | Job tracking (KSUID, status, steps) |
| `internal/engine/browser/detect.go` | 24-51 | `DetectBrowsers()` system scan |
| `internal/engine/browser/detect.go` | 67-74 | `BestDetected()` |
| `internal/engine/browser/detect.go` | 78-119 | `BestCached()` with auto-download |
| `internal/engine/browser/download.go` | 999-1021 | `Resolve()` (system browser path) |
| `internal/engine/browser/download.go` | 1025-1082 | `ResolveCached()` (cache-only path) |
| `internal/engine/browser/detect_windows.go` | 10-57 | Windows browser path candidates |
| `internal/engine/browser/path_windows.go` | 11-41 | `lookupBrowser()` Windows |
| `internal/engine/option.go` | 82 | `systemBrowser` field |
| `internal/engine/option.go` | 427 | `WithSystemBrowser()` option |
| `cmd/scout/scout.go` | 68 | `--system-browser` CLI flag |
| `cmd/scout/scout.go` | 121-139 | `main()` with `CleanStaleSessions()` |

---

## 6. Recommendations for Improvement

### High Priority
1. **Fix implicit reuse bug** (Section 2.2): Don't auto-set `reusableSession = true` just because `scout.pid` exists. Require explicit opt-in via `WithReusableSession()` or `--reuse-session` flag.
2. **Fix Windows ProcessAlive** (Section 2.5): Check process exit code via `GetExitCodeProcess()` to distinguish live from zombie processes.
3. **Eliminate double Close cleanup** (Section 2.8): Skip `ResetSession()` when `launcher.Cleanup()` already removed the data dir, or merge the two operations.

### Medium Priority
4. **Add rod fallback warning** (Section 3.4): Log a warning when rod's internal download is used, so users know they've left Scout's isolation boundary.
5. **CleanOrphans should remove dirs** (Section 4.3): Currently only removes `scout.pid`, leaving data dirs for `CleanStaleSessions`. Should remove the full session dir to prevent accumulation.
6. **Increase Windows retry budget** (Section 2.4): Chrome on Windows can hold locks for 2-5 seconds. Consider 5 retries at 500ms or exponential backoff.

### Low Priority
7. **Remove double CleanOrphans** (Section 2.1): Call it once in `New()` or once in `launchLocal()`, not both.
8. **Add session age limit**: Reusable sessions currently live forever. Consider a TTL (e.g., 7 days) after which they're cleaned even if `scout.pid` says reusable.
