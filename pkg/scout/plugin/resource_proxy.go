package plugin

import (
	"context"
	"encoding/json"
	"fmt"
)

// ResourceProxy forwards MCP resource reads to a plugin subprocess.
type ResourceProxy struct {
	client *Client
}

// NewResourceProxy creates a ResourceProxy for the given plugin client.
func NewResourceProxy(client *Client) *ResourceProxy {
	return &ResourceProxy{client: client}
}

// Read reads a resource by URI from the plugin.
func (p *ResourceProxy) Read(ctx context.Context, uri string) (string, string, error) {
	raw, err := p.client.Call(ctx, "resource/read", map[string]string{"uri": uri})
	if err != nil {
		return "", "", fmt.Errorf("plugin resource: read: %w", err)
	}

	var result struct {
		Content  string `json:"content"`
		MimeType string `json:"mimeType"`
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", fmt.Errorf("plugin resource: unmarshal: %w", err)
	}

	return result.Content, result.MimeType, nil
}

// List returns all resources available from the plugin.
func (p *ResourceProxy) List(ctx context.Context) ([]ResourceInfo, error) {
	raw, err := p.client.Call(ctx, "resource/list", nil)
	if err != nil {
		return nil, fmt.Errorf("plugin resource: list: %w", err)
	}

	var resources []ResourceInfo
	if err := json.Unmarshal(raw, &resources); err != nil {
		return nil, fmt.Errorf("plugin resource: list unmarshal: %w", err)
	}

	return resources, nil
}

// ResourceInfo describes a resource provided by a plugin.
type ResourceInfo struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType,omitempty"`
}
