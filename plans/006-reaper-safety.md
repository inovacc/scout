# Plan 006: The session reaper never kills a live browser it doesn't own

> **Executor instructions**: The reaper is safety-critical — a wrong change either kills live
> browsers or leaks zombies. Steps 1–3 are low-risk and independently valuable; Step 4 is the
> structural fix and has its own STOP gate. Verify each step; update `plans/README.md` when done.
> Do plan 001 first (it moves reaping off the synchronous startup path).
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- internal/engine/session/reaper.go internal/engine/session/process.go internal/engine/session/owner_windows.go internal/engine/session/session_track.go`
> Any change → compare excerpts to live code (STOP on mismatch).

## Status

- **Priority**: P1
- **Effort**: L
- **Risk**: HIGH
- **Depends on**: plans/001
- **Category**: bug (correctness)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

The reaper (`ReapOnce`, run at startup and every 2 min by the watchdog, and by every `Browser.New`
indirectly) decides which session dirs are "orphans" to kill. It infers ownership by **negative
signals**, and each has a false-positive that kills a live browser or preserves a zombie:

- [39] **Torn `scout.pid` read**: `WriteInfo` opens `scout.pid` with `O_TRUNC` and writes in place
  (`session_track.go:176`). A concurrent reader (`ReadInfo` inside `ReapOnce`) can observe an
  empty/partial file, conclude "corrupt → orphan", and **reap a live session mid-registration**.
  The `scout.lock` the design claims guards this is not honored by the reader.
- [40]/[69] **Elevated-owner self-reap**: `validateDirOwner` passes only when the dir owner SID
  equals the process token's user SID (`owner_windows.go:56`). A `scout` run **as Administrator**
  creates dirs owned by `BUILTIN\Administrators`, which fails the check, so within ~2 min the reaper
  treats the live elevated session as foreign and kills it.
- [77] **Expired reusable adopted then killed**: `FindReusable` matches `Reusable && Browser &&
  Headless` but **not** `!IsExpired()` (`session_track.go:331`), so a launch can adopt an expired
  session whose PID a concurrent reaper is about to kill — the launching browser dies.
- [38]/[65]/[78] **Owner identity by exe-name / bare PID**: the preserve gate is
  `info.ScoutPID != 0 && IsScoutProcess(info.ScoutPID)` (`reaper.go:92`), and `IsScoutProcess` →
  `isScoutExec` requires the basename to be exactly `scout`/`scout.exe` (`process.go:54`). So any
  library embedder, any renamed build (`scout-dev.exe`, GoReleaser `scout_windows_amd64.exe`), or a
  `go test` binary has its **live** session reaped; and because the owner PID has no start-token
  (only the *browser* PID does), a reused PID that happens to be a scout process preserves a dead
  session's zombie indefinitely.

Steps 1–3 fix the cases that bite the real `scout` CLI on Windows (torn read, elevated, expired).
Step 4 fixes the embedder/PID-reuse class structurally.

## Current state

```go
// reaper.go:86-101  preserve gates + reap
if info.Reusable && !info.IsExpired() { continue }           // preserve reusable-in-window
if info.ScoutPID != 0 && IsScoutProcess(info.ScoutPID) { continue } // preserve "live scout owner"
reapFolder(id, info.BrowserPID, info.BrowserStartToken, &stats)

// process.go:41-55  exe-name gate (used by IsScoutProcess)
func isScoutExec(exec string) bool {
    ...
    return lower == "scout" || lower == "scout.exe"          // ← embedders/renamed builds fail
}

// owner_windows.go:56  ownership gate
return windows.EqualSid(dirOwner, tokenUser.User.Sid)         // ← Administrators-owned dir fails

// session_track.go:176  in-place truncating write
f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)  // ← torn reads

