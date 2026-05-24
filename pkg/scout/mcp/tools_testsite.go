package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTestSiteTool exposes tools.TestSite as an MCP tool.
func registerTestSiteTool(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name: "test_site",
		Description: "Health-check a website: crawls from the URL, surfaces broken links, " +
			"console errors, JavaScript exceptions, and network failures. Returns a " +
			"structured HealthReport with severity-classified issues.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["url"],
			"properties": {
				"url":           {"type": "string", "description": "URL to health-check"},
				"depth":         {"type": "integer", "description": "maximum crawl depth (default 2)"},
				"concurrency":   {"type": "integer", "description": "concurrent page limit (default 3)"},
				"timeout":       {"type": "string",  "description": "overall timeout as Go duration (default 60s)"},
				"clickElements":{"type": "boolean", "description": "click interactive elements to surface latent JS errors"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.TestSiteInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("test_site: parse args: " + err.Error())
		}
		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		out, err := tools.TestSite(ctx, browser, in)
		if err != nil {
			return errResult(err.Error())
		}
		body, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return errResult("test_site: marshal result: " + err.Error())
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil
	})
}
