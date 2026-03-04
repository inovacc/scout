package plugin

import "context"

// Extractor is the interface for plugin-provided extractors.
type Extractor interface {
	Name() string
	Description() string
	Extract(ctx context.Context, html, url string, params map[string]any) (any, error)
}
