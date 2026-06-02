# Phase 01: Safety Net & Dead Code — Design

**Date:** 2026-03-29
**Status:** In-progress
**Requirements:** CLEAN-01, CLEAN-02, CLEAN-03, CLEAN-04, CLEAN-06, MCP-04
**Depends on:** Nothing (first phase)

---

## Design / Approach / Components

Three parallel workstreams, each a standalone plan:

### Plan 01-01: Must* Method Cleanup (CLEAN-01)
Delete 124 of 134 `Must*` methods from `internal/engine/lib/must.go` — only the ~10 methods with actual callers are retained. The inherited rod fork pattern called `panic()` on failure; the unused methods are pure dead weight. Approach: static analysis to enumerate callers, bulk delete, `go vet` to confirm no unused exports remain.

### Plan 01-02: Panic-to-Error Conversion + Recover Middleware (CLEAN-02, CLEAN-03)
Two tasks:
1. Convert 4 Scout-original `panic()` sites to error returns in `browser_rod.go`, `manifest.go`, and `identity.go`.
2. Add middleware-level `recover()` at three entry-point boundaries: gRPC chain interceptor, MCP tool wrapper, CLI root `RunE`. Middleware approach chosen over per-handler to minimize boilerplate while maximizing coverage.

**Completed sub-tasks (from summaries):** All tasks completed with PASSED self-check. Auto-fixed issues during execution included minor lint cleanups. Recovery middleware now wraps all three boundaries.

### Plan 01-03: Recipe Removal + Dead Export Cleanup + Truncate Dedup (CLEAN-04, CLEAN-06, MCP-04)
- `recipe.go` deleted (past 2026-04-15 deprecation date); duplicate `applyVars`/`findUnresolvedVars` helpers removed (also existed in `runbook.go`)
- Dead exports `FingerprintToProfile` and `createProvider` removed
- Duplicate `truncate()` in `tools_websocket.go` removed; single canonical version retained

---

## Rationale & Decisions

- **Must* cleanup scope:** Only callers confirmed by static analysis are kept. 124 of 134 have zero callers — safe to delete. `go vet` is the correctness gate.
- **Panic recovery strategy:** Middleware-level chosen (not per-handler) — three `recover()` boundaries cover all production entry points with minimal code. gRPC uses `ChainStreamInterceptor`/`ChainUnaryInterceptor`; MCP wraps the tool dispatch function; CLI wraps the root cobra `RunE`.
- **Recipe removal approach:** Clean delete, no redirect stub. Past deprecation date (2026-04-15). Duplicate `applyVars`/`findUnresolvedVars` between `recipe.go` and `runbook.go` removed together.
- **Dead export cleanup:** `FingerprintToProfile` and `createProvider` confirmed zero callers; removed without replacement.
- **Truncate dedup:** `tools_websocket.go` version could exceed `maxLen`; the canonical version in another file is correct.

---

## Constraints & Assumptions

- Rod fork is internalized — `Must*` deletions are safe because no external rod consumers exist.
- `go vet` and `go build ./internal/... ./pkg/...` are the compile-time gates.
- No test file may use a deleted `Must*` method — any such test is also deleted or rewritten.
- Phase 2 (Sessions & Isolation) reads session/process code touched here; Phase 1 must complete cleanly before Phase 2 execution.
- Discussion log (audit only): Must* scope, panic recovery strategy, recipe removal, dead export cleanup, truncate dedup, panic site prioritization all decided by "Claude's discretion" at user's request.

---

## Testing & Acceptance

**Success Criteria:**
1. `must.go` contains only the ~10 `Must*` methods that have actual callers; `go vet` reports no unused exports.
2. A panic in any rod-inherited code path does not crash the gRPC server or MCP process — the handler returns an error instead.
3. `browser_rod.go`, `manifest.go`, and `identity.go` return errors instead of calling `panic()`.
4. `recipe.go` is deleted from the codebase.
5. The duplicate `truncate()` in `tools_websocket.go` is removed; one canonical version exists.

**Verification gates (from Plan 01-02 summary):**
- `go build ./internal/... ./pkg/...` passes
- All three recover() boundaries confirmed by code inspection
- Self-check: PASSED for Plans 01-02 and 01-03

**No VERIFICATION.md file present** — phase in-progress; Plans 01-01 and 01-03 execution status unknown from summaries alone. Plan 01-02 summary confirms PASSED.

---

## Review Notes

Plan 01-02 self-check PASSED. Auto-fixed issues during execution (minor lint). No deviations from plan intent.
