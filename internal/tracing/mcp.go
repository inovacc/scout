package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// MCPToolSpan starts a span for an MCP tool call and returns a finish function.
func MCPToolSpan(ctx context.Context, toolName string) (context.Context, func(err error)) {
	ctx, span := Tracer().Start(ctx, "mcp.tool."+toolName,
		trace.WithAttributes(
			attribute.String("mcp.tool.name", toolName),
		),
	)

	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()
	}
}

// ScraperSpan starts a span for a scraper operation.
func ScraperSpan(ctx context.Context, mode string) (context.Context, func(itemCount int, err error)) {
	ctx, span := Tracer().Start(ctx, "scraper.scrape",
		trace.WithAttributes(
			attribute.String("scraper.mode", mode),
		),
	)

	return ctx, func(itemCount int, err error) {
		span.SetAttributes(attribute.Int("scraper.items", itemCount))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()
	}
}
