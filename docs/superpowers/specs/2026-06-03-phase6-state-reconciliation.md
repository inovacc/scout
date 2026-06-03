# Phase 6 "Shared Command Executor" — State Reconciliation

**Date:** 2026-06-03
**Reconciles:** `docs/superpowers/specs/2026-05-16-06-shared-command-executor-design.md` (dated 2026-05-16)
**Status:** Analysis — supersedes the literal package location in the original design
**Verified against:** real code at repo HEAD (`feat/aria-phase-a`)

---

## TL;DR

The Phase 6 design proposes creating a **new** package `internal/engine/executor/` of pure
command functions consumed by thin REPL + MCP adapters. Since the spec was written, commits
`20de82c`, `614d4d4` introduced **`pkg/scout/tools/`** — a unification layer that already
implements the spec's core idea (one typed function per capability, no transport concerns,
caller owns the browser) for **8 verb families / 10 exported verbs**. MCP already delegates
those 8 verbs to it. The spec is therefore **partly stale**: the architecture exists, but at a
different location and only ~30% of the surface is routed.

**Recommendation:** **ADOPT + EXTEND `pkg/scout/tools/` as THE shared executor.** Do not create
`internal/engine/executor/`. Route the REPL through `tools/` and port the remaining inline MCP
primitives into `tools/`. This is the lower-churn, DRY path that reuses shipped, MCP-proven work.

---

## 1. specStale verdict: **partly**

`pkg/scout/tools/` already *is* the intended shared executor, by a different name and location:

| Spec requirement (SHARE-01 / SHARE-02) | `pkg/scout/tools/` reality |
|---|---|
| "pure functions, one per capability" | ✅ `doc.go` mandates `Func(ctx, *scout.Browser, <Cap>Input) (<Cap>Output, error)`, one file per capability |
| "no browser-control logic in adapters; adapters parse input + format output" | ✅ for the 8 ported verbs — MCP handlers are `json.Unmarshal → ensureBrowser → tools.Verb → jsonResult` |
| "stateless w.r.t. session; caller provides Browser/Page" | ✅ `doc.go`: "Browser lifecycle is the caller's responsibility" |
| "context.Context for cancellation/tracing" | ⚠️ ctx accepted everywhere but mostly `_`-ignored; only `RunbookApply` threads it |
| "error convention `scout: subsystem: msg`" | ⚠️ uses `tools: <verb>:` prefix, not `scout:` (deliberate, layer-local) |
| "exists in `internal/engine/executor/`" | ❌ lives in `pkg/scout/tools/` instead |
| "REPL routed through executor" | ❌ **0 of 19** REPL browser commands route through it |
| "MCP routed through executor" | ⚠️ **8 of ~19** MCP verb-tools route through it; 12 still inline |

Why **partly** and not **yes**: the *pattern and package* exist and are correct, but the spec's
literal deliverable (`internal/engine/executor/`) does not, the REPL is entirely un-routed, and
the browser primitives (navigate/click/type/extract/eval/screenshot/pdf/snapshot/ws/session)
have **no `tools/` equivalent yet**. The "exactly 3 places to add a command" success criterion is
not yet achievable because the REPL leg of the tripod is missing.

Why not **no**: a genuine, transport-free, MCP-consumed shared layer is shipped and growing. The
design's intent is realized; only its address and coverage are out of date.

---

## 2. recommendedApproach: ADOPT + EXTEND `pkg/scout/tools/`

Adopt `pkg/scout/tools/` as THE shared executor and amend Phase 6 to target it instead of a new
`internal/engine/executor/`; the layer already satisfies the spec's contract, is proven by 8 live
MCP delegations, and re-uses engine result types as output aliases (no parallel hierarchy to
drift). The remaining work is **completion, not creation**: port the ~11 inline browser/capture/ws
primitives (plus session/open/swarm) into `tools/` verbs, then make the REPL a thin adapter that
dispatches to them. Creating a second pure-function package in `internal/engine/` would duplicate
an existing one and force the just-ported CLI + MCP call sites to migrate again — pure churn with
no DRY benefit.

---

## 3. gapCounts (verified against code)

