package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResourcePageURL(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "scout://page/url"})
	if err != nil {
		t.Fatalf("ReadResource url: %v", err)
	}

	if len(res.Contents) == 0 {
		t.Fatal("no resource contents returned")
	}

	if !strings.Contains(res.Contents[0].Text, ts.URL) {
		t.Errorf("expected URL containing %s, got: %s", ts.URL, res.Contents[0].Text)
	}
}

func TestResourcePageTitle(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "scout://page/title"})
	if err != nil {
		t.Fatalf("ReadResource title: %v", err)
	}

	if len(res.Contents) == 0 {
		t.Fatal("no resource contents returned")
	}

	if res.Contents[0].Text != "Test Page" {
		t.Errorf("expected 'Test Page', got: %s", res.Contents[0].Text)
	}
}

func TestResourcePageMarkdown(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "scout://page/markdown"})
	if err != nil {
		t.Fatalf("ReadResource markdown: %v", err)
	}

	if len(res.Contents) == 0 {
		t.Fatal("no resource contents returned")
	}

	if res.Contents[0].Text == "" {
		t.Error("expected non-empty markdown content")
	}

	if !strings.Contains(res.Contents[0].Text, "Hello Scout") {
		t.Errorf("expected 'Hello Scout' in markdown, got: %s", res.Contents[0].Text)
	}
}
