package mcp

import (
	"context"
	"log/slog"
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
