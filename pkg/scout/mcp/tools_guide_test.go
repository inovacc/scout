package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGuideStartMissingURL(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "guide_start", map[string]any{})
	if err != nil {
		t.Fatalf("guide_start: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for guide_start without url")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "url is required") {
		t.Errorf("expected 'url is required', got: %s", text)
	}
}

func TestGuideStepNoRecording(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "guide_step", map[string]any{"annotation": "test"})
	if err != nil {
		t.Fatalf("guide_step: %v", err)
	}

	if !result.IsError {
		t.Error("expected error when no guide recording is in progress")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "no guide recording") {
		t.Errorf("expected 'no guide recording' message, got: %s", text)
	}
}

func TestGuideFinishNoRecording(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "guide_finish", map[string]any{})
	if err != nil {
		t.Fatalf("guide_finish: %v", err)
	}

	if !result.IsError {
		t.Error("expected error when no guide recording is in progress")
	}
}

func TestGuideStartTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "guide_start", map[string]any{
		"url":   ts.URL + "/",
		"title": "Test Guide",
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("guide_start: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("guide_start error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Test Guide") {
		t.Errorf("expected 'Test Guide' in response, got: %s", text)
	}
}

func TestGuideFullFlow(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Start guide.
	result, err := callTool(ctx, cs, "guide_start", map[string]any{
		"url": ts.URL + "/",
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("guide_start: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("guide_start error: %s", text)
	}

	// Navigate to page2 for next step.
	navigateHelper(t, ctx, cs, ts.URL+"/page2")

	// Add a step.
	result, err = callTool(ctx, cs, "guide_step", map[string]any{
		"annotation": "Navigated to page 2",
	})
	if err != nil {
		t.Fatalf("guide_step: %v", err)
	}

	if result.IsError {
		t.Fatalf("guide_step error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	// Finish guide.
	result, err = callTool(ctx, cs, "guide_finish", map[string]any{})
	if err != nil {
		t.Fatalf("guide_finish: %v", err)
	}

	if result.IsError {
		t.Fatalf("guide_finish error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty markdown guide")
	}
}
