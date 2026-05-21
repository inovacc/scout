# Changelog

All notable changes to Scout are documented here.

## [Unreleased]

### Added — ARIA-Ref Phase A
- New `pkg/scout/aria/` package: accessibility-tree snapshots with stable `[ref=eN]` IDs, per-page snapshot store, structural diff with deterministic Summary, ref-to-element resolution via CDP `DOM.resolveNode`. Unit coverage: 59.5% (browser-dependent paths — `Capture`, `ResolveElement`, `collectChildFrameIDs` — require Chromium and are gated under `-tags integration`).
- New MCP resource `scout://snapshot/{page-id}` exposes the current snapshot as YAML.
- New MCP tool `browser_snapshot` captures and stores a snapshot of the current page; subsequent phases will wire ref-based action tools on top.
- Snapshot auto-invalidates on root-frame navigation, page close, and bursty DOM mutations (≥ 20 within 100ms via the bridge MutationObserver).
- Layering rule enforced via `golangci-lint` depguard: `pkg/scout/aria/` cannot import `pkg/scout/mcp`, `pkg/scout/agent`, or `pkg/scout/runbook`.
- `internal/engine/page.go` gains `ElementFromBackendNodeID(int) (*Element, error)` to bridge CDP backend IDs to live elements.

### Not yet
- Ref-based action tools (`browser_click`, `browser_type`, `browser_hover`, `browser_drag`, `browser_select_option`, `browser_key`, `browser_file_upload`) — Phase B.
- Capability gating (`--mcp-caps`) — Phase C.
- Observer tools (dialog/console/network) — Phase D.
- Tab tools — Phase E.
- Code-generation (`generate_test`) — Phase F.

### Known limitations (Phase A)
- Child-frame snapshot `Children` indices are frame-internal, not rebased to the combined `Nodes` slice — cross-frame parent/child links are broken at the boundary. Fixed in Phase B.
- `pkg/scout/mcp` has a pre-existing test hang (`TestExtractToolMissing`) unrelated to this work; Phase A test runs use `-run` filters or `-timeout` to avoid it.
- `golangci-lint` reports pre-existing `contextcheck` warnings in `pkg/scout/aria/axtree.go` and `pkg/scout/mcp/` (context not threaded through rod CDP call sites). These are inherited from the rod internalization pattern and are not regressions introduced by Phase A.
