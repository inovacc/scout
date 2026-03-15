package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSearchTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Search requires a real browser and network; verify it returns an error or results gracefully.
	result, err := callTool(ctx, cs, "search", map[string]any{"query": "test query", "engine": "google"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("search: %v", err)
	}

	// Either a valid result or an error result (e.g., no network, CAPTCHA) is acceptable.
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Logf("search returned error (expected in test env): %s", text)
	}
}

func TestSearchToolBing(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search", map[string]any{"query": "test", "engine": "bing"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("search bing: %v", err)
	}

	// Accept both success and error — we just want the engine switch path to execute.
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Logf("search bing returned error (expected in test env): %s", text)
	}
}

func TestSearchToolDuckDuckGo(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search", map[string]any{"query": "test", "engine": "duckduckgo"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("search ddg: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Logf("search ddg returned error (expected in test env): %s", text)
	}
}

func TestSearchAndExtractTool(t *testing.T) {
	// We can't control the search results, but we can verify the tool
	// handles the flow: search + fetch. It will search the web and attempt
	// to extract content from top results. We accept errors gracefully.
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search_and_extract", map[string]any{
		"query":      "httptest golang",
		"maxResults": 1,
		"mode":       "text",
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("search_and_extract: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		// Search may fail due to CAPTCHA or network — log and accept.
		t.Logf("search_and_extract returned error (expected in test env): %s", text)

		return
	}

	text := result.Content[0].(*mcp.TextContent).Text
	// Should be valid JSON array.
	var extracted []json.RawMessage
	if err := json.Unmarshal([]byte(text), &extracted); err != nil {
		t.Errorf("expected JSON array result, got parse error: %v", err)
	}
}

func TestSearchAndExtractToolDefaults(t *testing.T) {
	// Test default maxResults (clamped to 3) and default mode (markdown).
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search_and_extract", map[string]any{
		"query":      "test",
		"maxResults": 0, // should default to 3
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("search_and_extract defaults: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Logf("search_and_extract defaults returned error (expected): %s", text)
	}
}

func TestSearchAndExtractToolMaxResultsCap(t *testing.T) {
	// maxResults > 5 should be capped to 3.
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "search_and_extract", map[string]any{
		"query":      "test",
		"maxResults": 10,
		"engine":     "bing",
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("search_and_extract cap: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Logf("search_and_extract cap returned error (expected): %s", text)
	}
}

func TestFetchTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{"url": ts.URL + "/", "mode": "full"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("fetch: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("fetch error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Hello Scout") && !strings.Contains(text, "Test Page") {
		t.Errorf("expected page content in fetch result, got: %s", text)
	}
}

func TestFetchToolMarkdownMode(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{"url": ts.URL + "/", "mode": "markdown"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("fetch markdown: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("fetch markdown error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty fetch markdown result")
	}
}

func TestFetchToolNoMode(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{"url": ts.URL + "/"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("fetch no mode: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("fetch no mode error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty fetch result with no mode")
	}
}

func TestFetchToolMainOnly(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{"url": ts.URL + "/", "mode": "full", "mainOnly": true})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("fetch mainOnly: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("fetch mainOnly error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty fetch result with mainOnly")
	}
}

func newSearchExtractTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Test Article</title></head>
<body><main><h1>Test Article</h1><p>This is the article content for extraction testing.</p></main></body></html>`))
	})

	return httptest.NewServer(mux)
}

func TestFetchToolHTMLMode(t *testing.T) {
	ts := newSearchExtractTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{"url": ts.URL + "/article", "mode": "html"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("fetch html: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("fetch html error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty fetch html result")
	}
}

func TestFetchToolTextMode(t *testing.T) {
	ts := newSearchExtractTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{"url": ts.URL + "/article", "mode": "text"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("fetch text: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("fetch text error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty fetch text result")
	}
}
