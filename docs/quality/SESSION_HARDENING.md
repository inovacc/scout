# Session Mechanism — Hardening Audit

Scope: `internal/engine/session/` + `internal/engine/browser.go` registerSession/Close paths. Findings ranked by severity. Each item lists location, root cause, exploit/failure mode, and proposed fix.

## Status

| ID | Severity | Status | Commit |
|----|----------|--------|--------|
| H1 | HIGH | DONE | `1405f75` |
| H2 | HIGH | DONE | (this PR) |
| H3 | HIGH | DONE | `01c2c51` |
| H4 | HIGH | OPEN | — |
| M1–M6 | MED | OPEN | — |
| L1–L6 | LOW | OPEN | — |

## SEV-HIGH

### H1. `IsScoutProcess` substring match is over-permissive [DONE]

**Where:** `session/process_unix.go:42`, `process_windows.go:53` — `strings.Contains(strings.ToLower(p.Exec), "scout")`.

**Problem:** Matches any binary whose name contains the literal "scout" anywhere — `scoutsdk-helper`, `boyscout`, `whereisscout`, or an adversary-named `scout-tool.exe`. Two consequences:

1. False positive: `CleanOrphans` skips a session whose original scout died because some unrelated PID-reusing process happens to match. Browser leaks indefinitely.
2. False protection: an attacker who learns the heuristic names a binary to evade orphan cleanup, or to claim ownership of someone else's session dir.

**Fix:** Compare the *basename* of `p.Exec` exactly to `"scout"` / `"scout.exe"`, AND when a `SessionInfo.Exec` is stored, compare full paths. `EnrichSessionInfo` already records `Exec` and `BuildVersion` — wire them into the check.

### H2. TOCTOU between `ProcessAlive` and `Kill` (browser PID unverified) [DONE]

**Bonus finding during fix:** `ProcessAlive` on Windows was completely broken. `OpenProcess` was called with `PROCESS_QUERY_LIMITED_INFORMATION` only, but `WaitForSingleObject` requires the `SYNCHRONIZE` access right — every call returned `WAIT_FAILED` with "Access is denied", so `ProcessAlive` returned `false` for every PID including the current process. Result: `CleanOrphans`, `Reset`, and `CleanStaleSessions` all skipped their kill paths on Windows. Browsers leaked. Fixed by adding `SYNCHRONIZE` to the access mask. This also explains why `TestProcessAlive` and `TestCleanStaleSessions` had been failing on main.

**Where:** `session_track.go:214-220` (CleanOrphans), `:247-252` (Reset), `:351-355` (CleanStaleSessions).

**Pattern:** `if ProcessAlive(BrowserPID) { p, _ := os.FindProcess(BrowserPID); p.Kill() }`.

**Problem:** Between check and kill the kernel may reuse the PID for an unrelated process. Windows PID reuse is fast (often within seconds of exit). Browser PIDs are killed with **zero identity verification** — no equivalent to the gops check used for scout PIDs.

**Fix:** Record `BrowserStartTime` in `SessionInfo` at register time (`launcher.PID()` then read process start). Before kill, re-open process handle and verify start time matches. On Windows: `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` + `GetProcessTimes`. On Unix: `/proc/<pid>/stat` btime field (Linux) or `sysctl KERN_PROC_PID` (Darwin/BSD). Discard PID if start time differs.

### H3. `scout.pid` writes are non-atomic; corrupted file → session loss [DONE]

**Where:** `session_track.go:91` — `os.WriteFile(...path.../scout.pid, data, 0o644)`.

**Problem:** `os.WriteFile` truncates the existing file before writing. Crash between truncate and write yields a 0-byte file. On next startup, `ReadInfo` errors, `CleanStaleSessions` interprets "no scout.pid" as orphan, and **deletes the entire reusable session** — including cookies, auth tokens, and Chrome profile.

**Fix:** Write `scout.pid.tmp` in same directory → `f.Sync()` → `os.Rename` to `scout.pid` (atomic on same filesystem; Windows-safe). Same treatment for `job.json`.

### H4. Reusable-session takeover via predictable dir hash

**Where:** `session_track.go:474-490` — `Hash(rawURL, label) = sha256(domain + "\x00" + label)[:12]`.

**Problem:** Directory name is fully predictable from public inputs (URL + browser name). Local-coresident attacker pre-creates `~/.scout/sessions/<predicted>/data/` with a malicious Chrome profile (poisoned cookies, evil extensions). When the user next runs scout, `FindByDomain` reads the attacker-supplied `scout.pid`, the launcher reuses the poisoned profile, and any auth performed in that session leaks to the planted extension.

**Fix:** On session creation, write a random 32-byte `nonce` to `scout.pid`. On reuse, verify the binary's recorded `Exec` matches current `os.Executable()` (catches "different binary planted the dir"). Reject sessions whose dir mode is not 0o700 owned by current uid.

## SEV-MEDIUM

### M1. World-readable session dir + permissive mode

