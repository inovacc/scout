package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// --- mock handlers for new capabilities ---

type mockAuth struct {
	loginURL string
	detected bool
	session  *SessionData
	valid    bool
	reason   string
	err      error
}

func (m *mockAuth) LoginURL() string { return m.loginURL }

func (m *mockAuth) Detect(_ context.Context, _ PageState) (bool, error) {
	return m.detected, m.err
}

func (m *mockAuth) Capture(_ context.Context, _ PageState) (*SessionData, error) {
	return m.session, m.err
}

func (m *mockAuth) Validate(_ context.Context, _ SessionData) (bool, string, error) {
	return m.valid, m.reason, m.err
}

type mockResource struct {
	content  string
	mimeType string
	err      error
}

func (m *mockResource) Read(_ context.Context, _ string) (string, string, error) {
	return m.content, m.mimeType, m.err
}

type mockPrompt struct {
	messages []PromptMessage
	err      error
}

func (m *mockPrompt) Get(_ context.Context, _ string, _ map[string]string) ([]PromptMessage, error) {
	return m.messages, m.err
}

type mockSink struct {
	initErr  error
	writeErr error
	flushErr error
	closeErr error
}

func (m *mockSink) Init(_ context.Context, _ map[string]any) error  { return m.initErr }
func (m *mockSink) Write(_ context.Context, _ []map[string]any) error { return m.writeErr }
func (m *mockSink) Flush(_ context.Context) error                     { return m.flushErr }
func (m *mockSink) Close(_ context.Context) error                     { return m.closeErr }

type mockMiddleware struct {
	result *MiddlewareResult
	err    error
}

func (m *mockMiddleware) BeforeNavigate(_ context.Context, _ MiddlewareContext) (*MiddlewareResult, error) {
	return m.result, m.err
}

func (m *mockMiddleware) AfterLoad(_ context.Context, _ MiddlewareContext) (*MiddlewareResult, error) {
	return m.result, m.err
}

func (m *mockMiddleware) BeforeExtract(_ context.Context, _ MiddlewareContext) (*MiddlewareResult, error) {
	return m.result, m.err
}

func (m *mockMiddleware) OnError(_ context.Context, _ MiddlewareContext) (*MiddlewareResult, error) {
	return m.result, m.err
}

type mockEvent struct {
	called bool
}

func (m *mockEvent) OnEvent(_ context.Context, _ EventData) {
	m.called = true
}

type mockCommand struct {
	result *CommandResult
	err    error
}

func (m *mockCommand) Execute(_ context.Context, _ CommandParams) (*CommandResult, error) {
	return m.result, m.err
}

type mockCompletion struct {
	suggestions []string
	err         error
}

func (m *mockCompletion) Complete(_ context.Context, _ CompletionParams) ([]string, error) {
	return m.suggestions, m.err
}

// --- Registration tests ---

func TestRegisterAuth(t *testing.T) {
	s := NewServer()
	h := &mockAuth{loginURL: "https://example.com/login"}
	s.RegisterAuth(h)

	if s.auth == nil {
		t.Fatal("auth handler not registered")
	}
}

func TestRegisterResource(t *testing.T) {
	s := NewServer()
	h := &mockResource{content: "data", mimeType: "text/plain"}
	s.RegisterResource("myapp://data", h)

	if _, ok := s.resources["myapp://data"]; !ok {
		t.Fatal("resource handler not registered")
	}
}

func TestRegisterPrompt(t *testing.T) {
	s := NewServer()
	h := &mockPrompt{}
	s.RegisterPrompt("analyze", h)

	if _, ok := s.prompts["analyze"]; !ok {
		t.Fatal("prompt handler not registered")
	}
}

func TestRegisterSink(t *testing.T) {
	s := NewServer()
	h := &mockSink{}
	s.RegisterSink("s3", h)

	if _, ok := s.sinks["s3"]; !ok {
		t.Fatal("sink handler not registered")
	}
}

