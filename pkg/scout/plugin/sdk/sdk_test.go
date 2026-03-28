package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// --- mock handlers ---

type mockMode struct {
	results []Result
	err     error
}

func (m *mockMode) Scrape(_ context.Context, _ ScrapeParams) ([]Result, error) {
	return m.results, m.err
}

type mockExtractor struct {
	data any
	err  error
}

func (m *mockExtractor) Extract(_ context.Context, _ ExtractParams) (any, error) {
	return m.data, m.err
}

type mockTool struct {
	result *ToolResult
	err    error
}

func (m *mockTool) Call(_ context.Context, _ map[string]any) (*ToolResult, error) {
	return m.result, m.err
}

// --- helpers ---

// newTestServer creates a Server whose encoder writes to buf instead of stdout.
func newTestServer(buf *bytes.Buffer) *Server {
	s := &Server{
		modes:       make(map[string]ModeHandler),
		extractors:  make(map[string]ExtractorHandler),
		tools:       make(map[string]ToolHandler),
		commands:    make(map[string]CommandHandler),
		completions: make(map[string]CompletionHandler),
		resources:   make(map[string]ResourceHandler),
		prompts:     make(map[string]PromptHandler),
		sinks:       make(map[string]SinkHandler),
		encoder:     json.NewEncoder(buf),
	}

	return s
}

// decodeResponse is a generic JSON-RPC response envelope.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func decodeResponse(t *testing.T, buf *bytes.Buffer) rpcResponse {
	t.Helper()

	var resp rpcResponse
	if err := json.NewDecoder(buf).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return resp
}

// --- tests ---

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("NewServer returned nil")
	}

	if s.modes == nil || s.extractors == nil || s.tools == nil {
		t.Fatal("NewServer did not initialize maps")
	}

	if s.encoder == nil {
		t.Fatal("NewServer did not initialize encoder")
	}
}

func TestRegisterMode(t *testing.T) {
	s := NewServer()
	h := &mockMode{}
	s.RegisterMode("test-mode", h)

	if got, ok := s.modes["test-mode"]; !ok {
		t.Fatal("mode not registered")
	} else if got != h {
		t.Fatal("mode handler mismatch")
	}
}

func TestRegisterExtractor(t *testing.T) {
	s := NewServer()
	h := &mockExtractor{}
	s.RegisterExtractor("test-ext", h)

	if got, ok := s.extractors["test-ext"]; !ok {
		t.Fatal("extractor not registered")
	} else if got != h {
		t.Fatal("extractor handler mismatch")
	}
}

func TestRegisterTool(t *testing.T) {
	s := NewServer()
	h := &mockTool{}
	s.RegisterTool("test-tool", h)

	if got, ok := s.tools["test-tool"]; !ok {
		t.Fatal("tool not registered")
	} else if got != h {
		t.Fatal("tool handler mismatch")
	}
}

func TestTextResult(t *testing.T) {
	r := TextResult("hello")
	if r.IsError {
		t.Fatal("TextResult should not be an error")
	}

	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(r.Content))
	}

	if r.Content[0].Type != "text" {
		t.Fatalf("expected type 'text', got %q", r.Content[0].Type)
	}

	if r.Content[0].Text != "hello" {
		t.Fatalf("expected text 'hello', got %q", r.Content[0].Text)
	}
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("bad thing")
	if !r.IsError {
		t.Fatal("ErrorResult should have IsError=true")
	}

	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(r.Content))
	}

	if r.Content[0].Text != "bad thing" {
		t.Fatalf("expected text 'bad thing', got %q", r.Content[0].Text)
	}
}

