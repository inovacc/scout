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

func newContentTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html>
<head>
  <title>Content Test</title>
  <meta name="description" content="A test page for content extraction">
  <meta property="og:title" content="OG Title">
  <meta property="og:description" content="OG Description">
  <meta name="twitter:card" content="summary">
  <link rel="canonical" href="https://example.com/content">
</head>
<body>
  <h1>Content Page</h1>
  <p>Some paragraph text with a <a href="https://example.com">link</a>.</p>
  <table>
    <thead><tr><th>Name</th><th>Value</th></tr></thead>
    <tbody>
      <tr><td>Alpha</td><td>100</td></tr>
      <tr><td>Beta</td><td>200</td></tr>
    </tbody>
  </table>
</body></html>`))
	})
	return httptest.NewServer(mux)
}

func TestMarkdownTool(t *testing.T) {
	ts := newContentTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/content")

	result, err := callTool(ctx, cs, "markdown", map[string]any{})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("markdown: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("markdown error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Content Page") {
		t.Errorf("expected 'Content Page' in markdown, got: %s", text)
	}
}

func TestMarkdownToolWithOptions(t *testing.T) {
	ts := newContentTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/content")

	result, err := callTool(ctx, cs, "markdown", map[string]any{
		"mainOnly":     true,
		"includeLinks": false,
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("markdown with options: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("markdown error: %s", text)
	}

	// Should still contain content but links should be plain text.
	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty markdown")
	}
}

func TestTableTool(t *testing.T) {
	ts := newContentTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/content")

	result, err := callTool(ctx, cs, "table", map[string]any{})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("table: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("table error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var table struct {
		Headers []string   `json:"headers"`
		Rows    [][]string `json:"rows"`
	}
	if err := json.Unmarshal([]byte(text), &table); err != nil {
		t.Fatalf("unmarshal table: %v", err)
	}

	if len(table.Headers) != 2 || table.Headers[0] != "Name" || table.Headers[1] != "Value" {
		t.Errorf("unexpected headers: %v", table.Headers)
	}
	if len(table.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(table.Rows))
	}
	if len(table.Rows) > 0 && (table.Rows[0][0] != "Alpha" || table.Rows[0][1] != "100") {
		t.Errorf("unexpected first row: %v", table.Rows[0])
	}
}

func TestMetaTool(t *testing.T) {
	ts := newContentTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/content")

	result, err := callTool(ctx, cs, "meta", map[string]any{})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("meta: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("meta error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var meta struct {
		Title       string            `json:"Title"`
		Description string            `json:"Description"`
		Canonical   string            `json:"Canonical"`
		OG          map[string]string `json:"OG"`
		Twitter     map[string]string `json:"Twitter"`
	}
	if err := json.Unmarshal([]byte(text), &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}

	if meta.Title != "Content Test" {
		t.Errorf("expected title 'Content Test', got: %s", meta.Title)
	}
	if meta.Description != "A test page for content extraction" {
		t.Errorf("unexpected description: %s", meta.Description)
	}
	if meta.Canonical != "https://example.com/content" {
		t.Errorf("unexpected canonical: %s", meta.Canonical)
	}
	if meta.OG["og:title"] != "OG Title" {
		t.Errorf("unexpected og:title: %s", meta.OG["og:title"])
	}
	if meta.Twitter["twitter:card"] != "summary" {
		t.Errorf("unexpected twitter:card: %s", meta.Twitter["twitter:card"])
	}
}
