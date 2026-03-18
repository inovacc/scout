// scout-crawl is a Scout plugin providing web crawling and analysis MCP tools.
//
// Install: scout plugin install ./plugins/scout-crawl
// Or build: go build -o scout-crawl ./plugins/scout-crawl
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

func main() {
	srv := sdk.NewServer()
	srv.RegisterTool("crawl", sdk.ToolHandlerFunc(handleCrawl))
	srv.RegisterTool("detect", sdk.ToolHandlerFunc(handleDetect))
	srv.RegisterTool("swarm_crawl", sdk.ToolHandlerFunc(handleSwarmCrawl))

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func getBrowser() (*scout.Browser, error) {
	cdp := os.Getenv("SCOUT_CDP_ENDPOINT")
	if cdp != "" {
		return scout.New(scout.WithRemoteCDP(cdp))
	}

	return scout.New(scout.WithHeadless(true), scout.WithStealth(), scout.WithTimeout(0))
}

func handleCrawl(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	depth := 2
	if d, ok := args["depth"].(float64); ok && d > 0 {
		depth = int(d)
	}

	maxPages := 50
	if m, ok := args["maxPages"].(float64); ok && m > 0 {
		maxPages = int(m)
	}

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	defer func() { _ = browser.Close() }()

	var crawled []map[string]string

	handler := func(page *scout.Page, result *scout.CrawlResult) error {
		crawled = append(crawled, map[string]string{
			"url":   result.URL,
			"title": result.Title,
		})

		return nil
	}

	_, _ = browser.Crawl(url, handler,
		scout.WithCrawlMaxDepth(depth),
		scout.WithCrawlMaxPages(maxPages),
		scout.WithCrawlDelay(500*time.Millisecond),
	)

	return jsonToolResult(crawled)
}

func handleDetect(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	browser, err := getBrowser()
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(url)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	_ = page.WaitLoad()

	type detectResult struct {
		Frameworks []scout.FrameworkInfo `json:"frameworks"`
		PWA        *scout.PWAInfo       `json:"pwa"`
		RenderMode *scout.RenderInfo    `json:"renderMode"`
		TechStack  *scout.TechStack     `json:"techStack"`
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

	return jsonToolResult(result)
}

func handleSwarmCrawl(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	workers := 3
	if w, ok := args["workers"].(float64); ok && w > 0 {
		workers = int(w)
	}

	depth := 2
	if d, ok := args["depth"].(float64); ok && d > 0 {
		depth = int(d)
	}

	maxPages := 100
	if m, ok := args["maxPages"].(float64); ok && m > 0 {
		maxPages = int(m)
	}

	return sdk.TextResult(fmt.Sprintf("Swarm crawl queued: url=%s workers=%d depth=%d maxPages=%d", url, workers, depth, maxPages)), nil
}

func jsonToolResult(data any) (*sdk.ToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(string(b)), nil
}
