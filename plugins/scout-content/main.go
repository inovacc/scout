// scout-content is a Scout plugin providing content extraction MCP tools.
// It connects to a running Scout browser via CDP to extract markdown, tables,
// metadata, and generate PDFs.
//
// Install: scout plugin install ./plugins/scout-content
// Or build: go build -o scout-content ./plugins/scout-content
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

func main() {
	srv := sdk.NewServer()
	srv.RegisterTool("markdown", sdk.ToolHandlerFunc(handleMarkdown))
	srv.RegisterTool("table", sdk.ToolHandlerFunc(handleTable))
	srv.RegisterTool("meta", sdk.ToolHandlerFunc(handleMeta))
	srv.RegisterTool("pdf", sdk.ToolHandlerFunc(handlePDF))

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

// getBrowser creates a standalone headless browser for content extraction.
func getBrowser() (*scout.Browser, error) {
	cdp := os.Getenv("SCOUT_CDP_ENDPOINT")
	if cdp != "" {
		return scout.New(scout.WithRemoteCDP(cdp))
	}

	return scout.New(scout.WithHeadless(true), scout.WithTimeout(0))
}

func handleMarkdown(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("launch browser: %s", err)), nil
	}

	defer func() { _ = browser.Close() }()

	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("navigate: %s", err)), nil
	}

	_ = page.WaitLoad()

	var opts []scout.MarkdownOption

	if mainOnly, ok := args["mainOnly"].(bool); ok && mainOnly {
		opts = append(opts, scout.WithMainContentOnly())
	}

	if includeImages, ok := args["includeImages"].(bool); ok {
		opts = append(opts, scout.WithIncludeImages(includeImages))
	}

	if includeLinks, ok := args["includeLinks"].(bool); ok {
		opts = append(opts, scout.WithIncludeLinks(includeLinks))
	}

	md, err := page.Markdown(opts...)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(md), nil
}

func handleTable(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	selector, _ := args["selector"].(string)

	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("launch browser: %s", err)), nil
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("navigate: %s", err)), nil
	}

	_ = page.WaitLoad()

	if selector == "" {
		selector = "table"
	}

	table, err := page.ExtractTable(selector)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return jsonToolResult(table)
}

func handleMeta(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)

	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("launch browser: %s", err)), nil
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("navigate: %s", err)), nil
	}

	_ = page.WaitLoad()

	meta, err := page.ExtractMeta()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return jsonToolResult(meta)
}

func handlePDF(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	path, _ := args["path"].(string)

	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("launch browser: %s", err)), nil
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("navigate: %s", err)), nil
	}

	_ = page.WaitLoad()

	if path == "" {
		title, _ := page.Title()
		if title == "" {
			title = "page"
		}

		path = title + ".pdf"
	}

	data, err := page.PDF()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return sdk.ErrorResult(fmt.Sprintf("write PDF: %s", err)), nil
	}

	return sdk.TextResult(fmt.Sprintf("PDF saved to %s (%d bytes)", path, len(data))), nil
}

func jsonToolResult(data any) (*sdk.ToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(string(b)), nil
}
