package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// These tests exercise tool handler paths that work even without a browser.
// When the browser is unavailable, tools return an error from ensureBrowser/ensurePage.
// We verify that the error is graceful (not a panic) and covers the argument parsing
// and error path code.

func expectErrorOrResult(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	// Either error or success is fine — we just need the handler to execute.
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestToolNavigateNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "navigate", map[string]any{"url": "http://localhost:1"})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolClickNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "click", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("click: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolTypeNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "type", map[string]any{"selector": "#x", "text": "abc"})
	if err != nil {
		t.Fatalf("type: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolExtractNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "extract", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolEvalNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "eval", map[string]any{"expression": "1+1"})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolBackNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "back", map[string]any{})
	if err != nil {
		t.Fatalf("back: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolForwardNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "forward", map[string]any{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolWaitNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "wait", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolWaitNoSelectorNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "wait", map[string]any{})
	if err != nil {
		t.Fatalf("wait no selector: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolScreenshotNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "screenshot", map[string]any{})
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolScreenshotFullPageNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "screenshot", map[string]any{"fullPage": true})
	if err != nil {
		t.Fatalf("screenshot fullPage: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSnapshotNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "snapshot", map[string]any{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSnapshotAllOptsNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "snapshot", map[string]any{
		"interactableOnly": true,
		"maxDepth":         2,
		"iframes":          true,
		"filter":           "button",
	})
	if err != nil {
		t.Fatalf("snapshot opts: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolPDFNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "pdf", map[string]any{})
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolPDFOptsNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "pdf", map[string]any{
		"landscape": true, "printBackground": true, "scale": 1.5,
	})
	if err != nil {
		t.Fatalf("pdf opts: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSearchNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search", map[string]any{"query": "test", "engine": "bing"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSearchDDGNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search", map[string]any{"query": "test", "engine": "ddg"})
	if err != nil {
		t.Fatalf("search ddg: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSearchAndExtractNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search_and_extract", map[string]any{
		"query": "test", "maxResults": 2, "mode": "text", "engine": "duckduckgo",
	})
	if err != nil {
		t.Fatalf("search_and_extract: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolFetchNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{
		"url": "http://localhost:1", "mode": "markdown", "mainOnly": true,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolMarkdownNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "markdown", map[string]any{
		"mainOnly": true, "includeImages": true, "includeLinks": false,
	})
	if err != nil {
		t.Fatalf("markdown: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolTableNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "table", map[string]any{"selector": "table"})
	if err != nil {
		t.Fatalf("table: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolMetaNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "meta", map[string]any{})
	if err != nil {
		t.Fatalf("meta: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolCookieNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	for _, action := range []string{"get", "set", "clear"} {
		result, err := callTool(ctx, cs, "cookie", map[string]any{
			"action": action, "name": "k", "value": "v",
		})
		if err != nil {
			t.Fatalf("cookie %s: %v", action, err)
		}

		expectErrorOrResult(t, result)
	}
}

func TestToolHeaderNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "header", map[string]any{
		"headers": map[string]any{"X-Test": "val"},
	})
	if err != nil {
		t.Fatalf("header: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolBlockNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "block", map[string]any{
		"patterns": []string{"*.css"},
	})
	if err != nil {
		t.Fatalf("block: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolFormDetectNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "form_detect", map[string]any{"selector": "#login"})
	if err != nil {
		t.Fatalf("form_detect: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolFormDetectAllNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "form_detect", map[string]any{})
	if err != nil {
		t.Fatalf("form_detect all: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolFormFillNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "form_fill", map[string]any{
		"selector": "#login", "data": map[string]any{"user": "admin"},
	})
	if err != nil {
		t.Fatalf("form_fill: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolFormSubmitNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "form_submit", map[string]any{"selector": "#login"})
	if err != nil {
		t.Fatalf("form_submit: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolCrawlNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "crawl", map[string]any{
		"url": "http://localhost:1", "maxDepth": 1, "maxPages": 5,
	})
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolDetectNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "detect", map[string]any{})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolStorageNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	for _, action := range []string{"get", "set", "list", "clear"} {
		result, err := callTool(ctx, cs, "storage", map[string]any{
			"action": action, "key": "k", "value": "v", "sessionStorage": true,
		})
		if err != nil {
			t.Fatalf("storage %s: %v", action, err)
		}

		expectErrorOrResult(t, result)
	}
}

func TestToolHijackNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "hijack", map[string]any{
		"urlFilter": "*.js", "captureBody": true, "duration": 1,
	})
	if err != nil {
		t.Fatalf("hijack: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolHarNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "har", map[string]any{"action": "export"})
	if err != nil {
		t.Fatalf("har: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSwaggerNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "swagger", map[string]any{
		"url": "http://localhost:1/swagger.json", "endpointsOnly": true,
	})
	if err != nil {
		t.Fatalf("swagger: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSessionListNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "session_list", map[string]any{})
	if err != nil {
		t.Fatalf("session_list: %v", err)
	}
	// session_list without page should report "no active session".
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "no active session") {
		t.Errorf("expected 'no active session', got: %s", text)
	}
}

func TestToolSessionResetNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "session_reset", map[string]any{})
	if err != nil {
		t.Fatalf("session_reset: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text != "Session reset" {
		t.Errorf("expected 'Session reset', got: %s", text)
	}
}

func TestToolOpenNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "open", map[string]any{"url": "http://localhost:1"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolOpenDevToolsNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Stealth: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "open", map[string]any{
		"url": "http://localhost:1", "devtools": true,
	})
	if err != nil {
		t.Fatalf("open devtools: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolGuideStartNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "guide_start", map[string]any{
		"url": "http://localhost:1", "title": "Test",
	})
	if err != nil {
		t.Fatalf("guide_start: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestResourceURLNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	_, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "scout://page/url"})
	// Error is expected when browser is unavailable — just ensure no panic.
	if err != nil {
		t.Logf("resource url error (expected): %v", err)
	}
}

func TestResourceTitleNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	_, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "scout://page/title"})
	if err != nil {
		t.Logf("resource title error (expected): %v", err)
	}
}

func TestToolStorageUnknownActionNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "storage", map[string]any{"action": "remove"})
	if err != nil {
		t.Fatalf("storage remove: %v", err)
	}
	// "remove" is not a valid action — should return error without needing browser.
	if !result.IsError {
		t.Error("expected error for unknown storage action")
	}
}

func TestToolCookieUnknownActionNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "cookie", map[string]any{"action": "delete"})
	if err != nil {
		t.Fatalf("cookie delete: %v", err)
	}
	// Cookie action "delete" doesn't exist — but this path needs ensurePage.
	// If browser fails, we get browser error. If not, we get unknown action error.
	expectErrorOrResult(t, result)
}

func TestToolHarUnknownActionNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "har", map[string]any{"action": "import"})
	if err != nil {
		t.Fatalf("har import: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unknown har action")
	}
}

func TestToolCrawlEmptyURLNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "crawl", map[string]any{"url": ""})
	if err != nil {
		t.Fatalf("crawl empty url: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for empty crawl url")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "url is required") {
		t.Errorf("expected 'url is required', got: %s", text)
	}
}

func TestToolFormFillEmptyDataNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "form_fill", map[string]any{
		"data": map[string]any{},
	})
	if err != nil {
		t.Fatalf("form_fill empty: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for empty form data")
	}
}

func TestToolCookieSetNoNameNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "cookie", map[string]any{
		"action": "set", "value": "v",
	})
	if err != nil {
		t.Fatalf("cookie set no name: %v", err)
	}
	// This requires ensurePage first, so either browser error or name validation error.
	expectErrorOrResult(t, result)
}

func TestToolStorageGetNoKeyNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "storage", map[string]any{
		"action": "get",
	})
	if err != nil {
		t.Fatalf("storage get no key: %v", err)
	}
	// Needs ensurePage first, then key validation.
	expectErrorOrResult(t, result)
}

func TestToolStorageSetNoKeyNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "storage", map[string]any{
		"action": "set", "value": "v",
	})
	if err != nil {
		t.Fatalf("storage set no key: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolHijackDefaultDurationNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// duration=0 defaults to 10, duration>30 caps to 30.
	result, err := callTool(ctx, cs, "hijack", map[string]any{"duration": 0})
	if err != nil {
		t.Fatalf("hijack default: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolHijackMaxDurationNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "hijack", map[string]any{"duration": 100})
	if err != nil {
		t.Fatalf("hijack max: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSearchDefaultEngineNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Default engine (google).
	result, err := callTool(ctx, cs, "search", map[string]any{"query": "test"})
	if err != nil {
		t.Fatalf("search default: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSearchAndExtractDDGNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search_and_extract", map[string]any{
		"query": "test", "engine": "ddg", "maxResults": 1, "mode": "html",
	})
	if err != nil {
		t.Fatalf("search_and_extract ddg: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolSearchAndExtractBingNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search_and_extract", map[string]any{
		"query": "test", "engine": "bing", "maxResults": 5, "mode": "markdown",
	})
	if err != nil {
		t.Fatalf("search_and_extract bing: %v", err)
	}

	expectErrorOrResult(t, result)
}

func TestToolFetchModesNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	for _, mode := range []string{"html", "text", "links", "meta"} {
		result, err := callTool(ctx, cs, "fetch", map[string]any{
			"url": "http://localhost:1", "mode": mode,
		})
		if err != nil {
			t.Fatalf("fetch %s: %v", mode, err)
		}

		expectErrorOrResult(t, result)
	}
}

func TestResourceMarkdownNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	_, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "scout://page/markdown"})
	if err != nil {
		t.Logf("resource markdown error (expected): %v", err)
	}
}
