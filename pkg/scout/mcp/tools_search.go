package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

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
		Name:        "search_and_extract",
		Description: "Search the web and extract content from top results in one step. Combines search + browser-rendered fetch.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"search query"},"engine":{"type":"string","description":"search engine: google, bing, duckduckgo","default":"google"},"maxResults":{"type":"integer","description":"number of results to extract (1-5, default 3)"},"mode":{"type":"string","description":"extraction mode: markdown, text, full","default":"markdown"}},"required":["query"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Query      string `json:"query"`
			Engine     string `json:"engine"`
			MaxResults int    `json:"maxResults"`
			Mode       string `json:"mode"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.MaxResults <= 0 || args.MaxResults > 5 {
			args.MaxResults = 3
		}

		if args.Mode == "" {
			args.Mode = "markdown"
		}

		browser, err := state.ensureBrowser(ctx)
		if err != nil {
			return errResult(err.Error())
		}

		var searchOpts []scout.SearchOption

		switch args.Engine {
		case "bing":
			searchOpts = append(searchOpts, scout.WithSearchEngine(scout.Bing))
		case "duckduckgo", "ddg":
			searchOpts = append(searchOpts, scout.WithSearchEngine(scout.DuckDuckGo))
		}

		searchResults, err := browser.Search(args.Query, searchOpts...)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: search: %s", err))
		}

		type extractedResult struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
			Content string `json:"content,omitempty"`
			Error   string `json:"error,omitempty"`
		}

		limit := min(args.MaxResults, len(searchResults.Results))

		extracted := make([]extractedResult, limit)

		var wg sync.WaitGroup
		for i := range limit {
			wg.Add(1)

			go func(idx int) {
				defer wg.Done()

				r := searchResults.Results[idx]
				extracted[idx] = extractedResult{
					Title:   r.Title,
					URL:     r.URL,
					Snippet: r.Snippet,
				}

				fetchResult, fetchErr := browser.WebFetch(r.URL,
					scout.WithFetchMode(args.Mode),
					scout.WithFetchMainContent(),
				)
				if fetchErr != nil {
					extracted[idx].Error = fetchErr.Error()
				} else {
					extracted[idx].Content = fetchResult.Markdown
				}
			}(i)
		}

		wg.Wait()

		data, err := json.MarshalIndent(extracted, "", "  ")
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: marshal: %s", err))
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
