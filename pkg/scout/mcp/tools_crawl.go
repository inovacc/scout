package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerCrawlTool exposes tools.Crawl as an MCP tool. Single-host
// BFS — distinct from swarm_crawl (distributed workers).
func registerCrawlTool(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name: "crawl",
		Description: "Crawl a website starting from a URL (single-host BFS). " +
			"Returns per-page results: URL, title, links, status. For distributed " +
			"crawling across many workers, use swarm_crawl instead.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["url"],
			"properties":{
				"url":           {"type":"string","description":"URL to start crawling from"},
				"maxDepth":      {"type":"integer","description":"maximum crawl depth (default 3)"},
				"maxPages":      {"type":"integer","description":"maximum pages to crawl (default 100)"},
				"delay":         {"type":"string","description":"per-page delay as Go duration (default 500ms)"},
				"allowedDomains":{"type":"array","items":{"type":"string"},"description":"restrict crawl to these domains"},
				"concurrent":    {"type":"integer","description":"concurrent page limit (default 1 = sequential)"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in tools.CrawlInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult("crawl: parse args: " + err.Error())
		}
		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		out, err := tools.Crawl(ctx, browser, in)
		if err != nil {
			return errResult(err.Error())
		}
		return jsonResult(out)
	})
}
