package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/internal/metrics"
	"github.com/inovacc/scout/pkg/scout/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerSwarmTools adds swarm crawl tools.
func registerSwarmTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "swarm_crawl",
		Description: "Crawl a website using multiple browser workers in parallel. Discovers pages via BFS, respects depth/maxPages limits, and saves a crawl report.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url":      {"type": "string",  "description": "Seed URL to start crawling"},
				"workers":  {"type": "integer", "description": "Number of parallel browser workers (default 2, max 8)"},
				"depth":    {"type": "integer", "description": "Maximum BFS crawl depth (default 2)"},
				"maxPages": {"type": "integer", "description": "Maximum number of pages to crawl (default 50)"}
			},
			"required": ["url"]
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		state.touch()

		var in tools.SwarmCrawlInput
		if err := json.Unmarshal(req.Params.Arguments, &in); err != nil {
			return errResult(err.Error())
		}

		if in.URL == "" {
			return errResult("scout-mcp: swarm_crawl: url is required")
		}
		if err := state.checkURL(ctx, in.URL); err != nil {
			return errResult(err.Error())
		}

		b, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		out, err := tools.SwarmCrawl(ctx, b, in)
		if err != nil {
			return errResult(err.Error())
		}

		metrics.Get().NavigationsTotal.Add(int64(out.PagesCrawled))

		return jsonResult(out)
	})
}
