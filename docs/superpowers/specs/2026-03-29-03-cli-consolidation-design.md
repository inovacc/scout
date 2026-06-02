# Phase 03: CLI Consolidation — Design

**Date:** 2026-03-29
**Status:** Completed
**Requirements:** CLI-01, CLI-02, CLI-03, CLI-04, CLI-05, CLI-06, CLI-07, CLEAN-07, MCP-01
**Depends on:** Nothing (parallel-safe with Phase 1; completed before Phase 2)

---

## Design / Approach / Components

Five plans, all executed and verified:

### Plan 03-01: Merge credentials into auth (CLI-02)
`credentials capture` merged into `auth capture` as a single auth-capture flow. Top-level `credentials` command removed. Completed — self-check PASSED.

### Plan 03-02: Merge websearch into search (CLI-03)
`websearch` command merged into `search` with `--engine` flag. Redundant `search_engines` subcommands removed. `wikipedia` subcommand retained under `search`. `--fetch` flag added. Build verified: no `websearchCmd` references remain. Completed — verification passed.

### Plan 03-03: Remove markdown + group extract subcommands (CLI-04, CLI-06)
- `scout markdown` command removed (subset of `scout fetch --mode=markdown`)
- `scout extract parent` created; `table`, `meta`, `ai` (formerly `extract-ai` from `llm.go`) grouped under it
- `scout table`, `scout meta`, `scout extract-ai` removed as top-level commands
- Completed — self-check PASSED

### Plan 03-04: gRPC command grouping + screenshot dedup (CLI-05, CLI-07)
- 17 bare gRPC commands (click, type, title, url, etc.) moved under `scout grpc` subcommand group
- `screenshot.go` (gRPC-based, no URL) moved to `scout grpc screenshot` and `scout grpc pdf`
- `mcp_screenshot.go` (standalone, takes URL) promoted to root `scout screenshot`
- MCP server description corrected from "33 tools" to "18 tools" (MCP-01) — or phrased as "browser automation tools"
- Completed — self-check PASSED; `go build ./cmd/scout/` and `go build ./pkg/scout/mcp/` verified

### Plan 03-05: tls.Dialer fix + MCP tool count correction (CLEAN-07, MCP-01)
- Stale Go 1.15 `tls.Dialer` TODO in `internal/engine/lib/cdp/utils.go` resolved with real implementation
- MCP server description "33 tools" claim corrected
- Build verified: `grep "golang v1.15" cdp/utils.go` and `grep "33 tools" mcp/server.go` return empty
- Completed — self-check PASSED

---

## Rationale & Decisions

- **Breaking changes acceptable:** `scout click`, `scout type`, etc. now return "unknown command" — users must use `scout grpc click`. No alias needed (project policy: clean API over backwards compat).
- **Extract grouping:** `extract-ai` in `llm.go` promoted under `extractCmd` by variable reference (both in `main` package, no circular import). Renamed Use field to `"ai"` for ergonomics.
- **Screenshot dedup:** gRPC version (session-attached, no URL) under `scout grpc screenshot`; standalone version (launches own browser, takes URL) at `scout screenshot`. Cleanest split.
- **MCP tool count:** `addTracedTool` call count confirmed at 18; hard-coded "33" was stale from pre-plugin-migration.
- **tls.Dialer:** Go 1.15 added `tls.Dialer`; stale TODO referencing lack of it replaced with real implementation.

---

## Constraints & Assumptions

- `task build` (`go build ./cmd/scout/` and `go build ./pkg/...`) is the primary compile gate.
- No alias commands for removed top-level names.
- `scout --help` is visibly shorter post-consolidation (17 commands moved under `scout grpc`).
- CLI-01 (remove `scout recipe`) was listed as a Phase 3 requirement but appears deferred — `recipe.go` removal is in Phase 1 Plan 01-03; the CLI command removal follows.

---

## Testing & Acceptance

**Verification gates (all passed):**
- `go build ./cmd/scout/` exits 0
- `go build ./pkg/scout/mcp/` exits 0
- `scout click` produces "unknown command"
- `scout grpc click --help` works
- `scout grpc screenshot --help` works
- `scout screenshot --help` works (standalone URL version)
- `scout markdown` produces "unknown command"
- `scout table` produces "unknown command"
- `scout extract --help` lists: table, meta, ai
- `grep "33 tools\|33 tool" pkg/scout/mcp/server.go` returns no matches
- `grep "golang v1.15" internal/engine/lib/cdp/utils.go` returns empty
- `grep "websearchCmd\|AddCommand.*websearch" cmd/scout/` returns no matches

---

## Review Notes

All 5 plans self-check PASSED. Deviations: Plan 03-02 — minor auto-fixes during execution; Plan 03-05 — tls.Dialer implementation chose real fix over comment removal (deviation noted and accepted). No regressions reported.
