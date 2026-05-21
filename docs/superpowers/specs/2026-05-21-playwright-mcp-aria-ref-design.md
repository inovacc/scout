# Playwright-MCP ARIA-Ref Model in Scout — Design Spec

**Date:** 2026-05-21
**Status:** Approved — ready for implementation planning
**Scope:** Single milestone, 5–6 phases
**Owner:** Scout MCP subsystem (`pkg/scout/mcp/`) + new `pkg/scout/aria/` package

---

## 1. Background

`microsoft/playwright-mcp` exposes Playwright over MCP. Its real innovation is not "browser automation over MCP" but the **accessibility-tree + ref-id interaction model**:

1. `browser_snapshot` returns the accessibility tree as YAML, each interactive node tagged `[ref=e15]`.
2. Action tools (`browser_click`, `browser_type`, …) take a `ref` instead of a selector.
3. Every action response auto-appends a fresh post-action snapshot.

This makes LLM-driven automation deterministic — no selector hallucination, no "the third button" guessing.

Scout already has CDP via an internalized rod fork, the AX-tree types wired in (`internal/engine/lib/proto/accessibility.go`), an MCP server with 18 selector-based tools, a hijack subsystem more sophisticated than playwright-mcp's network capture, and a runbook subsystem ripe for code-gen synergy. The gap is the ref model and the tool surface around it.

## 2. Goal

Adopt the ARIA-ref model in Scout with **Scout-native naming and conventions**. We are stealing the idea, not the interface. No drop-in compatibility with playwright-mcp configs (Claude Desktop, Cursor, etc. pointing at playwright-mcp will not work pointing at Scout).

Non-goals:
- Drop-in playwright-mcp compatibility
- Emitting Playwright TypeScript code from session recording
- Replacing Scout's hijack subsystem with playwright-mcp's network model

## 3. Key Decisions (from brainstorm)

| Decision | Choice | Rationale |
|---|---|---|
| Goal shape | Adopt ARIA-ref model in Scout | Scout-native naming, no playwright-mcp config compatibility |
| Migration vs coexistence | **Clean migration** — ref-based replaces selector-based | Cleanest AI tool surface; matches Scout's "breaking changes acceptable" stance |
| MVP scope | **Full parity** (5–6 phases, single milestone) | User chose everything in one milestone |
| Snapshot delivery | **MCP resource + inline diff summary** | Best token economy. Snapshot lives at `scout://snapshot/<page-id>`; actions return compact deltas |
| Code-gen target | **Go + runbook YAML** (no Playwright TS) | Scout's users are Go devs; runbook integration falls out of recording |
| Package placement | `pkg/scout/aria/` (top-level, reusable) | MCP, agent HTTP, runbook, and library users all reuse the ref model |

## 4. Architecture

```
pkg/scout/aria/                     NEW — the ref model
  axtree.go                         CDP Accessibility.getFullAXTree wrapper + YAML rendering
  store.go                          per-page ref store: ref → backendNodeID, version counter
  diff.go                           snapshot-vs-snapshot diff → inline summary
  action.go                         typed Action values (Click/Type/Hover/Drag/Select/Key/Upload/…)
  resolve.go                        ref → *engine.Element resolution (cross-frame aware)
  errors.go                         StaleRefError, NoSnapshotError, AmbiguousRefError
  aria_test.go                      table-driven + real-browser tests
  testdata/                         AX-tree fixtures + expected YAML renders

pkg/scout/mcp/                      EXISTING — tools migrated/added
  tools_aria.go                     NEW: snapshot, click, type, hover, drag, select_option, key, file_upload, extract, wait (all ref-based)
  tools_browser.go                  TRIMMED: keeps navigate/eval/back/forward only; selector-based click/type/extract/wait/snapshot DELETED (their ref-based replacements live in tools_aria.go)
  tools_observe.go                  NEW: dialog_handler, console_messages, network_requests (wired to existing hijack)
  tools_capture.go                  EXISTING: screenshot/pdf untouched
  tools_session.go                  EXISTING: session_list/session_reset untouched
  tools_tabs.go                     NEW: tab_list, tab_new, tab_select, tab_close
  tools_codegen.go                  NEW: generate_test (format=go|runbook)
  resources.go                      EXTENDED: scout://snapshot/{page-id} resource template
  capabilities.go                   NEW: capability gating (default: all enabled)

pkg/scout/runbook/                  EXISTING — extended
  recorder.go                       wires to aria.Action stream

pkg/scout/agent/                    EXISTING — picks up ref-based provider for free
pkg/scout/scout.go                  EXISTING — facade re-exports: scout.AriaSnapshot, scout.Ref, etc.
```

