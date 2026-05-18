# Phase 1: Safety Net & Dead Code - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 01-safety-net-dead-code
**Areas discussed:** Must* cleanup scope, Panic recovery strategy, Recipe removal approach, Dead export cleanup, Truncate dedup strategy, Panic site prioritization

---

## Must* Cleanup Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Delete all unused | Remove 124 of 134 Must* methods, keep ~10 with callers | |
| Keep as public API | Preserve all Must* methods for external consumers | |
| You handle it | Claude decides | ✓ |

**User's choice:** You handle it (selected alongside all areas)
**Decision:** Delete all 124 unused Must* methods. No deprecation period — breaking changes OK.

---

## Panic Recovery Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Middleware-level recover() | One wrapper each for gRPC interceptor, MCP tool wrapper, CLI RunE | |
| Per-handler recover() | Individual recover() in each handler function | |
| You handle it | Claude decides | ✓ |

**User's choice:** You handle it
**Decision:** Middleware-level approach — three recover() boundaries (gRPC, MCP, CLI). Minimal boilerplate, maximum coverage.

---

## Recipe Removal Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Clean delete | Remove recipe.go entirely, no stub | |
| Leave redirect stub | Print "use runbook instead" message | |
| You handle it | Claude decides | ✓ |

**User's choice:** You handle it
**Decision:** Clean delete. Past deprecation date. Delete duplicate applyVars/findUnresolvedVars too.

---

## Dead Export Cleanup

| Option | Description | Selected |
|--------|-------------|----------|
| Aggressive cleanup | Delete all dead exports (FingerprintToProfile, createProvider, doc.go) | |
| Conservative | Keep exports, just mark deprecated | |
| You handle it | Claude decides | ✓ |

**User's choice:** You handle it
**Decision:** Aggressive cleanup — delete all dead exports.

---

## Truncate Dedup Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Keep helpers.go version | Correct implementation that respects maxLen | |
| Keep tools_websocket.go version | Simpler but can exceed maxLen | |
| You handle it | Claude decides | ✓ |

**User's choice:** You handle it
**Decision:** Keep helpers.go version (correct). Delete tools_websocket.go version.

---

## Panic Site Prioritization

| Option | Description | Selected |
|--------|-------------|----------|
| Fix all 4 Scout-original | Convert all Scout-original panics to error returns | |
| Fix only reachable | Only fix panics reachable from gRPC/MCP/CLI | |
| You handle it | Claude decides | ✓ |

**User's choice:** You handle it
**Decision:** Fix all 4 Scout-original panics. Rod-inherited panics covered by middleware recover().

---

## Claude's Discretion

- File organization after must.go cleanup
- Task sequencing within the phase
- Whether to add unit tests for recover() middleware

## Deferred Ideas

None — discussion stayed within phase scope.
