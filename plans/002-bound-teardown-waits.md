# Plan 002: Teardown and failed launches never hang or leak Chrome

> **Executor instructions**: Follow step by step; run each verification and confirm the expected
> result before continuing. Honor "STOP conditions". Update this plan's row in `plans/README.md`
> when done.
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- internal/engine/browser.go internal/engine/browser_rod.go internal/engine/lib/launcher/launcher.go`
> On any change to these files since `4ecf689`, compare "Current state" excerpts to live code; a
> mismatch is a STOP condition.

## Status

- **Priority**: P0
- **Effort**: M
- **Risk**: MED
- **Depends on**: none
- **Category**: bug (correctness)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

Closes findings [0]/[3]/[60] (`Browser.Close` issues an unbounded CDP `Browser.close` before it
ever reaches `Kill`), [35]/[68] (`launcher.Cleanup` starts with a bare `<-l.exit` that blocks
forever), [2]/[70] (`launcher.Kill` sleeps an unconditional 1s before even checking `pid==0`), and
[1] (`New()` leaks the spawned Chrome when construction fails after launch).

Every scout command that used a local browser calls `Browser.Close()` at the end. Close currently
does, in order: close the CDP connection (step 5) **then** kill the OS process (step 7). The CDP
close is a network round-trip to Chrome with **no deadline**. If Chrome is alive but not answering
CDP — a suspended/half-crashed renderer, a GPU hang, AV interference, the documented Windows
environment — Close blocks forever: the command finishes its actual work and then **hangs at exit**,
and the Chrome process is never killed because `Kill()` is step 7 and we never got there. This is a
direct match for "hangs" reports that happen *after* the useful work is done. `launcher.Cleanup`
adds a second unbounded wait (`<-l.exit`), and `Kill` adds a gratuitous 1s to every teardown. And
when `New()` itself fails after Chrome spawned, the process leaks for the caller's lifetime.

After this plan: teardown always makes forward progress under a hard deadline (bounded CDP close →
kill → bounded cleanup), and a failed launch kills what it spawned.

## Current state

- `internal/engine/browser_rod.go` — the CDP-level close, untimed:
  ```go
  // browser_rod.go:174-181
  func (b *rodBrowser) Close() error {
      if b.BrowserContextID == "" {
          return proto2.BrowserClose{}.Call(b)        // ← blocks on b.ctx (no deadline)
      }
      return proto2.TargetDisposeBrowserContext{BrowserContextID: b.BrowserContextID}.Call(b)
  }
  ```
  `.Call(b)` sends a CDP command over the websocket and waits on `b`'s context; `b.ctx` has no
  deadline, so a wedged Chrome never returns.
- `internal/engine/browser.go` — `Close()` ordering; CDP close (step 5) precedes `Kill` (step 7):
  ```go
  // browser.go:731-735   (step 5 — CDP close, no timeout)
  if b.browser != nil {
      closeErr = b.browser.Close()
      b.browser = nil
  }
  // browser.go:761-771   (step 7 — kill + cleanup, only reached if step 5 returned)
  if b.launcher != nil {
      b.launcher.Kill()
      if !b.opts.reusableSession && b.sessionID != "" {
          b.launcher.Cleanup()
      }
      b.launcher = nil
  }
  ```
- `internal/engine/lib/launcher/launcher.go` — `Kill` and `Cleanup`:
  ```go
  // launcher.go:592-608
  func (l *Launcher) Kill() {
      utils2.Sleep(1)            // ← unconditional 1s, BEFORE the pid==0 guard
      if l.PID() == 0 { return }
      killGroup(l.PID())
      p, err := os.FindProcess(l.PID())
      if err == nil { _ = p.Kill() }
  }
  // launcher.go:610-622
  func (l *Launcher) Cleanup() {
      <-l.exit                  // ← unbounded; only closes when cmd.Wait() returns
      dir := l.Get(flags.UserDataDir)
      for range 3 {
          if err := os.RemoveAll(dir); err == nil { return }
          time.Sleep(500 * time.Millisecond)
      }
  }
  ```
- `internal/engine/browser.go` — `New()` spawn→construct leak:
  ```go
  // browser.go:152-165
  u, l, err = launchLocal(o)          // Chrome is now RUNNING; l owns it
  if err != nil { return nil, err }
  b := newRodBrowser().ControlURL(u)
  ...
  if err := b.Connect(); err != nil {
      return nil, fmt.Errorf("scout: connect browser: %w", err)   // ← leaks l's Chrome
  }
  ```
  Any error between `launchLocal` success and the final `b.register()` returns without `l.Kill()`.

Conventions to match:
- CDP calls take their deadline from the object's context; the codebase already uses
  `context.WithTimeout(context.Background(), N*time.Second)` for the ADB cleanup at
  `browser.go:741`. Use the same idiom.
- Error prefix `scout: <subsystem>: %w`. `Close()` is nil-safe/idempotent via `closeOnce` — keep it.
- `utils2` is the internalized `internal/engine/lib/utils` alias.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Build | `go build ./cmd/scout/ && go build ./pkg/...` | exit 0 |
| Vet | `go vet ./internal/engine/... ` | exit 0 |
| Tests | `go test ./internal/engine/ ./internal/engine/lib/launcher/` | pass (browser tests may `t.Skip`) |
| Race | `go test -race ./internal/engine/` | pass |
| Lint | `task lint` | exit 0 |

## Scope

**In scope**:
- `internal/engine/browser_rod.go` — bound the CDP close with a deadline (Step 1).
- `internal/engine/browser.go` — ensure `Kill` runs even if CDP close fails/times out (Step 2);
  fix the `New()` launch-failure leak (Step 4).
- `internal/engine/lib/launcher/launcher.go` — bound `Cleanup`'s `<-l.exit` (Step 3); fix `Kill`'s
  sleep ordering (Step 3).
- Tests alongside each.

**Out of scope**:
- The reaper (`session/`) — plan 006.
- The MCP `reset()` path — plan 005 (it will benefit from this automatically via `Browser.Close`).
- Changing the *reusable*-session preservation behavior in step 6/7 of `Close`.

## Git workflow

- Branch: `advisor/002-bound-teardown`
- Conventional commits, e.g. `fix(browser): bound CDP close and always kill on teardown`.
- No push/PR unless asked.

## Steps

### Step 1: Bound the CDP close with a deadline

Add a bounded variant used by teardown. In `browser_rod.go`, give `rodBrowser` a close that
accepts a context, or add a helper that clones the browser onto a timeout context before `.Call`.
Simplest: add
```go
func (b *rodBrowser) CloseWithTimeout(d time.Duration) error {
    ctx, cancel := context.WithTimeout(b.ctx, d)
    defer cancel()
    bb := b.Context(ctx)   // rodBrowser has a Context(ctx) clone helper mirroring rodPage.Context
    if bb.BrowserContextID == "" {
        return proto2.BrowserClose{}.Call(bb)
    }
    return proto2.TargetDisposeBrowserContext{BrowserContextID: bb.BrowserContextID}.Call(bb)
}
```
If `rodBrowser` has no `Context(ctx)` clone helper, check how `rodPage.Context` (`context.go:60`)
is implemented and add the equivalent on `rodBrowser` (a shallow copy with a new `ctx`). Keep the
original `Close()` for callers that still want the unbounded behavior, or have `Close()` delegate to
`CloseWithTimeout(5 * time.Second)`.

**STOP** if `rodBrowser` shares mutable state that a shallow ctx-clone would corrupt — report the
struct shape instead of guessing.

**Verify**: `go build ./internal/engine/` → exit 0.

### Step 2: Make `Browser.Close` always reach `Kill`

In `browser.go` `Close()`, step 5 must not be able to block teardown. Change:
```go
// 5. Close CDP connection (bounded — a wedged Chrome must not hang teardown).
if b.browser != nil {
    closeErr = b.browser.CloseWithTimeout(5 * time.Second)
    b.browser = nil
}
```
The subsequent `Kill` (step 7) already runs unconditionally after step 5 returns; with step 5 now
bounded, a wedged Chrome falls through to `Kill` within 5s instead of hanging forever. Do **not**
reorder steps 5 and 7 (CDP close first is correct for a *healthy* browser — it lets Chrome release
the data-dir lock before the process dies); the fix is bounding step 5, not moving it.

**Verify**: `go build ./internal/engine/` and `go vet ./internal/engine/` → exit 0.

### Step 3: Fix `launcher.Kill` sleep ordering and bound `launcher.Cleanup`

In `launcher.go`:
- Move the `pid==0` guard **before** the sleep, and drop the sleep on the kill path (its stated
  purpose — "let child processes initialize" — belongs at *launch*, not when we are about to kill
  the whole tree). Target:
  ```go
  func (l *Launcher) Kill() {
      if l.PID() == 0 { // avoid killing the current process
          return
      }
      killGroup(l.PID())
      p, err := os.FindProcess(l.PID())
      if err == nil { _ = p.Kill() }
  }
  ```
  If you are not fully confident removing the sleep, at minimum move it after the `pid==0` guard so
  the no-op case is instant, and reduce it to `utils2.Sleep(0.1)`. State which you did in the commit.
- Bound the `<-l.exit` wait in `Cleanup`:
  ```go
  func (l *Launcher) Cleanup() {
      select {
      case <-l.exit:
      case <-time.After(5 * time.Second):
          // Chrome tree did not fully exit (taskkill failure / AV / surviving child
          // holding the stdio pipe). Proceed to best-effort dir removal anyway.
      }
      dir := l.Get(flags.UserDataDir)
      for range 3 {
          if err := os.RemoveAll(dir); err == nil { return }
          time.Sleep(500 * time.Millisecond)
      }
  }
  ```

**Verify**: `go build ./internal/engine/lib/launcher/` → exit 0.

### Step 4: Kill the spawned Chrome when `New()` fails after launch

In `browser.go` `New()`, ensure every error path between `launchLocal` success and the final
`register()` kills `l`. The cleanest form is a named-return `defer` guard immediately after
`launchLocal` succeeds:
```go
u, l, err = launchLocal(o)
if err != nil {
    return nil, err
}
launched := true
defer func() {
    if err != nil && launched && l != nil {   // err is the named return
        l.Kill()
    }
}()
```
Confirm `New`'s signature uses a named `err` return; if not, add one (`func New(...) (b *Browser, err error)`)
and audit that no early `return nil, someErr` shadows it. This makes `Connect()` failure (`:163`)
and any later setup failure clean up the process.

**STOP** if `New` does not use named returns and converting it would touch more than this function.

**Verify**: `go build ./internal/engine/` → exit 0.

## Test plan

- `internal/engine/lib/launcher/launcher_test.go` (add): assert `Kill()` on a launcher with
  `PID()==0` returns in <100ms (proves the sleep no longer gates the no-op path). Assert
  `Cleanup()` on a launcher whose `exit` channel never closes returns within ~6s (proves the bound).
  Use a launcher constructed without a real process, or `t.Skip` if that isn't possible without one.
- `internal/engine/browser_test.go`: with a real browser (`newTestBrowser`, skips if no Chromium),
  open + `Close()` and assert it returns promptly. A wedged-Chrome test is hard to automate; cover
  it in the manual proof below.
- `go test -race ./internal/engine/` must stay green (Step 2/4 touch `Close`/`New`).

## Done criteria

- [ ] `go build ./cmd/scout/ && go build ./pkg/...` exit 0.
- [ ] `grep -n "BrowserClose{}.Call" internal/engine/browser_rod.go` — the unbounded call is only
      inside `CloseWithTimeout` (or `Close` delegates to it); teardown uses the bounded path.
- [ ] `grep -n "utils2.Sleep(1)" internal/engine/lib/launcher/launcher.go` returns no match, OR the
      sleep now follows the `pid==0` guard (documented in the commit).
- [ ] `grep -n "case <-time.After" internal/engine/lib/launcher/launcher.go` matches (Cleanup bounded).
- [ ] `New()` has a `defer` that kills `l` on error after launch (visible in `browser.go`).
- [ ] `go test ./internal/engine/ ./internal/engine/lib/launcher/` passes; `go test -race ./internal/engine/` passes.
- [ ] `task lint` exit 0.
- [ ] `plans/README.md` row updated.

## STOP conditions

- Current-state excerpts don't match live code (drift).
- `rodBrowser` has no safe way to run a CDP call on a derived context (Step 1) — report the struct.
- Removing the `Kill` sleep breaks an existing launcher test that asserts child-process timing —
  keep the guarded-then-short-sleep variant and note it.
- Any step's verification fails twice after a reasonable fix attempt.

## Manual proof (required by project rule — tests stub these boundaries)

On the real Windows setup: open a headed session (`scout repl https://example.com`), then from
another terminal **suspend** the Chrome process (e.g. a debugger, or `Process Explorer → Suspend`).
Now exit the REPL. Before this plan: exit hangs indefinitely. After: exit returns within ~5s and
the (suspended) Chrome is killed. Confirm no orphaned `chrome.exe` under the session data dir
afterwards (`scout session list` / Task Manager).

## Maintenance notes

- The 5s close/cleanup deadlines are deliberately generous; if teardown feels slow on healthy
  exits, the healthy path returns as soon as Chrome answers — the deadline only bites when wedged.
- If plan 005 (MCP) or a daemon later calls `Browser.Close` from a timer goroutine, the bounded
  behavior here is what makes that safe; keep them bounded.
- Reviewer: scrutinize that Step 2 did not accidentally skip the reusable-session branch in step 6
  of `Close` (that branch must still run for reusable sessions).
