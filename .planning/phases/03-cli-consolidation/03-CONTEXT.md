# Phase 3: CLI Consolidation - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Merge overlapping CLI commands, remove deprecated ones, fix stale help text. Make the CLI surface coherent. Requirements: CLI-01 through CLI-07, CLEAN-07, MCP-01.

Note: CLI-01 (recipe removal) was completed in Phase 1. This phase handles the remaining 8 requirements.

</domain>

<decisions>
## Implementation Decisions

### gRPC Command Grouping (CLI-05)
- **D-01:** Move all 17 bare gRPC commands (click, type, title, url, screenshot, etc.) under a `scout grpc` subcommand group. E.g., `scout grpc click`, `scout grpc type`.
- **D-02:** The root-level aliases should be removed entirely. No deprecated aliases — breaking changes are acceptable.

### Search Merge (CLI-03)
- **D-03:** Merge `websearch` into `scout search` with an `--engine` flag (google, bing, ddg, etc.).
- **D-04:** Remove `scout websearch` command entirely.
- **D-05:** Remove `search_engines` subcommands (they were redundant with --engine flag).

### Auth/Credentials Merge (CLI-02)
- **D-06:** Unify all credential capture flows into `scout auth capture` with a `--format` flag for output format (cookies, headers, json).
- **D-07:** Remove `scout credentials` command entirely.
- **D-08:** Remove `scout profile capture` if it exists as a separate command — fold into `scout auth capture`.

### Markdown Removal (CLI-04)
- **D-09:** Remove standalone `scout markdown` command. `scout fetch --mode=markdown` is the canonical path. No deprecated alias.

### Extract Dedup (CLI-06)
- **D-10:** Consolidate duplicate `extract-*` subcommands alongside base commands. Claude determines the specific merge strategy.

### Screenshot Dedup (CLI-07)
- **D-11:** Consolidate screenshot command — one canonical version. If gRPC-based, it moves under `scout grpc screenshot`. Standalone screenshot functionality stays at root if it doesn't require daemon.

### Stale TODOs (CLEAN-07)
- **D-12:** Triage all stale TODOs inherited from rod fork. Fix the Go 1.15 tls.Dialer reference immediately. Close or document chromium-dependent ones as "upstream limitation".

### MCP Help Fix (MCP-01)
- **D-13:** Fix MCP server help text to show 18 tools (actual count after plugin migration), not 33.

### Claude's Discretion
- Specific file organization after command moves
- Whether to add backwards-compat warnings to stderr for removed commands (not required since breaking changes OK)
- How to handle any commands that import from moved packages
- Order of operations for the merges

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### CLI Overlap Research
- `.planning/research/cli-overlap.md` — Full command inventory, overlap analysis, merge proposals

### Codebase Structure
- `.planning/codebase/STRUCTURE.md` — Directory layout and command organization
- `.planning/codebase/ARCHITECTURE.md` — CLI layer description

### Key Source Files
- `cmd/scout/` — All CLI command files
- `cmd/scout/scout.go` — Root command, helpers
- `cmd/scout/helpers.go` — Shared baseOpts() helper
- `pkg/scout/mcp/server.go` — MCP tool registration (help text fix)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/scout/helpers.go` — `baseOpts()` shared across commands
- Cobra subcommand pattern — well-established for grouping

### Established Patterns
- One file per command group in cmd/scout/
- `baseOpts(cmd)` for common flags (headless, sandbox, browser, stealth)
- gRPC commands use `grpc/server` package for daemon communication

### Integration Points
- Root command in `cmd/scout/scout.go` — where subcommand groups are registered
- `pkg/scout/mcp/server.go` — tool count in help/description text

</code_context>

<specifics>
## Specific Ideas

No specific requirements — straightforward command merges guided by research.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-cli-consolidation*
*Context gathered: 2026-03-29*
