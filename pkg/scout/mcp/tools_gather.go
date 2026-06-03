package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerGatherTool exposes tools.Gather as an MCP tool. The CLI shim
// (cmd/scout/gather.go) delegates to the same tools.Gather — no
// transport-specific logic duplicated.
func registerGatherTool(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name: "gather",
		Description: "One-shot page intelligence collector. Navigates to a URL and returns " +
			"DOM, HAR, links, screenshots, cookies, metadata, detected frameworks, " +
			"and accessibility snapshot in a single call. Pass no flags for everything; " +
			"pass specific flags to scope the gather (e.g. screenshot+meta only).",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["url"],
			"properties": {
				"url":        {"type": "string", "description": "URL to gather intelligence from"},
				"html":       {"type": "boolean", "description": "include raw HTML"},
				"markdown":   {"type": "boolean", "description": "include markdown rendering"},
				"screenshot":{"type": "boolean", "description": "include base64 PNG screenshot"},
				"snapshot":  {"type": "boolean", "description": "include accessibility-tree snapshot"},
				"har":       {"type": "boolean", "description": "include HAR network recording"},
				"links":     {"type": "boolean", "description": "include extracted links"},
				"cookies":   {"type": "boolean", "description": "include page cookies"},
				"meta":      {"type": "boolean", "description": "include page metadata"},
				"frameworks":{"type": "boolean", "description": "include detected frontend frameworks"},
				"console":   {"type": "boolean", "description": "capture console output"},
				"all":       {"type": "boolean", "description": "include everything (default if no flags set)"},
				"timeout":   {"type": "string",  "description": "page load timeout as Go duration (default 30s)"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.GatherInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("gather: parse args: " + err.Error())
		}
		if err := state.checkURL(ctx, in.URL); err != nil {
			return errResult(err.Error())
		}
		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		out, err := tools.Gather(ctx, browser, in)
		if err != nil {
			return errResult(err.Error())
		}
		body, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return errResult("gather: marshal result: " + err.Error())
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil
	})
}