### Layering rule (strict)

`pkg/scout/aria/` depends ONLY on `internal/engine/` and `internal/engine/lib/proto`. It MUST NOT import `mcp/`, `agent/`, `runbook/`. Those depend on `aria`, never the other way. Enforced by a `golangci-lint` `depguard` rule and `go vet`.

### Capability gating

All capabilities default ON. CLI flag `--mcp-caps=core,tabs,vision,observe,codegen,files` lets operators disable groups. Tools register themselves with a capability tag; the MCP server filters at registration time.

| Capability | Tools gated |
|---|---|
| `core` | snapshot, click, type, hover, drag, select_option, key, navigate, back, forward, eval, wait |
| `tabs` | tab_list, tab_new, tab_select, tab_close |
| `vision` | screenshot, pdf |
| `observe` | dialog_handler, console_messages, network_requests |
| `codegen` | generate_test |
| `files` | file_upload |

## 5. Components

### 5.1 `axtree.go` — accessibility tree extractor

```go
func Capture(ctx context.Context, page *engine.Page, opts ...Option) (*Snapshot, error)

type Snapshot struct {
    PageID     string
    Version    uint64      // monotonic per-page
    Nodes      []Node      // flat list; Children indices form the tree
    URI        string      // scout://snapshot/<page-id>?v=<version>
    CapturedAt time.Time
    Truncated  bool        // true if cap hit
}

func (s *Snapshot) RenderYAML(w io.Writer) error
```

Implementation: runs CDP `Accessibility.getFullAXTree` on root frame + each child frame in parallel (bounded concurrency = 4). Walker assigns refs in document order. Cross-frame: refs prefixed with frame index (`e15` root, `f2:e3` second frame). Truncation cap: 10,000 nodes / 64 KB rendered output (configurable via `WithMaxNodes`, `WithMaxBytes`).

Ref format is documented and stable: `e<N>` for root frame, `f<F>:e<N>` for frame F. Refs are never recycled within a session; version bump issues new refs.

### 5.2 `store.go` — per-page ref store

```go
type Store struct { /* sync.RWMutex + map[pageID]*Snapshot */ }

func (s *Store) Put(pageID string, snap *Snapshot)
func (s *Store) Get(pageID string) (*Snapshot, bool)
func (s *Store) Resolve(pageID, ref string) (backendNodeID int64, err error)
```

One `aria.Store` per MCP server instance (lives in `mcpState`). One snapshot per `(sessionID, pageID)` tuple. Concurrent-safe.

### 5.3 `diff.go` — snapshot delta

```go
func Diff(old, new *Snapshot) Summary

type Summary struct {
    Added, Removed, Changed []NodeChange
}
func (s Summary) String() string // "3 elements added near ref=e15, ref=e22 removed"
```

Structural diff (tree shape + roles + accessible-name hash), not text. Inline summary stays small (1–3 lines typical).

### 5.4 `action.go` — typed action values

```go
type Action interface{ Kind() string }

type Click        struct{ Ref string; Modifiers []string }
type Type         struct{ Ref string; Text string; PressEnter bool }
type Hover        struct{ Ref string }
type Drag         struct{ FromRef, ToRef string }
type SelectOption struct{ Ref string; Values []string }
type Key          struct{ Ref string; Key string }
type FileUpload   struct{ Ref string; Paths []string }
```

JSON marshaling baked in. The runbook recorder sinks `aria.Action` values to disk; `generate_test --format=runbook` is `json.Marshal(actions)`. `--format=go` template-renders the same list to Scout Go code.

### 5.5 `resolve.go` + `errors.go` — execution glue

`resolve.Element(page, backendNodeID)` returns a live `*engine.Element` via CDP `DOM.resolveNode`.

Errors:

```go
var (
    ErrStaleRef     = errors.New("aria: stale ref")
    ErrNoSnapshot   = errors.New("aria: no snapshot for page")
    ErrAmbiguousRef = errors.New("aria: ref resolves to detached node")
    ErrCapture      = errors.New("aria: capture failed")
    ErrTruncated    = errors.New("aria: snapshot truncated by cap")
)

type StaleRefError     struct{ Ref string; HaveVersion, RequestedVersion uint64 }
type NoSnapshotError   struct{ PageID string }
type AmbiguousRefError struct{ Ref string; BackendNodeID int64 }
```

All wrap via `Unwrap()` returning the sentinel. Callers use `errors.Is`.

## 6. Data Flow — the ref lifecycle

### 6.1 Snapshot creation