// session_track.go:323-337  FindReusable — no IsExpired() check
if info.Reusable && info.Browser == browser && info.Headless == headless { return &sessions[i] }
```

Conventions:
- The **browser** PID already uses a start-token: `reapFolder` receives `info.BrowserStartToken`
  and calls `verifyProcess(browserPID, startToken)` (`reaper.go:119`). Model the owner token on it.
- Platform files: `owner_windows.go` (`//go:build windows`) with a non-Windows counterpart.
- Atomic file write idiom in Go: write to `scout.pid.tmp` then `os.Rename` over `scout.pid`
  (rename is atomic on the same volume, incl. Windows with `MoveFileEx`/`ReplaceFile` semantics for
  same-dir renames — `os.Rename` already does the right thing here).

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Build | `go build ./cmd/scout/ && go build ./pkg/...` | exit 0 |
| Win build | `GOOS=windows go build ./internal/engine/session/` | exit 0 |
| Tests | `go test ./internal/engine/session/` | pass |
| Race | `go test -race ./internal/engine/session/` | pass |
| Lint | `task lint` | exit 0 |

## Scope

**In scope**: `internal/engine/session/session_track.go` (atomic write + FindReusable),
`internal/engine/session/owner_windows.go` (elevated owner), and for Step 4
`internal/engine/session/process.go` + `reaper.go` + the `SessionInfo` struct/marshal. Tests.

**Out of scope**: the browser-scan/timeout work (plan 001); changing the 2-min watchdog interval;
the macOS/BSD `FindBrowsersUsingDataDir` gap (tracked in `docs/BACKLOG.md`).

## Steps

### Step 1: Write `scout.pid` atomically (fixes torn-read reap)

In `session_track.go` `WriteInfo`, write to a temp file in the same dir and `os.Rename` it over
`scout.pid`, so a reader always sees either the old or the new complete file — never a truncated
one:
```go
tmp := path + ".tmp"
f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
// write marshalBinary(info), f.Sync(), f.Close()
if err := os.Rename(tmp, path); err != nil { return fmt.Errorf("scout: replace scout.pid: %w", err) }
```
Keep the `f.Sync()` before rename. Ensure the tmp file is cleaned up on the write-error path.

**Verify**: `go test ./internal/engine/session/ -run WriteInfo` → pass; add a test that writes
concurrently while reading in a loop and asserts `ReadInfo` never returns a corrupt/partial record
(it returns either the prior or the new one). `go test -race` clean.

### Step 2: Accept elevated-owned dirs (fixes Administrator self-reap)

In `owner_windows.go` `validateDirOwner`, also accept the dir when its owner is
`BUILTIN\Administrators` **and** the current process token is elevated / a member of Administrators
(i.e. the elevated process legitimately owns dirs it created). Use `windows` well-known SID
(`CreateWellKnownSid(WinBuiltinAdministratorsSid)`) and `token.IsElevated()` /
group-membership check:
```go
if windows.EqualSid(dirOwner, tokenUser.User.Sid) { return true }
if adminsSid, err := wellKnownAdminsSid(); err == nil && windows.EqualSid(dirOwner, adminsSid) {
    if isElevatedOrAdminMember(token) { return true }   // elevated process owns Administrators-owned dirs
}
return false
```
Implement `wellKnownAdminsSid()` and `isElevatedOrAdminMember(token)` with `golang.org/x/sys/windows`
(already a dependency). **STOP** if the elevation/membership APIs aren't available in the pinned
`x/sys` version — report so the dependency can be bumped deliberately.

**Verify**: `GOOS=windows go build ./internal/engine/session/` → exit 0. Manual (elevated shell):
`scout repl`, leave >2 min, confirm the session survives (before: reaped).

### Step 3: `FindReusable` must skip expired sessions

Add the missing predicate so a launch never adopts a session the reaper will kill:
```go
if info.Reusable && !info.IsExpired() && info.Browser == browser && info.Headless == headless {
    return &sessions[i]
}
```

**Verify**: `go test ./internal/engine/session/ -run Reusable` → pass; add a test that an expired
reusable session is not returned by `FindReusable`.

### Step 4 (structural — HIGH risk): bind owner identity with a start-token

Give the **owner** (scout) PID the same positive-identity treatment the browser PID already has, so
the preserve decision no longer depends on the exe name or bare PID liveness.

