# Plan 007: Stop shipping features that silently do nothing (fix them or fail loudly)

> **Executor instructions**: Follow step by step; verify each; honor STOP conditions; update
> `plans/README.md` when done. Each step is independent — you may land them separately.
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- internal/engine/healthcheck.go internal/engine/crawl.go internal/engine/wait_smart.go internal/engine/websocket.go internal/engine/recorder.go internal/engine/gather.go internal/engine/swarm/coordinator.go`
> Any change → compare excerpts (STOP on mismatch).

## Status

- **Priority**: P1
- **Effort**: L
- **Risk**: MED
- **Depends on**: none
- **Category**: bug (correctness)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

A cluster of advertised capabilities either return wrong/empty results while reporting success, or
stall as if hung. Silent-wrong is worse than an error — the user (and any AI driving Scout) trusts a
result that is fake. The two highest-value here are also *hangs*: `scout test-site` stalls ~30s per
page. Findings: [13]/[25]/[58] (health-check per-page `wait()` stall + dead `--timeout`), [11]
(smart-wait JS is a SyntaxError → whole feature is dead), [61]/[72] (`WithCrawlConcurrent` no-op),
plus the fail-loud sweep: [15]/[26] console capture dead in `gather`, [16] `NetworkRecorder.Stop`
no-op, [27]/[49] `SendWebSocketMessage`/`ws_connections` stubs, [85] `swarm --depth` no-op.

The principle: **every one of these must either work or return a clear error / stop advertising the
knob.** No silent success.

## Current state

```go
// internal/engine/healthcheck.go:55-58   --timeout is discarded
ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
defer cancel()
_ = ctx // used indirectly via timeout awareness      // ← never passed to Crawl

// healthcheck.go:78   per-page wait blocks until the page deadline
wait := rodPage.EachEvent(
    func(e *proto.RuntimeConsoleAPICalled) { ... },   // callbacks return nothing →
    func(e *proto.RuntimeException...) { ... },        //   EachEvent's wait() never returns true
)
// ... wait() then blocks until the page's absolute 30s ctx dies (~25s dead time per page)