**REPL — `cmd/scout/repl.go`:** 21 `case` labels confirmed; no `scout/tools` import.
- **19 commands with direct browser logic, 0 routed through `tools/`** → navigate, eval, click,
  type, extract, screenshot, markdown, html, cookies, url, title, wait, back, forward, reload,
  tabs, tab, newtab, health. (Plus the pre-loop startup-URL path and per-iteration `page.URL()`
  prompt also call the facade directly.)
- 2 pure-control (no browser): help, exit/quit.

**MCP — `pkg/scout/mcp/tools_*.go`:** 7 delegating source files confirmed (crawl, form, gather,
report, runbook, sitemap, testsite).
- **8 verb-families / 10 registrations delegate** to `tools/`: crawl, form_detect, gather,
  sitemap, test_site, runbook_plan, runbook_apply, report_list, report_show, report_delete.
- **12 tools still inline** (19 registrations) → navigate, click, type, extract, eval, back,
  forward, wait, screenshot, snapshot, pdf, session_list, session_reset, open, swarm_crawl,
  ws_listen, ws_send, ws_connections, browser_snapshot (aria). `swarm_crawl` is the heaviest
  (`swarm.NewCoordinator` + worker orchestration + `engine.SaveReport` inline — confirmed lines
  70–203); `browser_snapshot` calls `aria.Capture`, not `tools/`.

**Net routing gap to close:** **19 REPL commands + ~12 MCP inline tools** still carry direct
browser logic. Of these, ~11 browser primitives are *shared* between REPL and MCP and need a
single new `tools/` verb each (the highest-leverage extractions: navigate, click, type, extract,
eval, back, forward, wait, screenshot, snapshot/pdf, html/markdown).

---

## 4. proposedTasks (ordered, TDD-friendly)

1. **Amend Phase 6 spec**: redirect SHARE-01 deliverable from `internal/engine/executor/` to
   `pkg/scout/tools/`; restate "3 places" as executor verb + REPL registration + MCP registration.
2. **Establish a REPL dispatch shim** (`cmd/scout/repl.go`): table of `name → tools.Verb`
   adapter funcs replacing the inline `switch`; wire help/exit as pure-control. No behavior change
   for already-ported verbs (none yet) — pure scaffold + test harness.
3. **Port navigation primitives to `tools/`**: `Navigate`, `Back`, `Forward`, `Reload`, `Wait`
   verbs with typed Input/Output + table tests using `newTestBrowser`. Route MCP navigate/back/
   forward/wait and REPL navigate/back/forward/reload/wait through them.
4. **Port interaction primitives**: `Click`, `Type`, `Extract`, `Eval` verbs + tests; route both
   MCP and REPL cases.
5. **Port content/read primitives**: `HTML`, `Markdown`, `Cookies`, `Title`, `URL`, `Tabs`/`Tab`/
   `NewTab` verbs + tests; route REPL (MCP has no equivalents — register where parity is wanted).
6. **Port capture primitives**: `Screenshot`, `Snapshot`, `PDF` verbs + tests; route MCP capture
   tools and REPL screenshot.
7. **Port `Health`/`TestSite` unification**: confirm REPL `health` uses the existing `tools.TestSite`
   verb (or a thin `Health` wrapper) so REPL and MCP share one implementation.
8. **Extract `SwarmCrawl` verb** from the inline MCP `swarm_crawl` handler (coordinator/worker +
   `SaveReport`) into `tools/`; re-point MCP to delegate; add test.
9. **Port WebSocket verbs** (`WSListen`, `WSSend`, `WSConnections`) and `BrowserSnapshot`(aria)
   into `tools/`; re-point MCP. Add tests.
10. **Thread `context.Context`** through all `tools/` verbs (replace `_ context.Context`) for
    cancellation + OpenTelemetry spans, satisfying the spec's context-propagation rationale.
11. **Add a parity/lint guard test**: assert every `tools/` verb is registered by both REPL and
    MCP (or explicitly waived), structurally enforcing the "3 places" rule and preventing drift.
12. **Update docs**: CLAUDE.md, ARCHITECTURE.md, and the Phase 6 spec's success criteria to point
    at `pkg/scout/tools/`; mark the original `internal/engine/executor/` location superseded.

