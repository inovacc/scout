# Scout — Stabilization Milestone: Overview Design

**Date:** 2026-03-29
**Status:** In-progress (Phase 3 completed; Phases 1, 2, 4, 5, 6 pending)
**Milestone:** v1.0 — Stabilization

---

## Vision

Transform Scout from a feature-rich but fragile codebase into a rock-solid browser automation library. Scout provides a CDP-based browser automation engine (internalized rod fork), a public Go facade at `pkg/scout/`, gRPC remote control with mTLS, an MCP server with 18 built-in tools, a plugin system, and a Cobra CLI with 50+ subcommands. The stabilization milestone addresses reliability, not features: eliminating dead code, hardening session lifecycle, consolidating the CLI, and unifying the REPL and MCP command layers.

---

## Requirements Summary

**Core Value:** Sessions must be rock-solid — open cleanly, close cleanly, never leak processes, and never touch the user's browser without explicit permission.

### Active Requirements (abbreviated)

| Area | Key Items |
|------|-----------|
| Sessions | SESS-01…SESS-06: cleanup resources, remove orphaned dirs, UUID-per-session, Windows ProcessAlive, single cleanup path |
| Browser Isolation | ISOL-01…ISOL-04: only ~/.scout/browsers/ cache; no rod fallback; --system-browser opt-in only |
| CLI Consolidation | CLI-01…CLI-07: remove recipe, merge auth/search, remove markdown, group gRPC commands, deduplicate screenshot |
| Code Cleanup | CLEAN-01…CLEAN-07: delete Must* dead code, recover() boundaries, panic-to-error, recipe dedup, browser detection consolidation |
| REPL UX | REPL-01…REPL-04: readline, html/cookies/reload/tabs commands, markdown tool, help improvements |
| MCP UX | MCP-01…MCP-04: fix tool count claim, add REPL-parity tools, improve schemas, fix truncate |
| Code Structure | STRUCT-01…STRUCT-03: split large files, consolidate logging, error prefix convention |
| Shared Infra | SHARE-01…SHARE-02: shared command executor, unified browser control flow |

### Constraints

- **Language:** Go 1.25 — no language changes
- **Architecture:** Keep layered pattern: `internal/engine/` → `pkg/scout/` → entry points
- **Testing:** Real browser + httptest; no mocks; Chromium required
- **Breaking changes:** Acceptable — clean API over backwards compat
- **Build:** Taskfile-based (`task build`, `task test`, `task check`)

---

## Architecture Overview

```
cmd/scout/              Unified Cobra CLI (50+ subcommands)
pkg/scout/              Public facade (functional options pattern)
  browser/              Browser type + options
  mcp/                  MCP server (18 tools)
  proxy/                API middleware proxy
  strategy/             Strategy/workflow executor
  agent/                AI agent framework (OpenAI/Anthropic schemas)
  monitor/              Visual regression testing
  scraper/              Scraper framework + 20 modes
  archive/              Archive utilities
internal/engine/        CDP automation engine (~100 non-test Go files)
  browser/              Browser detection + download management
  session/              Session lifecycle + cleanup
  lib/                  Internalized rod fork (CDP)
plugins/                12 standalone plugin binaries (subprocess JSON-RPC)
runbooks/               26 embedded preset runbooks
extensions/             Chrome extension (scout-bridge)
grpc/                   gRPC service definitions
```

**Key patterns:**
- Functional options (`WithBrowser()`, `WithReuseSession()`, etc.)
- CDP via internalized rod fork (not upstream rod)
- Session isolation: UUID v7 per `New()` call, `~/.scout/browsers/` only
- Recover middleware at gRPC/MCP/CLI boundaries (after Phase 1)
- Plugin system: subprocess JSON-RPC, 10 handler types

---

## Phase Index

| # | Slug | Status | Spec |
|---|------|--------|------|
| 1 | safety-net-dead-code | In-progress | [2026-03-29-01-safety-net-dead-code-design.md](2026-03-29-01-safety-net-dead-code-design.md) |
| 2 | sessions-isolation | In-progress | [2026-03-29-02-sessions-isolation-design.md](2026-03-29-02-sessions-isolation-design.md) |
| 3 | cli-consolidation | Completed | [2026-03-29-03-cli-consolidation-design.md](2026-03-29-03-cli-consolidation-design.md) |
| 4 | repl-mcp-ux | Planned | [2026-05-16-04-repl-mcp-ux-design.md](2026-05-16-04-repl-mcp-ux-design.md) |
| 5 | code-structure | Planned | [2026-05-16-05-code-structure-design.md](2026-05-16-05-code-structure-design.md) |
| 6 | shared-command-executor | Planned | [2026-05-16-06-shared-command-executor-design.md](2026-05-16-06-shared-command-executor-design.md) |

**Status breakdown:** Completed=1, In-progress=2, Planned=3

---

## Key Architectural Decisions

- **Internalized rod fork:** Rod is vendored/internalized; no upstream sync; Scout-original panics converted to errors in stabilization.
- **Session isolation policy:** UUID v7 per `New()` call by default; `WithReuseSession()` is the only opt-in for persistence. `WithReusableSession()` deprecated (removal 2026-05-01).
- **Browser isolation boundary:** `BestCached()` uses only `~/.scout/browsers/` cache; rod fallback (`~/.cache/rod/`) eliminated; system browsers require `--system-browser`.
- **Recover boundaries:** Three middleware-level `recover()` wrappers: gRPC interceptor, MCP tool wrapper, CLI root RunE.
- **Breaking changes acceptable:** CLI restructuring (gRPC commands under `scout grpc`, extract under `scout extract`) does not require migration paths.
- **Real-browser testing:** No mocks — `newTestBrowser` calls `t.Skipf` if Chromium unavailable.
