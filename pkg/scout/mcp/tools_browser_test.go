package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNavigateTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "navigate", map[string]any{"url": ts.URL + "/"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("CallTool navigate: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("navigate returned error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Test Page") {
		t.Errorf("expected title in response, got: %s", text)
	}
}

func TestNavigateToolBadParams(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Invalid JSON arguments via raw message
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "navigate",
		Arguments: json.RawMessage(`{invalid`),
	})
	if err != nil {
		// JSON-RPC level error is acceptable
		return
	}

	if !result.IsError {
		t.Error("expected error result for invalid JSON params")
	}
}

func TestExtractTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "extract", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if result.IsError {
		t.Fatalf("extract error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Hello Scout") {
		t.Errorf("expected 'Hello Scout', got: %s", text)
	}
}

func TestEvalTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "eval", map[string]any{"expression": "() => 1 + 2"})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	if result.IsError {
		t.Fatalf("eval error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "3") {
		t.Errorf("expected '3', got: %s", text)
	}
}

func TestClickTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "click", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("click: %v", err)
	}

	if result.IsError {
		t.Fatalf("click error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Clicked") {
		t.Errorf("expected 'Clicked' in response, got: %s", text)
	}
}

func TestTypeTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/form")

	result, err := callTool(ctx, cs, "type", map[string]any{"selector": "#username", "text": "testuser"})
	if err != nil {
		t.Fatalf("type: %v", err)
	}

	if result.IsError {
		t.Fatalf("type error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Typed") {
		t.Errorf("expected 'Typed' in response, got: %s", text)
	}
}

func TestWaitToolWithSelector(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "wait", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}

	if result.IsError {
		t.Fatalf("wait error: %s", result.Content[0].(*mcp.TextContent).Text)
	}
}

func TestWaitToolNoSelector(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "wait", map[string]any{})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}

	if result.IsError {
		t.Fatalf("wait error: %s", result.Content[0].(*mcp.TextContent).Text)
	}
}

func TestBackForwardTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Navigate to / then /page2.
	navigateHelper(t, ctx, cs, ts.URL+"/")
	navigateHelper(t, ctx, cs, ts.URL+"/page2")

	// Go back.
	result, err := callTool(ctx, cs, "back", map[string]any{})
	if err != nil {
		t.Fatalf("back: %v", err)
	}

	if result.IsError {
		t.Fatalf("back error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	// Verify we're back on page 1 by extracting h1.
	result, err = callTool(ctx, cs, "extract", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("extract after back: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Hello Scout") {
		t.Errorf("expected 'Hello Scout' after back, got: %s", text)
	}

	// Go forward.
	result, err = callTool(ctx, cs, "forward", map[string]any{})
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	if result.IsError {
		t.Fatalf("forward error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	// Verify we're on page 2.
	result, err = callTool(ctx, cs, "extract", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("extract after forward: %v", err)
	}

	text = result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Page 2") {
		t.Errorf("expected 'Page 2' after forward, got: %s", text)
	}
}
