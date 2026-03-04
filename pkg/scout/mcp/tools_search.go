package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerSearchTools adds search and fetch tools.
func registerSearchTools(server *mcp.Server, state *mcpState) {
	addTracedTool(server, &mcp.Tool{
		Name:        "search",
		Description: "Search the web using a search engine",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"search query"},"engine":{"type":"string","description":"search engine: google, bing, duckduckgo","default":"google"}},"required":["query"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Query  string `json:"query"`
			Engine string `json:"engine"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.SearchOption

		switch args.Engine {
		case "bing":
			opts = append(opts, scout.WithSearchEngine(scout.Bing))
		case "duckduckgo", "ddg":
			opts = append(opts, scout.WithSearchEngine(scout.DuckDuckGo))
		default:
			// google is the default
		}

		results, err := browser.Search(args.Query, opts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: search: %s", err))
		}

		data, err := json.Marshal(results) //nolint:musttag
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: marshal results: %s", err))
		}

		return textResult(string(data))
	})

	addTracedTool(server, &mcp.Tool{
		Name:        "fetch",
		Description: "Fetch a URL and extract its content as markdown, html, text, or metadata",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to fetch"},"mode":{"type":"string","description":"extraction mode: markdown, html, text, links, meta, full","default":"full"},"mainOnly":{"type":"boolean","description":"extract main content only using readability scoring"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL      string `json:"url"`
			Mode     string `json:"mode"`
			MainOnly bool   `json:"mainOnly"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var opts []scout.WebFetchOption
		if args.Mode != "" {
			opts = append(opts, scout.WithFetchMode(args.Mode))
		}

		if args.MainOnly {
			opts = append(opts, scout.WithFetchMainContent())
		}

		result, err := browser.WebFetch(args.URL, opts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: fetch: %s", err))
		}

		data, err := json.Marshal(result)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: marshal result: %s", err))
		}

		return textResult(string(data))
	})
}
