package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newAnalysisTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html>
<head><title>Analysis Home</title></head>
<body>
  <h1>Home</h1>
  <a href="/about">About</a>
  <a href="/contact">Contact</a>
</body></html>`))
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html>
<head><title>About</title></head>
<body>
  <h1>About</h1>
  <a href="/">Home</a>
  <a href="/contact">Contact</a>
</body></html>`))
	})
	mux.HandleFunc("/contact", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html>
<head><title>Contact</title></head>
<body>
  <h1>Contact</h1>
  <a href="/">Home</a>
</body></html>`))
	})

	return httptest.NewServer(mux)
}

func TestDetectTool(t *testing.T) {
	ts := newAnalysisTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "detect", map[string]any{})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("detect: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("detect error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var parsed struct {
		Frameworks json.RawMessage `json:"frameworks"`
		PWA        json.RawMessage `json:"pwa"`
		RenderMode json.RawMessage `json:"renderMode"`
		TechStack  json.RawMessage `json:"techStack"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("detect result is not valid JSON: %v\nraw: %s", err, text)
	}

	// Verify all top-level keys are present (may be null for simple test pages).
	if parsed.Frameworks == nil && parsed.PWA == nil && parsed.RenderMode == nil && parsed.TechStack == nil {
		t.Error("expected at least one non-nil field in detect result")
	}
}

func TestCrawlTool(t *testing.T) {
	ts := newAnalysisTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "crawl", map[string]any{
		"url":      ts.URL + "/",
		"maxDepth": 2,
		"maxPages": 10,
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("crawl: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("crawl error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var entries []struct {
		URL   string `json:"url"`
		Title string `json:"title"`
		Depth int    `json:"depth"`
	}
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("crawl result is not valid JSON array: %v\nraw: %s", err, text)
	}

	if len(entries) < 2 {
		t.Errorf("expected at least 2 crawled pages, got %d", len(entries))
	}

	// Verify the start URL is in results.
	found := false

	for _, e := range entries {
		if e.URL == ts.URL+"/" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected start URL %s in crawl results: %v", ts.URL+"/", entries)
	}
}

func TestCrawlToolMissingURL(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "crawl", map[string]any{})
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing url")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text != "url is required" {
		t.Errorf("expected 'url is required', got: %s", text)
	}
}
