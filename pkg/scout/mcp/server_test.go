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

// newTestHTTPServer returns an httptest.Server serving simple HTML pages.
func newTestHTTPServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Test Page</title></head><body><h1>Hello Scout</h1><p>Test content</p></body></html>`))
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Page Two</title></head><body><h1>Page 2</h1></body></html>`))
	})
	return httptest.NewServer(mux)
}

// connectTestClient creates an MCP server+client pair connected via in-memory transport.
func connectTestClient(t *testing.T, cfg ServerConfig) *mcp.ClientSession {
	t.Helper()

	server := NewServer(cfg)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)

	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()

	_, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() {
		_ = cs.Close()
	})

	return cs
}

// callTool wraps cs.CallTool with map[string]any arguments.
// The MCP SDK's CallToolParams.Arguments is `any` and gets JSON-marshaled once,
// so we must pass a map, not pre-marshaled json.RawMessage.
func callTool(ctx context.Context, cs *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

func skipIfNoBrowser(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "launch browser") ||
			strings.Contains(msg, "browser bin") ||
			strings.Contains(msg, "chrome") ||
			strings.Contains(msg, "chromium") ||
			strings.Contains(msg, "executable") ||
			strings.Contains(msg, "Failed to get the browser") {
			t.Skipf("browser not available: %v", err)
		}
	}
}

// navigateHelper navigates to url and skips/fatals on error.
func navigateHelper(t *testing.T, ctx context.Context, cs *mcp.ClientSession, url string) {
	t.Helper()
	result, err := callTool(ctx, cs, "navigate", map[string]any{"url": url})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("navigate: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("navigate error: %s", text)
	}
}

func TestNavigateTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "navigate", map[string]any{"url": ts.URL + "/"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("CallTool navigate: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("navigate returned error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Test Page") {
		t.Errorf("expected title in response, got: %s", text)
	}
}

func TestNavigateToolBadParams(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Invalid JSON arguments via raw message
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "navigate",
		Arguments: json.RawMessage(`{invalid`),
	})
	if err != nil {
		// JSON-RPC level error is acceptable
		return
	}

	if !result.IsError {
		t.Error("expected error result for invalid JSON params")
	}
}

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

func TestExtractTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "extract", map[string]any{"selector": "h1"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if result.IsError {
		t.Fatalf("extract error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Hello Scout") {
		t.Errorf("expected 'Hello Scout', got: %s", text)
	}
}

func TestEvalTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/")

	result, err := callTool(ctx, cs, "eval", map[string]any{"expression": "() => 1 + 2"})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	if result.IsError {
		t.Fatalf("eval error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "3") {
		t.Errorf("expected '3', got: %s", text)
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

func TestListTools(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expected := []string{"navigate", "click", "type", "screenshot", "snapshot", "extract", "eval", "back", "forward", "wait", "search", "fetch", "pdf", "session_list", "session_reset", "open", "ping", "curl", "markdown", "table", "meta", "cookie", "header", "block", "form_detect", "form_fill", "form_submit", "crawl", "detect", "storage", "hijack", "har", "swagger"}
	for _, name := range expected {
		if !toolNames[name] {
			t.Errorf("expected tool %q not found in server tools", name)
		}
	}
}

func TestListResources(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := cs.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}

	uris := make(map[string]bool)
	for _, res := range result.Resources {
		uris[res.URI] = true
	}

	expected := []string{"scout://page/markdown", "scout://page/url", "scout://page/title"}
	for _, uri := range expected {
		if !uris[uri] {
			t.Errorf("expected resource %q not found", uri)
		}
	}
}

func TestSearchTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	// Search requires a real browser and network; verify it returns an error or results gracefully.
	result, err := callTool(ctx, cs, "search", map[string]any{"query": "test query", "engine": "google"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("search: %v", err)
	}

	// Either a valid result or an error result (e.g., no network, CAPTCHA) is acceptable.
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Logf("search returned error (expected in test env): %s", text)
	}
}

func TestFetchTool(t *testing.T) {
	ts := newTestHTTPServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := callTool(ctx, cs, "fetch", map[string]any{"url": ts.URL + "/", "mode": "full"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("fetch: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("fetch error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "Hello Scout") && !strings.Contains(text, "Test Page") {
		t.Errorf("expected page content in fetch result, got: %s", text)
	}
}

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

func TestServeSSE(t *testing.T) {
	cfg := ServerConfig{Headless: true, Logger: slog.Default()}

	handler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		return NewServer(cfg)
	}, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Connect an SSE client to the test server.
	client := mcp.NewClient(&mcp.Implementation{Name: "test-sse-client", Version: "1.0.0"}, nil)
	transport := &mcp.SSEClientTransport{Endpoint: ts.URL}

	ctx := context.Background()
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("SSE client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// Verify tools are listed over SSE transport.
	result, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools over SSE: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["navigate"] {
		t.Error("expected 'navigate' tool over SSE transport")
	}
	if !toolNames["screenshot"] {
		t.Error("expected 'screenshot' tool over SSE transport")
	}
}

func TestServeSSEListenError(t *testing.T) {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use an invalid address to trigger a listen error.
	err := ServeSSE(ctx, logger, "invalid-addr-no-port", true, false, "", 0)
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

// toolErr wraps a string as an error for skipIfNoBrowser.
type toolErr struct{ msg string }

func (e *toolErr) Error() string { return e.msg }