func TestToolHandlerFunc(t *testing.T) {
	called := false
	fn := ToolHandlerFunc(func(_ context.Context, args map[string]any) (*ToolResult, error) {
		called = true
		name, _ := args["name"].(string)

		return TextResult("hi " + name), nil
	})

	res, err := fn.Call(context.Background(), map[string]any{"name": "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("handler function was not called")
	}

	if res.Content[0].Text != "hi world" {
		t.Fatalf("unexpected result text: %q", res.Content[0].Text)
	}
}

func TestCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(s *Server)
		expected []string
	}{
		{
			name:     "empty server",
			setup:    func(_ *Server) {},
			expected: nil,
		},
		{
			name: "modes only",
			setup: func(s *Server) {
				s.RegisterMode("m1", &mockMode{})
			},
			expected: []string{"scraper_mode"},
		},
		{
			name: "extractors only",
			setup: func(s *Server) {
				s.RegisterExtractor("e1", &mockExtractor{})
			},
			expected: []string{"extractor"},
		},
		{
			name: "tools only",
			setup: func(s *Server) {
				s.RegisterTool("t1", &mockTool{})
			},
			expected: []string{"mcp_tool"},
		},
		{
			name: "all capabilities",
			setup: func(s *Server) {
				s.RegisterMode("m1", &mockMode{})
				s.RegisterExtractor("e1", &mockExtractor{})
				s.RegisterTool("t1", &mockTool{})
			},
			expected: []string{"scraper_mode", "extractor", "mcp_tool"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			s := newTestServer(&buf)
			tc.setup(s)
			caps := s.capabilities()

			if len(caps) != len(tc.expected) {
				t.Fatalf("expected %d capabilities, got %d: %v", len(tc.expected), len(caps), caps)
			}

			for i, exp := range tc.expected {
				if caps[i] != exp {
					t.Errorf("capability[%d] = %q, want %q", i, caps[i], exp)
				}
			}
		})
	}
}

func TestHandleRequest_Initialize(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterMode("m1", &mockMode{})
	s.RegisterTool("t1", &mockTool{})

	req := &request{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	if resp.ID != 1 {
		t.Fatalf("expected id 1, got %d", resp.ID)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result["name"] != "plugin" {
		t.Fatalf("expected name 'plugin', got %q", result["name"])
	}

	if result["version"] != "1.0.0" {
		t.Fatalf("expected version '1.0.0', got %q", result["version"])
	}
}

func TestHandleRequest_Scrape(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		handler   *mockMode
		params    string
		wantErr   bool
		errCode   int
		wantCount int
	}{
		{
			name:      "success",
			mode:      "news",
			handler:   &mockMode{results: []Result{{Type: "article", ID: "1", Content: "hello"}}},
			params:    `{"mode":"news"}`,
			wantCount: 1,
		},
		{
			name:    "unknown mode",
			mode:    "",
			handler: nil,
			params:  `{"mode":"unknown"}`,
			wantErr: true,
			errCode: -32601,
		},
		{
			name:    "handler error",
			mode:    "fail",
			handler: &mockMode{err: fmt.Errorf("scrape failed")},
			params:  `{"mode":"fail"}`,
			wantErr: true,
			errCode: -32603,
		},
		{
			name:    "invalid params",
			mode:    "x",
			handler: &mockMode{},
			params:  `not json`,
			wantErr: true,
			errCode: -32602,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			s := newTestServer(&buf)

			if tc.handler != nil {
				s.RegisterMode(tc.mode, tc.handler)
			}

			req := &request{
				JSONRPC: "2.0",
				ID:      42,
				Method:  "scrape",
				Params:  json.RawMessage(tc.params),
			}
			s.handleRequest(context.Background(), req)

			resp := decodeResponse(t, &buf)
			if tc.wantErr {
				if resp.Error == nil {
					t.Fatal("expected error response")
				}

				if resp.Error.Code != tc.errCode {
					t.Fatalf("expected error code %d, got %d", tc.errCode, resp.Error.Code)
				}
			} else {
				if resp.Error != nil {
					t.Fatalf("unexpected error: %+v", resp.Error)
				}

				var results []Result
				if err := json.Unmarshal(resp.Result, &results); err != nil {
					t.Fatalf("failed to unmarshal results: %v", err)
				}

				if len(results) != tc.wantCount {
					t.Fatalf("expected %d results, got %d", tc.wantCount, len(results))
				}
			}
		})
	}
}

