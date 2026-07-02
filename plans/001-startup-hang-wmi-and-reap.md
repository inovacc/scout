# Plan 001: Every `scout` command starts within a bounded time, even when Windows WMI or a wedged session dir is slow

> **Executor instructions**: Follow this plan step by step. Run every verification command and
> confirm the expected result before moving on. If anything in "STOP conditions" occurs, stop and
> report — do not improvise. When done, update this plan's status row in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- internal/engine/session/browser_scan_windows.go cmd/scout/scout.go internal/engine/session/reaper.go internal/engine/session/session_track.go`
> If any of those changed since this plan was written, compare the "Current state" excerpts to the
> live code before proceeding; on a mismatch, treat it as a STOP condition.

## Status

- **Priority**: P0
- **Effort**: M
- **Risk**: MED
- **Depends on**: none
- **Category**: bug (correctness + performance)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

Closes findings [4]/[30] (untimed PowerShell/WMI scan on the startup path) and [71]/[74]
(synchronous session reaping in `main()`, ~11s per wedged dir).

`main()` runs `session.CleanStaleSessions()` **synchronously before every command** — including
`scout version`. That call reaps stale session dirs; for each candidate it scans running browsers
via `FindBrowsersUsingDataDir`, which on Windows shells out to PowerShell
(`Get-CimInstance Win32_Process`) **with no timeout**. On a machine where the WMI/CIM service is
slow or hung (a well-known Windows failure mode, and exactly the AV-heavy environment the user is
on), this blocks *every* scout invocation indefinitely with no output — the worst-case version of
the reported "most of the time it hangs." Even when WMI is healthy, a couple of AV/indexer-wedged
session dirs add ~11s each of synchronous `RemoveAll` backoff before the command does any work.

After this plan: the WMI scan has a hard deadline and fails closed (returns no PIDs) instead of
hanging; and startup reaping no longer blocks the command's real work.

## Current state

- `cmd/scout/scout.go` — `main()`; runs cleanup synchronously before `Execute()`:
  ```go
  // cmd/scout/scout.go:200-213
  _, _ = session.CleanStaleSessions()          // :203  ← SYNCHRONOUS, on every command
  session.StartCleanupRetrier(nil)             // :208  ← background retrier (already async)
  scout.EnsureReaperWatchdog()                 // :213  ← background watchdog (already async)
  ```
- `internal/engine/session/browser_scan_windows.go` — the WMI scan with no timeout:
  ```go
  // browser_scan_windows.go:30
  out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
  ```
  `//go:build windows`. `findBrowsersWindows(dataDir)` is called by `FindBrowsersUsingDataDir`
  (`browser_scan.go:28`), which is called by `reapFolder` (`reaper.go:134`), which is called by
  `ReapOnce` (`reaper.go:81,101`), which is called by `CleanStaleSessions`
  (`session_track.go:431`). So the untimed subprocess sits on the synchronous startup path.
- `internal/engine/session/reaper.go` — `ReapOnce()` (`:49`) iterates every session dir and calls
  `reapFolder`; `reapFolder` (`:117`) calls `FindBrowsersUsingDataDir(DataDir(id))` at `:134`.
- The **background** retrier (`StartCleanupRetrier`) and watchdog (`StartReaperWatchdog`) already
  run off-thread; only the initial `CleanStaleSessions()` in `main()` is synchronous.

Repo conventions to match:
- Error prefix `scout: <subsystem>: %w` (see `browser.go`), though this scan returns `nil` on
  error rather than propagating — keep that fail-closed contract.
- `exec.CommandContext` is the standard way to bound a subprocess. Platform-specific code lives in
  `*_windows.go` (build tag `//go:build windows`) — the stub for other OSes is
  `browser_scan_other.go`.
- Tests skip when a real browser/OS feature is unavailable (`t.Skip`) — match that.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Build CLI | `go build ./cmd/scout/` | exit 0 |
| Build pkg | `go build ./pkg/...` | exit 0 |
| Vet | `go vet ./internal/engine/session/ ./cmd/scout/` | exit 0 |
| Unit tests | `go test ./internal/engine/session/ ./cmd/scout/` | pass (browser tests may `t.Skip`) |
| Lint | `task lint` (golangci-lint v2) | exit 0 |

## Scope

**In scope**:
- `internal/engine/session/browser_scan_windows.go` — add a context deadline to the WMI subprocess.
- `internal/engine/session/browser_scan.go` — if a timeout must be threaded through
  `FindBrowsersUsingDataDir`, do it here (prefer an internal default so the exported signature is
  unchanged — see Step 1).
- `cmd/scout/scout.go` — make the initial reap non-blocking (Step 2).
- New test file `internal/engine/session/browser_scan_windows_test.go` (create).

**Out of scope** (do NOT touch):
- `reaper.go` preserve/kill logic — that is plan 006. Only its call to `FindBrowsersUsingDataDir`
  is affected, and only transitively via the scan's own new timeout.
- `StartCleanupRetrier` / `StartReaperWatchdog` — already async; leave them.
- The Linux matcher (`findBrowsersLinux`) — `/proc` reads are already bounded; do not add a
  subprocess there.

## Git workflow

- Branch: `advisor/001-startup-hang`
- Conventional commits (repo uses them — see `git log`: `fix(okf): …`). Example:
  `fix(session): bound WMI browser scan with a 3s deadline`.