// internal/engine/crawl.go:153-176   concurrency is a no-op (serial)
sem := make(chan struct{}, o.concurrent)
for len(queue) > 0 {
    ...
    sem <- struct{}{}                 // acquire
    page, err := b.NewPage(item.url)  // ← processed INLINE, same goroutine; no `go func()`
    ... <-sem                          // release

// internal/engine/wait_smart.go:186   generated JS is malformed
_, err = p.Eval(fmt.Sprintf(`(name) => %s(name)`, waitFrameworkReadyJS), fw.Name)
// waitFrameworkReadyJS is a function body/expression; wrapping it as `(name) => <expr>(name)`
// yields a SyntaxError, silently swallowed by the fallback at :188 → smart-wait never runs.
```
Other stubs (confirm each by reading): `internal/engine/websocket.go:180` `SendWebSocketMessage`
returns a "not supported" string but reports success; `pkg/scout/tools/websocket.go:124`
`ws_connections` reads `window.__scoutWSConnections` which is never set; `internal/engine/recorder.go`
`NetworkRecorder.Stop` closes a `stopCh` nothing consumes; `internal/engine/gather.go:165` console
capture discards the `EachEvent` wait func; `internal/engine/swarm/coordinator.go:148` enqueues all
URLs at `Depth: 1` ignoring `--depth`.

Conventions: `EachEvent(handlers...)` returns a `wait func()`; a handler that returns `true` stops
the pump. The idiomatic "listen for N seconds then stop" is to run the pump in a goroutine and
cancel via the page context / a timer, not to call `wait()` inline. Errors wrap `scout: <op>: %w`.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Build | `go build ./cmd/scout/ && go build ./pkg/...` | exit 0 |
| Tests | `go test ./internal/engine/ ./internal/engine/swarm/ ./pkg/scout/tools/` | pass |
| Lint | `task lint` | exit 0 |

## Scope

**In scope**: the files listed in Current state. Tests alongside.
**Out of scope**: implementing a full WebSocket send from scratch if it needs new CDP plumbing —
for that stub, the acceptable outcome is *fail loud* (return a clear "not supported" error), not a
new feature. Same for `ws_connections` if wiring `__scoutWSConnections` is non-trivial.

## Steps

### Step 1: Fix the health-check per-page stall and wire `--timeout`

- Thread the `ctx` (or `o.timeout`) into the crawl so `--timeout` bounds the run. Pass it to
  `b.Crawl(...)` / the page operations instead of `_ = ctx`.
- Stop calling `wait()` inline. Run the `EachEvent` pump in a goroutine and stop it when the page has
  finished loading (or after a short settle window), so each page contributes ~its load time, not
  ~30s. Pattern:
  ```go
  stop := rodPage.EachEvent(consoleCb, exceptionCb)   // returns a stop/wait func
  // navigate + WaitLoad already done by the crawler; give a short settle, then stop:
  go func() { stop() }()                               // pump runs in background
  // ... after collecting the page result, cancel the page ctx (or call the returned stopper) so
  //     the pump exits promptly instead of blocking to the 30s deadline.
  ```
  Verify the exact `EachEvent` contract in `browser_rod.go` (the pump loops until a callback returns
  true or the ctx ends) and choose the stop mechanism accordingly. **STOP** and report if there is
  no way to stop the pump other than the absolute page deadline.

**Verify**: `scout test-site` on a small local `httptest` site completes in ~(load time × pages),
not ~(30s × pages). Add a test using a fast local server asserting total time is well under
`pages × 5s`.

### Step 2: Fix the smart-wait JS wrapping (or remove the feature honestly)

Inspect `waitFrameworkReadyJS`. If it is a function *expression* (`function(name){...}` or
`(name)=>{...}`), the correct call is `p.Eval(waitFrameworkReadyJS, fw.Name)` directly (rod's `Eval`
already wraps a function and applies args). If it is a *statement body*, wrap it as
`function(name){ <body> }`. Produce valid JS and prove the wait actually runs (not the silent
fallback):
```go
_, err = p.Eval(waitFrameworkReadyJS, fw.Name)   // pass the function as-is; Eval supplies args
if err != nil { _ = p.WaitLoad(); return nil }
```
If making it correct is out of scope, then **remove** `WithSmartWait`/`WaitFrameworkReady` from the
advertised API and CLAUDE.md rather than shipping dead code. Prefer fixing.

**Verify**: add a test that a page exposing the framework global has `WaitFrameworkReady` return
without falling back (e.g. assert a side-effect the framework-specific JS produces). `t.Skip` w/o Chromium.

### Step 3: Make `WithCrawlConcurrent` real (or delete the flag)

Either spawn worker goroutines bounded by `sem` (real concurrency) or remove the `--concurrency`
flag and the `sem` so the docs don't lie. Preferred: real concurrency — process each dequeued item
in `go func(){ defer func(){<-sem}(); ... }()`, guard shared `queue`/`results`/`visited` with the
existing `mu`, and wait for in-flight workers before returning. **STOP** if the crawl's ordering or
`visited` dedup can't be made safe under concurrency without a larger refactor — in that case delete
the flag and document serial behavior, and file the concurrency work as a follow-up.

**Verify**: with `--concurrency 4` on a local multi-page site, assert (via timing or a counter of
concurrently-open pages) that more than one page is in flight. Or, if deleted: `grep -rn "concurrent"`
shows the flag is gone and `sem` removed.

### Step 4: Fail-loud sweep for the remaining stubs

For each of these, make it either work or return a clear error — no silent success:
- `SendWebSocketMessage` (`websocket.go:180`): if a real CDP send isn't wired, return
  `fmt.Errorf("scout: websocket send not supported; use page eval")` so callers see the failure.
- `ws_connections` (`pkg/scout/tools/websocket.go`): if `__scoutWSConnections` is never populated,
  either populate it via the WS interceptor or return an explicit "no WS monitoring active" result,
  not `[]`.
- `NetworkRecorder.Stop` (`recorder.go`): actually stop recording (consume `stopCh` in the record
  loop) so the documented contract holds.
- `gather` console capture (`gather.go:165`): don't discard the `EachEvent` wait func — run the pump
  like Step 1 so `--console` returns real entries.
- `swarm --depth` (`coordinator.go:148`): carry the parent depth (`Depth: parent.Depth+1`) and
  enforce `MaxDepth`, or remove `--depth` from the CLI. Preferred: enforce it.

Do these as separate commits. Each needs a test proving the new behavior (real result or explicit
error).

**Verify**: `go test ./internal/engine/ ./internal/engine/swarm/ ./pkg/scout/tools/` → pass.

## Test plan

- Health-check timing test (local server) — no per-page 30s stall.
- Smart-wait runs (not fallback).
- Crawl concurrency observable, or flag removed.
- Each Step-4 stub: real result or explicit error asserted.

## Done criteria

- [ ] `grep -n "_ = ctx // used indirectly" internal/engine/healthcheck.go` returns no match.
- [ ] `scout test-site` on a local N-page site finishes in ≪ `N×30s` (timed test asserts it).
- [ ] Smart-wait test proves the framework-specific JS executed (no silent fallback), or the feature
      is removed from the API + docs.
- [ ] `--concurrency` is either observably concurrent or removed (no dead `sem`).
- [ ] Each Step-4 stub returns a real result or an explicit error (tests assert it).
- [ ] `go build ./cmd/scout/ && go build ./pkg/...` exit 0; `task lint` exit 0.
- [ ] `plans/README.md` row updated.

## STOP conditions

- Excerpts drifted from `4ecf689`.
- `EachEvent` offers no stop mechanism besides the absolute page deadline (Step 1) — report.
- Crawl can't be made concurrency-safe without a larger refactor (Step 3) — delete the flag, file
  follow-up.
- Any stub in Step 4 needs substantial new CDP plumbing — choose fail-loud and file the real
  implementation as a follow-up rather than half-building it.

## Maintenance notes

- The root pattern is "a returned `wait`/`stop` func discarded, or a flag read into a var never
  enforced." When reviewing new features, grep for discarded `EachEvent` results and for option
  fields with no read site.
- Reviewer: the health-check timing test is the highest-value gate (it doubles as the hang fix).
