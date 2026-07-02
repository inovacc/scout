# Plan 004: The default 30s "operation timeout" stops killing long-lived pages

> **Executor instructions**: This plan changes a core timeout semantic — read it fully before
> editing. Verify each step; honor STOP conditions; update `plans/README.md` when done.
>
> **Drift check (run first)**: `git diff --stat 4ecf689..HEAD -- internal/engine/browser.go internal/engine/context.go internal/engine/page.go`
> On any change, compare excerpts to live code (STOP on mismatch).

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: none (but 005 depends on this)
- **Category**: bug (correctness)
- **Planned at**: commit `4ecf689`, 2026-07-02

## Why this matters

Closes findings [41]/[46]/[57]. Scout's documented "30s operation timeout" is actually a **fixed
page-lifetime deadline**: `NewPage` calls `rodPage.Timeout(b.opts.timeout)` **once** and stores the
returned clone as the permanent `Page`. `rodPage.Timeout(d)` builds `context.WithTimeout(p.ctx, d)`
— an **absolute** deadline measured from page-creation. So **30 seconds after a tab is created,
every CDP operation on it fails** with `context deadline exceeded` — `eval`, `click`, `extract`,
`screenshot`, even `page.URL()`.

Concretely: `scout repl` becomes unusable after 30s of interactive use (every command errors); any
workflow that opens a page, waits, then acts (login flows, monitoring, multi-step automation) dies
mid-flight. To dodge it, the long-lived surfaces pass `WithTimeout(0)` — which disables the timeout
**entirely**, creating the opposite failure (plan 005: a bad selector then blocks forever). There is
no middle setting where "each operation gets 30s" — the very thing users think the flag means.

After this plan: `timeout` means **per-operation**, so a page stays usable for its whole life and a
single slow operation still fails fast.

## Current state

- `internal/engine/context.go` — `Timeout` builds an absolute deadline from the current context:
  ```go
  // context.go:75-79
  func (p *rodPage) Timeout(d time.Duration) *rodPage {
      ctx, cancel := context.WithTimeout(p.ctx, d)
      return p.Context(context.WithValue(ctx, timeoutContextKey{}, &timeoutContextVal{p.ctx, cancel}))
  }
  ```
  This is internalized-rod API. Rod's *intended* usage is `page.Timeout(d).SomeOp()` — a fresh
  timed clone **per operation**, then discard it. `CancelTimeout` (`:82`) pops back to the parent.
- `internal/engine/browser.go` — `NewPage` stores the timed clone permanently:
  ```go
  // browser.go:572-576
  if b.opts.timeout > 0 {
      rodPage = rodPage.Timeout(b.opts.timeout)   // ← absolute deadline, kept forever
  }
  p := &Page{page: rodPage, browser: b}
  ```
  Because the *stored* page carries the timeout context, every later call inherits the same expiring
  deadline instead of getting its own.

Conventions:
- `defaults()` sets `timeout = 30 * time.Second` (see `internal/engine/option.go`).
- `WithTimeout(0)` is the documented "disable" value; keep that meaning.
- Public methods return `error` last; wrap with `scout: <op>: %w`.
- Rod's `Page.Timeout(d)` returns a clone — calling it is cheap and side-effect-free except the
  `cancel` it stashes; the standard idiom is `p.page.Timeout(d).Navigate(url)`.

## Commands you will need

| Purpose | Command | Expected |
|---------|---------|----------|
| Build | `go build ./cmd/scout/ && go build ./pkg/...` | exit 0 |
| Tests | `go test ./internal/engine/` | pass (browser tests may `t.Skip`) |
| Race | `go test -race ./internal/engine/` | pass |
| Lint | `task lint` | exit 0 |

## Scope

**In scope**:
- `internal/engine/browser.go` — stop storing the timed clone as the permanent page (Step 1).
- `internal/engine/page.go` (and the operation sites it calls) — apply the timeout per operation
  (Step 2). Find these with `grep -rn "p.page\." internal/engine/*.go` and focus on the CDP-calling
  methods (Navigate, WaitLoad, Eval, screenshot, element ops).
- Tests.

**Out of scope**:
- `context.go` `Timeout`/`CancelTimeout` internals — do not change rod's primitive; change how
  scout *uses* it.
- The MCP `WithTimeout(0)` decision — plan 005 will switch MCP to a real per-op timeout once this
  lands.

## Steps

### Step 1: Stop baking the deadline into the stored page

