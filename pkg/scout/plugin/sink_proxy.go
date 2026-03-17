package plugin

import (
	"context"
	"encoding/json"
	"fmt"
)

// SinkProxy forwards output sink operations to a plugin subprocess via JSON-RPC.
type SinkProxy struct {
	client *Client
	name   string
}

// NewSinkProxy creates a SinkProxy for the given plugin client and sink name.
func NewSinkProxy(client *Client, name string) *SinkProxy {
	return &SinkProxy{client: client, name: name}
}

// Name returns the sink identifier.
func (p *SinkProxy) Name() string { return p.name }

// Init initializes the sink connection with the given config.
func (p *SinkProxy) Init(ctx context.Context, config map[string]any) error {
	_, err := p.client.Call(ctx, "sink/init", map[string]any{
		"name":   p.name,
		"config": config,
	})
	if err != nil {
		return fmt.Errorf("plugin sink: init: %w", err)
	}

	return nil
}

// Write sends a batch of results to the sink.
func (p *SinkProxy) Write(ctx context.Context, results []any) error {
	_, err := p.client.Call(ctx, "sink/write", map[string]any{
		"name":    p.name,
		"results": results,
	})
	if err != nil {
		return fmt.Errorf("plugin sink: write: %w", err)
	}

	return nil
}

// WriteSingle sends a single result to the sink.
func (p *SinkProxy) WriteSingle(ctx context.Context, result any) error {
	return p.Write(ctx, []any{result})
}

// Flush ensures all buffered data is written.
func (p *SinkProxy) Flush(ctx context.Context) error {
	_, err := p.client.Call(ctx, "sink/flush", map[string]any{"name": p.name})
	if err != nil {
		return fmt.Errorf("plugin sink: flush: %w", err)
	}

	return nil
}

// Close gracefully shuts down the sink.
func (p *SinkProxy) Close(ctx context.Context) error {
	_, err := p.client.Call(ctx, "sink/close", map[string]any{"name": p.name})
	if err != nil {
		return fmt.Errorf("plugin sink: close: %w", err)
	}

	return nil
}

// ListSinks returns sink names from a manifest's sink entries.
func ListSinks(raw json.RawMessage) ([]SinkInfo, error) {
	var sinks []SinkInfo
	if err := json.Unmarshal(raw, &sinks); err != nil {
		return nil, fmt.Errorf("plugin sink: list: %w", err)
	}

	return sinks, nil
}

// SinkInfo describes a sink provided by a plugin.
type SinkInfo struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	ConfigSchema map[string]any `json:"config_schema,omitempty"`
}