---

## 5. openQuestions (decide before detailed planning)

1. **Executor location — `pkg/scout/tools/` vs `internal/engine/executor/`.** The spec mandates
   the latter; the shipped layer is the former (public, transport-free, MCP-consumed). Confirm we
   amend the spec to adopt `pkg/scout/tools/` rather than relocate ~10 verbs + all call sites.
2. **`tools/` is public API (`pkg/`) — is that intended for the executor?** Making it the shared
   executor cements `pkg/scout/tools/` as a public surface third parties may import. Acceptable, or
   should the executor stay internal (which would argue for `internal/`)?
3. **REPL-only read verbs (html, markdown, cookies, url, title, tabs/tab/newtab) have no MCP twin.**
   Do we add matching MCP tools for full parity (the spec's stated goal), or register these in REPL
   only and explicitly waive them in the parity guard?
4. **Error-prefix convention.** `tools/` uses `tools: <verb>:`; the spec/Phase 5 convention is
   `scout: subsystem:`. Standardize on one before porting many verbs.
5. **`open` and `session_*` tools manage browser lifecycle**, which `doc.go` says is the *caller's*
   responsibility. Keep these as adapter-level concerns (not `tools/` verbs), or model session ops
   as a distinct executor category?

---

## Appendix A — Inventory: `pkg/scout/tools/` (the shared layer)

`doc.go` declares the contract (verified verbatim): ONE typed func per capability,
`Func(ctx, *scout.Browser, <Cap>Input) (<Cap>Output, error)`, no transport concerns (no cobra, no
MCP types, no JSON marshalling), `jsonschema:` tags drive MCP inputSchema via reflection, browser
lifecycle is the caller's responsibility.

### Exported verbs (10 across 7 capability files)

| Verb | File | Signature | Driver | Output type |
|------|------|-----------|--------|-------------|
| `Gather` | gather.go | `(_ ctx, b *scout.Browser, in GatherInput) (*GatherOutput, error)` | browser | `= scout.GatherResult` (alias) |
| `Crawl` | crawl.go | `(_ ctx, b *scout.Browser, in CrawlInput) (*CrawlOutput, error)` | browser | own struct (StartURL/Pages/Total) |
| `Sitemap` | sitemap.go | `(_ ctx, b *scout.Browser, in SitemapInput) (*SitemapOutput, error)` | browser | `= scout.SitemapResult` (alias) |
| `TestSite` | testsite.go | `(_ ctx, b *scout.Browser, in TestSiteInput) (*TestSiteOutput, error)` | browser | `= scout.HealthReport` (alias) |
| `FormDetect` | form.go | `(_ ctx, b *scout.Browser, in FormDetectInput) (*FormDetectOutput, error)` | browser | own struct (Form/Forms/Total) |
| `RunbookPlan` | runbook.go | `(_ ctx, b *scout.Browser, in RunbookPlanInput) (*RunbookPlanOutput, error)` | browser | `= runbook.ExecutionPlan` (alias) |
| `RunbookApply` | runbook.go | `(ctx, b *scout.Browser, in RunbookApplyInput) (*RunbookApplyOutput, error)` | browser | `= runbook.Result` (alias) |
| `ReportList` | report.go | `(_ ctx, _ ReportListInput) (*ReportListOutput, error)` | **pure** (FS) | own struct |
| `ReportShow` | report.go | `(_ ctx, in ReportShowInput) (*ReportShowOutput, error)` | **pure** (FS) | own struct |
| `ReportDelete` | report.go | `(_ ctx, in ReportDeleteInput) (*ReportDeleteOutput, error)` | **pure** (FS) | own struct |

### Observed conventions (consistent across all files)
- **Two signature shapes**: browser-driven `(ctx, *scout.Browser, Input)→(*Output, error)`; pure
  `(ctx, Input)→(*Output, error)`. Reports are the only pure verbs (FS over `~/.scout/reports/`).
- **ctx accepted but mostly unused** (`_ context.Context`); only `RunbookApply` threads it.
- **Guard pattern** in every browser verb: nil-browser then required-field check, both returning
  `fmt.Errorf("tools: <verb>: ...")`. Uniform `tools: <verb>:` prefix.
- **Options conversion** in unexported `(in XInput) toXOptions() ([]scout.XOption, error)`
  (gather/crawl/sitemap/testsite); defaults applied here (crawl depth 3/pages 100; testsite depth
  2/conc 3/timeout 60s). Duration fields are `string` parsed via `time.ParseDuration`.
- **Output reuse**: 5 of 10 outputs are `=` type aliases to engine/runbook result types; the 4
  own-structs add `Total`/wrapper framing (Crawl, FormDetect, Report*).
- **Input tag duality**: `json:` + `jsonschema:` on every field; `omitempty` on optionals — drives
  MCP inputSchema and (per doc) CLI cobra flag wiring.

**Capabilities present:** gather, crawl, sitemap, test-site, form-detect, runbook plan/apply,
report list/show/delete. **Not yet ported:** navigate/click/type/extract/eval/screenshot/pdf/
snapshot, swarm, websocket, session — still live in MCP/CLI directly. This package is the genuine
candidate shared executor: pure dependency on `pkg/scout` (+ `pkg/scout/runbook`), zero transport
imports.

---

## Appendix B — Inventory: REPL (`cmd/scout/repl.go`)

Single inline `switch` in `replCmd.RunE` (21 `case` labels confirmed). Imports only `pkg/scout`
facade + cobra — **no `pkg/scout/tools/` import**. Every browser command calls facade methods
directly on `b` (Browser) or `page` (Page). Zero delegation.

| Command (aliases) | Direct browser calls? | Args | Output |
|---|---|---|---|
| `navigate` (`go`,`nav`) | **yes** — `b.NewPage`, `page.WaitLoad`, `page.Close`, `page.Title` | `<url>` | `Page: <title>` |
| `eval` | **yes** — `page.Eval(expr)` | `<js expression>` | eval result |
| `click` | **yes** — `page.Element`, `el.Click` | `<selector>` | `clicked` |
| `type` | **yes** — `page.Element`, `el.Input` | `<selector> <text>` | `typed` |
| `extract` | **yes** — `page.ExtractText` | `<selector>` | element text |
| `screenshot` | **yes** — `page.Screenshot` (+`os.WriteFile`) | `[file]` (def `screenshot.png`) | `Saved: <file>` |
| `markdown` (`md`) | **yes** — `page.Markdown` | none | page markdown |
| `html` | **yes** — `page.HTML` | none | page HTML |
| `cookies` | **yes** — `page.GetCookies` | none | JSON cookies |
| `url` | **yes** — `page.URL` | none | current URL |
| `title` | **yes** — `page.Title` | none | page title |
| `wait` | **yes** — `page.WaitLoad` / `page.Element` | `[selector]` | `page loaded` / `found: <sel>` |
| `back` | **yes** — `page.NavigateBack` | none | (errors only) |
| `forward` | **yes** — `page.NavigateForward` | none | (errors only) |
| `reload` | **yes** — `page.Reload` | none | `reloaded` |
| `tabs` | **yes** — `b.Pages`, `p.URL`, `p.Title` | none | tab list (`* ` marks current) |
| `tab` | **yes** — `b.Pages`, `page.Title` (switch via slice index) | `<index>` | `Switched to: <title>` |
| `newtab` | **yes** — `b.NewPage`, `page.WaitLoad` | `[url]` | `new tab opened` |
| `health` | **yes** — `page.URL`, `b.HealthCheck(...)` | none | `Pages/Duration/Issues` + lines |
| `help` | no — `printREPLHelp(out)` (pure control) | none | help text |
| `exit` (`quit`) | no — returns from loop (pure control) | none | `Bye!` |

**Also:** the setup path (before the loop) makes direct `b.NewPage` + `page.WaitLoad` for the
optional startup `[url]`; the loop prompt calls `page.URL()` directly each iteration.

### Counts
- Total commands: 21 distinct (24 incl. aliases). Direct browser logic: **19**. Delegate to
  `tools/`: **0**. Pure control: **2** (help, exit/quit).
- **Routing note:** the REPL is the largest remaining cluster of un-routed direct browser calls —
  the inverse of the recently-ported CLI verbs (`20de82c`, `614d4d4`).

---

## Appendix C — Inventory: MCP (`pkg/scout/mcp/tools_*.go`)

"Delegates" = calls a `pkg/scout/tools/` verb; "Inline" = drives the page/browser directly. 7
delegating source files confirmed; `tools_browser.go` has **no** `tools.*` calls.

| tool | file | delegates to `tools/`? | inline calls (if any) |
|------|------|----------------------|-----------------------|
| navigate | tools_browser.go | no | ensurePage, page.Navigate, page.WaitLoad, page.Title/URL |
| click | tools_browser.go | no | ensurePage, page.Element, el.Click |
| type | tools_browser.go | no | ensurePage, page.Element, el.Input |
| extract | tools_browser.go | no | ensurePage, page.Element, el.Text |
| eval | tools_browser.go | no | ensurePage, page.Eval |
| back | tools_browser.go | no | ensurePage, page.NavigateBack |
| forward | tools_browser.go | no | ensurePage, page.NavigateForward |
| wait | tools_browser.go | no | ensurePage, page.WaitSelector, page.WaitLoad |
| screenshot | tools_capture.go | no | ensurePage, page.Screenshot / page.FullScreenshot |
| snapshot | tools_capture.go | no | ensurePage, page.SnapshotWithOptions |
| pdf | tools_capture.go | no | ensurePage, page.PDF / page.PDFWithOptions |
| session_list | tools_session.go | no | ensurePage, page.URL/Title |
| session_reset | tools_session.go | no | state.reset() |
| open | tools_session.go | no | scout.New, b.NewPage, page.WaitLoad |
| swarm_crawl | tools_swarm.go | no | swarm.NewCoordinator/NewWorker, engine.SaveReport (full inline) |
| ws_listen | tools_websocket.go | no | ensurePage, page.MonitorWebSockets |
| ws_send | tools_websocket.go | no | ensurePage, page.Eval |
| ws_connections | tools_websocket.go | no | ensurePage, page.Eval |
| browser_snapshot | tools_aria.go | no | ensurePage, aria.Capture, state.ariaStore.Put |
| crawl | tools_crawl.go | **yes** | tools.Crawl(ctx, browser, in) |
| form_detect | tools_form.go | **yes** | tools.FormDetect(ctx, browser, in) |
| gather | tools_gather.go | **yes** | tools.Gather(ctx, browser, in) |
| sitemap | tools_sitemap.go | **yes** | tools.Sitemap(ctx, browser, in) |
| test_site | tools_testsite.go | **yes** | tools.TestSite(ctx, browser, in) |
| runbook_plan | tools_runbook.go | **yes** | tools.RunbookPlan(ctx, browser, in) |
| runbook_apply | tools_runbook.go | **yes** | tools.RunbookApply(ctx, browser, in) |
| report_list/show/delete | tools_report.go | **yes** | tools.Report*(ctx, in) — no browser |

### Clean summary
- **Delegating (calls `tools.*`):** crawl, form_detect, gather, sitemap, test_site, runbook_plan,
  runbook_apply, report_list, report_show, report_delete — **10 registrations / 8 verb families**.
- **Inline (direct page/browser/engine/aria logic):** navigate, click, type, extract, eval, back,
  forward, wait, screenshot, snapshot, pdf, session_list, session_reset, open, swarm_crawl,
  ws_listen, ws_send, ws_connections, browser_snapshot — **19 registrations / 12 distinct tools**.

### Notes
- `browser_snapshot` calls `aria.Capture` (pkg/scout/aria), not `tools/` — counted inline.
- `swarm_crawl` is the heaviest inline tool (coordinator/worker + `engine.SaveReport`, lines
  70–203) — strong candidate for a `tools.SwarmCrawl` verb extraction.
- All delegating handlers share one shape: `json.Unmarshal → state.ensureBrowser(ctx) →
  tools.Verb(ctx, browser, in) → jsonResult` (report_* skip ensureBrowser; FS-only).
