# Stabilization Research Summary

**Project:** Scout Browser Automation
**Domain:** Codebase stabilization, CLI restructuring, UX improvements
**Researched:** 2026-03-29
**Confidence:** HIGH (all findings from direct source code analysis)

## Executive Summary

- **Session leak bug**: Deterministic session hashing (`SessionHash(url, browser)`) silently enables reuse when a stale `scout.pid` exists, leaking cookies/localStorage between independent runs. This is the highest-severity correctness bug found.
- **60 top-level CLI commands with 5 overlap clusters**: `recipe` duplicates `runbook` (removal due 2026-04-15), three auth-capture flows do the same thing, `search`/`websearch` overlap, and crawl-related commands are fragmented across 6 entry points.
- **1,100 lines of dead code in `must.go`**: 124 of 134 Must* methods have zero callers. Removing them also eliminates 124 panic-on-error paths.
- **REPL and MCP duplicate interaction logic independently** with divergent capabilities: REPL lacks snapshot/pdf/full-page-screenshot; MCP lacks html/cookies/reload/tabs. Neither shares a command executor layer.
- **15 panic sites in non-test code** (4 Scout-original, 11 rod-inherited) can crash the gRPC server or MCP process. No `recover()` boundaries exist at handler entry points.

## Session & Isolation Issues

**Source:** `.planning/research/sessions-isolation.md`

### Bugs requiring fixes

| ID | Severity | Location | Issue |
|----|----------|----------|-------|
| S1 | CRITICAL | `browser.go:292-305` | Deterministic hash auto-sets `reusableSession=true` when stale `scout.pid` exists, leaking session state |
| S2 | HIGH | `process_windows.go:14-27` | `ProcessAlive` uses `OpenProcess` without checking exit code -- zombies appear alive, blocking cleanup |
| S3 | MEDIUM | `browser.go:714-724` | Non-reusable `Close()` does double cleanup: `launcher.Cleanup()` + `ResetSession()` adds 500ms latency |
| S4 | MEDIUM | `session_track.go:341-350` | Windows file lock retry budget (3x200ms = 600ms) too short for Chrome (2-5s lock hold) |
| S5 | LOW | `browser.go:65,270` | Double `CleanOrphans()` call on every `New()` -- harmless but wasteful |
| S6 | LOW | `browser.go:340-341` | Rod fallback bypasses Scout's browser isolation silently (no log warning) |

### Isolation architecture (sound)

Two-mode browser isolation (`--system-browser` flag) is well-designed. Default cache-only mode uses `~/.scout/browsers/` exclusively. `BestCached()` auto-downloads Chrome for Testing. The only leak path is the rod fallback (S6).

## CLI Overlap Analysis

**Source:** `.planning/research/cli-overlap.md`

### Commands to remove/merge

| Priority | Action | Files affected | Lines saved |
|----------|--------|---------------|-------------|
| P0 | Delete `recipe.go` (deprecated, due 2026-04-15) | `cmd/scout/recipe.go` (664 lines) | ~664 |
| P1 | Merge `credentials` into `auth` (add `--plaintext` flag) | `credentials.go` -> `auth.go` | ~200 |
| P1 | Merge `websearch` and `search` (websearch is superset) | `websearch.go`, `search.go`, `search_engines.go` | ~150 |
| P1 | Deprecate `markdown` (already `fetch --mode=markdown`) | `markdown.go` | ~80 |
| P2 | Group crawl commands under `crawl` parent | `crawl.go`, `map.go`, `sitemap.go`, `knowledge.go` | net 0 (restructure) |
| P2 | Group `table`, `meta`, `extract-ai` under `extract` parent | `extract.go`, `llm.go` | net 0 (restructure) |
| P3 | Group 17 gRPC-dependent commands under `page` parent | `inspect.go`, `interact.go`, `navigate.go` | net 0 (breaking change) |

### Proposed structure

Top-level commands reduce from ~60 to ~45. The P0-P1 actions are safe (remove deprecated + merge clear duplicates). P2 restructures naming. P3 is breaking and should be deferred.

### Inconsistencies to fix

- `fetch` uses `--url` flag; most commands use positional args
- No shared URL normalization helper (each command does `strings.HasPrefix` checks)
- `--format` flag behavior varies across commands (root persistent flag vs local flag vs ignored)

## Code Quality Hotspots

**Source:** `.planning/research/code-quality.md`

### Largest files requiring splits

| File | Lines | Action |
|------|-------|--------|
| `grpc/server/server.go` | 1398 | Split by domain (already has section comments as boundaries) |
| `internal/engine/must.go` | 1267 | Delete 124 unused methods (~1100 lines) |
| `internal/engine/browser/download.go` | 1120 | Split Chrome/Brave/Edge into separate files |
| `internal/engine/page_rod.go` | 1103 | Split into navigation/eval/capture |
| `internal/engine/github.go` | 1006 | Split into auth/repo/search/API |
| `cmd/scout/bridge.go` | 993 | Split into server/client/commands |

