package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
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

	mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Form Page</title></head><body><form><input type="text" name="username" id="username"><button type="submit">Submit</button></form></body></html>`))
	})
	mux.HandleFunc("/table", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Table Page</title></head><body><table><thead><tr><th>Name</th><th>Age</th></tr></thead><tbody><tr><td>Alice</td><td>30</td></tr><tr><td>Bob</td><td>25</td></tr></tbody></table></body></html>`))
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
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("navigate error: %s", text)
	}
}

// toolError wraps a string as an error for skipIfNoBrowser.
type toolError struct{ msg string }

func (e *toolError) Error() string { return e.msg }

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

	expected := []string{"navigate", "click", "type", "screenshot", "snapshot", "pdf", "extract", "eval", "back", "forward", "wait", "session_list", "session_reset", "open", "swarm_crawl", "ws_listen", "ws_send", "ws_connections"}
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

	ctx := t.Context()

	// Use an invalid address to trigger a listen error.
	err := ServeSSE(ctx, logger, "invalid-addr-no-port", true, false, "", 0)
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

func TestSanitizeMCPName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello-world", "hello_world"},
		{"https://example.com/api", "https_example_com_api"},
		{"my tool name", "my_tool_name"},
		{"CamelCase123", "CamelCase123"},
		{"---leading", "leading"},
		{"trailing---", "trailing"},
		{"multi---dashes", "multi_dashes"},
		{"a.b.c", "a_b_c"},
		{"", ""},
		{"@#$%", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeMCPName(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeMCPName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRegisterWebMCPTools(t *testing.T) {
	cfg := ServerConfig{Headless: true}
	server := NewServer(cfg)

	tools := []scout.WebMCPTool{
		{
			Name:        "get_data",
			Description: "Gets some data",
			ServerURL:   "https://api.example.com",
			Source:      "meta",
		},
		{
			Name:        "post_form",
			Description: "Posts a form",
			ServerURL:   "",
			Source:      "well-known",
		},
	}

	callCount := 0
	callFn := func(name string, params map[string]any) (*scout.WebMCPToolResult, error) {
		callCount++
		return &scout.WebMCPToolResult{Content: "ok"}, nil
	}

	RegisterWebMCPTools(server, tools, callFn)

	// Connect and verify tools are registered.
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
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

	defer func() { _ = cs.Close() }()

	result, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	// Look for our registered WebMCP tools.
	foundGetData := false
	foundPostForm := false

	for _, tool := range result.Tools {
		if strings.Contains(tool.Name, "get_data") {
			foundGetData = true
		}

		if strings.Contains(tool.Name, "post_form") {
			foundPostForm = true
		}
	}

	if !foundGetData {
		t.Error("expected webmcp tool containing 'get_data' in tool list")
	}

	if !foundPostForm {
		t.Error("expected webmcp tool containing 'post_form' in tool list")
	}
}

func TestNewServerNilLogger(t *testing.T) {
	// Passing nil Logger should not panic.
	server := NewServer(ServerConfig{Headless: true})
	if server == nil {
		t.Fatal("NewServer returned nil with nil logger")
	}
}

func TestNewServerStealth(t *testing.T) {
	server := NewServer(ServerConfig{Headless: true, Stealth: true, Logger: slog.Default()})
	if server == nil {
		t.Fatal("NewServer returned nil with stealth=true")
	}
}

func TestNewServerBrowserBin(t *testing.T) {
	server := NewServer(ServerConfig{Headless: true, BrowserBin: "/nonexistent/chrome", Logger: slog.Default()})
	if server == nil {
		t.Fatal("NewServer returned nil with browserBin set")
	}
}

func TestCallToolClickBadJSON(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "click",
		Arguments: json.RawMessage(`{invalid`),
	})
	if err != nil {
		return
	}

	if !result.IsError {
		t.Error("expected error for invalid JSON args")
	}
}

func TestCallToolTypeBadJSON(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "type",
		Arguments: json.RawMessage(`{invalid`),
	})
	if err != nil {
		return
	}

	if !result.IsError {
		t.Error("expected error for invalid JSON args")
	}
}

func TestCallToolExtractBadJSON(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "extract",
		Arguments: json.RawMessage(`{invalid`),
	})
	if err != nil {
		return
	}

	if !result.IsError {
		t.Error("expected error for invalid JSON args")
	}
}

func TestCallToolEvalBadJSON(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "eval",
		Arguments: json.RawMessage(`{invalid`),
	})
	if err != nil {
		return
	}

	if !result.IsError {
		t.Error("expected error for invalid JSON args")
	}
}