func TestRegisterMiddleware(t *testing.T) {
	s := NewServer()
	h := &mockMiddleware{result: AllowResult()}
	s.RegisterMiddleware(h)

	if s.middleware == nil {
		t.Fatal("middleware handler not registered")
	}
}

func TestOnEvent(t *testing.T) {
	s := NewServer()
	h := &mockEvent{}
	s.OnEvent(h)

	if s.eventHandler == nil {
		t.Fatal("event handler not registered")
	}
}

func TestRegisterCommand(t *testing.T) {
	s := NewServer()
	h := &mockCommand{result: CommandOutput("done")}
	s.RegisterCommand("extract", h)

	if _, ok := s.commands["extract"]; !ok {
		t.Fatal("command handler not registered")
	}
}

func TestRegisterCompletion(t *testing.T) {
	s := NewServer()
	h := &mockCompletion{suggestions: []string{"--verbose", "--format"}}
	s.RegisterCompletion("extract", h)

	if _, ok := s.completions["extract"]; !ok {
		t.Fatal("completion handler not registered")
	}
}

// --- handleRequest tests for auth ---

func TestHandleRequest_AuthLoginURL(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{loginURL: "https://example.com/login"})

	req := &request{JSONRPC: "2.0", ID: 1, Method: "auth/login_url"}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if result["url"] != "https://example.com/login" {
		t.Errorf("url = %q", result["url"])
	}
}

func TestHandleRequest_AuthLoginURL_NoHandler(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 1, Method: "auth/login_url"}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error when no auth handler")
	}
}