func TestHandleRequest_Extract(t *testing.T) {
	tests := []struct {
		name    string
		extName string
		handler *mockExtractor
		params  string
		wantErr bool
		errCode int
	}{
		{
			name:    "success",
			extName: "prices",
			handler: &mockExtractor{data: map[string]any{"price": 9.99}},
			params:  `{"name":"prices","html":"<div>9.99</div>","url":"https://example.com"}`,
		},
		{
			name:    "unknown extractor",
			extName: "",
			handler: nil,
			params:  `{"name":"missing"}`,
			wantErr: true,
			errCode: -32601,
		},
		{
			name:    "handler error",
			extName: "broken",
			handler: &mockExtractor{err: fmt.Errorf("extract failed")},
			params:  `{"name":"broken","html":"<div/>","url":"http://x"}`,
			wantErr: true,
			errCode: -32603,
		},
		{
			name:    "invalid params",
			extName: "x",
			handler: &mockExtractor{},
			params:  `{bad}`,
			wantErr: true,
			errCode: -32602,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			s := newTestServer(&buf)
			if tc.handler != nil {
				s.RegisterExtractor(tc.extName, tc.handler)
			}

			req := &request{
				JSONRPC: "2.0",
				ID:      7,
				Method:  "extract",
				Params:  json.RawMessage(tc.params),
			}
			s.handleRequest(context.Background(), req)

			resp := decodeResponse(t, &buf)
			if tc.wantErr {
				if resp.Error == nil {
					t.Fatal("expected error response")
				}

				if resp.Error.Code != tc.errCode {
					t.Fatalf("expected error code %d, got %d", tc.errCode, resp.Error.Code)
				}
			} else if resp.Error != nil {
				t.Fatalf("unexpected error: %+v", resp.Error)
			}
		})
	}
}

func TestHandleRequest_ToolCall(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		handler *mockTool
		params  string
		wantErr bool
		errCode int
	}{
		{
			name:    "success",
			tool:    "greet",
			handler: &mockTool{result: TextResult("hello")},
			params:  `{"name":"greet","arguments":{"who":"world"}}`,
		},
		{
			name:    "unknown tool",
			tool:    "",
			handler: nil,
			params:  `{"name":"missing"}`,
			wantErr: true,
			errCode: -32601,
		},
		{
			name:    "handler error",
			tool:    "fail",
			handler: &mockTool{err: fmt.Errorf("tool broke")},
			params:  `{"name":"fail"}`,
			wantErr: true,
			errCode: -32603,
		},
		{
			name:    "invalid params",
			tool:    "x",
			handler: &mockTool{},
			params:  `not-json`,
			wantErr: true,
			errCode: -32602,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			s := newTestServer(&buf)

			if tc.handler != nil {
				s.RegisterTool(tc.tool, tc.handler)
			}

			req := &request{
				JSONRPC: "2.0",
				ID:      99,
				Method:  "tool/call",
				Params:  json.RawMessage(tc.params),
			}
			s.handleRequest(context.Background(), req)

			resp := decodeResponse(t, &buf)
			if tc.wantErr {
				if resp.Error == nil {
					t.Fatal("expected error response")
				}

				if resp.Error.Code != tc.errCode {
					t.Fatalf("expected error code %d, got %d", tc.errCode, resp.Error.Code)
				}
			} else {
				if resp.Error != nil {
					t.Fatalf("unexpected error: %+v", resp.Error)
				}

				var result ToolResult
				if err := json.Unmarshal(resp.Result, &result); err != nil {
					t.Fatalf("failed to unmarshal tool result: %v", err)
				}

				if len(result.Content) == 0 {
					t.Fatal("expected non-empty content")
				}
			}
		})
	}
}

