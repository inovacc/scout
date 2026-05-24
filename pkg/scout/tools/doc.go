// Package tools is Scout's unified capability layer — the single source
// of truth for what Scout can do, independent of transport.
//
// Each capability is exposed as ONE typed function:
//
//	Func(ctx, *scout.Browser, <Capability>Input) (<Capability>Output, error)
//
// Both the CLI (`cmd/scout/*.go`) and the MCP server
// (`pkg/scout/mcp/tools_*.go`) delegate here. Adding a new tool means
// adding a file in this package and registering it from both surfaces.
// Removing a tool means deleting the file — the missing reference will
// fail the build until both surfaces are updated.
//
// Design rules:
//
//   - No transport concerns above this line. No cobra, no MCP types,
//     no JSON marshalling. Input/Output are plain Go structs.
//   - Input structs use `jsonschema:"..."` tags so the MCP server can
//     auto-generate its inputSchema (modelcontextprotocol go-sdk reads
//     these tags via reflection).
//   - Output structs are re-exports of engine result types where
//     possible — no parallel hierarchy to drift.
//   - Browser lifecycle is the caller's responsibility. CLI creates
//     fresh; MCP server uses the shared state.ensureBrowser browser.
//
// See `2026-05-24-plugin-first-okr.md` (Initiative #2) for the
// rationale and OKR context.
package tools