1. AI calls `browser_snapshot` tool OR reads `scout://snapshot/<page-id>` resource.
2. MCP tool dispatches to `aria.Capture(ctx, page)`.
3. Capture runs CDP `Accessibility.getFullAXTree` on root frame + each child frame (parallel, bounded concurrency = 4).
4. Walker assigns refs in document order. Version counter bumps.
5. Store stores under `pageID`. Resource read returns YAML render.

### 6.2 Action execution

1. AI calls e.g. `browser_click {ref: "e15"}`.
2. Tool calls `store.Resolve(pageID, "e15")` → `backendNodeID`.
   - Stale or unknown → return `StaleRefError` with re-snapshot hint. **No retry.**
3. `resolve.Element(page, backendNodeID)` → live `*engine.Element`.
4. Element method called (`.Click()`, `.Input()`, …).
5. **Post-action snapshot capture**: re-run `aria.Capture`, diff against pre-action snapshot.
6. Response: `{result, summary, snapshot_version, snapshot_uri}`.

### 6.3 Snapshot invalidation triggers

| Trigger | Effect |
|---|---|
| Navigation (root frame) | Store entry cleared; next ref resolution = `NoSnapshotError` |
| Page close (`targetDestroyed`) | Entry removed |
| Action that mutates DOM | Auto re-capture in step 5; version bumps |
| Manual `browser_snapshot` call | Force-refresh; version bumps |
| Bridge-detected major DOM mutation (≥ 20 mutations / 100 ms) | Mark dirty; next action re-captures before resolve |

The bridge already injects MutationObserver scripts; we hook the existing signal rather than running our own.

### 6.4 Concurrency model

- One `aria.Store` per MCP server instance.
- One snapshot per `(sessionID, pageID)` tuple.
- Multi-tab: each tab is a separate page, separate snapshot. Tab switch does not invalidate other tabs.
- `Capture` is concurrent-safe; multiple AI sessions hitting the same MCP server work independently per session.

### 6.5 Token-cost ceiling

A page with ~200 interactive nodes renders to ~6 KB YAML. Cap at 10k nodes / 64 KB rendered output by default. Beyond cap → truncate + `Truncated: true` flag + summary note. Refs still valid for captured subtree.

## 7. Error Handling

### 7.1 Layer 1 — `aria/` package: typed errors only

The package returns Go-typed errors. Tools translate them to MCP responses. Error wrapping convention: `fmt.Errorf("scout: aria: capture: %w", err)`.

### 7.2 Layer 2 — MCP tools: structured responses with hints

Tool wrapper catches each typed error and produces an MCP error response with an actionable hint baked into the text content. Hints are **deterministic strings** (matched verbatim by the test suite) so AI-side prompt templates can rely on them.

Example: stale ref returns:
> `Stale ref e15. Current snapshot version is 14, you used version 11. Re-read scout://snapshot/<page-id> for fresh refs.`

All MCP error responses set `result.IsError = true`, content is `mcp.TextContent`.

### 7.3 Layer 3 — Recoverable vs unrecoverable

| Failure | Recovery |
|---|---|
| Stale ref | AI fault → fail loud. No auto-retry. |
| Detached node mid-action | Race → return `AmbiguousRefError`. No auto-retry. |
| CDP capture timeout | Transient → bubble up; AI may retry. Default timeout: 5s, configurable. |
| Capture during navigation | Page state inconsistent → return `ErrCapture`; AI should `wait` + retry. |
| Snapshot truncated | Soft failure → snapshot returned with `Truncated: true`. |

**Hard rule:** the aria package never retries internally. Every retry is the AI's decision, visible in its tool-call log.

### 7.4 Logging

`slog.Warn` with attrs: `page_id`, `version`, `ref`, `kind`. No URLs/text at WARN. OTel trace span set to `Error` with typed error in `Description`. Wires up automatically via existing `addTracedTool()`.

## 8. Testing Strategy

### 8.1 Layer 1 — `aria/` unit tests (no browser)

Table-driven tests for pure logic.

| Target | Test approach |
|---|---|
| `axtree.go` rendering | Feed fixture AX-tree JSON → assert YAML output matches golden |
| `store.go` | Concurrent Put/Get/Resolve with `-race`. Stale-version returns typed error |
| `diff.go` | Pairs of fixture snapshots → assert Summary content + String format |
| `action.go` | JSON round-trip every action type (protects code-gen output) |

Fixtures live in `pkg/scout/aria/testdata/` as `.json` (input) + `.yaml` (expected). Updateable via `go test -update`.

### 8.2 Layer 2 — `aria/` integration tests (real browser)

Real Chromium via existing `newTestBrowser(t)` pattern.

- Snapshot integrity (fixture HTML via `httptest.Server`)
- Ref resolution → action → re-capture cycle
- Cross-frame refs resolve correctly
- Stale-ref behavior on navigation
- Detached-node race produces `AmbiguousRefError`
- Truncation cap on a 50k-node generated page

