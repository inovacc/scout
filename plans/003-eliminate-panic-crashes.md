# Plan 003: A single bad event, race, or malformed CDP frame can no longer crash the whole process

> **Executor instructions**: Follow step by step; verify each step; honor STOP conditions; update
> `plans/README.md` when done.
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- internal/engine/hijack_session.go pkg/scout/plugin/mode_proxy.go internal/engine/lib/cdp/client.go internal/engine/utils.go internal/engine/session/reaper.go`
> Any change → compare excerpts to live code before proceeding (STOP on mismatch).

## Status

- **Priority**: P0
- **Effort**: M
- **Risk**: MED
- **Depends on**: none
- **Category**: bug (correctness)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

Closes the "closes unexpectedly" cluster — five independent ways a normal event crashes the entire
scout process (CLI, MCP server, scraper, flow capture):

- [12] `SessionHijacker.Stop` closes `h.events` while `emit` sends to it → **send on closed channel** panic.
- [53] plugin `mode_proxy` closes the result channel in one goroutine while a second still sends
  streamed results → **send on closed channel** panic.
- [48]/[79] CDP `consumeMessages` calls `utils.E(json.Unmarshal(...))`, which **panics** on any
  malformed frame (a websocket control/close frame, a protocol-drifted event from a newer Chrome)
  with **no `recover`** on the read goroutine → whole process dies.
- [5] the reaper watchdog's `recover()` sits at the top of the goroutine func, so after recovering
  it **returns** — one panic permanently disables orphan reaping (the comment claims the opposite).
- [89] `Browser.Close` nils `b.browser`/`b.launcher` with no lock shared with `NewPage`/`Done`,
  which read them check-then-act → a concurrent close can nil-deref panic.

A Go panic on any goroutine with no `recover` terminates the process. For a stdio MCP server that
means it vanishes mid-session; for the CLI it means an abrupt exit with a stack trace. These are
the literal "it closes" symptom.

## Current state

- `internal/engine/hijack_session.go`:
  ```go
  // :96-110  Stop closes both channels under h.mu
  func (h *SessionHijacker) Stop() {
      ...
      h.mu.Lock(); defer h.mu.Unlock()
      if !h.stopped {
          h.stopped = true
          close(h.stopCh)
          close(h.events)     // ← closed here
      }
  }
  // :112-123  emit checks stopCh WITHOUT h.mu, then sends to h.events
  func (h *SessionHijacker) emit(ev HijackEvent) {
      select { case <-h.stopCh: return; default: }
      select { case h.events <- ev:  // ← panics if Stop closed h.events after the check above
      default: }
  }
  ```
- `pkg/scout/plugin/mode_proxy.go:42-109`: goroutine A does `defer close(ch)` and returns when the
  scrape `Call` returns; goroutine B (`:77`) sends streamed `result` notifications to the **same**
  `ch` at `:92` (`case ch <- r:`) — if a notification lands after A closed `ch`, `:92` panics.
- `internal/engine/lib/cdp/client.go:132-157`: the read loop; `utils.E(err)` at `:151` and `:157`
  (`E` panics on non-nil error — see below) with no `recover` in `consumeMessages`:
  ```go
  err = json.Unmarshal(data, &id); utils.E(err)          // :150-151
  ...
  err := json.Unmarshal(data, &evt); utils.E(err)        // :156-157
  ```
  `utils.E` (`internal/engine/lib/utils/utils.go`) panics when passed a non-nil error. `utils.go:62`
  in the engine (`utils2.E(json.Unmarshal(msg.data, e))`) is the same pattern in event dispatch.
- `internal/engine/session/reaper.go:201-221`: the watchdog goroutine — `recover()` in a `defer`
  at the **top** of the goroutine func; after recover the func returns, so the `for` loop at `:213`
  never resumes. One panic in `ReapOnce` kills the watchdog for the process lifetime.

Conventions:
- The repo already uses buffered channels + `select{...; default:}` for best-effort emit (see
  `emit`). The fix pattern is a `recover()` around the send, or a done-guarded close, or `sync`
  coordination — match the least-invasive one per site.
- Error prefix `scout: <subsystem>: %w`.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Build | `go build ./cmd/scout/ && go build ./pkg/...` | exit 0 |
| Race | `go test -race ./internal/engine/ ./pkg/scout/plugin/ ./internal/engine/lib/cdp/` | pass |
| Tests | `go test ./internal/engine/ ./pkg/scout/plugin/ ./internal/engine/session/` | pass |
| Lint | `task lint` | exit 0 |

## Scope

**In scope**: `internal/engine/hijack_session.go`, `pkg/scout/plugin/mode_proxy.go`,
`internal/engine/lib/cdp/client.go`, `internal/engine/session/reaper.go`, and — only if Step 5 is
taken — the read/close synchronization in `internal/engine/browser.go` `Close`/`NewPage`/`Done`.
Add tests beside each.

**Out of scope**: changing `utils.E`'s panic contract globally (it is internalized-rod API used in
hundreds of places); the fix is a `recover` at the goroutine boundary, not editing `E`. The MCP
idle/reset race is plan 005.

## Steps

### Step 1: Fix the hijack `Stop`/`emit` send-on-closed race

Do **not** close `h.events` in `Stop`. A closed channel is not required to signal the consumer —
`stopCh` already does. Change `Stop` to close only `stopCh`, and make `emit` take `h.mu` (or an
`RLock`) for the duration of its send so it cannot race the `stopped` flag:
```go
func (h *SessionHijacker) Stop() {
    if h == nil { return }
    h.mu.Lock(); defer h.mu.Unlock()
    if !h.stopped {
        h.stopped = true
        close(h.stopCh)          // do NOT close h.events
    }
}
func (h *SessionHijacker) emit(ev HijackEvent) {
    h.mu.Lock(); defer h.mu.Unlock()
    if h.stopped { return }
    select { case h.events <- ev: default: } // drop if slow; never send after stop
}
```
Consumers already range over `Events()` until *their* context ends or Ctrl+C (see plan 005/008 for
the consumer side); they do not rely on `h.events` being closed. If a consumer *does* rely on close
to unblock a `range`, add a separate `done` signal instead of closing the shared channel — **STOP
and report** if you find such a consumer, since that changes the fix shape.

**Verify**: `go test -race ./internal/engine/ -run Hijack` → pass.

### Step 2: Fix the plugin `mode_proxy` streamed-result race

The two goroutines share `ch` with no coordination. Make a single owner close `ch`. Simplest: have
goroutine B (the notification drainer) be the sole writer-after-response, and use a `sync.Once` +
recover, or restructure so only one goroutine closes. Minimal robust fix — guard every send with a
recover-free `select` on a shared `done` channel and close `ch` exactly once after **both**
goroutines finish (use a `sync.WaitGroup`):
```go
var wg sync.WaitGroup
wg.Add(2)
go func() { defer wg.Done(); /* scrape Call → send results, NO close(ch) */ }()
go func() { defer wg.Done(); /* drain notifications until ctx.Done/closed, NO close(ch) */ }()
go func() { wg.Wait(); close(ch) }()   // sole closer
```
Ensure both worker goroutines stop on `ctx.Done()` so the closer isn't blocked. Keep the buffered
`ch` (cap 32).

**Verify**: `go test -race ./pkg/scout/plugin/` → pass.

### Step 3: Add `recover` to the CDP read goroutine and handle non-JSON frames

In `client.go` `consumeMessages`, a malformed frame must not crash the process. Two changes:
- Replace the panicking `utils.E(json.Unmarshal(...))` at `:151` and `:157` with explicit error
  handling that **skips the frame** and logs, rather than panicking:
  ```go
  if err := json.Unmarshal(data, &id); err != nil {
      cdp.logger.Println("cdp: skip unparseable frame:", err)
      continue
  }
  ```
  Same for the event unmarshal at `:156-157`.
- As defense in depth, wrap the goroutine body in a `recover` that logs and returns cleanly (which
  flushes pending calls with an error via the existing `:138-141` path) instead of crashing:
  ```go
  func (cdp *Client) consumeMessages() {
      defer func() {
          if r := recover(); r != nil {
              cdp.logger.Println("cdp: consumeMessages panic recovered:", r)
          }
          close(cdp.event)
      }()
      ...
  }
  ```
  Keep the existing `defer close(cdp.event)` behavior — fold it into the recover defer so `event`
  is still closed exactly once on exit.

**Verify**: `go test ./internal/engine/lib/cdp/` → pass. Add a test feeding a non-JSON `[]byte`
frame through a fake `ws.Read` and asserting no panic (model after existing cdp tests; `t.Skip` if
the transport can't be faked without a browser).

### Step 4: Make the reaper watchdog survive a panic

Move the `recover` so it guards **each iteration**, not the whole goroutine, so the loop continues:
```go
go func() {
    ticker := time.NewTicker(interval); defer ticker.Stop()
    for {
        select {
        case <-done: return
        case <-ticker.C:
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        slog.Error("scout: reaper watchdog iteration panic", "panic", r)
                    }
                }()
                _ = ReapOnce()
            }()
        }
    }
}()
```

**Verify**: `go test ./internal/engine/session/ -run Watchdog` → pass (add a test if none exists:
inject a panicking reap once and assert the ticker keeps firing — or if injection isn't wired,
assert the structure via a small refactor that takes the iteration func as a parameter).

### Step 5 (optional, only if race detector flags it): guard `Browser.Close` vs `NewPage`/`Done`

Run `go test -race ./internal/engine/`. If [89] reproduces (concurrent `Close` nil-deref), add a
mutex around the read of `b.browser`/`b.launcher` in `NewPage`/`Pages`/`Done` that is shared with
the write in `Close` (`closeOnce` alone does not synchronize *readers*). If the race detector does
**not** flag it under existing tests, leave a maintenance note and do not expand scope — record that
[89] needs a dedicated concurrent-close test to confirm.

## Test plan

- Hijack: race test that runs `emit` in a tight loop while `Stop` is called (no panic).
- Plugin: race test that streams a `result` notification concurrently with the scrape response
  returning (no panic); model after existing `pkg/scout/plugin/*_test.go`.
- CDP: feed a malformed frame; assert graceful skip, no process panic.
- Reaper: iteration panic does not stop the watchdog.
- All: `go test -race ./internal/engine/ ./pkg/scout/plugin/ ./internal/engine/lib/cdp/` green.

## Done criteria

- [ ] `grep -n "close(h.events)" internal/engine/hijack_session.go` returns no match.
- [ ] `grep -n "utils.E(" internal/engine/lib/cdp/client.go` returns no match inside `consumeMessages`
      (the unmarshal panics are gone); a `recover()` is present in that function.
- [ ] `mode_proxy.go` has a single closer of `ch` (no `defer close(ch)` inside a worker that shares
      `ch` with another sender).
- [ ] Reaper watchdog `recover` is inside the per-tick func, not the goroutine top.
- [ ] `go test -race ./internal/engine/ ./pkg/scout/plugin/ ./internal/engine/lib/cdp/` passes.
- [ ] `go build ./cmd/scout/ && go build ./pkg/...` exit 0; `task lint` exit 0.
- [ ] `plans/README.md` row updated.

## STOP conditions

- A hijack consumer relies on `h.events` being closed to unblock (Step 1) — report it.
- `utils.E` at `:151`/`:157` cannot be removed without breaking a caller that catches its panic
  (unlikely, but check) — report.
- Race detector reveals a broader `Browser` concurrency problem than [89] — report scope rather than
  expanding this plan.

## Maintenance notes

- The root cause across all five is "no `recover` at a goroutine boundary + shared-channel close
  races." Any new long-lived goroutine or new event channel should adopt the same
  single-closer / recover discipline. Consider a tiny `internal/safe.Go(fn)` helper that wraps a
  goroutine body in a logging `recover`, and route new goroutines through it.
- Reviewer: the highest-value assertion is the race tests — approve on those going green under
  `-race`, not on the diff reading plausibly.
