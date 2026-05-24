package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerSitemapTool exposes tools.Sitemap as an MCP tool.
func registerSitemapTool(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name: "sitemap",
		Description: "Crawl a site from a starting URL and return a structured sitemap " +
			"with per-page DOM, markdown, and discovered links. Distinct from swarm_crawl " +
			"in that it extracts content per page rather than just discovering URLs.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"required": ["url"],
			"properties": {
				"url":            {"type": "string",  "description": "URL to start sitemap extraction from"},
				"maxDepth":       {"type": "integer", "description": "maximum crawl depth"},
				"maxPages":       {"type": "integer", "description": "maximum pages to visit"},
				"delay":          {"type": "string",  "description": "per-page delay as Go duration (politeness)"},
				"allowedDomains":{"type": "array", "items": {"type": "string"}, "description": "only follow links matching these domains"},
				"domDepth":       {"type": "integer", "description": "per-page DOM depth (0 = unlimited)"},
				"selector":       {"type": "string",  "description": "CSS selector to scope DOM extraction"},
				"mainOnly":       {"type": "boolean", "description": "extract only the page's <main> element"},
				"skipJson":       {"type": "boolean", "description": "omit DOM JSON (markdown only)"},
				"skipMarkdown":  {"type": "boolean", "description": "omit markdown rendering"},
				"outputDir":      {"type": "string",  "description": "if set, also write per-page artifacts to this directory"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.SitemapInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("sitemap: parse args: " + err.Error())
		}
		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		out, err := tools.Sitemap(ctx, browser, in)
		if err != nil {
			return errResult(err.Error())
		}
		body, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return errResult("sitemap: marshal result: " + err.Error())
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil
	})
}