### 8.3 Layer 3 — MCP tool tests (`pkg/scout/mcp/tools_aria_test.go`)

Uses Scout's existing in-memory MCP client/server pattern.

- Each migrated tool round-trips through MCP
- Each new tool has happy-path + error-path tests
- `scout://snapshot/<page-id>` resource returns current YAML; version param honored
- Capability gating: `--mcp-caps=core` → `vision/codegen/observe` tools not registered
- Error-hint contracts: stale-ref / no-snapshot / detached-ref produce documented hint string verbatim

### 8.4 Layer 4 — E2E / recorder integration

- Drive multi-step session → recorder captures `aria.Action` stream
- `generate_test --format=runbook` → byte-identical to golden
- `generate_test --format=go` → compiles under `goimports` + `go vet`
- Replay via `scout runbook apply` → resulting AX-tree snapshot matches the post-recording snapshot (ignoring timestamps + version numbers)

### 8.5 Coverage targets

- `pkg/scout/aria/`: **≥ 90%**
- `pkg/scout/mcp/tools_aria.go` + `tools_observe.go` + `tools_codegen.go`: **≥ 80%**
- Migration: every existing `tools_browser.go` test referencing selector-based click/type/extract/wait rewritten against ref-based equivalents — no test count regression.

### 8.6 CI

- Unit tests run on every PR via `task test`.
- Browser tests gated by Chromium availability (existing convention).
- `golangci-lint` `depguard` rule enforces the layering rule from §4.

## 9. Phased Implementation

| Phase | Goal | Deliverables |
|---|---|---|
| **A** Foundation | `pkg/scout/aria/` package + `browser_snapshot` tool | axtree.go, store.go, diff.go, resolve.go, errors.go; new MCP resource; `browser_snapshot` tool wired |
| **B** Action set | All ref-based interaction tools | click, type, hover, drag, select_option, key, file_upload; migrated wait + extract; deletion of selector-based equivalents in `tools_browser.go` |
| **C** Capability gating | `--mcp-caps=` flag + tool tagging | `capabilities.go`, server registration filter, CLI plumbing |
| **D** Observers | dialog_handler, console_messages, network_requests | New `tools_observe.go` wired to existing hijack subsystem |
| **E** Tab tools | tab_list, tab_new, tab_select, tab_close | Extract from `tools_session.go` into `tools_tabs.go` |
| **F** Code-gen | `generate_test` tool + recorder integration | `tools_codegen.go`, runbook recorder wired to `aria.Action` stream, Go template renderer |

Each phase is shippable in isolation. Phase A is the foundation; B–F can land in parallel by different developers once A is merged.

## 10. Out of Scope

- Drop-in compatibility with `playwright-mcp` configs
- Emitting Playwright TypeScript code from sessions
- Auto-retry on stale refs (AI must re-snapshot deliberately)
- Visual-regression integration with the new ref model (separate concern handled by `pkg/scout/monitor`)
- Inline-snapshot delivery mode (rejected in favor of MCP resource + diff summary)

## 11. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| AX-tree from CDP omits non-semantic but interactive elements | Walker augments with role-inference fallback for known patterns (e.g., div + click handler); documented in `axtree.go` |
| Ref invalidation race (action mid-DOM-mutation) | `AmbiguousRefError` returned; AI re-snapshots. No silent recovery. |
| Snapshot capture latency on huge SPAs | Capture timeout (5s default, configurable); truncation cap at 10k nodes |
| Existing MCP tool users break on migration | Documented breaking change in CHANGELOG; CLAUDE.md migration note; old tool names produce a clear "renamed to X" error for one release |
| Cross-frame ref collisions | Frame-prefix scheme (`f<F>:e<N>`); test fixture covers nested iframes |
| Hint string churn breaks AI prompt templates | Hint strings are part of the public contract; changes go through ADR |

## 12. License & Attribution

`microsoft/playwright-mcp` is Apache 2.0; Scout is BSD 3-Clause. We reimplement patterns in Go from scratch — no source is copied. A `NOTICE` entry crediting the playwright-mcp design is added when Phase A ships.

## 13. Open Questions (defer to implementation)

- Exact YAML schema for cross-frame snapshot rendering (frame as nested block vs flat prefix)
- Whether `dialog_handler` should be a subscriber tool (returns events) or a configure-once tool (sets default action)
- `wait` is ref-based per §4; open question is whether the same tool also accepts non-ref conditions (`network_idle`, `load_state`, timeout-only) or whether those split into a separate `wait_for` tool

Implementation plan will resolve these.
