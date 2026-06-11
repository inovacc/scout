// This file intentionally carries no package-level doc comment; the canonical
// package documentation lives in tracing.go.
//
// Behavior summary: tracing wires OpenTelemetry into Scout. It is a no-op
// unless SCOUT_TRACE=1 or OTEL_EXPORTER_OTLP_ENDPOINT is set; it exposes
// MCPToolSpan and ScraperSpan helpers and auto-instruments MCP tools.

package tracing
