package sdk

import "context"

// SinkHandler handles output sink operations from Scout.
type SinkHandler interface {
	// Init initializes the sink with configuration.
	Init(ctx context.Context, config map[string]any) error

	// Write sends a batch of results to the sink.
	Write(ctx context.Context, results []map[string]any) error

	// Flush ensures all buffered data is written.
	Flush(ctx context.Context) error

	// Close gracefully shuts down the sink.
	Close(ctx context.Context) error
}

// SinkHandlerFunc adapts functions to SinkHandler.
type SinkHandlerFunc struct {
	InitFn  func(ctx context.Context, config map[string]any) error
	WriteFn func(ctx context.Context, results []map[string]any) error
	FlushFn func(ctx context.Context) error
	CloseFn func(ctx context.Context) error
}

func (f SinkHandlerFunc) Init(ctx context.Context, config map[string]any) error {
	if f.InitFn != nil {
		return f.InitFn(ctx, config)
	}

	return nil
}

func (f SinkHandlerFunc) Write(ctx context.Context, results []map[string]any) error {
	if f.WriteFn != nil {
		return f.WriteFn(ctx, results)
	}

	return nil
}

func (f SinkHandlerFunc) Flush(ctx context.Context) error {
	if f.FlushFn != nil {
		return f.FlushFn(ctx)
	}

	return nil
}

func (f SinkHandlerFunc) Close(ctx context.Context) error {
	if f.CloseFn != nil {
		return f.CloseFn(ctx)
	}

	return nil
}
