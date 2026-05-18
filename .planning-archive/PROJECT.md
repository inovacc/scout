# Scout — Stabilization Milestone

## What This Is

Scout is a Go browser automation library with an internalized rod fork, public facade API, gRPC service, MCP server, plugin system, REPL, and 50+ CLI commands. It provides headless and headed browser control for scraping, testing, monitoring, and AI agent integration.

## Core Value

Sessions must be rock-solid: open cleanly, close cleanly, never leak processes, and never touch the user's browser without explicit permission.

## Requirements

### Validated

- Browser automation via internalized rod fork (CDP) — existing
- Public facade at `pkg/scout/` with functional options — existing
- gRPC remote browser control with mTLS — existing
- MCP server with 18 built-in tools — existing
- Plugin system (subprocess JSON-RPC, marketplace, SDK) — existing
- CLI with 50+ Cobra subcommands — existing
- Stealth/anti-detection evasions — existing
- Session tracking with gops process detection — existing
- Swarm distributed crawling — existing
- REPL interactive shell (20 commands) — existing
- Scraper framework with 20+ modes — existing
- Browser detection and download management — existing
- Fingerprint rotation strategies — existing
- Research presets with caching — existing
- Health check / test-site functionality — existing
- Visual regression monitoring — existing
- AI agent framework (OpenAI/Anthropic tool schemas) — existing
- Runbook system (extract, automate, plan/apply) — existing
- WebSocket capture and HAR export — existing
- Cloud upload (Google Drive, OneDrive) — existing
- OpenTelemetry tracing — existing
- Electron support — existing
- Mobile automation (ADB, touch emulation) — existing

### Active

- [ ] Fix session lifecycle: sessions leak, don't clean up on exit
- [ ] Browser isolation by default: never use user's system browser unless explicitly requested
- [ ] Enhance REPL UX: better interaction flow, discoverability
- [ ] Enhance MCP UX: better tool ergonomics and feedback
- [ ] Merge overlapping CLI commands: reduce 50+ to a coherent set
- [ ] Split large files (must.go 1267 lines, others)
- [ ] Remove dead code and deprecated commands (recipe, deprecated flags)
- [ ] Consistent error handling: convert panics to error returns
- [ ] Clean stale TODOs inherited from rod fork
- [ ] Consolidate duplicate browser detection code (internal + pkg)
- [ ] Consistent patterns across codebase

### Out of Scope

- New features (scraper modes, plugins, protocols) — stabilize first
- New integrations or LLM providers — not this milestone
- Performance optimization — correctness before speed
- Documentation rewrite — docs update after code stabilizes
- Backwards compatibility — breaking changes acceptable for clean API

## Context

- Brownfield Go project (~2038 lines of codebase analysis in `.planning/codebase/`)
- Core engine has ~100 non-test Go files in `internal/engine/`
- Must-panic pattern (1267 lines in `must.go`) inherited from rod fork
- Multiple panic sites in library code that can crash gRPC/MCP/CLI
- Duplicate browser detection in `internal/engine/browser/` and `pkg/scout/browser/`
- `scout recipe` deprecated, removal date 2026-04-15 (imminent)
- Stale TODOs referencing Go 1.15, upstream Chrome bugs
- Session cleanup runs on every invocation but sessions still leak
- Browser isolation partially implemented (`BestCached()` auto-downloads) but system browsers still reachable by default

## Constraints

- **Language**: Go 1.25 — no language changes
- **Architecture**: Keep layered pattern (internal engine -> pkg facade -> entry points)
- **Testing**: Real browser + httptest, no mocks. Tests require Chromium available
- **Breaking changes**: Acceptable — clean API over backwards compat
- **Build**: Taskfile-based. `task build`, `task test`, `task check`

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Deep restructure over surgical fixes | User wants consistent patterns, not patches | -- Pending |
| Breaking changes allowed | Clean API is priority over backwards compat | -- Pending |
| Sessions & isolation first | Foundational stability before UX or cleanup | -- Pending |
| Analyze CLI overlaps automatically | User defers to codebase analysis for merge decisions | -- Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? -> Move to Out of Scope with reason
2. Requirements validated? -> Move to Validated with phase reference
3. New requirements emerged? -> Add to Active
4. Decisions to log? -> Add to Key Decisions
5. "What This Is" still accurate? -> Update if drifted

**After each milestone:**
1. Full review of all sections
2. Core Value check -- still the right priority?
3. Audit Out of Scope -- reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-03-29 after initialization*