### Panic sites (top risk)

| Location | Reachable from | Fix |
|----------|---------------|-----|
| `lib/input/keyboard.go:61` | gRPC `PressKey`, MCP `type` | Add `recover()` at handler boundary |
| `browser_rod.go:160,380` | Any `New()` call | Convert to error return |
| `browser/manifest.go:76` | Browser download | Convert to error return |
| 30+ `utils.E()` calls | Various | Add `recover()` at gRPC/MCP boundaries |

### Dead code

- **124 unused Must* methods** in `must.go` (~1100 lines) -- delete immediately
- `createProvider` in `llm.go` -- marked `//nolint:unused`, zero callers
- `FingerprintToProfile` in `fingerprint.go` -- only caller is its own test
- `recipe.go` / `runbook.go` share byte-identical `applyVars()` and `findUnresolvedVars()` -- extract to shared helper before recipe deletion

### Duplicate implementations

- `pkg/scout/browser/` re-implements detection/download logic from `internal/engine/browser/` (~380 lines). Should delegate instead.
- `truncate()` defined in both `helpers.go:137` and `tools_websocket.go:155` with different behavior (helpers version is correct).

### Pattern violations

- 657 `fmt.Errorf` calls lack the `"scout: ..."` prefix convention
- 4 logging mechanisms coexist: `slog` (preferred), `fmt.Fprint(os.Stderr)`, `log.Printf`, `utils.Log`

## REPL & MCP UX Gaps

**Source:** `.planning/research/repl-mcp-ux.md`

### REPL critical issues

1. **No readline support** -- uses raw `bufio.Scanner`, no history, no line editing, no tab completion
2. **`navigate` creates new page every time** (calls `b.NewPage()` instead of `page.Navigate()`), losing cookies/localStorage
3. **No `WaitLoad` timeout** -- hangs indefinitely on SPAs (MCP has 15s timeout)
4. **`type` command can't handle selectors with spaces** -- `SplitN(line, " ", 3)` parsing limitation

### Capability matrix (what exists where)

| Capability | REPL | MCP | Gap owner |
|------------|------|-----|-----------|
| Accessibility snapshot | NO | YES | REPL |
| PDF generation | NO | YES | REPL |
| Full-page screenshot | NO | YES | REPL |
| Get page HTML | YES | NO | MCP |
| Get cookies | YES | NO | MCP |
| Reload page | YES | NO | MCP |
| Tab management | YES | NO | MCP |
| WaitStable/WaitIdle | NO | NO | Both |

### MCP fixes needed

- **Stale help text**: `cmd/scout/mcp.go:36-44` lists 33 tools but only 18 exist (15 migrated to plugins)
- **Error message inconsistency**: Three different prefix conventions across 18 tools
- **Input schemas are raw JSON strings** instead of typed Go structs with `jsonschema` tags

### Shared command executor opportunity

REPL `click` (repl.go:129-149) and MCP `click` (tools_browser.go:58-85) implement identical logic independently. A shared `Command` interface layer would eliminate duplication for ~10 overlapping operations and guarantee feature parity going forward.

## Recommended Phase Order

### Phase 1: Dead Code Removal & Safety Net (low risk, high impact)

**Rationale:** Largest line-count reduction with zero behavior change. Establishes safety boundaries before restructuring.

**Delivers:**
- Delete 124 unused Must* methods (-1100 lines)
- Delete `recipe.go` (post 2026-04-15, -664 lines)
- Delete `createProvider`, `FingerprintToProfile` dead exports
- Add `recover()` at gRPC handler entry (`grpc/server/server.go`) and MCP tool wrapper (`addTracedTool`)
- Convert 4 Scout-original panics to error returns
- Fix duplicate `truncate()` (delete websocket version, import from helpers or shared package)

**Avoids:** Panic crashes taking down gRPC server (Pitfall: 15 panic sites)

### Phase 2: Session Bug Fixes (correctness-critical)

**Rationale:** S1 (implicit reuse) is a data leak bug. Must fix before any session-related restructuring.

**Delivers:**
- Fix S1: Require explicit `WithReusableSession()` opt-in, don't auto-set from stale `scout.pid`
- Fix S2: Windows `ProcessAlive` to check `GetExitCodeProcess()`
- Fix S3: Eliminate double cleanup in non-reusable `Close()` (remove redundant `ResetSession` call)
- Fix S4: Increase Windows retry budget to 5x500ms with exponential backoff
- Fix S5: Remove double `CleanOrphans()` call
- Fix S6: Log warning on rod fallback

**Avoids:** Session state leaks, Windows cleanup failures

### Phase 3: CLI Consolidation (P0-P1 merges)

