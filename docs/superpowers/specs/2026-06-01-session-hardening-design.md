# Session Hardening — Enforce Clean Sessions & Zombie-Instance Kill

- **Date:** 2026-06-01
- **Status:** Design (approved shape, pending spec review)
- **Branch:** `feat/aria-phase-a`
- **Core value (from CLAUDE.md):** *Sessions must be rock-solid: open cleanly, close cleanly, never leak processes, and never touch the user's browser without explicit permission.*

---

## 1. Problem

A parallel subsystem map (6 analysts over session, process-liveness, launcher, signal, daemon, and the untracked `browser_scan*.go`) confirmed the worst-case holes against the real tree:

- **`cmd/scout/scout.go` `main()` has zero signal handling** — `Ctrl-C` / `kill` / panic during any one-shot command (`scrape`, `navigate`, `record`) skips `Browser.Close()` → chrome + `scout.lock` leak until the next launch.
- **No Windows Job Object** — chrome's renderer/GPU/utility children are not guaranteed to die with scout; `taskkill /F /T` is best-effort and AV/permission-sensitive.
- **No `CTRL_CLOSE`/`CTRL_SHUTDOWN` handler** — console-close and `taskkill /F` bypass cleanup entirely (worst for the daemon).
- **`FindBrowsersUsingDataDir` is dead code** (`internal/engine/session/browser_scan.go`, re-export wrapper in `session_track.go`, no caller) — the *only* scan-by-`--user-data-dir` kill path is never invoked, so a corrupt/missing `scout.pid` leaves an **un-killable zombie browser**.
- **Cleanup retry budget too short off the startup path** (`Launcher.Cleanup` ~1.5 s; `Browser.Close`'s `RemoveAll` single-shot, no re-enqueue) vs Windows AV/OneDrive/Search-Indexer holding LevelDB/SQLite handles 5–15 s → leaked locked dirs.
- **Daemon crash leaks every in-flight gRPC session** — the session map is in-memory only with no reconciliation; `GracefulStop` never iterates it.
- Plus: `autofree.recycleBrowser` skips `Launcher.Cleanup`; per-browser watchdog fan-out; `StartCleanupRetrier(nil)` never stops; TOCTOU between verify and kill; macOS/BSD have no start-token.

## 2. The invariant (acceptance contract)

> **At rest (no live scout process), `<scouthome>\sessions\` is empty.** A session folder may exist **only** while a live, identity-verified scout process owns it. For every folder: the recorded parent (scout) PID must be alive+verified, and every scout-launched browser must have a live scout parent — **no orphaned chrome, no ownerless folders.**

`<scouthome>` resolves via `internal/engine/scouthome` (Windows default `%LOCALAPPDATA%\scout`). The verification harness checks exactly this (see §9).

## 3. Scope decisions (locked)

| # | Decision | Choice | Consequence |
|---|----------|--------|-------------|
| 1 | Crash-path bar | **Best-effort + next-startup GC** | No Job Object, no native `CTRL_CLOSE`. Keep a lightweight `SIGINT`/`SIGTERM` handler (best-effort tier). Bulletproof, aggressive next-startup + watchdog reaping is the guarantee. |
| 2 | Corrupt-pid zombies | **Scan + kill any holder** (no identity gate) | Revive `FindBrowsersUsingDataDir`; kill any process holding the data dir with **no** start-token/identity verification — **but path-bounded** (see reconciliation). |
| 3 | Daemon restart | **Reconcile + adopt/kill orphans** | On daemon start, diff in-memory map vs on-disk dirs vs live PIDs; deterministically kill+remove prior-instance orphans. |
| 4 | Stuck dirs | **Escalate + force-break** | After N retries, surface via `scout session list --pending`, then force-delete via lock-break/`rmdir`. |

### 3.1 Reconciliations (flagged to and accepted by user)

- **Signal handler retained under "best-effort."** `SIGINT`/`SIGTERM` are the signals we *can* catch; handling them is best-effort and does not constitute the OS-guaranteed tier (which needs Job Object + native `CTRL_CLOSE`, both out of scope).
- **`--user-data-dir` path floor retained despite "no path limit."** Aggressive = **no identity/start-token gate** (we kill even processes we cannot verify). The single safety floor kept is: **the process's `--user-data-dir` must resolve under `<scouthome>\sessions\`.** This is the one boundary separating "kill zombies" from "kill the user's real Chrome," and *never touch the user's browser* is a hard CLAUDE.md rule. Killing by process *name* alone is explicitly forbidden.

## 4. Out of scope (deferred, tracked in `docs/BACKLOG.md`)

- Windows **Job Object** (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`) — the only true OS guarantee against orphaned chrome children on `SIGKILL`/console-close.
- Native Windows **`CTRL_CLOSE_EVENT`/`CTRL_SHUTDOWN_EVENT`** handler.
- macOS/BSD **start-token** implementation (`ProcessStartToken`/`ProcessParentPID` currently empty → PID-reuse protection degrades to alive-only).

