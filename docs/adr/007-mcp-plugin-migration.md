# ADR-007: MCP Tool Migration to CLI Command Plugins

**Date:** 2026-03-16
**Status:** Proposed
**Context:** Phase 63 (CLI Command Plugin Capability) enables plugins to provide CLI commands. This ADR defines the strategy to migrate MCP tools from the monolithic `pkg/scout/mcp/` package into standalone plugins, reducing binary size and enabling independent release cycles.

## Decision

Migrate MCP tools in 5 waves over 5 phases, using the `cli_command` + `mcp_tool` dual-capability plugin pattern. Each plugin binary is both a CLI command and an MCP tool provider.

## Icon Identity

Scout's brand identity was generated via IconForge:
- Source: `build/icons/scout.svg`
- Colors: Navy (#1E3A5F) → Steel Blue (#4A90D9) gradient, Amber (#F5A623) accent
- Assets: `build/icons/{png,windows,macos,linux,favicon}/`
- Plugin icons should derive from the same palette with capability-specific accent colors

## Architecture: Dual-Capability Plugins

Each migrated plugin provides **both** `cli_command` and `mcp_tool` capabilities from a single binary. This means:
- `scout search "query"` invokes the CLI command path (via `command/execute` RPC)
- MCP clients call the `search` tool (via `tool/call` RPC)
- Same binary, same logic, two entry points

```
plugin.json:
{
  "name": "scout-search",
  "version": "1.0.0",
  "command": "./scout-search",
  "capabilities": ["cli_command", "mcp_tool"],
  "commands": [...],
  "tools": [...]
}
```

## What Stays Built-In (NOT migrated)

These remain in `pkg/scout/mcp/server.go` permanently:

| Tool | Reason |
|------|--------|
| `navigate` | Core browser primitive — all other tools depend on it |
| `click` | Core browser primitive |
| `type` | Core browser primitive |
| `extract` | Core browser primitive |
| `eval` | Core browser primitive |
| `back` / `forward` | Core browser primitive |
| `wait` | Core browser primitive |
| `screenshot` | Core capture — essential for LLM vision |
| `snapshot` | Core capture — essential for LLM accessibility |
| `session_list` / `session_reset` | Session lifecycle management |
| `open` | Headed browser launch — process lifecycle |

**Total: 12 tools stay built-in** (the browser primitives + session management).

## Migration Waves

### Wave 1 — Diagnostics & Reports (Low Risk)

**Tools:** `ping`, `curl`, `report_list`, `report_show`, `report_delete`
**Source files:** `diag.go`, `tools_report.go`
**Risk:** None — no browser state dependency, pure request/response
**Plugin:** `scout-diag` (ping, curl) + `scout-reports` (report_list/show/delete)

**Steps:**
1. Create `plugins/scout-diag/main.go` with `CommandHandler` for ping/curl
2. Create `plugins/scout-diag/plugin.json` with dual capabilities
3. Verify identical output vs built-in
4. Add deprecation warning to built-in diag.go tools
5. After 30 days: remove `diag.go`, `tools_report.go` from `pkg/scout/mcp/`

### Wave 2 — Content Extraction (Medium Risk)

**Tools:** `markdown`, `table`, `meta`, `pdf`
**Source files:** `tools_content.go`, `tools_capture.go` (pdf only)
**Risk:** Medium — requires browser, but stateless page reads
**Plugin:** `scout-content` with `requires_browser: true`

**Steps:**
1. Create `plugins/scout-content/main.go` — receives CDP endpoint, connects browser, extracts content
2. Test: `scout markdown https://example.com` via plugin matches built-in output
3. MCP tool registration: `markdown`, `table`, `meta`, `pdf` all in one plugin
4. Deprecation warning in built-in `registerCaptureTools`/`registerContentTools`
5. After 30 days: remove from monolith

### Wave 3 — Search & Fetch (Medium Risk)

**Tools:** `search`, `search_and_extract`, `fetch`
**Source files:** `tools_search.go`
**Risk:** Medium — search uses browser for DuckDuckGo, fetch uses browser for JS rendering
**Plugin:** `scout-search` with `requires_browser: true`

**Steps:**
1. Create `plugins/scout-search/main.go`
2. Handle both browser-based and HTTP-only fetch modes
3. Test parallel fetch in `search_and_extract` works via CDP forwarding
4. Deprecation + removal after 30 days

### Wave 4 — Network & Forms (Higher Risk)

**Tools:** `cookie`, `header`, `block`, `form_detect`, `form_fill`, `form_submit`, `storage`, `hijack`, `har`, `swagger`
**Source files:** `tools_network.go`, `tools_form.go`, `tools_inspect.go`
**Risk:** Higher — these mutate browser state (cookies, headers, storage)
**Plugins:** `scout-network` (cookie, header, block, storage, hijack, har, swagger) + `scout-forms` (form_detect, form_fill, form_submit)

**Key challenge:** State mutation via CDP must be coordinated. The plugin receives the CDP endpoint and can set cookies/headers directly on the browser. But if multiple plugins share a browser, mutations can conflict.

**Mitigation:** Browser context isolation. Each plugin command invocation that mutates state gets its own browser context (incognito tab group) unless `--shared-session` is passed.

**Steps:**
1. Create both plugins with `requires_browser: true`
2. Add `session_dir` sharing for cookie persistence
3. Test cookie set → navigate → cookie read workflow
4. Deprecation + removal after 30 days

### Wave 5 — Analysis & Guides (Highest Risk)

**Tools:** `crawl`, `detect`, `swarm_crawl`, `guide_start`, `guide_step`, `guide_finish`
**Source files:** `tools_analysis.go`, `tools_swarm.go`, `tools_guide.go`
**Risk:** Highest — crawl and swarm manage multiple pages, guide is stateful (recorder)
**Plugins:** `scout-crawl` (crawl, detect, swarm_crawl) + `scout-guide` (guide_start, guide_step, guide_finish)

**Key challenge:** Guide is a multi-step stateful workflow. The recorder state must persist across `guide_start` → `guide_step` (N times) → `guide_finish`. This requires the plugin process to stay alive between calls.

**Mitigation:** Plugin processes are already long-lived (started on first call, kept running). The SDK server maintains state in-process. The guide recorder lives in the plugin's memory.

**Steps:**
1. Create `scout-crawl` — receives CDP endpoint, manages its own page pool for crawl
2. Create `scout-guide` — maintains recorder state across calls
3. Test: complete guide workflow via plugin produces identical markdown
4. Deprecation + removal after 30 days

## Deprecation Protocol

For each wave:

1. **Day 0:** Plugin released. Built-in tools log deprecation warning to stderr:
   ```
   WARN scout: "markdown" MCP tool is deprecated, install scout-content plugin
   ```
2. **Day 0-30:** Both built-in and plugin work. If plugin is installed, it takes precedence (plugin tools shadow built-in tools in MCP server).
3. **Day 30:** Built-in code removed in a dedicated cleanup commit. Plugin becomes the only provider.

Track each deprecation in `docs/BACKLOG.md` with tag: `DEPRECATION: remove <tool> after YYYY-MM-DD`.

## Plugin Distribution

Each plugin ships as:
- **Go source:** `plugins/<name>/main.go` + `plugin.json` in the scout repo (for `go install`)
- **Pre-built binaries:** GitHub Releases per OS/arch (`scout-search_linux_amd64.tar.gz`)
- **Install command:** `scout plugin install https://github.com/inovacc/scout/releases/download/plugins-v1.0.0/scout-search_<os>_<arch>.tar.gz`

## MCP Server Changes

The MCP server (`pkg/scout/mcp/server.go`) evolves:

**Before (monolithic):**
```go
func NewServer(cfg ServerConfig, ...) *mcp.Server {
    registerBrowserTools(server, state)      // 8 tools
    registerCaptureTools(server, state)      // 3 tools
    registerContentTools(server, state)      // 3 tools
    registerNetworkTools(server, state)      // 3 tools
    registerFormTools(server, state)         // 3 tools
    registerSearchTools(server, state)       // 3 tools
    registerSessionTools(server, state)      // 3 tools
    registerAnalysisTools(server, state)     // 2 tools
    registerInspectTools(server, state)      // 4 tools
    registerGuideTools(server, state)        // 3 tools
    registerReportTools(server, state)       // 3 tools
    registerSwarmTools(server, state)        // 1 tool
    registerDiagTools(server, state)         // 2 tool
    // + plugin tools
    cfg.PluginManager.RegisterMCPTools(server)
}
```

**After (lean core + plugins):**
```go
func NewServer(cfg ServerConfig, ...) *mcp.Server {
    registerBrowserTools(server, state)      // 8 tools  (navigate, click, type, extract, eval, back, forward, wait)
    registerCaptureTools(server, state)      // 2 tools  (screenshot, snapshot)
    registerSessionTools(server, state)      // 3 tools  (session_list, session_reset, open)
    // Everything else comes from plugins:
    cfg.PluginManager.RegisterMCPTools(server)
}
```

**Core binary reduction:** From 41 tools to 13 tools. The 28 migrated tools move to ~6 plugin binaries.

## Installation UX

### Fresh Install
```bash
# Install scout (core only — 13 MCP tools)
go install github.com/inovacc/scout/cmd/scout@latest

# Install all official plugins (adds 28 more tools + CLI commands)
scout plugin install scout-diag scout-reports scout-content scout-search scout-network scout-forms scout-crawl scout-guide
```

### Upgrade from Monolithic
```bash
# After updating scout binary, MCP tools that moved to plugins show warnings:
scout mcp
# WARN: 28 MCP tools deprecated — install plugins for full functionality:
#   scout plugin install scout-content scout-search ...

# One-liner to install all official plugins:
scout plugin install --official
```

The `--official` flag fetches the latest plugin manifest from GitHub and installs all official plugin binaries.

## `--builtin` Escape Hatch

During the 30-day deprecation window, if a plugin version has a bug:
```bash
# Force built-in implementation (bypass plugin)
scout --builtin markdown https://example.com
```

This flag was implemented in Phase 63 (CLI Command Plugin Capability) and disables plugin command replacement for that invocation.

## Verification Checklist

For each wave:
- [ ] Plugin binary builds: `go build ./plugins/<name>/`
- [ ] Plugin installs: `scout plugin install ./plugins/<name>/`
- [ ] CLI command works: `scout <command> <args>` via plugin
- [ ] MCP tool works: tool callable via MCP protocol
- [ ] Output matches: diff built-in vs plugin output for 5 test URLs
- [ ] Deprecation warning: built-in logs warning when plugin not installed
- [ ] `--builtin` escape: built-in works when forced
- [ ] Startup perf: `time scout version` < 50ms with all plugins discovered
- [ ] Tests pass: `go test ./pkg/scout/plugin/... ./pkg/scout/mcp/...`

## Timeline

| Wave | Start | Plugin Release | Built-in Removal |
|------|-------|---------------|-----------------|
| 1 (Diagnostics) | Phase 64 | Day 0 | Day 30 |
| 2 (Content) | Phase 65 | Day 0 | Day 30 |
| 3 (Search) | Phase 66 | Day 0 | Day 30 |
| 4 (Network/Forms) | Phase 67 | Day 0 | Day 30 |
| 5 (Analysis/Guides) | Phase 68 | Day 0 | Day 30 |

Waves can overlap. Target: all 5 waves complete within 3 months.
