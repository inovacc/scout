# Requirements: Scout Stabilization

**Defined:** 2026-03-29
**Core Value:** Sessions must be rock-solid: open cleanly, close cleanly, never leak processes, and never touch the user's browser without explicit permission.

## v1 Requirements

### Sessions

- [ ] **SESS-01**: Sessions clean up all resources (process, PID file, data dir) on Browser.Close()
- [ ] **SESS-02**: CleanStaleSessions removes orphaned Chrome data directories, not just PID files
- [ ] **SESS-03**: New sessions never implicitly reuse stale session state (fix deterministic hash reuse bug)
- [ ] **SESS-04**: Windows ProcessAlive correctly detects terminated processes (fix OpenProcess false positives)
- [ ] **SESS-05**: Session close avoids redundant double cleanup (remove launcher.Cleanup + ResetSession overlap)
- [ ] **SESS-06**: Windows file lock retries are sufficient to outlast Chrome handle release

### Browser Isolation

- [ ] **ISOL-01**: Default browser resolution uses only ~/.scout/browsers/ cache, never system-installed browsers
- [ ] **ISOL-02**: Rod fallback path is eliminated or gated behind explicit opt-in (no silent ~/.cache/rod/ escape)
- [ ] **ISOL-03**: --system-browser flag is the only way to use system-installed browsers
- [ ] **ISOL-04**: Browser.BestCached() auto-downloads Chrome for Testing when cache is empty without touching system browsers

### CLI Consolidation

- [ ] **CLI-01**: Remove deprecated `scout recipe` command (past 2026-04-15 removal date)
- [ ] **CLI-02**: Merge `credentials capture` into `auth capture` as a single auth-capture flow
- [ ] **CLI-03**: Merge `websearch` into `search` with --engine flag (remove redundant search_engines subcommands)
- [ ] **CLI-04**: Remove standalone `markdown` command (subset of `fetch --mode=markdown`)
- [ ] **CLI-05**: Move 17 bare gRPC commands (click, type, title, url, etc.) under a `remote` or `grpc` subcommand group
- [ ] **CLI-06**: Consolidate duplicate extract-* subcommands alongside base commands
- [ ] **CLI-07**: Deduplicate screenshot command (gRPC-based root vs standalone)

### Code Cleanup

- [ ] **CLEAN-01**: Delete ~1,100 lines of unused Must* methods from must.go (124 of 134 have zero callers)
- [ ] **CLEAN-02**: Add recover() wrappers at gRPC, MCP, and CLI entry points to catch panics from rod-inherited code
- [ ] **CLEAN-03**: Convert 4 Scout-original panic sites to error returns (browser_rod.go, manifest.go, identity.go)
- [ ] **CLEAN-04**: Remove duplicate code between recipe.go and runbook.go (applyVars/findUnresolvedVars)
- [ ] **CLEAN-05**: Consolidate browser detection into internal/engine/browser/ with pkg/scout/browser/ delegating
- [ ] **CLEAN-06**: Remove dead exports (FingerprintToProfile, createProvider)
- [ ] **CLEAN-07**: Triage and resolve stale TODOs inherited from rod fork (especially Go 1.15 tls.Dialer reference)

### Code Structure

- [ ] **STRUCT-01**: Split files over 1,000 lines into cohesive sub-files (must.go after cleanup, others identified by research)
- [ ] **STRUCT-02**: Consolidate logging to slog (remove competing logging mechanisms)
- [ ] **STRUCT-03**: Establish consistent error prefix convention (scout: subsystem: message) across codebase

### REPL UX

- [ ] **REPL-01**: Add readline support (history, tab completion, line editing)
- [ ] **REPL-02**: Fix navigate command to reuse existing page instead of creating new one (preserve cookies/state)
- [ ] **REPL-03**: Add missing capabilities from MCP: snapshot, pdf, full-page screenshot, WebSocket tools
- [ ] **REPL-04**: Improve help system with command descriptions and usage examples

### MCP UX

- [ ] **MCP-01**: Fix stale help text claiming 33 tools (actual: 18 after plugin migration)
- [ ] **MCP-02**: Add missing capabilities from REPL: html, cookies, reload, tabs, markdown-as-tool
- [ ] **MCP-03**: Improve tool descriptions and input schemas for better AI agent ergonomics
- [ ] **MCP-04**: Fix duplicate truncate function (tools_websocket.go version can exceed maxLen)

### Shared Infrastructure

- [ ] **SHARE-01**: Create shared command executor consumed by both REPL and MCP (eliminate zero-shared-logic problem)
- [ ] **SHARE-02**: Unify browser control flow (open, navigate, interact, extract, close) across REPL and MCP

## v2 Requirements

### Error Handling

- **ERR-01**: Migrate 657 fmt.Errorf calls to consistent "scout: subsystem:" prefix convention
- **ERR-02**: Add structured error types for common failure modes (browser not found, session expired, CDP disconnected)

### CLI Polish

- **CLIP-01**: Consolidate 5 crawl-like commands (crawl, map, sitemap extract, knowledge, swarm) into unified crawl subcommand
- **CLIP-02**: Add shell completion generation for bash/zsh/fish/powershell

### Testing

- **TEST-01**: Add integration tests for session lifecycle (create, use, cleanup)
- **TEST-02**: Add Windows-specific tests for process detection and file lock handling

## Out of Scope

| Feature | Reason |
|---------|--------|
| New scraper modes or plugins | Stabilize first, no new features this milestone |
| New LLM provider integrations | Not related to stabilization |
| Performance optimization | Correctness before speed |
| Documentation rewrite | Docs update after code stabilizes |
| New protocol support | Not related to stabilization |
| Rod upstream sync | Internalized fork is the path forward |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| (populated during roadmap creation) | | |

**Coverage:**
- v1 requirements: 30 total
- Mapped to phases: 0 (pending roadmap)
- Unmapped: 30

---
*Requirements defined: 2026-03-29*
*Last updated: 2026-03-29 after initial definition*
