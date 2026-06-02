/*
Copyright (c) 2026 Security Research
*/
package mcp

import "context"

// Adapter is a named wrapper around the package-level Sample function.
// Use it when you want a subsystem-scoped client that satisfies a
// Summarize(ctx, prompt) ([]byte, error) interface without importing the
// full internal/mcp domain adapter layer.
type Adapter struct {
	name string
}

// NewAdapter returns an Adapter bound to the given subsystem name.
// The name is used only for logging / diagnostics; it does not affect
// routing. All Adapters share the package-level session singleton
// installed by SetSession.
func NewAdapter(name string) *Adapter {
	return &Adapter{name: name}
}

// Sample delegates to the package-level Sample function. It calls the
// connected MCP host via sampling/createMessage and returns the text
// response as raw bytes. Returns a non-nil error if no session is wired.
func (a *Adapter) Sample(ctx context.Context, prompt string) ([]byte, error) {
	return Sample(ctx, prompt)
}