In `browser.go` `NewPage`, do **not** store the timed clone. Keep the plain page and remember the
per-operation budget on the `Page` wrapper instead:
```go
// browser.go ~572
p := &Page{page: rodPage, browser: b, opTimeout: b.opts.timeout}
```
Add an `opTimeout time.Duration` field to `Page` (in `page.go`). Remove the
`rodPage = rodPage.Timeout(...)` line.

**Verify**: `go build ./internal/engine/` → exit 0.

### Step 2: Apply the timeout per operation

Introduce one helper on `Page` that returns a per-call timed rod page, and route the CDP-calling
methods through it:
```go
// page.go
func (p *Page) timed() *rodPage {
    if p.opTimeout > 0 {
        return p.page.Timeout(p.opTimeout)
    }
    return p.page
}
```
Then in each operation method that currently uses `p.page.X()` for a bounded CDP action, use
`p.timed().X()` so **that call** gets a fresh `opTimeout` budget. Example (Navigate/WaitLoad/Eval/
Screenshot/Element). Do **not** wrap intentionally-unbounded waits (e.g. an explicit
`WaitClose`/live-monitor) in `timed()` — those are supposed to block; check each site's intent.

Because `Page.Timeout(d)` clones and stashes a `cancel`, prefer calling `timed()` once per public
method and reusing the clone within that method, rather than per sub-call, to avoid leaking cancels.
If a method chains several rod calls that should share one budget, capture `tp := p.timed()` at the
top and use `tp` throughout.

**STOP** if a method holds the page across an intentionally long wait *and* a bounded op in the same
call — report it so the budget boundary can be decided explicitly rather than guessed.

**Verify**: `go build ./internal/engine/` and `go vet ./internal/engine/` → exit 0.

### Step 3: Keep `WithTimeout(0)` meaning "no per-op timeout"

`opTimeout == 0` must skip `timed()`'s wrap (the helper above already does this). Confirm the
long-lived surfaces that currently pass `WithTimeout(0)` still compile and behave (they now get
"unbounded per op", same as before — plan 005 will give MCP a real bounded value on top of this).

**Verify**: `grep -rn "WithTimeout(0)" pkg/ internal/ cmd/` still compiles; `go build ./...` for the
buildable roots (`./cmd/scout/`, `./pkg/...`).

## Test plan

- New `internal/engine/page_timeout_test.go` (needs a browser; `t.Skip` if none): open a page,
  `time.Sleep(b.opts.timeout + 5s)` (use a **short** `WithTimeout(2*time.Second)` in the test), then
  run an operation (e.g. `page.Eval("1+1")`) and assert it **succeeds** — proving the page did not
  die of old age. Before this plan the op would fail with `context deadline exceeded`.
- Add the inverse: a genuinely slow operation (navigate to a `httptest` server that sleeps > the
  per-op timeout) still fails fast with a deadline error — proving the per-op budget still bites.
- `go test -race ./internal/engine/` green (the `timed()` clones must not race).

## Done criteria

- [ ] `grep -n "rodPage = rodPage.Timeout" internal/engine/browser.go` returns no match.
- [ ] `Page` has an `opTimeout` field and a `timed()` helper; CDP operation methods use it.
- [ ] New test proves a page older than its timeout still executes an operation; and a slow op still
      times out. `go test ./internal/engine/ -run Timeout` passes (or skips w/o Chromium).
- [ ] `go build ./cmd/scout/ && go build ./pkg/...` exit 0; `go test -race ./internal/engine/` passes.
- [ ] `task lint` exit 0.
- [ ] Manual: `scout repl https://example.com`, wait >60s, run `eval document.title` — it returns a
      value (before: `context deadline exceeded`).
- [ ] `plans/README.md` row updated.

## STOP conditions

- Current-state excerpts don't match (drift).
- The set of "operation methods" is larger/messier than a dozen sites and you cannot cleanly tell
  bounded ops from intentional waits — report the list rather than guessing; a wrong wrap turns an
  intentional wait into a spurious timeout.
- Any method depends on the stored page *already* carrying a timeout context (search for
  `.CancelTimeout()` / `timeoutContextKey` usage) — report before removing the stored deadline.

## Maintenance notes

- This is the counterpart to plan 005: once per-op timeouts exist, the MCP server should pass a real
  `WithTimeout` (e.g. 30–60s) instead of `WithTimeout(0)`, so a bad selector fails fast instead of
  wedging the transport. Note that cross-reference in the 005 work.
- Any new `Page` method that performs a CDP round-trip should go through `timed()`; document that on
  the helper.
- Reviewer: the load-bearing test is "old page still works" — approve on that, plus a scan that no
  intentional long-wait got wrapped in `timed()`.