func TestHandleRequest_AuthDetect_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{detected: true})

	params, _ := json.Marshal(PageState{URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 2, Method: "auth/detect", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result map[string]bool
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if !result["detected"] {
		t.Error("expected detected=true")
	}
}

func TestHandleRequest_AuthDetect_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{err: fmt.Errorf("detect failed")})

	params, _ := json.Marshal(PageState{URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 2, Method: "auth/detect", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_AuthDetect_NoHandler(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(PageState{URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 2, Method: "auth/detect", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error when no auth handler")
	}
}

func TestHandleRequest_AuthDetect_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{})

	req := &request{JSONRPC: "2.0", ID: 2, Method: "auth/detect", Params: json.RawMessage(`{bad}`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_AuthCapture_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{session: &SessionData{Provider: "github", URL: "https://github.com"}})

	params, _ := json.Marshal(PageState{URL: "https://github.com"})
	req := &request{JSONRPC: "2.0", ID: 3, Method: "auth/capture", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_AuthCapture_NoHandler(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(PageState{URL: "https://github.com"})
	req := &request{JSONRPC: "2.0", ID: 3, Method: "auth/capture", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error when no auth handler")
	}
}

func TestHandleRequest_AuthCapture_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{})

	req := &request{JSONRPC: "2.0", ID: 3, Method: "auth/capture", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_AuthCapture_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{err: fmt.Errorf("capture failed")})

	params, _ := json.Marshal(PageState{URL: "https://github.com"})
	req := &request{JSONRPC: "2.0", ID: 3, Method: "auth/capture", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_AuthValidate_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{valid: true})

	params, _ := json.Marshal(SessionData{Provider: "test"})
	req := &request{JSONRPC: "2.0", ID: 4, Method: "auth/validate", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_AuthValidate_NoHandler(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(SessionData{Provider: "test"})
	req := &request{JSONRPC: "2.0", ID: 4, Method: "auth/validate", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error when no auth handler")
	}
}

func TestHandleRequest_AuthValidate_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{})

	req := &request{JSONRPC: "2.0", ID: 4, Method: "auth/validate", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_AuthValidate_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{err: fmt.Errorf("validate failed")})

	params, _ := json.Marshal(SessionData{Provider: "test"})
	req := &request{JSONRPC: "2.0", ID: 4, Method: "auth/validate", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

// --- handleRequest tests for resource ---

func TestHandleRequest_ResourceRead_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterResource("myapp://data", &mockResource{content: "hello", mimeType: "text/plain"})

	params, _ := json.Marshal(map[string]string{"uri": "myapp://data"})
	req := &request{JSONRPC: "2.0", ID: 5, Method: "resource/read", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	if result["content"] != "hello" {
		t.Errorf("content = %q", result["content"])
	}
}

func TestHandleRequest_ResourceRead_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(map[string]string{"uri": "myapp://missing"})
	req := &request{JSONRPC: "2.0", ID: 5, Method: "resource/read", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for unknown resource")
	}
}

func TestHandleRequest_ResourceRead_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterResource("myapp://fail", &mockResource{err: fmt.Errorf("read failed")})

	params, _ := json.Marshal(map[string]string{"uri": "myapp://fail"})
	req := &request{JSONRPC: "2.0", ID: 5, Method: "resource/read", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_ResourceRead_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 5, Method: "resource/read", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_ResourceList(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterResource("myapp://a", &mockResource{})
	s.RegisterResource("myapp://b", &mockResource{})

	req := &request{JSONRPC: "2.0", ID: 6, Method: "resource/list"}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var list []map[string]string
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 {
		t.Errorf("got %d resources, want 2", len(list))
	}
}

// --- handleRequest tests for prompt ---

func TestHandleRequest_PromptGet_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterPrompt("analyze", &mockPrompt{
		messages: []PromptMessage{{Role: "user", Content: "Analyze this"}},
	})

	params, _ := json.Marshal(map[string]any{"name": "analyze", "arguments": map[string]string{"topic": "Go"}})
	req := &request{JSONRPC: "2.0", ID: 7, Method: "prompt/get", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_PromptGet_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(map[string]any{"name": "missing"})
	req := &request{JSONRPC: "2.0", ID: 7, Method: "prompt/get", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for unknown prompt")
	}
}

func TestHandleRequest_PromptGet_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterPrompt("fail", &mockPrompt{err: fmt.Errorf("prompt failed")})

	params, _ := json.Marshal(map[string]any{"name": "fail"})
	req := &request{JSONRPC: "2.0", ID: 7, Method: "prompt/get", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_PromptGet_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 7, Method: "prompt/get", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_PromptList(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterPrompt("p1", &mockPrompt{})
	s.RegisterPrompt("p2", &mockPrompt{})

	req := &request{JSONRPC: "2.0", ID: 8, Method: "prompt/list"}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var list []map[string]string
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 {
		t.Errorf("got %d prompts, want 2", len(list))
	}
}

// --- handleRequest tests for sink ---

func TestHandleRequest_SinkInit_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("s3", &mockSink{})

	params, _ := json.Marshal(map[string]any{"name": "s3", "config": map[string]any{"bucket": "test"}})
	req := &request{JSONRPC: "2.0", ID: 9, Method: "sink/init", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_SinkInit_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(map[string]any{"name": "missing", "config": map[string]any{}})
	req := &request{JSONRPC: "2.0", ID: 9, Method: "sink/init", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for unknown sink")
	}
}

func TestHandleRequest_SinkInit_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("fail", &mockSink{initErr: fmt.Errorf("init failed")})

	params, _ := json.Marshal(map[string]any{"name": "fail", "config": map[string]any{}})
	req := &request{JSONRPC: "2.0", ID: 9, Method: "sink/init", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_SinkInit_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 9, Method: "sink/init", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_SinkWrite_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("s3", &mockSink{})

	params, _ := json.Marshal(map[string]any{"name": "s3", "results": []map[string]any{{"key": "val"}}})
	req := &request{JSONRPC: "2.0", ID: 10, Method: "sink/write", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_SinkWrite_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(map[string]any{"name": "missing", "results": []map[string]any{}})
	req := &request{JSONRPC: "2.0", ID: 10, Method: "sink/write", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_SinkWrite_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("fail", &mockSink{writeErr: fmt.Errorf("write failed")})

	params, _ := json.Marshal(map[string]any{"name": "fail", "results": []map[string]any{{"a": 1}}})
	req := &request{JSONRPC: "2.0", ID: 10, Method: "sink/write", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_SinkWrite_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 10, Method: "sink/write", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_SinkFlush_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("s3", &mockSink{})

	params, _ := json.Marshal(map[string]string{"name": "s3"})
	req := &request{JSONRPC: "2.0", ID: 11, Method: "sink/flush", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_SinkFlush_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(map[string]string{"name": "missing"})
	req := &request{JSONRPC: "2.0", ID: 11, Method: "sink/flush", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_SinkFlush_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("fail", &mockSink{flushErr: fmt.Errorf("flush failed")})

	params, _ := json.Marshal(map[string]string{"name": "fail"})
	req := &request{JSONRPC: "2.0", ID: 11, Method: "sink/flush", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_SinkFlush_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 11, Method: "sink/flush", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_SinkClose_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("s3", &mockSink{})

	params, _ := json.Marshal(map[string]string{"name": "s3"})
	req := &request{JSONRPC: "2.0", ID: 12, Method: "sink/close", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_SinkClose_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(map[string]string{"name": "missing"})
	req := &request{JSONRPC: "2.0", ID: 12, Method: "sink/close", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_SinkClose_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterSink("fail", &mockSink{closeErr: fmt.Errorf("close failed")})

	params, _ := json.Marshal(map[string]string{"name": "fail"})
	req := &request{JSONRPC: "2.0", ID: 12, Method: "sink/close", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_SinkClose_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 12, Method: "sink/close", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

// --- handleRequest tests for middleware ---

func TestHandleRequest_Middleware_BeforeNavigate(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterMiddleware(&mockMiddleware{result: AllowResult()})

	params, _ := json.Marshal(MiddlewareContext{Hook: "before_navigate", URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 13, Method: "middleware/before_navigate", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_Middleware_AfterLoad(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterMiddleware(&mockMiddleware{result: AllowResult()})

	params, _ := json.Marshal(MiddlewareContext{Hook: "after_load", URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 14, Method: "middleware/after_load", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_Middleware_BeforeExtract(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterMiddleware(&mockMiddleware{result: AllowResult()})

	params, _ := json.Marshal(MiddlewareContext{Hook: "before_extract", URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 15, Method: "middleware/before_extract", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_Middleware_OnError(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterMiddleware(&mockMiddleware{result: RetryResult()})

	params, _ := json.Marshal(MiddlewareContext{Hook: "on_error", URL: "https://example.com", Error: "timeout"})
	req := &request{JSONRPC: "2.0", ID: 16, Method: "middleware/on_error", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_Middleware_NoHandler(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(MiddlewareContext{Hook: "before_navigate", URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 17, Method: "middleware/before_navigate", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error when no middleware handler")
	}
}

func TestHandleRequest_Middleware_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterMiddleware(&mockMiddleware{result: AllowResult()})

	req := &request{JSONRPC: "2.0", ID: 18, Method: "middleware/before_navigate", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestHandleRequest_Middleware_HandlerError(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterMiddleware(&mockMiddleware{err: fmt.Errorf("middleware error")})

	params, _ := json.Marshal(MiddlewareContext{Hook: "before_navigate", URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 19, Method: "middleware/before_navigate", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

// --- handleRequest tests for event ---

func TestHandleRequest_EventEmit(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	eh := &mockEvent{}
	s.OnEvent(eh)

	params, _ := json.Marshal(EventData{Type: "navigation", URL: "https://example.com"})
	req := &request{JSONRPC: "2.0", ID: 20, Method: "event/emit", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	if !eh.called {
		t.Error("event handler was not called")
	}
}

func TestHandleRequest_EventEmit_NoHandler(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(EventData{Type: "navigation"})
	req := &request{JSONRPC: "2.0", ID: 20, Method: "event/emit", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	// Should still succeed (just no handler called).
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

// --- handleRequest tests for command ---

func TestHandleRequest_CommandExecute_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterCommand("extract", &mockCommand{result: CommandOutput("extracted data")})

	params, _ := json.Marshal(CommandParams{Command: "extract", Args: []string{"https://example.com"}})
	req := &request{JSONRPC: "2.0", ID: 21, Method: "command/execute", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_CommandExecute_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(CommandParams{Command: "missing"})
	req := &request{JSONRPC: "2.0", ID: 21, Method: "command/execute", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestHandleRequest_CommandExecute_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterCommand("fail", &mockCommand{err: fmt.Errorf("command failed")})

	params, _ := json.Marshal(CommandParams{Command: "fail"})
	req := &request{JSONRPC: "2.0", ID: 21, Method: "command/execute", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_CommandExecute_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 21, Method: "command/execute", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

// --- handleRequest tests for completion ---

func TestHandleRequest_CommandComplete_Success(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterCompletion("extract", &mockCompletion{suggestions: []string{"--format", "--output"}})

	params, _ := json.Marshal(CompletionParams{Command: "extract", Args: []string{"url"}, ToComp: "--"})
	req := &request{JSONRPC: "2.0", ID: 22, Method: "command/complete", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var suggestions []string
	if err := json.Unmarshal(resp.Result, &suggestions); err != nil {
		t.Fatal(err)
	}

	if len(suggestions) != 2 {
		t.Errorf("got %d suggestions, want 2", len(suggestions))
	}
}

func TestHandleRequest_CommandComplete_NotFound(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	params, _ := json.Marshal(CompletionParams{Command: "missing"})
	req := &request{JSONRPC: "2.0", ID: 22, Method: "command/complete", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	// Should return empty list, not error.
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestHandleRequest_CommandComplete_Error(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterCompletion("fail", &mockCompletion{err: fmt.Errorf("complete failed")})

	params, _ := json.Marshal(CompletionParams{Command: "fail"})
	req := &request{JSONRPC: "2.0", ID: 22, Method: "command/complete", Params: params}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestHandleRequest_CommandComplete_InvalidParams(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)

	req := &request{JSONRPC: "2.0", ID: 22, Method: "command/complete", Params: json.RawMessage(`bad`)}
	s.handleRequest(context.Background(), req)

	resp := decodeResponse(t, &buf)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
}

// --- Capabilities extended ---

func TestCapabilities_Extended(t *testing.T) {
	var buf bytes.Buffer

	s := newTestServer(&buf)
	s.RegisterAuth(&mockAuth{})
	s.RegisterMiddleware(&mockMiddleware{result: AllowResult()})
	s.OnEvent(&mockEvent{})
	s.RegisterCommand("cmd", &mockCommand{})
	s.RegisterSink("sink", &mockSink{})
	s.RegisterResource("uri", &mockResource{})
	s.RegisterPrompt("prompt", &mockPrompt{})

	caps := s.capabilities()

	// Should include auth_provider and browser_middleware.
	found := make(map[string]bool)
	for _, c := range caps {
		found[c] = true
	}

	if !found["auth_provider"] {
		t.Error("missing auth_provider capability")
	}

	if !found["browser_middleware"] {
		t.Error("missing browser_middleware capability")
	}
}

// --- SDK helper function tests ---

func TestAllowResult(t *testing.T) {
	r := AllowResult()
	if r.Action != "allow" {
		t.Errorf("Action = %q, want allow", r.Action)
	}
}

func TestBlockResult(t *testing.T) {
	r := BlockResult()
	if r.Action != "block" {
		t.Errorf("Action = %q, want block", r.Action)
	}
}

func TestModifyURLResult(t *testing.T) {
	r := ModifyURLResult("https://modified.com")
	if r.Action != "modify" {
		t.Errorf("Action = %q, want modify", r.Action)
	}

	if r.ModifiedURL != "https://modified.com" {
		t.Errorf("ModifiedURL = %q", r.ModifiedURL)
	}
}

func TestRetryResult(t *testing.T) {
	r := RetryResult()
	if r.Action != "retry" {
		t.Errorf("Action = %q, want retry", r.Action)
	}
}

func TestCommandOutput(t *testing.T) {
	r := CommandOutput("hello")
	if r.Output != "hello" {
		t.Errorf("Output = %q", r.Output)
	}

	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d", r.ExitCode)
	}
}

func TestCommandError(t *testing.T) {
	r := CommandError("failed", 1)
	if r.Output != "failed" {
		t.Errorf("Output = %q", r.Output)
	}

	if r.ExitCode != 1 {
		t.Errorf("ExitCode = %d", r.ExitCode)
	}
}

// --- ResourceHandlerFunc test ---

func TestResourceHandlerFunc(t *testing.T) {
	fn := ResourceHandlerFunc(func(_ context.Context, uri string) (string, string, error) {
		return "data for " + uri, "text/plain", nil
	})

	content, mime, err := fn.Read(context.Background(), "myapp://test")
	if err != nil {
		t.Fatal(err)
	}

	if content != "data for myapp://test" {
		t.Errorf("content = %q", content)
	}

	if mime != "text/plain" {
		t.Errorf("mime = %q", mime)
	}
}

// --- PromptHandlerFunc test ---

func TestPromptHandlerFunc(t *testing.T) {
	fn := PromptHandlerFunc(func(_ context.Context, name string, _ map[string]string) ([]PromptMessage, error) {
		return []PromptMessage{{Role: "user", Content: "For " + name}}, nil
	})

	msgs, err := fn.Get(context.Background(), "analyze", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(msgs) != 1 {
		t.Fatalf("got %d messages", len(msgs))
	}
}

// --- SinkHandlerFunc test ---

func TestSinkHandlerFunc_Defaults(t *testing.T) {
	fn := SinkHandlerFunc{}

	if err := fn.Init(context.Background(), nil); err != nil {
		t.Errorf("Init() = %v", err)
	}

	if err := fn.Write(context.Background(), nil); err != nil {
		t.Errorf("Write() = %v", err)
	}

	if err := fn.Flush(context.Background()); err != nil {
		t.Errorf("Flush() = %v", err)
	}

	if err := fn.Close(context.Background()); err != nil {
		t.Errorf("Close() = %v", err)
	}
}

func TestSinkHandlerFunc_WithFunctions(t *testing.T) {
	initCalled := false
	writeCalled := false
	flushCalled := false
	closeCalled := false

	fn := SinkHandlerFunc{
		InitFn:  func(_ context.Context, _ map[string]any) error { initCalled = true; return nil },
		WriteFn: func(_ context.Context, _ []map[string]any) error { writeCalled = true; return nil },
		FlushFn: func(_ context.Context) error { flushCalled = true; return nil },
		CloseFn: func(_ context.Context) error { closeCalled = true; return nil },
	}

	ctx := context.Background()
	_ = fn.Init(ctx, nil)
	_ = fn.Write(ctx, nil)
	_ = fn.Flush(ctx)
	_ = fn.Close(ctx)

	if !initCalled || !writeCalled || !flushCalled || !closeCalled {
		t.Error("not all functions were called")
	}
}

// --- EventHandlerFunc test ---

func TestEventHandlerFunc(t *testing.T) {
	called := false
	fn := EventHandlerFunc(func(_ context.Context, _ EventData) {
		called = true
	})

	fn.OnEvent(context.Background(), EventData{Type: "test"})
	if !called {
		t.Error("handler function was not called")
	}
}

// --- CommandHandlerFunc test ---

func TestCommandHandlerFunc(t *testing.T) {
	fn := CommandHandlerFunc(func(_ context.Context, params CommandParams) (*CommandResult, error) {
		return CommandOutput("ran " + params.Command), nil
	})

	result, err := fn.Execute(context.Background(), CommandParams{Command: "test"})
	if err != nil {
		t.Fatal(err)
	}

	if result.Output != "ran test" {
		t.Errorf("Output = %q", result.Output)
	}
}

// --- ConnectBrowser (unit-testable parts only) ---

func TestConnectBrowser_NilContext(t *testing.T) {
	_, err := ConnectBrowser(nil)
	if err == nil {
		t.Fatal("expected error for nil browser context")
	}
}

func TestConnectBrowser_EmptyCDP(t *testing.T) {
	_, err := ConnectBrowser(&BrowserContext{})
	if err == nil {
		t.Fatal("expected error for empty CDP endpoint")
	}
}
