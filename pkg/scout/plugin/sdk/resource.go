package sdk

import "context"

// ResourceHandler handles resource read requests from Scout.
type ResourceHandler interface {
	Read(ctx context.Context, uri string) (content string, mimeType string, err error)
}

// ResourceHandlerFunc adapts a function to ResourceHandler.
type ResourceHandlerFunc func(ctx context.Context, uri string) (string, string, error)

func (f ResourceHandlerFunc) Read(ctx context.Context, uri string) (string, string, error) {
	return f(ctx, uri)
}
