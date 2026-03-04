package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newNetworkTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/net", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Net</title></head><body><p>ok</p></body></html>`))
	})

	return httptest.NewServer(mux)
}

func TestCookieToolGetEmpty(t *testing.T) {
	ts := newNetworkTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/net")

	result, err := callTool(ctx, cs, "cookie", map[string]any{"action": "get"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("cookie get: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("cookie get error: %s", text)
	}
}

func TestCookieToolSetAndGet(t *testing.T) {
	ts := newNetworkTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/net")

	// Set a cookie.
	result, err := callTool(ctx, cs, "cookie", map[string]any{
		"action": "set", "name": "testcookie", "value": "testvalue",
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("cookie set: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("cookie set error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "testcookie") {
		t.Errorf("expected confirmation with cookie name, got: %s", text)
	}

	// Get cookies and verify the set cookie exists.
	result, err = callTool(ctx, cs, "cookie", map[string]any{"action": "get"})
	if err != nil {
		t.Fatalf("cookie get: %v", err)
	}

	if result.IsError {
		t.Fatalf("cookie get error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	getText := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(getText, "testcookie") {
		t.Errorf("expected 'testcookie' in cookies, got: %s", getText)
	}
}

func TestCookieToolClear(t *testing.T) {
	ts := newNetworkTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/net")

	// Set a cookie first.
	_, err := callTool(ctx, cs, "cookie", map[string]any{
		"action": "set", "name": "removeme", "value": "val",
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("cookie set: %v", err)
	}

	// Clear cookies.
	result, err := callTool(ctx, cs, "cookie", map[string]any{"action": "clear"})
	if err != nil {
		t.Fatalf("cookie clear: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		t.Fatalf("cookie clear error: %s", text)
	}

	if !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "cleared") {
		t.Error("expected 'cleared' confirmation")
	}
}

func TestCookieToolUnknownAction(t *testing.T) {
	ts := newNetworkTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/net")

	result, err := callTool(ctx, cs, "cookie", map[string]any{"action": "invalid"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("cookie invalid: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unknown cookie action")
	}
}

func TestCookieToolSetNoName(t *testing.T) {
	ts := newNetworkTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/net")

	result, err := callTool(ctx, cs, "cookie", map[string]any{
		"action": "set", "value": "noname",
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("cookie set no name: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for set without name")
	}
}

func TestHeaderTool(t *testing.T) {
	ts := newNetworkTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/net")

	result, err := callTool(ctx, cs, "header", map[string]any{
		"headers": map[string]any{"X-Custom": "test-value"},
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("header: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("header error: %s", text)
	}
}

func TestBlockTool(t *testing.T) {
	ts := newNetworkTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/net")

	result, err := callTool(ctx, cs, "block", map[string]any{
		"patterns": []string{"*.css", "*analytics*"},
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("block: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("block error: %s", text)
	}
}
