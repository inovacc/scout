// Package tracing wires OpenTelemetry into Scout. No-op unless SCOUT_TRACE=1
// or OTEL_EXPORTER_OTLP_ENDPOINT is set; exposes MCPToolSpan and ScraperSpan
// helpers and auto-instruments MCP tools.
package tracing