func TestCallToolOpenBadJSON(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "open",
		Arguments: json.RawMessage(`{invalid`),
	})
	if err != nil {
		return
	}

	if !result.IsError {
		t.Error("expected error for invalid JSON args")
	}
}

func TestRegisterWebMCPToolsCall(t *testing.T) {
	cfg := ServerConfig{Headless: true}
	server := NewServer(cfg)

	var (
		calledName   string
		calledParams map[string]any
	)

	tools := []scout.WebMCPTool{
		{
			Name:        "test_action",
			Description: "Test action tool",
			ServerURL:   "https://api.test.com",
			Source:      "meta",
		},
	}

	callFn := func(name string, params map[string]any) (*scout.WebMCPToolResult, error) {
		calledName = name
		calledParams = params

		return &scout.WebMCPToolResult{Content: "result ok"}, nil
	}

	RegisterWebMCPTools(server, tools, callFn)

	// Connect and call the tool.
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
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

	defer func() { _ = cs.Close() }()

	// Call the registered WebMCP tool.
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "webmcp_https_api_test_com_test_action",
		Arguments: map[string]any{"key": "value"},
	})
	if err != nil {
		t.Fatalf("call webmcp tool: %v", err)
	}

	if result.IsError {
		t.Fatalf("webmcp tool error: %s", result.Content[0].(*mcp.TextContent).Text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text != "result ok" {
		t.Errorf("expected 'result ok', got: %s", text)
	}

	if calledName != "test_action" {
		t.Errorf("expected callFn name 'test_action', got: %s", calledName)
	}

	if calledParams["key"] != "value" {
		t.Errorf("expected params key=value, got: %v", calledParams)
	}
}

func TestRegisterWebMCPToolsError(t *testing.T) {
	cfg := ServerConfig{Headless: true}
	server := NewServer(cfg)

	tools := []scout.WebMCPTool{
		{
			Name:        "fail_tool",
			Description: "Fails",
			Source:      "well-known",
		},
	}

	callFn := func(name string, params map[string]any) (*scout.WebMCPToolResult, error) {
		return &scout.WebMCPToolResult{Content: "bad thing happened", IsError: true}, nil
	}

	RegisterWebMCPTools(server, tools, callFn)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
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

	defer func() { _ = cs.Close() }()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "webmcp_well_known_fail_tool",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call fail tool: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for failing WebMCP tool")
	}
}

func TestNewServerIdleTimeout(t *testing.T) {
	cfg := ServerConfig{
		Headless:    true,
		IdleTimeout: 5 * time.Second,
	}

	called := false
	server := NewServer(cfg, func() { called = true })

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	// The cancel function should not have been called yet.
	if called {
		t.Error("cancelOnIdle should not be called at construction")
	}
}

func TestNewServerIdleTimeoutNilCallback(t *testing.T) {
	// Passing nil callback should not create an idle timer.
	cfg := ServerConfig{
		Headless:    true,
		IdleTimeout: 5 * time.Second,
	}

	server := NewServer(cfg, nil)
	if server == nil {
		t.Fatal("NewServer returned nil with nil callback")
	}
}

func TestNewServerNoIdleTimeout(t *testing.T) {
	// IdleTimeout=0 with a callback should not create an idle timer.
	cfg := ServerConfig{
		Headless:    true,
		IdleTimeout: 0,
	}

	server := NewServer(cfg, func() {})
	if server == nil {
		t.Fatal("NewServer returned nil with zero timeout")
	}
}

func TestJsonResultMarshalError(t *testing.T) {
	// channels can't be marshalled.
	ch := make(chan int)

	result, err := jsonResult(ch)
	if err != nil {
		t.Fatalf("jsonResult: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for unmarshalable value")
	}
}

func TestWebMCPToolsWithInputSchema(t *testing.T) {
	cfg := ServerConfig{Headless: true}
	server := NewServer(cfg)

	tools := []scout.WebMCPTool{
		{
			Name:        "schema_tool",
			Description: "Tool with schema",
			Source:      "meta",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
		},
	}

	callFn := func(name string, params map[string]any) (*scout.WebMCPToolResult, error) {
		return &scout.WebMCPToolResult{Content: "ok"}, nil
	}

	RegisterWebMCPTools(server, tools, callFn)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()

	_, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	defer func() { _ = cs.Close() }()

	// Verify tool is listed.
	result, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	found := false

	for _, tool := range result.Tools {
		if strings.Contains(tool.Name, "schema_tool") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected webmcp tool containing 'schema_tool'")
	}

	// Call with no args.
	callResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "webmcp_meta_schema_tool",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call schema tool: %v", err)
	}

	if callResult.IsError {
		t.Errorf("unexpected error: %s", callResult.Content[0].(*mcp.TextContent).Text)
	}
}
