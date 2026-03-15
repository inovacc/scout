package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolProxy wraps a plugin-provided MCP tool, forwarding calls to the subprocess.
type ToolProxy struct {
	entry    ToolEntry
	manifest *Manifest
	manager  *Manager
}

// Register adds this tool to an MCP server.
func (t *ToolProxy) Register(server *mcp.Server) {
	tool := &mcp.Tool{
		Name:        fmt.Sprintf("plugin_%s_%s", t.manifest.Name, t.entry.Name),
		Description: t.entry.Description,
	}

	if t.entry.InputSchema != nil {
		data, err := json.Marshal(t.entry.InputSchema)
		if err == nil {
			tool.InputSchema = data
		}
	}

	server.AddTool(tool, t.handler)
}

func (t *ToolProxy) handler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := t.manager.getClient(t.manifest) //nolint:contextcheck // getClient manages process lifecycle, not request context
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("plugin: start failed: %s", err)}},
			IsError: true,
		}, nil
	}

	params := map[string]any{
		"name":      t.entry.Name,
		"arguments": req.Params.Arguments,
	}

	result, err := client.Call(ctx, "tool/call", params)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("plugin: call failed: %s", err)}},
			IsError: true,
		}, nil
	}

	var toolResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(result, &toolResult); err != nil {
		return &mcp.CallToolResult{ //nolint:nilerr // graceful fallback: return raw text when JSON parse fails
			Content: []mcp.Content{&mcp.TextContent{Text: string(result)}},
		}, nil
	}

	mcpContent := make([]mcp.Content, 0, len(toolResult.Content))
	for _, c := range toolResult.Content {
		mcpContent = append(mcpContent, &mcp.TextContent{Text: c.Text})
	}

	if len(mcpContent) == 0 {
		mcpContent = []mcp.Content{&mcp.TextContent{Text: string(result)}}
	}

	return &mcp.CallToolResult{
		Content: mcpContent,
		IsError: toolResult.IsError,
	}, nil
}
