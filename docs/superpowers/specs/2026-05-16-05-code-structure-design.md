# Phase 05: Code Structure — Design

**Date:** 2026-05-16
**Status:** Planned
**Requirements:** STRUCT-01, STRUCT-02, STRUCT-03, CLEAN-05
**Depends on:** Phase 4

> Synthesized from ROADMAP.md. No phase directory exists yet.

---

## Design / Approach / Components

Internal restructuring to make the codebase maintainable at scale.

### STRUCT-01: Split Large Files
Files over 1,000 lines split into cohesive sub-files. Primary candidates:
- `must.go` (after Phase 1 cleanup — remaining ~10 methods may stay, but the file had 1,267 lines pre-cleanup)
- Other large files identified by research (codebase analysis in `.planning/codebase/`)

Split strategy: group by semantic responsibility, not line count. Each sub-file gets a single clear purpose.

### STRUCT-02: Consolidate Logging to slog
Remove competing logging mechanisms. Scout has at least two logging approaches; consolidate to `log/slog` with structured JSON output. All log calls use `slog.Info`, `slog.Error`, etc. with key-value pairs. Logger writes to stderr (MCP stdout = JSON-RPC wire).

### STRUCT-03: Error Prefix Convention
Establish consistent error prefix convention: `scout: subsystem: message` across the codebase. 657 `fmt.Errorf` calls identified for migration in v2 requirements (ERR-01). Phase 5 establishes the convention and migrates the highest-impact paths; full migration is ERR-01/ERR-02 (v2).

### CLEAN-05: Browser Detection Consolidation
Consolidate browser detection into `internal/engine/browser/` with `pkg/scout/browser/` delegating to it. Currently duplicated: the engine package and the public facade both contain browser detection logic.

---

## Rationale & Decisions

- Logging consolidation is a prerequisite for reliable structured telemetry (OpenTelemetry tracing already uses slog; competing loggers create noise).
- Error prefix convention improves debuggability — callers can pinpoint the originating subsystem from any error message.
- Browser detection duplication created subtle behavioral divergence; single source of truth in `internal/engine/browser/` is the authoritative path.
- File splits are refactors, not behavior changes — all existing tests must pass before and after.

---

## Constraints & Assumptions

- Go 1.25: `log/slog` is stdlib — no new dependency for logging consolidation.
- Phase 6 (Shared Command Executor) builds on the clean structure established here.
- ERR-01 (657 `fmt.Errorf` migrations) is v2 scope — Phase 5 only establishes the convention and migrates highest-impact paths.
- File splits must preserve all exported API signatures exactly.

---

## Testing & Acceptance

**Success Criteria (from ROADMAP):**
- No source file over 1,000 lines (post-cleanup)
- All logging goes through `slog` — no competing logging calls
- `scout: subsystem: message` prefix present on all new error paths
- `pkg/scout/browser/` delegates to `internal/engine/browser/` — no duplicate detection logic
- All existing tests pass after structural changes (`task test`)
