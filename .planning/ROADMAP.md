# Roadmap: Scout Stabilization

## Overview

Six phases transform Scout from a feature-rich but fragile codebase into a rock-solid library. The journey starts with a safety net (recover boundaries + dead code removal), then fixes the correctness bugs that matter most (sessions and isolation), then works outward to CLI consolidation, UX parity between REPL and MCP, internal restructuring, and finally a shared command executor that locks in parity permanently.

## Phases

- [ ] **Phase 1: Safety Net & Dead Code** - Remove 1,100+ lines of dead code and add panic recovery at entry points
- [ ] **Phase 2: Sessions & Isolation** - Fix all session lifecycle bugs and harden browser isolation
- [ ] **Phase 3: CLI Consolidation** - Remove deprecated commands and merge overlapping ones
- [ ] **Phase 4: REPL & MCP UX** - Close capability gaps and fix ergonomics in both interfaces
- [ ] **Phase 5: Code Structure** - Split large files, consolidate logging, deduplicate browser detection
- [ ] **Phase 6: Shared Command Executor** - Unified command layer consumed by both REPL and MCP

## Phase Details

### Phase 1: Safety Net & Dead Code
**Goal**: Eliminate dead code and ensure panics cannot crash long-running processes
**Depends on**: Nothing (first phase)
**Requirements**: CLEAN-01, CLEAN-02, CLEAN-03, CLEAN-04, CLEAN-06, MCP-04
**UI hint**: no

**Success Criteria** (what must be TRUE):
  1. `must.go` contains only the ~10 Must* methods that have actual callers; `go vet` reports no unused exports
  2. A panic in any rod-inherited code path does not crash the gRPC server or MCP process — the handler returns an error instead
  3. `browser_rod.go`, `manifest.go`, and `identity.go` return errors instead of calling `panic()`
  4. `recipe.go` is deleted from the codebase (past its 2026-04-15 removal date)
  5. The duplicate `truncate()` in `tools_websocket.go` is removed; one canonical version exists

**Plans**: 3 plans

Plans:
- [ ] 01-01-PLAN.md — Delete 124 unused Must* methods from must.go (CLEAN-01)
- [ ] 01-02-PLAN.md — Convert 4 Scout-original panics to errors + add recover() middleware at gRPC/MCP boundaries (CLEAN-02, CLEAN-03)
- [ ] 01-03-PLAN.md — Delete recipe.go, remove dead exports, fix truncate() duplicate (CLEAN-04, CLEAN-06, MCP-04)

### Phase 2: Sessions & Isolation
**Goal**: Sessions open cleanly, close cleanly, never leak processes, and never touch the user's browser without explicit permission
**Depends on**: Phase 1
**Requirements**: SESS-01, SESS-02, SESS-03, SESS-04, SESS-05, SESS-06, ISOL-01, ISOL-02, ISOL-03, ISOL-04
**UI hint**: no

**Success Criteria** (what must be TRUE):
  1. Running the same URL twice in separate `Browser` instances never shares cookies, localStorage, or session state between runs
  2. After `Browser.Close()`, no orphaned Chrome processes or data directories remain under `~/.scout/sessions/`
  3. On Windows, `ProcessAlive` correctly reports a terminated process as dead (no zombie false-positives blocking cleanup)
  4. The default `New()` call never opens a system-installed browser — only `~/.scout/browsers/` cache or auto-downloaded Chrome for Testing
  5. `--system-browser` is the sole opt-in path to system browsers; rod fallback is gated or eliminated

**Plans**: 5 plans

Plans:
- [ ] 02-01-PLAN.md — Write Wave 0 test scaffolds (all 10 requirements, RED state) (SESS-01..06, ISOL-01..03)
- [x] 02-02-PLAN.md — Fix Windows ProcessAlive with WaitForSingleObject; add retry constants 5x500ms (SESS-04, SESS-06)
- [x] 02-03-PLAN.md — Replace SessionHash with UUID v7; add WithReuseSession() opt-in (SESS-03)
- [ ] 02-04-PLAN.md — Consolidate Browser.Close() to single cleanup path; fix CleanOrphans to remove full dirs (SESS-01, SESS-02, SESS-05)
- [x] 02-05-PLAN.md — Eliminate rod fallback; explicit error when no cached browser (ISOL-01, ISOL-02, ISOL-03, ISOL-04)

