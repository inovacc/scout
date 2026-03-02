package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("cookie get error: %s", text)
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
		skipIfNoBrowser(t, &toolErr{text})
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
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("block error: %s", text)
	}
}