func TestHandleRequest_UnknownMethod(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 5, Method: "bogus"}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}

	if resp.Error.Code != -32601 {
		t.Fatalf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestEmit(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	err := s.Emit("progress", map[string]any{"pct": 50})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	var msg map[string]any
	if err := json.NewDecoder(&buf).Decode(&msg); err != nil {
		t.Fatalf("failed to decode emitted message: %v", err)
	}

	if msg["jsonrpc"] != "2.0" {
		t.Fatalf("expected jsonrpc 2.0, got %v", msg["jsonrpc"])
	}

	if msg["method"] != "progress" {
		t.Fatalf("expected method 'progress', got %v", msg["method"])
	}

	if msg["id"] != nil {
		t.Fatal("notification should not have an id")
	}
}

func TestEmitResult(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	err := s.EmitResult(Result{Type: "article", ID: "abc", Content: "test"})
	if err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	var msg map[string]any
	if err := json.NewDecoder(&buf).Decode(&msg); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if msg["method"] != "result" {
		t.Fatalf("expected method 'result', got %v", msg["method"])
	}

	params, ok := msg["params"].(map[string]any)
	if !ok {
		t.Fatal("params not a map")
	}

	if params["id"] != "abc" {
		t.Fatalf("expected id 'abc', got %v", params["id"])
	}
}

func TestEmitLog(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	err := s.EmitLog("info", "starting up")
	if err != nil {
		t.Fatalf("EmitLog failed: %v", err)
	}

	var msg map[string]any
	if err := json.NewDecoder(&buf).Decode(&msg); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if msg["method"] != "log" {
		t.Fatalf("expected method 'log', got %v", msg["method"])
	}

	params, ok := msg["params"].(map[string]any)
	if !ok {
		t.Fatal("params not a map")
	}

	if params["level"] != "info" {
		t.Fatalf("expected level 'info', got %v", params["level"])
	}

	if params["message"] != "starting up" {
		t.Fatalf("expected message 'starting up', got %v", params["message"])
	}
}

func TestSendError(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.sendError(10, -32600, "invalid request")

	resp := decodeResponse(t, &buf)
	if resp.ID != 10 {
		t.Fatalf("expected id 10, got %d", resp.ID)
	}

	if resp.Error == nil {
		t.Fatal("expected error in response")
	}

	if resp.Error.Code != -32600 {
		t.Fatalf("expected code -32600, got %d", resp.Error.Code)
	}

	if resp.Error.Message != "invalid request" {
		t.Fatalf("expected message 'invalid request', got %q", resp.Error.Message)
	}
}

func TestSendResult(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	s.sendResult(20, map[string]string{"status": "ok"})

	resp := decodeResponse(t, &buf)
	if resp.ID != 20 {
		t.Fatalf("expected id 20, got %d", resp.ID)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %q", result["status"])
	}
}

func TestToolHandlerFunc_Error(t *testing.T) {
	fn := ToolHandlerFunc(func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return nil, fmt.Errorf("something went wrong")
	})

	res, err := fn.Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if res != nil {
		t.Fatal("expected nil result on error")
	}
}

func TestRegisterMultipleModes(t *testing.T) {
	s := NewServer()
	s.RegisterMode("a", &mockMode{})
	s.RegisterMode("b", &mockMode{})
	s.RegisterMode("a", &mockMode{}) // overwrite

	if len(s.modes) != 2 {
		t.Fatalf("expected 2 modes, got %d", len(s.modes))
	}
}

func TestScrapeResultFields(t *testing.T) {
	r := Result{
		Type:      "post",
		Source:    "reddit",
		ID:        "xyz",
		Timestamp: "2025-01-01T00:00:00Z",
		Author:    "user1",
		Content:   "hello world",
		URL:       "https://example.com",
		Metadata:  map[string]any{"score": 42},
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Type != "post" || decoded.Source != "reddit" || decoded.ID != "xyz" {
		t.Fatal("fields mismatch after round-trip")
	}

	if decoded.Metadata["score"] != float64(42) {
		t.Fatalf("metadata score mismatch: %v", decoded.Metadata["score"])
	}
}

func TestToolResultJSON(t *testing.T) {
	tr := &ToolResult{
		Content: []ContentItem{
			{Type: "text", Text: "line1"},
			{Type: "text", Text: "line2"},
		},
		IsError: true,
	}

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.Content) != 2 {
		t.Fatalf("expected 2 content items, got %d", len(decoded.Content))
	}

	if !decoded.IsError {
		t.Fatal("expected IsError=true")
	}
}

func TestExtractParamsJSON(t *testing.T) {
	ep := ExtractParams{
		Name:   "price",
		HTML:   "<span>$9.99</span>",
		URL:    "https://shop.example.com",
		Params: map[string]any{"currency": "USD"},
	}

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ExtractParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Name != "price" || decoded.URL != "https://shop.example.com" {
		t.Fatal("field mismatch after round-trip")
	}
}

func TestScrapeParamsJSON(t *testing.T) {
	sp := ScrapeParams{
		Mode:    "news",
		Options: map[string]any{"limit": float64(10)},
	}

	data, err := json.Marshal(sp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ScrapeParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Mode != "news" {
		t.Fatalf("expected mode 'news', got %q", decoded.Mode)
	}
}
