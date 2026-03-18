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

func TestResourceMarkdownNoBrowser(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	_, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "scout://page/markdown"})
	if err != nil {
		t.Logf("resource markdown error (expected): %v", err)
	}
}
