package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBrowserSnapshotTool_ReturnsYAML(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Navigate to the form page which has a Submit button.
	navResult, err := callTool(ctx, cs, "navigate", map[string]any{"url": ts.URL + "/form"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("navigate: %v", err)
	}
	if navResult.IsError {
		text := navResult.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("navigate error: %s", text)
	}

	// Call browser_snapshot tool.
	result, err := callTool(ctx, cs, "browser_snapshot", nil)
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("browser_snapshot: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("browser_snapshot error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	if !strings.Contains(text, "snapshot_uri=scout://snapshot/") {
		t.Errorf("snapshot text missing snapshot_uri:\n%s", text)
	}
	if !strings.Contains(text, `[ref=`) {
		t.Errorf("snapshot text missing [ref=] tags:\n%s", text)
	}
}

func TestBrowserSnapshotTool_Registered(t *testing.T) {
	// Unit test: verify the tool is registered on the server even without a real browser.
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	tools, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	for _, tool := range tools.Tools {
		if tool.Name == "browser_snapshot" {
			return
		}
	}
	t.Errorf("browser_snapshot tool not found in registered tools")
}
