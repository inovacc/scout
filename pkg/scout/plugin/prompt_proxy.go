package plugin

import (
	"context"
	"encoding/json"
	"fmt"
)

// PromptProxy forwards MCP prompt requests to a plugin subprocess.
type PromptProxy struct {
	client *Client
}

// NewPromptProxy creates a PromptProxy for the given plugin client.
func NewPromptProxy(client *Client) *PromptProxy {
	return &PromptProxy{client: client}
}

// Get retrieves a prompt by name with the given arguments.
func (p *PromptProxy) Get(ctx context.Context, name string, arguments map[string]string) ([]PromptMessage, error) {
	raw, err := p.client.Call(ctx, "prompt/get", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("plugin prompt: get: %w", err)
	}

	var messages []PromptMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, fmt.Errorf("plugin prompt: unmarshal: %w", err)
	}

	return messages, nil
}

// List returns all prompts available from the plugin.
func (p *PromptProxy) List(ctx context.Context) ([]PromptInfo, error) {
	raw, err := p.client.Call(ctx, "prompt/list", nil)
	if err != nil {
		return nil, fmt.Errorf("plugin prompt: list: %w", err)
	}

	var prompts []PromptInfo
	if err := json.Unmarshal(raw, &prompts); err != nil {
		return nil, fmt.Errorf("plugin prompt: list unmarshal: %w", err)
	}

	return prompts, nil
}

// PromptMessage is a single message in a prompt response.
type PromptMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// PromptInfo describes a prompt provided by a plugin.
type PromptInfo struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a prompt argument.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}
