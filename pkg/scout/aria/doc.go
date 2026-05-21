// Package aria implements the accessibility-tree (ARIA) snapshot + ref-id
// interaction model that powers Scout's MCP tools.
//
// Snapshots are captured via the CDP Accessibility.getFullAXTree command and
// rendered as YAML with each interactive node tagged [ref=eN] (root frame) or
// [ref=fF:eN] (frame F). Refs are stable within a snapshot version; navigation
// or DOM mutation invalidates them.
//
// Strict layering: this package imports only internal/engine and
// internal/engine/lib/proto. It MUST NOT import pkg/scout/mcp,
// pkg/scout/agent, or pkg/scout/runbook — those packages depend on aria,
// never the other way. The depguard rule in .golangci.yml enforces this.
package aria
