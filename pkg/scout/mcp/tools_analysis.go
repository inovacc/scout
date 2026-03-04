package mcp

import (
	"context"
	"encoding/json"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerAnalysisTools adds crawl and detect tools.
func registerAnalysisTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "crawl",
		Description: "Crawl a website discovering linked pages via BFS",
		InputSchema: json.RawMessage(`{"type":"object","required":["url"],"properties":{"url":{"type":"string","description":"start URL to crawl"},"maxDepth":{"type":"integer","description":"maximum crawl depth (default 2)"},"maxPages":{"type":"integer","description":"maximum pages to visit (default 50)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL      string `json:"url"`
			MaxDepth *int   `json:"maxDepth"`
			MaxPages *int   `json:"maxPages"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.URL == "" {
			return errResult("url is required")
		}

		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		maxDepth := 2
		if args.MaxDepth != nil {
			maxDepth = *args.MaxDepth
		}

		maxPages := 50
		if args.MaxPages != nil {
			maxPages = *args.MaxPages
		}

		results, err := browser.Crawl(args.URL, nil,
			scout.WithCrawlMaxDepth(maxDepth),
			scout.WithCrawlMaxPages(maxPages),
		)
		if err != nil {
			return errResult(err.Error())
		}

		type crawlEntry struct {
			URL   string `json:"url"`
			Title string `json:"title"`
			Depth int    `json:"depth"`
		}

		entries := make([]crawlEntry, 0, len(results))
		for _, r := range results {
			entries = append(entries, crawlEntry{
				URL:   r.URL,
				Title: r.Title,
				Depth: r.Depth,
			})
		}

		return jsonResult(entries)
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "detect",
		Description: "Detect technologies, frameworks, PWA support, and render mode on the current page",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		type detectResult struct {
			Frameworks []scout.FrameworkInfo `json:"frameworks"`
			PWA        *scout.PWAInfo        `json:"pwa"`
			RenderMode *scout.RenderInfo     `json:"renderMode"`
			TechStack  *scout.TechStack      `json:"techStack"`
		}

		var result detectResult

		if fw, err := page.DetectFrameworks(); err == nil {
			result.Frameworks = fw
		}

		if pwa, err := page.DetectPWA(); err == nil {
			result.PWA = pwa
		}

		if rm, err := page.DetectRenderMode(); err == nil {
			result.RenderMode = rm
		}

		if ts, err := page.DetectTechStack(); err == nil {
			result.TechStack = ts
		}

		return jsonResult(result)
	})
}