### Phase 3: CLI Consolidation
**Goal**: The CLI surface is coherent — no duplicate commands, no deprecated commands, and overlapping commands are merged
**Depends on**: Phase 2
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04, CLI-05, CLI-06, CLI-07, CLEAN-07, MCP-01
**UI hint**: no

**Success Criteria** (what must be TRUE):
  1. `scout recipe` no longer exists; `scout runbook` is the single entry point for runbook operations
  2. `scout auth capture` handles all credential capture flows; `scout credentials` is removed
  3. `scout search --engine <name>` replaces both `scout search` and `scout websearch`; the old `search_engines` subcommands are gone
  4. `scout markdown` is removed; `scout fetch --mode=markdown` is the documented path
  5. The MCP server's `--help` output lists 18 tools, not 33

**Plans**: 5 plans

Plans:
- [ ] 03-01-PLAN.md � Delete recipe.go + merge credentials into auth (CLI-01, CLI-02)
- [ ] 03-02-PLAN.md � Merge websearch into search, remove search_engines subcommands (CLI-03)
- [ ] 03-03-PLAN.md � Remove markdown command, group table/meta/extract-ai under extract (CLI-04, CLI-06)
- [ ] 03-04-PLAN.md � Group 17 gRPC commands under scout grpc, consolidate screenshot (CLI-05, CLI-07)
- [ ] 03-05-PLAN.md � Fix tls.Dialer TODO, fix MCP tool count description (CLEAN-07, MCP-01)

### Phase 4: REPL & MCP UX
**Goal**: REPL and MCP have matching core capabilities and both are pleasant to use
**Depends on**: Phase 3
**Requirements**: REPL-01, REPL-02, REPL-03, REPL-04, MCP-02, MCP-03
**UI hint**: no

**Success Criteria** (what must be TRUE):
  1. The REPL has command history (up-arrow), tab completion, and line editing without restarting
  2. `navigate <url>` in an active REPL session preserves cookies and page state (does not open a new tab)
  3. `snapshot`, `pdf`, and `fullscreenshot` work in the REPL; `html`, `cookies`, `reload`, and `markdown` work as MCP tools
  4. REPL `help` shows a description and usage example for every command
  5. MCP tool input schemas use typed Go structs with `jsonschema` tags; AI agents receive accurate schema information

**Plans**: TBD

### Phase 5: Code Structure
**Goal**: Large files are split into cohesive units, logging is unified, and browser detection has one implementation
**Depends on**: Phase 4
**Requirements**: STRUCT-01, STRUCT-02, STRUCT-03, CLEAN-05
**UI hint**: no

**Success Criteria** (what must be TRUE):
  1. No non-test Go file in the codebase exceeds 1,000 lines
  2. All log output in library code goes through `slog`; `fmt.Fprint(os.Stderr)`, `log.Printf`, and `utils.Log` calls are eliminated from `internal/` and `pkg/`
  3. `pkg/scout/browser/` contains no duplicated detection or download logic — it delegates entirely to `internal/engine/browser/`
  4. New errors added during this phase follow the `"scout: subsystem: message"` prefix convention

**Plans**: TBD

### Phase 6: Shared Command Executor
**Goal**: REPL and MCP share a single command executor layer, guaranteeing permanent feature parity
**Depends on**: Phase 5
**Requirements**: SHARE-01, SHARE-02
**UI hint**: no

**Success Criteria** (what must be TRUE):
  1. A shared `Command` interface or executor package exists that both REPL and MCP consume for the ~10 overlapping browser operations
  2. Adding a new browser operation to the executor automatically makes it available in both REPL and MCP without duplicating implementation
  3. The REPL and MCP pass the same integration tests for all shared operations

**Plans**: TBD

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Safety Net & Dead Code | 0/3 | Not started | - |
| 2. Sessions & Isolation | 3/5 | In Progress|  |
| 3. CLI Consolidation | 0/TBD | Not started | - |
| 4. REPL & MCP UX | 0/TBD | Not started | - |
| 5. Code Structure | 0/TBD | Not started | - |
| 6. Shared Command Executor | 0/TBD | Not started | - |