`session_track.go:82` — `os.MkdirAll(dir, 0o755)`. Session `data/` contains Chrome profile with cookies and OAuth tokens (scraper auth modes encrypt-on-write but Chrome's own cookie SQLite is plaintext under the profile). On multi-user systems, other UIDs can read it.

**Fix:** Create `SessionsDir` parent at `0o700`, each `<id>/` at `0o700`, write `scout.pid` and `job.json` at `0o600`. Explicit `os.Chmod` after `MkdirAll` (umask-independent).

### M2. `SessionsDir` falls back to `/tmp` when `$HOME` unset

`session_track.go:30` — `filepath.Join(os.TempDir(), "scout", "sessions")`. World-readable, shared, often surviving across logins. Auth tokens leak.

**Fix:** Fail closed: if `UserHomeDir` returns error, refuse to start and require explicit `SCOUT_HOME` or `--data-dir`.

### M3. Concurrent-process race on reusable sessions (no lock)

Two scout invocations targeting the same `(domain, browser)` both pass `FindByDomain` → both `WriteInfo` → both think they own the Browser. `BrowserPID` and `ScoutPID` flip-flop.

**Fix:** Hold an `flock` (Unix) / `LockFileEx` (Windows) on `<id>/.lock` across the read-claim-write sequence in `registerSession`. Drop on `Close`.

### M4. `RootDomain` hardcoded two-part TLD map is incomplete

`session_track.go:431-437` — misses `gov.uk`, `ac.uk`, `co.il`, `co.za` extensions, ccTLD additions added after 2018. Wrong root → wrong dir hash → cookies from `school.ac.uk` land in the same bucket as `bank.ac.uk`.

**Fix:** Use `golang.org/x/net/publicsuffix` (PSL). Already vendored transitively.

### M5. Unverified, silently-skipped read in `List`

`session_track.go:136-139` — `ReadInfo` error → `continue` with no log. Distinguishes "no metadata file" from "corrupted metadata" by ignoring both. Hides H3's failure mode.

**Fix:** Log corrupted (size > 0 but unparseable) cases via slog warn; treat differently from missing-file in `CleanStaleSessions` (move to `<id>/scout.pid.corrupt` instead of removing the whole dir).

### M6. Retry-loop swallows final error

`session_track.go:225-230`, `:258-263`, `:357-366` — last `os.RemoveAll` error after retry budget is discarded. No metric, no log, no return value escalation. Cleanup failures are invisible.

**Fix:** Capture last err, return it (or log + counter via `internal/metrics`). Watchdog should escalate after N consecutive failures.

## SEV-LOW

### L1. `removeRetryWait` is unconditional 500 ms × 5 = up to 2.5 s

`session_track.go:225` — blocks `Close()` path in MCP/agent shutdown. Better: exponential backoff (50/100/200/400/800 ms) and accept eventual cleanup via the startup `CleanStaleSessions` pass.

### L2. `Reset()` always sleeps 500 ms even if browser already dead

`session_track.go:251` — make conditional on `Kill()` actually firing.

### L3. `ResetAll()` swallows per-session errors

`session_track.go:290-292` — operator sees "removed N" but not which failed. Return error list or log per-session.

### L4. `DomainHash` truncated to 12 bytes

`session_track.go:467` — birthday-bound 2^48. Honest use is fine; with the H4 fix this becomes moot, but consider 16 bytes (32 hex) for safety margin.

### L5. `registerSession` derives sessionID from launcher's `UserDataDir` basename

`browser.go:759` — `filepath.Base(filepath.Dir(dataDir))`. Coupling is fragile. Any caller that overrides `--user-data-dir` silently breaks session tracking. Plumb the `b.sessionID` set in `New()` through to `registerSession` instead.

### L6. `StartOrphanWatchdog` has no panic recovery

`session_track.go:383` — bare goroutine, panic in `CleanOrphans` (e.g. nil deref in gops on a goprocess race) kills the watchdog silently. Wrap in `defer recover() { slog.Error(...) }` and respawn.

## Suggested Implementation Order

1. **H3** (atomic write) — single-file change, prevents catastrophic data loss. Quick.
2. **H1** (exact-match `IsScoutProcess`) — wrong heuristic across the entire orphan story. Quick.
3. **M1 + M2** (file modes + `$HOME` fallback) — security boundary, lands cheaply.
4. **H2** (start-time verification) — larger; adds `BrowserStartTime` field, per-OS lookup. Medium.
5. **M3** (lock on reusable claim) — adds `flock`/`LockFileEx`. Medium.
6. **H4** (nonce + exec verification) — depends on H1 + M3 landing first. Medium.
7. **M4** (publicsuffix) — drop the manual TLD map. Quick.
8. Low-severity bundle (L1–L6) as cleanup.

## Files Touched

- `internal/engine/session/session_track.go` — atomic writes, locking, exec/start verification, publicsuffix, retry/backoff, error escalation
- `internal/engine/session/process_unix.go`, `process_windows.go` — exact-match `IsScoutProcess`, add `ProcessStartTime(pid)`
- `internal/engine/browser.go` — plumb `sessionID` into `registerSession`, drop basename derivation
- `internal/engine/session/job.go` — same atomic-write treatment as `scout.pid`

## Test Additions

- `TestScoutPidCrashRecovery` — kill mid-write, restart, verify session preserved
- `TestIsScoutProcessFalsePositive` — name a binary `boyscout` in a tempdir, run it, ensure `IsScoutProcess` returns false for that PID
- `TestPIDReuseRejection` — record BrowserPID + start time, kill, spawn new sleep with reused PID, verify `Reset` does not kill it
- `TestSessionDirPredictableTakeover` — pre-create `~/.scout/sessions/<predicted>/data/` with poisoned profile and `scout.pid`, run scout against same domain, verify it refuses reuse (after H4 fix)
- `TestConcurrentRegisterRace` — `go run` two scout processes against same reusable domain, verify only one claims and the other waits/errors
