# Plan 005: The stdio MCP server survives idle, bad selectors, Chrome crashes, and client disconnects

> **Executor instructions**: Larger plan — read fully. Verify each step; honor STOP conditions;
> update `plans/README.md` when done. Do plans 002 and 004 first (this reuses bounded `Close` and
> per-operation timeouts).
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- pkg/scout/mcp/server.go pkg/scout/mcp/tools_browser.go pkg/scout/mcp/tools_session.go cmd/scout/scout.go cmd/scout/mcp.go internal/idle/`
> Any change → compare excerpts to live code (STOP on mismatch).

## Status

- **Priority**: P1
- **Effort**: L
- **Risk**: MED
- **Depends on**: plans/002 (bounded Close), plans/004 (per-op timeout)
- **Category**: bug (correctness + dx)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

The stdio MCP server is a **long-lived daemon bolted onto a per-command engine**, and it inherits
every lifecycle gap. This plan closes the prior `docs/RELIABILITY-ANALYSIS.md` items (re-confirmed
still-present) plus new ones:

- **P0-1 + [8] idle-suicide, now also mid-flight**: the idle timer's callback cancels the server
  context (`server.go:249-256` → the `cancel` passed by `Serve` at `:374`), so after 5 minutes idle
  the **process exits** and Claude Code cannot reach it again. Worse, `touch()` only fires at call
  *start* (`ensureBrowser`, `:54`), so a single long tool call (crawl/test_site) that outlasts the
  timeout has its browser reset **mid-operation**.
- **[45]/[47] bad selector hangs the whole server**: `ensureBrowser` passes `scout.WithTimeout(0)`
  (`:65`), disabling per-op timeouts; a `click`/`wait` on a selector that never appears blocks the
  handler forever, and because the stdio transport processes calls sequentially, the entire server
  goes silent — indistinguishable from a crash.
- **[9] no teardown on disconnect**: `Serve` (`:359-377`) has no `defer state.reset()`; when the
  client disconnects (stdin EOF) and `Run` returns, the headless Chrome and session dir are orphaned.
- **P1-1 no liveness / [33] unactionable errors**: `ensureBrowser` returns the cached `s.browser`
  whenever non-nil (`:58-60`) with no health probe — after Chrome dies, every call errors forever
  with a bare `EOF`; the AI client can't tell "browser died, reset" from "bad selector, retry".
- **[10] `open` tool leaks a headed browser**: `tools_session.go` `open` calls `scout.New` for a
  second, headed browser that is never tracked on `mcpState` and never closed on idle/reset.

Net effect = the exact user report: "use it once, then it hangs or Claude can't reach it again."

## Current state

```go
// pkg/scout/mcp/server.go:53-92  ensureBrowser
func (s *mcpState) ensureBrowser(_ context.Context) (*scout.Browser, error) {
    s.touch()                       // resets idle timer — only at call START
    s.mu.Lock(); defer s.mu.Unlock()
    if s.browser != nil { return s.browser, nil }   // ← no liveness check (P1-1)
    opts := []scout.Option{
        scout.WithHeadless(s.config.Headless),
        scout.WithNoSandbox(),
        scout.WithTimeout(0),       // ← disables per-op timeout (feeds [45]/[47])
    }
    ...
    b, err := scout.New(opts...) //nolint:contextcheck   // ← context discarded (P0-2 path)
    ...
}

// server.go:247-257  idle callback cancels the server ctx
state.idle = idle.New(cfg.IdleTimeout, func() {
    ...
    state.reset()
    cb()                            // ← cb == Serve's cancel → process exits (P0-1)
})

