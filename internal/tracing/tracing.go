// Package tracing provides OpenTelemetry instrumentation for Scout.
//
// Usage:
//
//	shutdown, err := tracing.Init(ctx, tracing.Config{ServiceName: "scout"})
//	defer shutdown(ctx)
//
// Or with OTEL_EXPORTER_OTLP_ENDPOINT environment variable set.
// When no exporter is configured, tracing is a no-op.
package tracing

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/inovacc/scout"

// Tracer returns the Scout tracer instance.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// Config configures the tracing provider.
type Config struct {
	ServiceName string
	Exporter    sdktrace.SpanExporter // nil = auto-detect from env or stdout
}

// Init initializes the global OpenTelemetry tracer provider.
// Returns a shutdown function that should be deferred.
// If SCOUT_TRACE is not set and no exporter is provided, tracing is a no-op.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "scout"
	}

	// Only enable if explicitly requested or exporter provided.
	if cfg.Exporter == nil {
		if os.Getenv("SCOUT_TRACE") == "" && os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
			return func(context.Context) error { return nil }, nil
		}

		// Default to stdout exporter for SCOUT_TRACE=1.
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}

		cfg.Exporter = exp
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(cfg.Exporter),
		sdktrace.WithResource(newResource(cfg.ServiceName)),
	)

	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// Start begins a new span with the given name.
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	opts := make([]trace.SpanStartOption, 0, 1)
	if len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	return Tracer().Start(ctx, name, opts...)
}
