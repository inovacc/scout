package mcp

import (
	"context"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSnapshotTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "snapshot", map[string]any{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty snapshot")
	}
}

func TestPDFTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "pdf", map[string]any{})
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}

	if result.IsError {
		t.Fatalf("pdf error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	img, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatal("expected ImageContent for PDF result")
	}

	if img.MIMEType != "application/pdf" {
		t.Errorf("expected application/pdf, got: %s", img.MIMEType)
	}

	if len(img.Data) == 0 {
		t.Error("expected non-empty PDF data")
	}
}

func TestScreenshotTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "screenshot", map[string]any{})
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	if result.IsError {
		t.Fatalf("screenshot error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	img, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatal("expected ImageContent for screenshot result")
	}

	if img.MIMEType != "image/png" {
		t.Errorf("expected image/png, got: %s", img.MIMEType)
	}

	if len(img.Data) == 0 {
		t.Error("expected non-empty screenshot data")
	}
}

func TestScreenshotFullPage(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "screenshot", map[string]any{"fullPage": true})
	if err != nil {
		t.Fatalf("screenshot fullPage: %v", err)
	}

	if result.IsError {
		t.Fatalf("screenshot error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	img, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatal("expected ImageContent for fullPage screenshot")
	}

	if img.MIMEType != "image/png" {
		t.Errorf("expected image/png, got: %s", img.MIMEType)
	}

	if len(img.Data) == 0 {
		t.Error("expected non-empty fullPage screenshot data")
	}
}

func TestSnapshotInteractableOnly(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "snapshot", map[string]any{"interactableOnly": true})
	if err != nil {
		t.Fatalf("snapshot interactableOnly: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty snapshot with interactableOnly")
	}
}

func TestPDFToolWithOptions(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "pdf", map[string]any{"landscape": true, "printBackground": true})
	if err != nil {
		t.Fatalf("pdf with options: %v", err)
	}

	if result.IsError {
		t.Fatalf("pdf error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	img, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatal("expected ImageContent")
	}

	if len(img.Data) == 0 {
		t.Error("expected non-empty PDF data")
	}
}
