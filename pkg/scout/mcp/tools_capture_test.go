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

func TestSnapshotMaxDepth(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "snapshot", map[string]any{"maxDepth": 2})
	if err != nil {
		t.Fatalf("snapshot maxDepth: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot maxDepth error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty snapshot with maxDepth")
	}
}

func TestSnapshotMaxDepthOne(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	// maxDepth=1 should produce a shallower tree than unlimited.
	result, err := callTool(ctx, cs, "snapshot", map[string]any{"maxDepth": 1})
	if err != nil {
		t.Fatalf("snapshot maxDepth=1: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot maxDepth=1 error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	shallow := result.Content[0].(*mcp.TextContent).Text
	if shallow == "" {
		t.Error("expected non-empty snapshot with maxDepth=1")
	}

	// Full depth snapshot for comparison.
	resultFull, err := callTool(ctx, cs, "snapshot", map[string]any{})
	if err != nil {
		t.Fatalf("snapshot full: %v", err)
	}

	full := resultFull.Content[0].(*mcp.TextContent).Text
	// Shallow tree should be no longer than full tree.
	if len(shallow) > len(full) {
		t.Errorf("shallow snapshot (%d chars) should not be longer than full snapshot (%d chars)", len(shallow), len(full))
	}
}

func TestSnapshotFilter(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/form")

	// Filter for "button" should narrow results.
	result, err := callTool(ctx, cs, "snapshot", map[string]any{"filter": "button"})
	if err != nil {
		t.Fatalf("snapshot filter: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot filter error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty snapshot with filter")
	}
}

func TestSnapshotFilterNoMatch(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	// Filter for something that doesn't exist.
	result, err := callTool(ctx, cs, "snapshot", map[string]any{"filter": "xyznonexistent"})
	if err != nil {
		t.Fatalf("snapshot filter no match: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot filter no match error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	// May be empty or minimal — just ensure it doesn't crash.
}

func TestSnapshotIframes(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "snapshot", map[string]any{"iframes": true})
	if err != nil {
		t.Fatalf("snapshot iframes: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot iframes error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty snapshot with iframes enabled")
	}
}

func TestSnapshotCombinedOptions(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	// Combine multiple options to hit all branches.
	result, err := callTool(ctx, cs, "snapshot", map[string]any{
		"interactableOnly": true,
		"maxDepth":         3,
		"iframes":          true,
		"filter":           "heading",
	})
	if err != nil {
		t.Fatalf("snapshot combined: %v", err)
	}

	if result.IsError {
		t.Fatalf("snapshot combined error: %s", result.Content[0].(*mcp.TextContent).Text)
	}
}

func TestScreenshotWithSelector(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	// Default screenshot (no fullPage).
	result, err := callTool(ctx, cs, "screenshot", map[string]any{"fullPage": false})
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	if result.IsError {
		t.Fatalf("screenshot error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	img, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatal("expected ImageContent")
	}

	if len(img.Data) == 0 {
		t.Error("expected non-empty screenshot data")
	}
}

func TestPDFToolScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "pdf", map[string]any{"scale": 0.5})
	if err != nil {
		t.Fatalf("pdf with scale: %v", err)
	}

	if result.IsError {
		t.Fatalf("pdf scale error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	img, ok := result.Content[0].(*mcp.ImageContent)
	if !ok {
		t.Fatal("expected ImageContent")
	}

	if len(img.Data) == 0 {
		t.Error("expected non-empty PDF data")
	}
}

func TestPDFToolWithOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
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