// server.go:359-377  Serve — no defer teardown
func Serve(ctx context.Context, ...) error {
    ctx, cancel := context.WithCancel(ctx); defer cancel()
    ...
    server := NewServer(cfg, cancel)
    return server.Run(ctx, &mcp.StdioTransport{})   // ← on return, browser leaks ([9])
}
```
- `cmd/scout/scout.go:77` sets the default `--idle-timeout` to `5 * time.Minute`; `cmd/scout/mcp.go`
  passes it into `Serve`.
- `internal/idle/` is the timer (`idle.New(d, cb)`, `Reset()`).

Conventions:
- Tool handlers return errors as `mcp.TextContent` with `result.IsError = true` (see
  `tools_browser.go`). Match that for the new actionable messages.
- `state.reset()` (`server.go:127`) already closes page then browser — after plan 002 its
  `Browser.Close` is bounded.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Build | `go build ./cmd/scout/ && go build ./pkg/...` | exit 0 |
| Tests | `go test ./pkg/scout/mcp/` | pass |
| Race | `go test -race ./pkg/scout/mcp/` | pass |
| Lint | `task lint` | exit 0 |

## Scope

**In scope**: `pkg/scout/mcp/server.go`, `pkg/scout/mcp/tools_session.go` (the `open` leak),
`cmd/scout/scout.go` + `cmd/scout/mcp.go` (idle-timeout default for the stdio path), tests.

**Out of scope**: the engine `Close`/timeout internals (plans 002/004); gRPC/agent servers (separate
daemon-hardening pass); the SSRF policy filter.

## Steps

### Step 1: The idle timer releases the browser but NEVER kills the transport

Change the idle callback so it only calls `state.reset()` (drop the browser + page) and does **not**
call `cb()`. The stdio server's lifetime is owned by the client (Claude Code); scout must not
self-terminate. Keep `cb`/`cancel` wired for genuine shutdown (context cancel from the parent), but
remove it from the idle path:
```go
state.idle = idle.New(cfg.IdleTimeout, func() {
    if cfg.Logger != nil { cfg.Logger.Info("idle: releasing browser; server stays up", "timeout", cfg.IdleTimeout) }
    state.reset()          // release Chrome + session dir; next tool call re-launches lazily
    // do NOT call cb() — the client owns this process's lifetime
})
```
Next tool call hits `ensureBrowser`, which (after Step 3) re-launches. This turns idle from "server
dies" into "browser is reclaimed, transparently re-created on demand".

**Verify**: `go build ./pkg/scout/mcp/` → exit 0.

### Step 2: Don't let idle fire mid-call; keep the timer alive across long calls

Add an in-flight counter so the idle timer cannot reset the browser while a tool call is running.
Wrap tool dispatch (the `addTracedTool` wrapper in `server.go`, which every tool goes through) to
`atomic.AddInt32(&s.inFlight, 1)` on entry / `-1` + `s.touch()` on exit, and have the idle callback
no-op when `inFlight > 0`:
```go
state.idle = idle.New(cfg.IdleTimeout, func() {
    if atomic.LoadInt32(&s.inFlight) > 0 { s.touch(); return } // busy: re-arm, don't reset
    ...reset...
})
```
`touch()` on call **exit** (not just entry) ensures a long call resets the clock when it finishes.

**Verify**: `go test -race ./pkg/scout/mcp/` → pass.

### Step 3: Liveness check + re-launch in `ensureBrowser`; give tools a real timeout

- Before returning the cached browser, probe liveness; if dead, drop and re-launch:
  ```go
  if s.browser != nil {
      if s.browserAlive() {   // cheap CDP/version ping or process check
          return s.browser, nil
      }
      _ = s.browser.Close()   // bounded via plan 002
      s.browser, s.page = nil, nil
  }
  ```
  Implement `browserAlive()` with the cheapest reliable signal available (e.g. a bounded
  `Browser.Version()`/CDP ping, or `launcher` process-alive). **STOP** and report if there is no
  non-destructive liveness signal on `scout.Browser` — that becomes a small engine addition to spec.
- Replace `scout.WithTimeout(0)` with a real per-operation timeout now that plan 004 makes it
  per-op: `scout.WithTimeout(60 * time.Second)` (or a configurable `--mcp-op-timeout`). A bad
  selector then fails the single tool call in 60s with a clear error instead of wedging the server.

**Verify**: `go build ./pkg/scout/mcp/` → exit 0; `grep -n "WithTimeout(0)" pkg/scout/mcp/server.go`
returns no match.

### Step 4: Tear down the browser when `Serve` returns

Add teardown so a client disconnect (stdin EOF) doesn't orphan Chrome:
```go
func Serve(ctx context.Context, ...) error {
    ...
    server := NewServer(cfg, cancel)
    defer state.reset()   // NewServer must expose its state, or return (server, state)
    return server.Run(ctx, &mcp.StdioTransport{})
}
```
`NewServer` currently returns only `*mcp.Server`; change it to also return `*mcpState` (or add an
unexported accessor) so `Serve` can `defer state.reset()`. Keep the exported `NewServer` signature
stable if other callers exist — `grep -rn "NewServer(" pkg/ cmd/` first; if external callers exist,
add `NewServerWithState` instead of changing the signature.

**Verify**: `go build ./pkg/scout/mcp/ ./cmd/scout/` → exit 0.

### Step 5: Actionable tool errors after a browser death

In the tool error path (`tools_browser.go` and wherever handlers build `errResult`), detect the
connection-lost sentinel (add one in plan 003's CDP error wrapping, or match `io.EOF`/`wsarecv`)
and return a message the AI can act on, e.g.:
`"scout: browser connection lost — the page/Chrome was closed or crashed. Call session_reset then retry."`
This lets the client self-recover instead of retrying the same doomed call.

**Verify**: `go test ./pkg/scout/mcp/` → pass; add a unit test asserting the friendly message is
produced for a simulated connection-lost error.

### Step 6: Track and close the `open` tool's headed browser

In `tools_session.go`, store the browser created by `open` on `mcpState` (e.g. `s.openBrowsers`)
and close them in `state.reset()` and on `Serve` teardown. If `open` is intentionally
fire-and-forget (headed window the user drives until they close it), at minimum register it with the
engine's live-browser cleanup so signal/idle teardown reaps it — **STOP and report** which semantic
is intended if unclear from the tool's doc comment.

**Verify**: `go build ./pkg/scout/mcp/` → exit 0.

### Step 7: Default the stdio idle-timeout conservatively

With Step 1 making idle non-fatal, the 5-minute default is now safe (it only reclaims the browser),
so it can stay — but document it. If you prefer belt-and-suspenders, set the default to a larger
value (e.g. 30m) for the stdio path in `cmd/scout/mcp.go` so short idle gaps don't churn the browser.
Do not set it to 0 (that would keep Chrome forever). Record the choice in the commit.

**Verify**: `go build ./cmd/scout/` → exit 0.

## Test plan

- `pkg/scout/mcp/server_test.go`: (a) idle callback with `inFlight>0` does not reset; (b) idle
  callback does not invoke `cb`; (c) `ensureBrowser` re-launches when the cached browser reports
  dead (inject a fake dead browser); (d) connection-lost error maps to the friendly message.
- Manual proof (required — the real failure modes are transport/process boundaries): run scout as
  the Claude Code MCP server; (1) exercise a tool, wait >idle-timeout, exercise again → still
  responds; (2) kill the headless `chrome.exe`, call a tool → it re-launches and responds; (3) send
  a `click` with a bogus selector → fails within the op-timeout with an actionable message, server
  stays up; (4) disconnect the client → no orphan `chrome.exe` remains.

## Done criteria

- [ ] `grep -n "cb()" pkg/scout/mcp/server.go` shows `cb` is **not** called from the idle callback.
- [ ] `grep -n "WithTimeout(0)" pkg/scout/mcp/server.go` returns no match.
- [ ] `ensureBrowser` has a liveness check + re-launch; `Serve` has `defer state.reset()` (or
      equivalent teardown).
- [ ] Idle callback no-ops while `inFlight > 0`.
- [ ] `open`'s browser is tracked and closed on reset/teardown (or explicitly registered for cleanup).
- [ ] `go test -race ./pkg/scout/mcp/` passes; `go build ./cmd/scout/ && go build ./pkg/...` exit 0.
- [ ] All four manual proofs pass on the real setup.
- [ ] `plans/README.md` row updated.

## STOP conditions

- Excerpts drifted from `4ecf689`.
- `scout.Browser` exposes no non-destructive liveness signal (Step 3) — report; it needs a small
  engine addition first.
- `NewServer` has external callers that make returning state a breaking change (Step 4) — use an
  additive `NewServerWithState`.
- The `open` tool's intended lifetime is ambiguous (Step 6) — report rather than guess.

## Maintenance notes

- This plan + plans 002/004 together are the fix for the top-line "use once then dead" report.
- The idle-releases-browser pattern here should become the template for the gRPC/agent/swarm servers
  (see README direction finding #1) — extract a shared `ServerSession` lifecycle rather than copying.
- Reviewer: the manual proofs are the real gate; unit tests here necessarily stub the transport.
