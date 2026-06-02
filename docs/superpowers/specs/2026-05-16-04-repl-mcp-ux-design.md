# Phase 04: REPL & MCP UX — Design

**Date:** 2026-05-16
**Status:** Planned
**Requirements:** REPL-01, REPL-02, REPL-03, REPL-04, MCP-02, MCP-03, MCP-04
**Depends on:** Phase 2 (Sessions & Isolation)

> Synthesized from ROADMAP.md. No phase directory exists yet.

---

## Design / Approach / Components

Close capability gaps and fix ergonomics in both the REPL interactive shell and the MCP server.

### REPL UX (REPL-01 through REPL-04)
- **REPL-01:** Add readline support — history, tab completion, line editing. Current REPL is a raw scanner with no line editing.
- **REPL-02:** Add missing REPL commands to reach parity: `html`, `cookies`, `reload`, `tabs`.
- **REPL-03:** Expose `markdown` as a REPL command (currently REPL has 20 commands; markdown is fetch-mode only).
- **REPL-04:** Improve REPL help output — command descriptions, usage examples.

### MCP UX (MCP-02, MCP-03, MCP-04)
- **MCP-02:** Add missing capabilities from REPL: `html`, `cookies`, `reload`, `tabs`, `markdown-as-tool`.
- **MCP-03:** Improve tool descriptions and input schemas for better AI agent ergonomics — clearer parameter names, required vs optional, enum values.
- **MCP-04:** Fix duplicate `truncate` function in `tools_websocket.go` (version can exceed `maxLen`). Note: may overlap with Phase 1 Plan 01-03 — verify not already resolved.

---

## Rationale & Decisions

- REPL and MCP share zero logic today (the "zero-shared-logic problem" — addressed fully in Phase 6). This phase improves each interface independently before the unified executor is built.
- Readline library selection: to be determined at spec time (options: `github.com/chzyer/readline`, `github.com/peterh/liner`, or `golang.org/x/term`).
- MCP tool schema improvements target AI agent consumers — descriptions must be unambiguous for LLM tool selection.

---

## Constraints & Assumptions

- REPL runs in `scout repl [url]` — standalone local browser shell, no daemon required.
- MCP server has 18 built-in tools after Phase 3 corrections.
- Phase 6 (Shared Command Executor) will supersede some of this phase's per-interface implementations. REPL-02/MCP-02 additions must be designed with the shared executor in mind.
- MCP-04 truncate fix: verify against Phase 1 Plan 01-03 completion before re-implementing.

---

## Testing & Acceptance

**Success Criteria (from ROADMAP):**
- REPL supports readline (history, tab completion, line editing)
- `html`, `cookies`, `reload`, `tabs`, `markdown` commands available in REPL
- MCP exposes html, cookies, reload, tabs, markdown as tools
- MCP tool descriptions score higher on AI agent tool-selection accuracy
- `truncate()` in `tools_websocket.go` respects `maxLen` in all code paths

**Testing approach:** Real browser + httptest; `newTestBrowser` skips if Chromium unavailable. REPL commands tested via subprocess stdin/stdout simulation.