These are recorded as `DEPRECATION`/follow-up items; they are the natural "phase B" if the best-effort tier proves insufficient.

## 5. Architecture — one process-wide `sessions.Reaper`

Today cleanup is spread across three uncoordinated paths: `CleanStaleSessions()` (startup), per-browser `StartOrphanWatchdog()` → `CleanOrphans()` (every 2 min), and `StartCleanupRetrier(nil)` (every 60 s). The hardening **consolidates the reaping decision into one component** so behavior is identical wherever it runs.

```
sessions.Reaper
├── Reap(ctx) ([]ReapResult, error)      // one full pass over <scouthome>/sessions
│   └── for each folder:
│         classify(folder) -> Owned | Orphan | ReusableExpired | Stuck
│         if not Owned: killHolders(folder) ; removeDir(folder)  (retry -> force-break)
├── StartWatchdog(ctx, interval)         // single process-wide loop (replaces N per-browser watchdogs)
└── PendingStuck() []StuckDir            // backs `scout session list --pending`
```

- **Startup:** `main()` calls `Reaper.Reap(ctx)` once (replaces direct `CleanStaleSessions`), then `Reaper.StartWatchdog`.
- **Per-browser** watchdogs are removed; one process-wide watchdog scans all folders (gap #15).
- The retrier gains a real `done` channel closed on exit (gap #10); stuck dirs flow into `PendingStuck()`.

### 5.1 Ownership model (`classify`)

A folder is **Owned** iff **all** hold:
1. `scout.pid` parses (432-byte `SCT1` v1 record).
2. recorded `ScoutPID` is **alive** (`WaitForSingleObject`/`Signal(0)`) **and identity-verified** (`IsScoutProcess` basename + `ProcessStartToken`).
3. recorded `BrowserPID` is alive (best-effort; absence alone does not mark Owned).

Otherwise:
- **ReusableExpired** — reusable session whose `IsExpired()` trips → reap.
- **Orphan** — any non-owned, non-reusable, or owner-dead folder → reap.
- **Stuck** — reap attempted but dir removal blocked (locked) → escalate.

### 5.2 Reap action (Orphan / ReusableExpired)

1. If `scout.pid` readable: kill recorded `BrowserPID` (re-check liveness immediately before kill to shrink the TOCTOU window, gap #9).
2. **`killHolders(folder)`** — scan-and-kill *any* process whose `--user-data-dir` is under `folder` (revives `FindBrowsersUsingDataDir`), **no identity gate**, **path-bounded to `sessions\`**.
3. `removeDir(folder)` — retry with backoff; on persistent failure, `recordCleanupFailure()` (the non-startup `Browser.Close` path must also do this, gap #5) → retrier → **force-break** after N attempts/T duration.

## 6. Work items

1. **Reaper engine** (`internal/engine/session/reaper.go`, new) — consolidates `CleanStaleSessions`/`CleanOrphans`/watchdog into one path-bounded, aggressive pass. Single process-wide watchdog; retrier `done` channel. Existing exported entry points (`CleanStaleSessions`, `CleanOrphans`) become thin wrappers over `Reaper.Reap` for compatibility (deprecation note below).
2. **Wire scan-and-kill** — make `browser_scan.go` live: `killHolders` calls it; cross-platform (`browser_scan_windows.go` via CIM/WMI or toolhelp32 command-line scan; `browser_scan_other.go` via `/proc/<pid>/cmdline`). Bounded to `sessions\`. Delete the unused re-export wrapper in `session_track.go`. Kills gap #4.
3. **`main()` signal handler** (`cmd/scout/scout.go`) — `signal.Notify(SIGINT, SIGTERM)` → cancel root ctx → close active session + flush HAR → `os.Exit`. Best-effort tier (gap #1). Idempotent with existing per-command handlers.
4. **Daemon reconcile** (`grpc/server/server.go`, `cmd/scout/server.go`) — on start: `Reaper`-driven diff of in-memory map vs on-disk dirs vs live PIDs → kill+remove prior-instance orphans; wrap shutdown in panic-recovery that iterates the session map calling `DestroySession`; bound each `Close()` with a timeout; idle callback stops hijacker goroutines + flushes HAR (gaps #3, #6, #7).
5. **`autofree.recycleBrowser`** (`internal/engine/autofree.go`) — call `Launcher.Cleanup()` (wait-for-exit + remove user-data-dir) after `Kill()` (gap #8).
6. **Stuck-dir escalation** — `scout session list --pending` subcommand surfaces `Reaper.PendingStuck()`; force-break (lock-break/`rmdir /s /q`) after N retries/T duration (gaps #5, #11).
7. **Verification harness** — `scout session doctor` (productionized from `.scripts/session-doctor.ps1`) + Go integration test (§9).

## 7. Error handling

- Every kill is **best-effort + logged** (`slog`), never fatal; wrapped `scout: reaper: ...`.
- Kills are **path-bounded**: `killHolders` refuses any target whose `--user-data-dir` does not resolve under `<scouthome>\sessions\`.
- **TOCTOU:** re-check liveness immediately before each `Kill()`; identity-verify recorded PIDs (the metadata path), accept unverifiable holders only via the path-bounded scan.
- **Force-break** only after the retrier exhausts N attempts/T duration; logged at WARN with the dir path.
- Reaper passes are **idempotent** and safe to run concurrently from startup + watchdog (folder-level guarding via existing `scout.lock`).

## 8. Cross-platform notes

- **Windows** is the dominant risk surface: command-line scan via CIM (`Win32_Process`) or `CreateToolhelp32Snapshot`; removal tolerant of 5–15 s file locks via retrier + force-break; PID reuse is fast → keep the immediate-before-kill liveness re-check.
- **Unix:** `Setpgid` process groups already reap chrome children when `Kill()` is reached; scan via `/proc/<pid>/cmdline`. Zombie (defunct) scout entries are transient and reaped by init; classify treats a defunct PID as not-Owned.

## 9. Verification harness (the acceptance test)

**`scout session doctor`** (new subcommand; mirrors `.scripts/session-doctor.ps1` cross-platform):
- enumerates `sessions\` folders;
- enumerates live scout + scout-launched browser processes;
- asserts the §2 invariant and prints per-folder parent/child PID mapping;
- exit code ≠ 0 on any violation (orphan chrome, ownerless folder).

**Go integration test** (`internal/engine/session/reaper_kill_test.go`, real browser + httptest per project convention; `t.Skip` if no Chromium):
1. launch a non-reusable session; record scout PID + browser PID + folder.
2. simulate hard crash: kill the *scout* process with `SIGKILL`/`TerminateProcess` (no `Close()`), leaving chrome orphaned + folder present.
3. run `Reaper.Reap(ctx)`.
4. assert: orphaned browser PID is dead; folder removed (or, if locked, force-broken within T); doctor reports the invariant holds.
5. negative control: a folder owned by a *live* verified scout is **not** reaped; a non-scout chrome with a foreign `--user-data-dir` is **never** touched.

## 10. Backward-compat / deprecation

- `CleanStaleSessions`/`CleanOrphans` retained as exported thin wrappers over `Reaper.Reap` (no breaking signature change); mark per-browser `StartOrphanWatchdog` `// Deprecated:` in favor of the single process-wide watchdog, removal after 2026-07-15, tracked in `docs/BACKLOG.md`.
- The dead `FindBrowsersUsingDataDir` re-export wrapper in `session_track.go` is removed (it has no callers).

## 11. Risks & open items

- **Aggressive no-identity-gate kill** could, in a pathological case, kill a chrome that legitimately reuses a scout session `--user-data-dir` across a fast PID reuse — mitigated by the path floor + immediate liveness re-check; accepted per Q2.
- **Force-break** of a locked dir whose browser is genuinely still writing — mitigated by only force-breaking *after* `killHolders` and N retries; the holder should already be dead.
- Consolidating 3 cleanup paths risks regressing a subtle case (reusable-session preservation) — covered by negative-control tests.

## 12. File-change map (verify file:line at plan time)

| File | Change |
|------|--------|
| `internal/engine/session/reaper.go` (new) | Reaper engine, classify, killHolders, removeDir, watchdog, PendingStuck |
| `internal/engine/session/browser_scan*.go` | wire live; cross-platform cmdline scan bounded to `sessions\` |
| `internal/engine/session/session_track.go` | route `CleanStaleSessions`/`CleanOrphans` → Reaper; remove dead wrapper; `recordCleanupFailure` on `Close` path |
| `internal/engine/session/cleanup_retry.go` | `done` channel; force-break escalation; PendingStuck feed |
| `cmd/scout/scout.go` | `SIGINT`/`SIGTERM` handler; single watchdog start |
| `cmd/scout/server.go` | panic-recovered daemon shutdown iterating session map |
| `grpc/server/server.go` | startup reconcile; idle callback flush/stop; bounded Close |
| `internal/engine/autofree.go` | `recycleBrowser` calls `Launcher.Cleanup` |
| `cmd/scout/session.go` (or equivalent) | `scout session doctor`, `scout session list --pending` |
| `internal/engine/session/reaper_kill_test.go` (new) | crash→reap integration test + negative controls |

## 13. Success criteria

1. After `SIGKILL` of scout mid-session, the next `Reaper.Reap` (startup or watchdog) leaves **zero** orphaned chrome and **zero** ownerless folders.
2. `scout session doctor` exits 0 on a clean machine and ≠ 0 with a precise report when the invariant is violated.
3. A user's real Chrome (foreign `--user-data-dir`) is **never** killed by any reap path.
4. Windows file-locked dirs reach a terminal state (removed or force-broken + surfaced) — never silently leaked.
5. Daemon restart deterministically reaps prior-instance sessions.