- Do NOT push or open a PR unless the operator asked.

## Steps

### Step 1: Give the Windows WMI scan a hard deadline

In `browser_scan_windows.go`, replace the untimed `exec.Command(...).Output()` with a
context-bounded call. Use a package-level default (e.g. `const browserScanTimeout = 3 * time.Second`)
so the exported `FindBrowsersUsingDataDir` signature does not change.

Target shape:
```go
ctx, cancel := context.WithTimeout(context.Background(), browserScanTimeout)
defer cancel()
out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
if err != nil {
    // Includes context.DeadlineExceeded: fail closed (no PIDs) rather than hang.
    return nil
}
```
Keep the existing `//go:build windows` tag and the rest of the parsing loop unchanged. Add the
`context`/`time` imports.

**Verify**: `go build ./internal/engine/session/` (on Windows or with `GOOS=windows go build`) → exit 0.

### Step 2: Make the initial startup reap non-blocking

In `cmd/scout/scout.go`, the synchronous `_, _ = session.CleanStaleSessions()` at `:203` must not
gate the command. The background retrier (`:208`) and watchdog (`:213`) already handle ongoing
cleanup, so the initial pass can run in a goroutine. Change `:203` to:
```go
// Kick the first stale-session sweep off the critical path; the retrier and
// watchdog below own ongoing cleanup. A slow WMI/CIM service or a wedged dir
// must never delay the user's command (finding 071/074).
go func() { _, _ = session.CleanStaleSessions() }()
```
Leave `StartCleanupRetrier` and `EnsureReaperWatchdog` exactly as they are.

**Rationale for safety**: `CleanStaleSessions`/`ReapOnce` is documented idempotent and
concurrency-safe (`reaper.go:47-48`), and it only ever *reaps* dirs whose owner is dead — moving
it off the main goroutine changes timing, not correctness. The only behavioral change is that a
brand-new command no longer waits for the previous run's leftovers to be swept before starting;
they are swept concurrently and by the retrier.

**Verify**: `go build ./cmd/scout/` → exit 0; `go vet ./cmd/scout/` → exit 0.

### Step 3: Regression test for the scan timeout (Windows-only)

Create `internal/engine/session/browser_scan_windows_test.go` (`//go:build windows`). Assert that
`findBrowsersWindows` returns within a bounded time even if PowerShell is slow. Because you cannot
easily stub `exec`, test the contract you can: that `findBrowsersWindows("")` returns `nil`
immediately (existing guard) and that a call with a real dataDir returns within, say,
`browserScanTimeout + 2s`. Use `t.Skip` if `powershell` is not on `PATH`.

Pattern to follow: `internal/engine/session/reaper_acceptance_test.go` (skip-if-unavailable style).

Target assertion shape:
```go
done := make(chan []int, 1)
go func() { done <- findBrowsersWindows(t.TempDir()) }()
select {
case <-done: // ok
case <-time.After(browserScanTimeout + 2*time.Second):
    t.Fatal("findBrowsersWindows exceeded its deadline — scan is not bounded")
}
```

**Verify**: `go test ./internal/engine/session/ -run WindowsScan` → pass or skip.

## Test plan

- New: `browser_scan_windows_test.go` — asserts the scan is bounded (the core regression).
- Existing `internal/engine/session/` tests must still pass (`go test ./internal/engine/session/`).
- Manual proof (Windows, required by project rule): with the fix, `scout version` must return in
  well under a second even if you artificially slow WMI. Simplest check: time `scout version`
  before and after — it should not spend seconds in a WMI subprocess. `Measure-Command { scout version }`.

## Done criteria

- [ ] `go build ./cmd/scout/` and `go build ./pkg/...` exit 0.
- [ ] `GOOS=windows go build ./internal/engine/session/` exits 0 (the Windows file compiles).
- [ ] `grep -n "exec.Command(" internal/engine/session/browser_scan_windows.go` returns **no**
      match (it must be `exec.CommandContext`).
- [ ] `grep -n "go func() { _, _ = session.CleanStaleSessions" cmd/scout/scout.go` matches (reap is async).
- [ ] `go test ./internal/engine/session/ ./cmd/scout/` passes (skips allowed).
- [ ] `task lint` exits 0.
- [ ] `plans/README.md` status row updated.

## STOP conditions

Stop and report if:
- The "Current state" excerpts don't match live code (drift since `4ecf689`).
- `CleanStaleSessions` turns out to have a caller that relies on it completing before `Execute()`
  (search `grep -rn "CleanStaleSessions" cmd/ internal/ pkg/`). If a command depends on the
  synchronous sweep having finished, do NOT make it async — report instead.
- Making the reap async introduces a data race flagged by `go test -race ./internal/engine/session/`.

## Maintenance notes

- If a future change makes `FindBrowsersUsingDataDir` callable from a request path (not just
  startup), thread a real caller `context` through instead of the package-default timeout.
- Reviewer should confirm the 3s default is comfortably above a healthy WMI response (~tens of ms)
  but well below "user thinks it hung."
- The macOS/BSD `FindBrowsersUsingDataDir` returns `nil` today (documented in `docs/BACKLOG.md`);
  this plan doesn't change that. Plan 006 revisits reaper coverage.
