# Phase 06: Shared Command Executor — Design

**Date:** 2026-05-16
**Status:** Planned
**Requirements:** SHARE-01, SHARE-02
**Depends on:** Phase 5

> Synthesized from ROADMAP.md. No phase directory exists yet.

---

## Design / Approach / Components

Build a unified command executor layer consumed by both REPL and MCP, permanently locking in capability parity.

### SHARE-01: Shared Command Executor
Create a new internal package (e.g., `internal/engine/executor/`) that implements all browser commands as pure functions:
```
executor.Navigate(ctx, browser, url) error
executor.Click(ctx, page, selector) error
executor.Extract(ctx, page, opts) (*ExtractResult, error)
executor.Screenshot(ctx, page, opts) ([]byte, error)
// ... all 20+ command types
```

Both REPL and MCP dispatch to this layer. Neither interface contains browser control logic — they are thin adapters that parse input and format output.

### SHARE-02: Unified Browser Control Flow
Standardize the open → navigate → interact → extract → close flow across REPL and MCP. Current state: REPL uses direct rod calls; MCP uses tool-specific implementations. After this phase, both use `executor.*` functions.

**Result:** Adding a new capability requires implementing it once in `executor/`, then exposing it in both REPL (one command registration) and MCP (one `AddTool` call).

---

## Rationale & Decisions

- The "zero-shared-logic problem": REPL and MCP were independently implemented, causing drift (REPL had commands MCP lacked, and vice versa). Phase 4 added parity; Phase 6 enforces it structurally.
- The shared executor is stateless with respect to the browser session — callers provide the `Browser`/`Page` they own. Session management stays in `internal/engine/session/`.
- Context propagation: all executor functions accept `context.Context` for cancellation and tracing (OpenTelemetry spans).
- Error handling: executor returns typed errors using the `scout: subsystem: message` convention from Phase 5.

---

## Constraints & Assumptions

- Phase 5 code structure changes (slog, error prefix, file splits) must be complete before executor refactor.
- Executor package must not import `cmd/scout/` or any REPL/MCP-specific package — it is a pure internal library.
- REPL and MCP become thin adapters: input parsing + `executor.*` call + output formatting. No browser logic in adapters.
- Plugin system (subprocess JSON-RPC) is out of scope — plugins have their own dispatch mechanism.
- All existing REPL and MCP integration tests must pass after the refactor (behavior unchanged).

---

## Testing & Acceptance

**Success Criteria (from ROADMAP):**
- `internal/engine/executor/` package exists with all browser commands implemented
- REPL `RunE` functions contain no direct rod/CDP calls — all routed through executor
- MCP tool handlers contain no direct rod/CDP calls — all routed through executor
- Adding a new command requires changes in exactly 3 places: executor implementation, REPL registration, MCP tool registration
- All existing tests pass (`task test`)
- `task check` (lint + vet) passes

**Testing approach:** Unit tests for executor functions using `newTestBrowser` (real Chromium); integration tests for REPL and MCP adapters via subprocess stdin/stdout and MCP JSON-RPC protocol respectively.
