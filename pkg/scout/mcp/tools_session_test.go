package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSessionListTool(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Before any navigation, session should report no active session.
	result, err := callTool(ctx, cs, "session_list", map[string]any{})
	if err != nil {
		t.Fatalf("session_list: %v", err)
	}

	if result.IsError {
		t.Fatalf("session_list error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "no active session") {
		t.Errorf("expected 'no active session' before navigation, got: %s", text)
	}

	// Navigate, then check again.
	ts := newTestHTTPServer()
	defer ts.Close()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err = callTool(ctx, cs, "session_list", map[string]any{})
	if err != nil {
		t.Fatalf("session_list after navigate: %v", err)
	}

	text = result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "active") || !strings.Contains(text, "Test Page") {
		t.Errorf("expected active session with title, got: %s", text)
	}
}

func TestSessionResetTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	// Reset the session.
	result, err := callTool(ctx, cs, "session_reset", map[string]any{})
	if err != nil {
		t.Fatalf("session_reset: %v", err)
	}

	if result.IsError {
		t.Fatalf("session_reset error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text != "Session reset" {
		t.Errorf("expected 'Session reset', got: %s", text)
	}

	// After reset, session_list should show no active session.
	result, err = callTool(ctx, cs, "session_list", map[string]any{})
	if err != nil {
		t.Fatalf("session_list after reset: %v", err)
	}

	text = result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "no active session") {
		t.Errorf("expected 'no active session' after reset, got: %s", text)
	}

	// Navigate again to verify re-initialization works.
	navigateHelper(t, ctx, cs, ts.URL+"/page2")

	result, err = callTool(ctx, cs, "session_list", map[string]any{})
	if err != nil {
		t.Fatalf("session_list after re-navigate: %v", err)
	}

	text = result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Page Two") {
		t.Errorf("expected 'Page Two' after re-navigate, got: %s", text)
	}
}