- Add `ScoutStartToken string` to `SessionInfo` (mirror `BrowserStartToken`); populate it at
  register from the same source `BrowserStartToken` uses (process start time / boot-id — read how
  `verifyProcess`/`BrowserStartToken` derives it and reuse that).
- Change the preserve gate to a positive check:
  ```go
  if info.ScoutPID != 0 && verifyProcess(info.ScoutPID, info.ScoutStartToken) { continue }
  ```
  `verifyProcess(pid, token)` already means "this exact process, not a PID-reuse imposter" for
  browsers; reusing it for the owner makes the check independent of the binary's *name*, fixing
  embedders/renamed builds ([38]/[65]) **and** PID reuse ([78]) at once.
- Keep `isScoutExec` only as a *secondary* fast-path if you wish, but the token must be the
  authority. This means the reaper preserves any live owner regardless of exe name — which is
  correct: an embedder's live session must not be reaped.

**STOP and report** before doing Step 4 if:
- `verifyProcess`/start-token derivation is not portable to the owner PID (e.g. it relies on
  gops-registered browser metadata that the scout owner doesn't have), or
- adding a field to `SessionInfo` breaks the fixed-width binary `scout.pid` record
  (`marshalBinary`) in a way that isn't backward-compatible with existing on-disk records — the
  432-byte layout has reserved space, but confirm before consuming it.
In either case, land Steps 1–3 (which stand alone) and report Step 4 as a follow-up with findings.

**Verify**: `go test ./internal/engine/session/` and `go test -race` → pass; add a test that a live
session owned by a **renamed** binary (or simulated by writing a valid token for the current PID)
is preserved by `ReapOnce`.

## Test plan

- Atomic write: concurrent write/read loop, no torn reads (`-race`).
- Elevated owner: unit-test `validateDirOwner` accepts an Administrators-owned temp dir when the
  helper reports elevated (may require faking the token check behind an interface — or `t.Skip`
  when not elevated).
- FindReusable: expired session excluded.
- Step 4: renamed-owner session preserved; PID-reuse imposter not preserved.
- The existing `reaper_acceptance_test.go` safety-floor tests (never-kill-self, path-bounded) must
  stay green — these are the guardrail that the reaper still only ever touches `sessions/`.

## Done criteria

- [ ] `grep -n "O_TRUNC" internal/engine/session/session_track.go` in `WriteInfo` is gone (atomic
      rename in use).
- [ ] `FindReusable` includes `!info.IsExpired()`.
- [ ] `validateDirOwner` accepts elevated-Administrators ownership (Windows).
- [ ] If Step 4 landed: preserve gate uses `verifyProcess(ScoutPID, ScoutStartToken)`; a renamed-
      binary live session is preserved by a `ReapOnce` test. If deferred: Steps 1–3 landed and Step 4
      is filed as a follow-up with the STOP reason.
- [ ] `go test ./internal/engine/session/` and `go test -race ./internal/engine/session/` pass,
      including the existing acceptance/safety-floor tests.
- [ ] `GOOS=windows go build ./internal/engine/session/` exit 0; `task lint` exit 0.
- [ ] `plans/README.md` row updated.

## STOP conditions

- Excerpts drifted from `4ecf689`.
- Any change makes an existing `reaper_acceptance_test.go` safety test fail — that test is the floor
  that guarantees the reaper never touches the user's real Chrome; a failure means the change is
  unsafe. Revert and report.
- Step 4's token derivation isn't portable to the owner PID, or breaks the `scout.pid` binary layout
  (see Step 4 STOP gate).

## Maintenance notes

- [94] (launcher's `processAlive` duplicates `session.ProcessAlive` with the documented Windows
  zombie bug) is a related, lower-value cleanup: once Step 4 centralizes identity, consider having
  the launcher reuse `session.ProcessAlive` instead of its own `GetExitCodeProcess==STILL_ACTIVE`
  copy. Not required for this plan.
- Reviewer: focus on the acceptance/safety-floor tests and the concurrent write/read test — those
  are what prove the reaper got safer, not just different.
- This plan is the tactical fix; README direction finding #2 (a first-class `ServerSession`
  ownership contract) is the strategic version that would also cover the daemon surfaces.
