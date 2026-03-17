// scout-search is a Scout plugin providing web search and content fetching MCP tools.
//
// Install: scout plugin install ./plugins/scout-search
// Or build: go build -o scout-search ./plugins/scout-search
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

func main() {
	srv := sdk.NewServer()
	srv.RegisterTool("search", sdk.ToolHandlerFunc(handleSearch))
	srv.RegisterTool("search_and_extract", sdk.ToolHandlerFunc(handleSearchAndExtract))
	srv.RegisterTool("fetch", sdk.ToolHandlerFunc(handleFetch))

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func handleSearch(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return sdk.ErrorResult("query is required"), nil
	}

	engine, _ := args["engine"].(string)
	if engine == "" {
		engine = "google"
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	browser, err := scout.New(scout.WithHeadless(true), scout.WithStealth(), scout.WithTimeout(0))
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("launch browser: %s", err)), nil
	}

	defer func() { _ = browser.Close() }()

	results, err := browser.SearchAll(query, scout.WithSearchEngine(parseEngine(engine)))
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("search: %s", err)), nil
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return jsonResult(results)
}

func handleSearchAndExtract(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return sdk.ErrorResult("query is required"), nil
	}

	engine, _ := args["engine"].(string)
	if engine == "" {
		engine = "google"
	}

	limit := 3
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	browser, err := scout.New(scout.WithHeadless(true), scout.WithStealth(), scout.WithTimeout(0))
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("launch browser: %s", err)), nil
	}

	defer func() { _ = browser.Close() }()

	// Search first.
	results, err := browser.SearchAll(query, scout.WithSearchEngine(parseEngine(engine)))
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("search: %s", err)), nil
	}

	// Fetch content from top results in parallel.
	type fetchResult struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	var (
		mu       sync.Mutex
		fetched  []fetchResult
		wg       sync.WaitGroup
	)

	for i, r := range results {
		if i >= limit {
			break
		}

		wg.Add(1)

		go func(url, title string) {
			defer wg.Done()

			page, err := browser.NewPage(url)
			if err != nil {
				return
			}

			_ = page.WaitLoad()

			md, err := page.Markdown(scout.WithMainContentOnly())
			if err != nil {
				return
			}

			mu.Lock()
			fetched = append(fetched, fetchResult{URL: url, Title: title, Content: md})
			mu.Unlock()
		}(r.URL, r.Title)
	}

	wg.Wait()

	return jsonResult(fetched)
}

func handleFetch(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	format, _ := args["format"].(string)
	if format == "" {
		format = "markdown"
	}

	browser, err := scout.New(scout.WithHeadless(true), scout.WithStealth(), scout.WithTimeout(0))
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("launch browser: %s", err)), nil
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("navigate: %s", err)), nil
	}

	_ = page.WaitLoad()

	selector, _ := args["selector"].(string)

	switch format {
	case "markdown":
		var opts []scout.MarkdownOption
		opts = append(opts, scout.WithMainContentOnly())

		md, err := page.Markdown(opts...)
		if err != nil {
			return sdk.ErrorResult(err.Error()), nil
		}

		return sdk.TextResult(md), nil

	case "text":
		if selector != "" {
			text, err := page.ExtractText(selector)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(text), nil
		}

		html, err := page.HTML()
		if err != nil {
			return sdk.ErrorResult(err.Error()), nil
		}

		return sdk.TextResult(html), nil

	case "html":
		html, err := page.HTML()
		if err != nil {
			return sdk.ErrorResult(err.Error()), nil
		}

		return sdk.TextResult(html), nil

	default:
		return sdk.ErrorResult(fmt.Sprintf("unknown format: %s", format)), nil
	}
}

func parseEngine(s string) scout.SearchEngine {
	switch s {
	case "bing":
		return scout.Bing
	case "duckduckgo", "ddg":
		return scout.DuckDuckGo
	default:
		return scout.Google
	}
}

func jsonResult(data any) (*sdk.ToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(string(b)), nil
}
