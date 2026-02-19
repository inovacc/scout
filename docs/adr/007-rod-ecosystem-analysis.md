# ADR 007: go-rod Ecosystem Analysis & Feature Adoption

**Date:** 2026-02-19
**Status:** Accepted

## Context

Analysis of the go-rod ecosystem repositories and open issues to identify features, patterns, and bug fixes that should be incorporated into Scout's internal rod package (`pkg/rod/`) and wrapper layer (`pkg/scout/`).

### Repositories Analyzed

| Repository | Purpose | Stars | Last Commit | Rod Version | Status |
|------------|---------|-------|-------------|-------------|--------|
| [go-rod/rod](https://github.com/go-rod/rod) | Core browser automation | 5.7k+ | Active | v0.116.2 | Active |
| [go-rod/wayang](https://github.com/go-rod/wayang) | JSON scripting layer | 27 | 2020-07-07 | v0.41.0 | Abandoned |
| [go-rod/bartender](https://github.com/go-rod/bartender) | SEO dynamic rendering proxy | 23 | 2023-07-31 | v0.114.1 | Dormant |
| [go-rod/rod-mcp](https://github.com/go-rod/rod-mcp) | MCP server for LLMs | 30 | 2025-04-16 | v0.116.2 | Active |

### Scout's Internal Rod State

Scout internalizes rod v0.116.2 as a full copy in `pkg/rod/` with import path rewrites. No local modifications exist yet (per `.dep-track.json`). This analysis identifies the first set of patches to apply.

---

## 1. go-rod/wayang — JSON Scripting Layer

### Overview

Declarative JSON-based browser automation. 30 built-in actions (navigate, click, type, wait, eval, conditionals, loops). CLI runner + Go API. Action dispatch via `map[string]actionFunc` registered in `init()`.

### Architecture

```
wayang/
├── model.go      — Action (map[string]interface{}), Program, Runner, RuntimeError
├── wayang.go     — RunProgram/RunAction/RunActions, Close, logging
├── impl.go       — 30 action implementations
├── cli/main.go   — CLI entry point
└── fixtures/     — HTML test fixtures
```

### Key Patterns

- **`$selector` references** — Named selectors defined once, referenced by `$name` in action steps. Enables refactoring without touching every step.
- **Key-value store** — `runner.ENV` map for passing data between actions. `store` action writes, `logStore` reads.
- **Error as value** — Actions return `interface{}` — either result or `RuntimeError`. No panics.
- **Custom action macros** — Define reusable action sequences, reference via `$actionName`.

### Relevance to Scout

**Low.** Scout's recipe system (`pkg/scout/recipe/`) already surpasses wayang. Wayang is abandoned (2020), pinned to rod v0.41.0, missing screenshots/PDF/network/cookies/stealth.

### What to Adopt

| Feature | Adopt? | Where | Notes |
|---------|--------|-------|-------|
| Named selector `$refs` | Yes | `pkg/scout/recipe/` | Allow `selectors` map in recipe JSON, resolve `$name` at runtime |
| Action macros | No | — | Recipe system already has step composition |
| JSON scripting engine | No | — | Recipe system is superior |

---

## 2. go-rod/bartender — SEO Dynamic Rendering Proxy

### Overview

HTTP reverse proxy that detects crawler User-Agents and renders SPAs with headless Chrome before returning HTML. Normal users get proxied directly.

### Architecture

```go
type Bartender struct {
    addr          string
    target        *url.URL
    proxy         *httputil.ReverseProxy
    bypassList    map[string]bool
    pool          rod.PagePool
    blockRequests []string
    maxWait       time.Duration
}
```

### Key Patterns

#### Browser Pool with Per-Slot Isolation

Each pool slot gets its own browser instance via `launcher.New()`. Not page reuse within one browser — full process isolation per slot.

```go
func (b *Bartender) newPage() *rod.Page {
    l := launcher.New()
    go l.Cleanup()
    page := rod.New().ControlURL(l.MustLaunch()).MustConnect().MustPage()
    // ... hijack setup ...
    return page
}
```

#### AutoFree — Periodic Browser Recycling

Goroutine loop that pulls a page from the pool, closes its entire browser, puts `nil` back so a fresh browser is created on next use. Prevents memory leaks from long-running browser processes.

```go
func (b *Bartender) AutoFree(interval time.Duration) {
    go func() {
        for {
            time.Sleep(interval)
            page := b.getPage()
            browser := page.Browser()
            _ = browser.Close()
            b.pool.Put(nil)
        }
    }()
}
```

#### Request Blocking

```go
router := page.HijackRequests()
for _, pattern := range b.blockRequests {
    router.MustAdd(pattern, func(ctx *rod.Hijack) {
        ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
    })
}
go router.Run()
```

#### MaxWait with sync.Once

Ensures response is written exactly once — either when the page stabilizes or when timeout fires:

```go
once := sync.Once{}
go func() {
    time.Sleep(b.maxWait)
    once.Do(func() { /* write current HTML */ })
}()
_ = page.Navigate(u)
_ = page.WaitStable(time.Second)
body, _ := page.HTML()
once.Do(func() { /* write rendered HTML */ })
```

### Relevance to Scout

**Medium.** Not a library to import, but contains production-tested patterns.

### What to Adopt

| Feature | Adopt? | Where | Notes |
|---------|--------|-------|-------|
| AutoFree browser recycling | Yes | `grpc/server/` | Periodic browser restart for daemon sessions |
| Request blocking on fetch | Yes | `pkg/scout/webfetch.go` | `WithBlockPatterns(...)` option |
| UA-based routing | No | — | Scout is a library, not a proxy |
| `sync.Once` timeout pattern | Yes | `pkg/scout/` | For safe `WaitStable` with hard timeout |
| Browser-per-slot isolation | No | — | Scout uses single browser per session |

---

## 3. go-rod/rod-mcp — MCP Server for LLMs

### Overview

MCP server exposing 12 tools over stdio for LLM-driven browser automation. Uses accessibility snapshots with `[ref=N]` markers for element addressing.

### Architecture

```
rod-mcp/
├── main.go           — Entry point
├── server.go         — MCP server, tool registration
├── tools/
│   ├── common.go     — 8 tools: navigate, back, forward, reload, press, screenshot, eval, close
│   └── snapshot.go   — 4 tools: snapshot, click, fill, selector
├── types/
│   ├── context.go    — Browser/page lifecycle, lazy init, Execute() wrapper
│   ├── snapshot.go   — Accessibility snapshot capture + iframe traversal
│   └── js/
│       ├── snapshotter.js     — ~1500 line ARIA tree builder
│       └── js.go              — Embed directives
└── utils/            — Element lookup, helpers
```

### Key Feature: Accessibility Snapshot System

The most valuable pattern in the entire ecosystem. Injects JS that:

1. Walks the DOM computing ARIA roles (implicit from tag + explicit `role` attribute)
2. Computes accessible names (`aria-label`, `aria-labelledby`, `alt`, `title`, native text)
3. Captures states (checked, disabled, expanded, level, pressed, selected)
4. Outputs YAML with `[ref=s{gen}e{id}]` markers per element
5. For iframes, recursively captures sub-frame snapshots with `f{N}` prefixed refs
6. `LocatorInFrame(ref)` resolves ref strings back to elements

**Example output:**

```yaml
- navigation "Main":
  - link "Home" [ref=s1e1]
  - link "About" [ref=s1e2]
- main:
  - heading "Welcome" [ref=s1e3]: level=1
  - textbox "Search" [ref=s1e4]
  - button "Submit" [ref=s1e5]
```

### MCP Tool Pattern

Every tool handler wrapped by `Context.Execute()`:
- On error → `mcp.NewToolResultError()`
- If `WitSnapshot: true` → auto-append fresh snapshot after action
- Mutation tools always append snapshots so LLM has updated view

### 12 MCP Tools

| Tool | Description | Snapshot After? |
|------|-------------|-----------------|
| `rod_navigate` | Navigate to URL | Yes (text mode) |
| `rod_go_back` | Browser back | Yes |
| `rod_go_forward` | Browser forward | Yes |
| `rod_reload` | Reload page | Yes |
| `rod_press` | Press keyboard key | No |
| `rod_screenshot` | Take screenshot | No |
| `rod_evaluate` | Execute JavaScript | No |
| `rod_close_browser` | Close browser | No |
| `rod_snapshot` | Capture ARIA snapshot | — |
| `rod_click` | Click by ref | Yes |
| `rod_fill` | Type into field by ref | Yes |
| `rod_selector` | Select dropdown by ref | Yes |

### Weaknesses

- Zero tests
- No license
- Uses unofficial MCP SDK (`mark3labs/mcp-go` not `modelcontextprotocol/go-sdk`)
- Single page only (no multi-tab/session)
- No stealth, cookies, network interception, HAR, extraction
- Vision mode is empty stub
- Stdio transport only

### Relevance to Scout

**HIGH.** Scout's MCP server would immediately surpass rod-mcp with 25+ tools, stealth, multi-session, HAR, extraction, search, and LLM pipelines.

### What to Adopt

| Feature | Adopt? | Where | Notes |
|---------|--------|-------|-------|
| Accessibility snapshot | Yes | `pkg/scout/snapshot.go` | Port `snapshotter.js` + Go wrapper |
| MCP transport | Yes | `cmd/scout/mcp.go` | Map gRPC RPCs to MCP tools via official Go SDK |
| Ref-based element lookup | Yes | `pkg/scout/snapshot.go` | `LocatorInFrame(ref)` pattern |
| Iframe snapshot traversal | Yes | `pkg/scout/snapshot.go` | Recursive frame capture |
| Auto-snapshot after mutation | Yes | MCP tool handlers | Post-action snapshot for LLM context |
| `WaitDOMStable` after nav | Already done | `pkg/scout/` | Scout uses `WaitLoad`/`WaitStable` |
| Vision mode | No | — | Empty stub, nothing to port |

---

## 4. go-rod/rod Open Issues (93 total)

### Issue Distribution

| Category | Count | Key Themes |
|----------|-------|------------|
| Questions / Usage Help | 54 | Proxy, waits, iframes, crashes, launch failures |
| Feature Requests | 14 | Fingerprinting, cloud browsers, ARM, SPA waits |
| Bugs | 13 | WaitStable panics, context propagation, regressions |
| Discussion / Upstream | 7 | Google login, Firefox, CDP limitations |

### Issues to Patch in Scout's Internal Rod (`pkg/rod/`)

#### Critical — Apply to Fork

| Issue | Title | Root Cause | Fix | Effort |
|-------|-------|------------|-----|--------|
| [#1103](https://github.com/go-rod/rod/issues/1103) | Segfault on connection loss | `Page.getJSCtxID()` nil dereference when WebSocket disconnected | Add nil-guard to `page_eval.go`, return typed `ErrDisconnected` | Low |
| [#1179](https://github.com/go-rod/rod/issues/1179) | Context not passed through | `page.go:851` internal op ignores user context | Pass page context through to internal call (1-line fix) | Low |
| [#1206](https://github.com/go-rod/rod/issues/1206) | Context caching inconsistency | `Page.Info()`/`Activate()`/`TriggerFavicon()` use browser ctx instead of page ctx | Change to `p.browser.Context(p.ctx)` in 3 methods | Low |

#### High — Apply at Scout Wrapper Level

| Issue | Title | Root Cause | Fix Location | Effort |
|-------|-------|------------|-------------- |--------|
| [#1157](https://github.com/go-rod/rod/issues/1157) | WaitStable panic | "Execution context was destroyed" during redirect/reload | `pkg/scout/page.go` — catch + retry in `WaitLoad`/`WaitStable` | Low |
| [#1224](https://github.com/go-rod/rod/issues/1224) | WaitStable vs WaitRequestIdle | WaitStable internally consumes WaitRequestIdle | `pkg/scout/` — document incompatibility, add `WaitSafe()` wrapper | Low |
| [#865](https://github.com/go-rod/rod/issues/865) | Zombie processes (3yr old, 14 comments) | `browser.Close()` doesn't kill all child processes | `pkg/scout/browser.go` — enumerate + kill Chrome child PIDs on Close | Medium |
| [#982](https://github.com/go-rod/rod/issues/982) | Panic on bad regexp in hijack | `regexp.MustCompile()` panics on invalid patterns | `pkg/scout/network.go` — pre-validate with `regexp.Compile()` | Trivial |

#### Already Solved by Scout

| Issue | Title | Scout's Solution |
|-------|-------|-----------------|
| [#1162](https://github.com/go-rod/rod/issues/1162) | Context-level init scripts | `stealth.Page()` + bridge injection in `NewPage()` pipeline |
| [#905](https://github.com/go-rod/rod/issues/905) | Browser fingerprint support | `pkg/stealth/` with 5 custom evasions + extract-stealth-evasions |
| [#951](https://github.com/go-rod/rod/issues/951) | Chromium auto-download stuck at 114 | `DownloadBrave()` + multi-browser support bypasses stale Chromium |

#### Enhancement Backlog (from Issues)

| Issue | Title | What Scout Could Add | Priority |
|-------|-------|---------------------|----------|
| [#905](https://github.com/go-rod/rod/issues/905) | Browser fingerprint | Integrate [forgeron](https://github.com/Ta0uf19/forgeron) for diverse fingerprint generation | P2 |
| [#1092](https://github.com/go-rod/rod/issues/1092) | BrightData/cloud browsers | `WithRemoteCDP(endpoint)` for managed scraping services | P3 |
| [#1175](https://github.com/go-rod/rod/issues/1175) | Replace gson with gjson | Add `EvalResult.RawJSON()` convenience method | P3 |

---

## 5. Comparison Matrix

### Scout vs rod-mcp (MCP Server)

| Feature | rod-mcp | Scout (current) | Scout (planned) |
|---------|---------|-----------------|-----------------|
| MCP tools | 12 | 0 | 30+ |
| MCP SDK | mark3labs (unofficial) | — | modelcontextprotocol (official) |
| Stealth | None | Full stealth package | Enhanced with forgeron |
| Multi-session | No | Yes (gRPC) | Yes (MCP + gRPC) |
| HAR recording | No | Yes | Yes |
| Extensions | No | CRX download + load | CRX + bridge |
| Extraction | No | Tables, meta, markdown, forms | + accessibility snapshot |
| Search | No | Multi-engine | + WebSearch pipeline |
| LLM integration | No | 6 providers | + MCP tool integration |
| Accessibility snapshot | Yes (1500-line JS) | No | Yes (ported) |
| Tests | None | 80%+ | 80%+ |

### Scout vs bartender (Browser Pool)

| Feature | bartender | Scout |
|---------|-----------|-------|
| Browser pool | `rod.PagePool` per-slot | Single browser per session |
| AutoFree | Yes (periodic recycle) | No (planned for daemon) |
| Request blocking | `HijackRequests` + pattern fail | `HijackRouter` (manual) → `WithBlockPatterns` planned |
| UA detection | `mileusna/useragent` | N/A (library, not proxy) |
| Render proxy | Yes | N/A |

### Scout vs wayang (Declarative Automation)

| Feature | wayang | Scout Recipe |
|---------|--------|-------------|
| Format | JSON actions | JSON recipes (extract + automate) |
| Actions | 30 | 50+ (via recipe + library API) |
| Named selectors | `$ref` pattern | Not yet (planned) |
| Conditionals | if/has/not | Not in recipes (in library API) |
| AI-assisted | No | Yes (LLM recipe generation) |
| Tests | Basic | 81.5% coverage |
| Maintained | No (2020) | Active |

---

## 6. Rod Internal Package Patches

### Patch Plan for `pkg/rod/`

These are the first modifications to Scout's internal rod fork. Each patch addresses a confirmed upstream bug with no fix/PR.

#### Patch 1: Nil-guard on disconnected page (#1103)

**File:** `pkg/rod/page_eval.go`
**Change:** Guard `getJSCtxID()` against nil page/connection, return `ErrDisconnected`

#### Patch 2: Context propagation (#1179)

**File:** `pkg/rod/page.go` (line ~851)
**Change:** Pass page's context through to internal operation

#### Patch 3: Page context in Info/Activate/TriggerFavicon (#1206)

**Files:** `pkg/rod/page.go`
**Change:** Use `p.browser.Context(p.ctx)` instead of `p.browser.ctx` in 3 methods

### Wrapper-Level Fixes for `pkg/scout/`

#### Fix 1: WaitStable panic recovery (#1157)

**File:** `pkg/scout/page.go`
**Change:** Wrap `WaitLoad`/`WaitStable` with panic recovery + retry on "Execution context was destroyed"

#### Fix 2: Safe wait method (#1224)

**File:** `pkg/scout/page.go`
**Change:** Add `Page.WaitSafe(timeout)` — combines `WaitLoad` + timeout guard, no `WaitRequestIdle` conflict

#### Fix 3: Zombie process cleanup (#865)

**File:** `pkg/scout/browser.go`
**Change:** On `Close()`, walk Chrome process tree, kill orphan child processes

#### Fix 4: Hijack regexp validation (#982)

**File:** `pkg/scout/network.go`
**Change:** Pre-validate pattern with `regexp.Compile()` before passing to rod's `Add()`

---

## 7. Feature Adoption Summary

### New Files to Create

| File | Source | Description |
|------|--------|-------------|
| `pkg/scout/snapshot.go` | rod-mcp `types/snapshot.go` + `types/js/` | Accessibility snapshot with ARIA tree, ref markers, iframe traversal |
| `pkg/scout/snapshot_js.go` | rod-mcp `types/js/snapshotter.js` | Embedded JS for ARIA snapshot engine |
| `cmd/scout/mcp.go` | rod-mcp `server.go` + `tools/` | MCP stdio transport exposing Scout tools |
| `docs/adr/007-rod-ecosystem-analysis.md` | This document | Analysis record |

### Existing Files to Modify

| File | Change | Source |
|------|--------|--------|
| `pkg/rod/page_eval.go` | Nil-guard patch | Rod #1103 |
| `pkg/rod/page.go` | Context propagation patches | Rod #1179, #1206 |
| `pkg/rod/.dep-track.json` | Record local modifications | — |
| `pkg/scout/page.go` | WaitSafe + panic recovery | Rod #1157, #1224 |
| `pkg/scout/browser.go` | Process cleanup on Close | Rod #865 |
| `pkg/scout/network.go` | Regexp pre-validation | Rod #982 |
| `pkg/scout/webfetch.go` | `WithBlockPatterns()` option | Bartender pattern |
| `pkg/scout/option.go` | `WithRemoteCDP()` option | Rod #1092 |

---

## Decision

Adopt the following in priority order:

1. **Rod fork patches** (#1103, #1179, #1206) — Critical stability fixes
2. **Wrapper-level fixes** (#1157, #865, #982) — Production hardening
3. **Accessibility snapshot** — Port from rod-mcp for LLM automation
4. **MCP transport** — Expose Scout tools via MCP using official Go SDK
5. **Request blocking** — `WithBlockPatterns()` from bartender pattern
6. **Named recipe selectors** — `$ref` pattern from wayang
7. **AutoFree browser recycling** — From bartender for daemon mode
8. **Remote CDP** — `WithRemoteCDP()` for cloud browser support

## Consequences

- `pkg/rod/.dep-track.json` `localModifications` array will no longer be empty
- Future rod upstream updates require careful merge of patched files
- MCP transport adds `modelcontextprotocol/go-sdk` as a dependency
- Accessibility snapshot adds ~30KB of embedded JS