**Rationale:** Remove clear duplicates and merge overlapping commands. Low risk because deprecated commands have explicit deadlines and merge targets are supersets.

**Delivers:**
- Confirm `recipe.go` deleted (Phase 1)
- Extract shared `applyVars`/`findUnresolvedVars` to `helpers.go` or `runbook` package
- Merge `credentials` into `auth` with `--plaintext` flag
- Merge `websearch` + `search` (rename websearch to search, keep `--engine` flag)
- Deprecate `markdown` command (point to `fetch --mode=markdown`)
- Fix stale MCP help text (33 -> 18 tools)
- Normalize URL input convention (positional args everywhere)

**Avoids:** Further API surface sprawl

### Phase 4: REPL & MCP UX Improvements

**Rationale:** Depends on Phase 3 (shared helpers established). These are the user-facing quality improvements.

**Delivers:**
- Add readline library to REPL (`github.com/chzyer/readline`)
- Fix REPL `navigate` to reuse page (`page.Navigate()` instead of `b.NewPage()`)
- Add WaitLoad 15s timeout to REPL (match MCP)
- Add missing REPL commands: `snapshot`, `pdf`, `fullscreenshot`
- Add missing MCP tools: `markdown`, `html`, `cookies`, `reload`
- Standardize MCP error messages to `"scout-mcp: <tool>: <error>"`
- Migrate MCP input schemas to typed Go structs

**Research flag:** Readline library choice needs validation (chzyer/readline vs peterh/liner vs charmbracelet/bubbles).

### Phase 5: Code Structure Improvements (larger refactors)

**Rationale:** These are higher-effort changes that improve maintainability but don't fix bugs. Do after correctness and UX.

**Delivers:**
- Split `grpc/server/server.go` (1398 lines) by domain
- Split `browser/download.go` (1120 lines) into per-browser files
- Have `pkg/scout/browser/` delegate to `internal/engine/browser/` (-380 lines)
- Consolidate logging to `slog` only
- Group crawl commands under `crawl` parent (P2 CLI restructure)
- Group extraction commands under `extract` parent

**Research flag:** CLI grouping (crawl/extract parents) is a breaking change -- needs migration strategy research.

### Phase 6: Shared Command Executor (future)

**Rationale:** Maximum ROI only after REPL and MCP capabilities stabilize. Building the shared layer too early locks in the wrong abstractions.

**Delivers:**
- Shared `Command` interface for browser operations
- REPL and MCP both consume the same executor
- Feature parity guaranteed by construction
- Group gRPC-dependent commands under `page` parent (P3, breaking)

### Phase Ordering Rationale

- Phase 1 before Phase 2: Safety net (`recover()`) must exist before touching session lifecycle code
- Phase 2 before Phase 3: Session correctness bugs are higher severity than CLI naming issues
- Phase 3 before Phase 4: CLI merges create shared helpers that Phase 4 builds on
- Phase 4 before Phase 5: User-facing improvements before internal restructuring
- Phase 5 before Phase 6: File structure must stabilize before abstracting command executor layer

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Sessions/Isolation | HIGH | Direct code tracing, line-by-line analysis of all paths |
| CLI Overlap | HIGH | Exhaustive inventory of all 60+ commands with implementation comparison |
| Code Quality | HIGH | Quantified metrics: line counts, caller counts, grep-verified dead code |
| REPL/MCP UX | HIGH | Complete source read of both interfaces with capability matrix |

**Overall confidence:** HIGH -- all four research files are based on direct source code analysis, not external documentation or inference.

### Gaps to Address

- **Windows session cleanup timing**: The exact Chrome file lock duration on Windows is empirical. The 5x500ms recommendation may still be insufficient -- needs real-world testing.
- **Readline library selection**: Three viable options exist. Need to evaluate binary size impact and Windows compatibility before committing.
- **CLI breaking change strategy**: P2/P3 command grouping changes affect scripting users. Need to decide on alias duration and deprecation timeline.
- **Error prefix migration (657 calls)**: May not be worth doing all at once. Could be a lint rule enforced on new code only.
- **`pkg/scout/browser/` vs `internal/engine/browser/` delegation**: Need to verify that the public API signatures can wrap the internal API without breaking external consumers of `pkg/scout/browser/`.

## Sources

### Primary (HIGH confidence)
- Direct source code analysis of `internal/engine/browser.go`, `session/session_track.go`, `session/process_*.go`
- Direct source code analysis of all `cmd/scout/*.go` files (~80 files, ~200 cobra.Command definitions)
- Exhaustive `grep` of `panic(`, `utils.E(`, `Must` across entire codebase
- Complete read of `cmd/scout/repl.go` (454 lines) and `pkg/scout/mcp/` (all tool files)

---
*Research completed: 2026-03-29*
*Ready for roadmap: yes*
