# Phase 1: Safety Net & Dead Code - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Eliminate dead code and ensure panics cannot crash long-running processes (gRPC server, MCP server, CLI). This phase creates a safety net before touching session lifecycle in Phase 2.

Requirements: CLEAN-01, CLEAN-02, CLEAN-03, CLEAN-04, CLEAN-06, MCP-04

</domain>

<decisions>
## Implementation Decisions

### Must* Method Cleanup (CLEAN-01)
- **D-01:** Delete all 124 unused Must* methods from `internal/engine/must.go`. Keep only the ~10 methods with actual callers. File should shrink from 1267 lines to ~150 lines.
- **D-02:** No deprecation period needed — breaking changes are acceptable this milestone. If any external consumer exists, they can use the error-returning versions.

### Panic Recovery Boundaries (CLEAN-02)
- **D-03:** Add `recover()` at three middleware-level boundaries:
  1. gRPC unary/stream interceptor — wraps all RPC handlers
  2. MCP `addTracedTool` wrapper — wraps all 18 MCP tool handlers
  3. CLI `RunE` wrapper — wraps all Cobra command handlers
- **D-04:** Recovered panics become error returns with stack trace logged via slog. The error message should include the panic value and the originating file:line.
- **D-05:** Do NOT add per-function recover() — middleware approach minimizes boilerplate.

### Scout-Original Panic Conversion (CLEAN-03)
- **D-06:** Convert all 4 Scout-original panic sites to error returns:
  - `internal/engine/browser_rod.go:160` — conflicting config panic → return error
  - `internal/engine/browser_rod.go:380` — double wait panic → return error
  - `internal/engine/browser/manifest.go:76` — JSON parse panic → return error
  - `pkg/scout/identity/identity.go:97` — luhnify panic → return error
- **D-07:** The 11 rod-inherited panic sites are covered by the recover() middleware (D-03). No code changes needed for those in this phase.

### Recipe Removal (CLEAN-04)
- **D-08:** Clean delete `cmd/scout/recipe.go` entirely — no stub, no redirect message. The 2026-04-15 deprecation date has passed.
- **D-09:** Delete duplicate `applyVars` and `findUnresolvedVars` functions from recipe.go. The canonical versions live in runbook.go.
- **D-10:** Verify no internal references to `recipe` remain after deletion (grep for "recipe" across codebase).

### Dead Export Cleanup (CLEAN-06)
- **D-11:** Delete `FingerprintToProfile` — test-only export. Move to test file if any test depends on it.
- **D-12:** Delete `createProvider` (has `nolint:unused` annotation — confirms it's dead).
- **D-13:** Delete deprecated root `doc.go` — the redirect notice has served its purpose.

### Truncate Deduplication (MCP-04)
- **D-14:** Keep `truncate()` from `helpers.go` (correct implementation that respects maxLen). Delete the version in `tools_websocket.go` and import from helpers.
- **D-15:** Verify no other duplicate utility functions exist across MCP tool files.

### Claude's Discretion
- File organization after must.go cleanup — Claude decides how to structure remaining Must* methods
- Order of operations within the phase — Claude decides task sequencing
- Whether to add unit tests for the recover() middleware — recommended but Claude decides scope

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Dead Code Analysis
- `.planning/research/code-quality.md` — Full inventory of Must* callers, panic sites, dead exports, duplicate code

### Session & Isolation Context (Phase 2 dependency)
- `.planning/research/sessions-isolation.md` — Session bugs that Phase 2 will fix (this phase creates the safety net first)

### REPL/MCP Analysis
- `.planning/research/repl-mcp-ux.md` — Details on duplicate truncate function and MCP tool registration patterns

### Codebase Structure
- `.planning/codebase/ARCHITECTURE.md` — Layer boundaries (CLI -> facade -> engine -> lib)
- `.planning/codebase/CONCERNS.md` — Full list of tech debt items with file paths and line numbers

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/scout/mcp/server.go` has `addTracedTool()` wrapper — ideal hook point for MCP recover() boundary
- `grpc/server` already has interceptor registration — add recover() interceptor there
- `cmd/scout/helpers.go` has shared `baseOpts()` — similar pattern for wrapping RunE

### Established Patterns
- Error wrapping: `fmt.Errorf("scout: subsystem: %w", err)` — use this for recovered panic errors
- Logging: `slog` is the designated logger — log recovered panics at Error level
- Nil-safety: existing pattern of nil-safe Close() methods — maintain consistency

### Integration Points
- `cmd/scout/scout.go` — root command where CLI-level recover wrapper would be added
- `grpc/server` — gRPC server setup where interceptor is registered
- `pkg/scout/mcp/server.go` — MCP tool registration where tool wrapper wraps handlers

</code_context>

<specifics>
## Specific Ideas

No specific requirements — straightforward cleanup phase with clear targets from research.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 01-safety-net-dead-code*
*Context gathered: 2026-03-29*
