# ADR-009: MCP Tool Deprecation Cleanup

## Status

Scheduled for 2026-04-16 (30 days after plugin release on 2026-03-17)

## Context

28 of 41 MCP tools have been migrated to standalone plugin binaries. The built-in
implementations are deprecated with removal scheduled for 2026-04-16. This ADR
documents exactly which files and functions to remove.

## Files to Remove

| File | Tools | Replacement Plugin |
|------|-------|-------------------|
| `pkg/scout/mcp/diag.go` | ping, curl | `scout-diag` |
| `pkg/scout/mcp/tools_report.go` | report_list, report_show, report_delete | `scout-reports` |
| `pkg/scout/mcp/tools_content.go` | markdown, table, meta | `scout-content` |
| `pkg/scout/mcp/tools_search.go` | search, search_and_extract, fetch | `scout-search` |
| `pkg/scout/mcp/tools_network.go` | cookie, header, block | `scout-network` |
| `pkg/scout/mcp/tools_inspect.go` | storage, hijack, har, swagger | `scout-network` |
| `pkg/scout/mcp/tools_form.go` | form_detect, form_fill, form_submit | `scout-forms` |
| `pkg/scout/mcp/tools_analysis.go` | crawl, detect | `scout-crawl` |
| `pkg/scout/mcp/tools_guide.go` | guide_start, guide_step, guide_finish | `scout-guide` |

## Functions to Remove from server.go

```go
registerDiagTools(server, state)
registerReportTools(server, state)
registerContentTools(server, state)
registerSearchTools(server, state)
registerNetworkTools(server, state)
registerFormTools(server, state)
registerInspectTools(server, state)
registerAnalysisTools(server, state)
registerGuideTools(server, state)
```

## Test Files to Update

Remove or migrate corresponding test files:
- `pkg/scout/mcp/diag_test.go`
- `pkg/scout/mcp/tools_content_test.go`
- `pkg/scout/mcp/tools_search_test.go`
- `pkg/scout/mcp/tools_nobrowser_test.go` (partial — some tests cover remaining tools)

## Tools That Stay Built-In (13 tools)

navigate, click, type, extract, eval, back, forward, wait, screenshot,
snapshot, session_list, session_reset, open

## Cleanup Checklist

- [ ] Remove 9 deprecated files listed above
- [ ] Remove registration calls from `server.go`
- [ ] Update test files
- [ ] Update MCP tool count in CLAUDE.md (41 → 13)
- [ ] Update ROADMAP with cleanup completion
- [ ] Tag new release

## Decision

Proceed with removal on 2026-04-16 unless users report issues with plugin
alternatives during the deprecation window.
