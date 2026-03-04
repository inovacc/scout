package tracing

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTracer creates a test tracer with a synchronous exporter.
func setupTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithResource(newResource("scout-test")),
	)
	otel.SetTracerProvider(tp)

	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	return exp
}

func TestInit_NoOp(t *testing.T) {
	t.Setenv("SCOUT_TRACE", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Init(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestInit_WithExporter(t *testing.T) {
	exp := setupTracer(t)

	ctx, span := Start(context.Background(), "test-span")
	_ = ctx
	span.End()

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	if spans[0].Name != "test-span" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "test-span")
	}
}

func TestMCPToolSpan_Success(t *testing.T) {
	exp := setupTracer(t)

	ctx, finish := MCPToolSpan(context.Background(), "navigate")
	_ = ctx
	finish(nil)

	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "mcp.tool.navigate" {
			found = true
			if s.Status.Code != codes.Ok {
				t.Errorf("status = %v, want Ok", s.Status.Code)
			}
		}
	}

	if !found {
		t.Error("mcp.tool.navigate span not found")
	}
}

func TestMCPToolSpan_Error(t *testing.T) {
	exp := setupTracer(t)

	ctx, finish := MCPToolSpan(context.Background(), "click")
	_ = ctx
	finish(errors.New("element not found"))

	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "mcp.tool.click" {
			found = true
			if s.Status.Code != codes.Error {
				t.Errorf("status = %v, want Error", s.Status.Code)
			}
		}
	}

	if !found {
		t.Error("mcp.tool.click span not found")
	}
}

func TestScraperSpan(t *testing.T) {
	exp := setupTracer(t)

	ctx, finish := ScraperSpan(context.Background(), "google")
	_ = ctx
	finish(5, nil)

	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "scraper.scrape" {
			found = true
			for _, attr := range s.Attributes {
				if string(attr.Key) == "scraper.items" && attr.Value.AsInt64() != 5 {
					t.Errorf("scraper.items = %d, want 5", attr.Value.AsInt64())
				}
			}
		}
	}

	if !found {
		t.Error("scraper.scrape span not found")
	}
}

func TestNewResource(t *testing.T) {
	r := newResource("test-service")
	if r == nil {
		t.Fatal("resource is nil")
	}
}

var _ sdktrace.SpanExporter = (*tracetest.InMemoryExporter)(nil)
