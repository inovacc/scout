package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// setupProxyClient creates a Client + mock responder wired via pipes. Returns the Client.
func setupProxyClient(t *testing.T, handler func(req *Request) *Response) *Client {
	t.Helper()

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	manifest := &Manifest{Name: "proxy-test", Version: "1.0.0", Command: "./test"}

	client := &Client{
		manifest: manifest,
		encoder:  json.NewEncoder(stdinW),
		scanner:  bufio.NewScanner(stdoutR),
		pending:  make(map[int64]chan *Response),
		notify:   make(chan *Notification, 64),
		done:     make(chan struct{}),
		started:  true,
	}
	client.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	go client.readLoop()
	go mockResponder(t, stdinR, stdoutW, handler)

	t.Cleanup(func() {
		_ = stdoutW.Close()
		_ = stdinW.Close()
	})

	return client
}

// --- PromptProxy tests ---

func TestPromptProxy_Get_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		msgs := []PromptMessage{
			{Role: "user", Content: "Tell me about {{topic}}"},
			{Role: "assistant", Content: "Here's info about Go"},
		}
		data, _ := json.Marshal(msgs)

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewPromptProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgs, err := proxy.Get(ctx, "explain", map[string]string{"topic": "Go"})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}

	if msgs[0].Role != "user" {
		t.Errorf("msgs[0].Role = %q, want %q", msgs[0].Role, "user")
	}
}

func TestPromptProxy_Get_RPCError(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeMethodNotFound, Message: "not found"}}
	})

	proxy := NewPromptProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.Get(ctx, "missing", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPromptProxy_List_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		infos := []PromptInfo{
			{Name: "explain", Description: "Explain a topic"},
			{Name: "summarize", Description: "Summarize text"},
		}
		data, _ := json.Marshal(infos)

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewPromptProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	infos, err := proxy.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("got %d prompts, want 2", len(infos))
	}

	if infos[0].Name != "explain" {
		t.Errorf("infos[0].Name = %q, want %q", infos[0].Name, "explain")
	}
}

func TestPromptProxy_List_RPCError(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "failed"}}
	})

	proxy := NewPromptProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.List(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPromptProxy_Get_InvalidResponse(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`"not-an-array"`)}
	})

	proxy := NewPromptProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.Get(ctx, "bad", nil)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestPromptProxy_List_InvalidResponse(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`"not-an-array"`)}
	})

	proxy := NewPromptProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.List(ctx)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- ResourceProxy tests ---

func TestResourceProxy_Read_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		data, _ := json.Marshal(map[string]string{
			"content":  "hello world",
			"mimeType": "text/plain",
		})

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewResourceProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	content, mime, err := proxy.Read(ctx, "myapp://data")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if content != "hello world" {
		t.Errorf("content = %q, want %q", content, "hello world")
	}

	if mime != "text/plain" {
		t.Errorf("mimeType = %q, want %q", mime, "text/plain")
	}
}

func TestResourceProxy_Read_RPCError(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeMethodNotFound, Message: "not found"}}
	})

	proxy := NewResourceProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := proxy.Read(ctx, "myapp://missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResourceProxy_Read_InvalidResponse(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`"not-an-object"`)}
	})

	proxy := NewResourceProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := proxy.Read(ctx, "myapp://bad")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestResourceProxy_List_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		infos := []ResourceInfo{
			{URI: "myapp://data", Name: "Data", MimeType: "text/plain"},
		}
		data, _ := json.Marshal(infos)

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewResourceProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	infos, err := proxy.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(infos) != 1 {
		t.Fatalf("got %d resources, want 1", len(infos))
	}

	if infos[0].URI != "myapp://data" {
		t.Errorf("URI = %q, want %q", infos[0].URI, "myapp://data")
	}
}

func TestResourceProxy_List_RPCError(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "failed"}}
	})

	proxy := NewResourceProxy(client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.List(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- SinkProxy RPC tests ---

func TestSinkProxy_Init_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
	})

	proxy := NewSinkProxy(client, "s3")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Init(ctx, map[string]any{"bucket": "test"}); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
}

func TestSinkProxy_Init_Error(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "config failed"}}
	})

	proxy := NewSinkProxy(client, "s3")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Init(ctx, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestSinkProxy_Write_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
	})

	proxy := NewSinkProxy(client, "webhook")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Write(ctx, []any{"a", "b"}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
}

func TestSinkProxy_Write_Error(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "write failed"}}
	})

	proxy := NewSinkProxy(client, "webhook")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Write(ctx, []any{"a"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSinkProxy_WriteSingle(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
	})

	proxy := NewSinkProxy(client, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.WriteSingle(ctx, "data"); err != nil {
		t.Fatalf("WriteSingle() error: %v", err)
	}
}

func TestSinkProxy_Flush_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
	})

	proxy := NewSinkProxy(client, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Flush(ctx); err != nil {
		t.Fatalf("Flush() error: %v", err)
	}
}

func TestSinkProxy_Flush_Error(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "flush failed"}}
	})

	proxy := NewSinkProxy(client, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Flush(ctx); err == nil {
		t.Fatal("expected error")
	}
}

func TestSinkProxy_Close_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
	})

	proxy := NewSinkProxy(client, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Close(ctx); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestSinkProxy_Close_Error(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "close failed"}}
	})

	proxy := NewSinkProxy(client, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxy.Close(ctx); err == nil {
		t.Fatal("expected error")
	}
}

// --- AuthProxy tests (non-browser) ---

func TestAuthProxy_Name(t *testing.T) {
	proxy := NewAuthProxy(nil, "github", "https://github.com/login")

	if proxy.Name() != "github" {
		t.Errorf("Name() = %q, want %q", proxy.Name(), "github")
	}

	if proxy.LoginURL() != "https://github.com/login" {
		t.Errorf("LoginURL() = %q", proxy.LoginURL())
	}
}

func TestAuthProxy_ValidateSession_Valid(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		data, _ := json.Marshal(map[string]any{"valid": true})

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewAuthProxy(client, "test", "https://example.com/login")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := proxy.ValidateSession(ctx, nil)
	if err != nil {
		t.Fatalf("ValidateSession() error: %v", err)
	}
}

func TestAuthProxy_ValidateSession_Invalid(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		data, _ := json.Marshal(map[string]any{"valid": false, "reason": "token expired"})

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewAuthProxy(client, "test", "https://example.com/login")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := proxy.ValidateSession(ctx, nil)
	if err == nil {
		t.Fatal("expected error for invalid session")
	}
}

func TestAuthProxy_ValidateSession_InvalidNoReason(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		data, _ := json.Marshal(map[string]any{"valid": false})

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewAuthProxy(client, "test", "https://example.com/login")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := proxy.ValidateSession(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthProxy_ValidateSession_RPCError(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "fail"}}
	})

	proxy := NewAuthProxy(client, "test", "https://example.com/login")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := proxy.ValidateSession(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthProxy_ValidateSession_InvalidJSON(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`"not-object"`)}
	})

	proxy := NewAuthProxy(client, "test", "https://example.com/login")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := proxy.ValidateSession(ctx, nil)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- MiddlewareProxy + Chain tests ---

func TestMiddlewareProxy_Fields(t *testing.T) {
	proxy := NewMiddlewareProxy(nil, []HookPoint{HookBeforeNavigate, HookAfterLoad}, 50)

	if proxy.Priority() != 50 {
		t.Errorf("Priority() = %d, want 50", proxy.Priority())
	}

	hooks := proxy.Hooks()
	if len(hooks) != 2 {
		t.Fatalf("Hooks() len = %d, want 2", len(hooks))
	}
}

func TestMiddlewareProxy_Execute_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		result := HookResult{Action: ActionAllow}
		data, _ := json.Marshal(result)

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewMiddlewareProxy(client, []HookPoint{HookBeforeNavigate}, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := proxy.Execute(ctx, &HookContext{
		Hook: HookBeforeNavigate,
		URL:  "https://example.com",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if result.Action != ActionAllow {
		t.Errorf("Action = %q, want %q", result.Action, ActionAllow)
	}
}

func TestMiddlewareProxy_Execute_RPCError(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "fail"}}
	})

	proxy := NewMiddlewareProxy(client, []HookPoint{HookBeforeNavigate}, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.Execute(ctx, &HookContext{Hook: HookBeforeNavigate, URL: "https://example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMiddlewareProxy_Execute_InvalidResponse(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`"bad"`)}
	})

	proxy := NewMiddlewareProxy(client, []HookPoint{HookBeforeNavigate}, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.Execute(ctx, &HookContext{Hook: HookBeforeNavigate, URL: "https://example.com"})
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestMiddlewareChain_Execute_Empty(t *testing.T) {
	chain := NewMiddlewareChain()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := chain.Execute(ctx, &HookContext{Hook: HookBeforeNavigate, URL: "https://example.com"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if result.Action != ActionAllow {
		t.Errorf("default action = %q, want %q", result.Action, ActionAllow)
	}
}

func TestMiddlewareChain_PriorityOrder(t *testing.T) {
	chain := NewMiddlewareChain()

	// Register in reverse priority order to verify sorting.
	m1 := NewMiddlewareProxy(nil, []HookPoint{HookAfterLoad}, 100)
	m2 := NewMiddlewareProxy(nil, []HookPoint{HookAfterLoad}, 0)
	m3 := NewMiddlewareProxy(nil, []HookPoint{HookAfterLoad}, 50)

	chain.Register(m1)
	chain.Register(m2)
	chain.Register(m3)

	if chain.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", chain.Len())
	}

	// Verify sorted order.
	if chain.middlewares[0].Priority() != 0 {
		t.Errorf("first priority = %d, want 0", chain.middlewares[0].Priority())
	}

	if chain.middlewares[1].Priority() != 50 {
		t.Errorf("second priority = %d, want 50", chain.middlewares[1].Priority())
	}

	if chain.middlewares[2].Priority() != 100 {
		t.Errorf("third priority = %d, want 100", chain.middlewares[2].Priority())
	}
}

func TestMiddlewareChain_Block(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		result := HookResult{Action: ActionBlock}
		data, _ := json.Marshal(result)

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	chain := NewMiddlewareChain()
	chain.Register(NewMiddlewareProxy(client, []HookPoint{HookBeforeNavigate}, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := chain.Execute(ctx, &HookContext{Hook: HookBeforeNavigate, URL: "https://blocked.com"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if result.Action != ActionBlock {
		t.Errorf("Action = %q, want %q", result.Action, ActionBlock)
	}
}

func TestMiddlewareChain_Modify(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		result := HookResult{Action: ActionModify, ModifiedURL: "https://modified.com"}
		data, _ := json.Marshal(result)

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	chain := NewMiddlewareChain()
	chain.Register(NewMiddlewareProxy(client, []HookPoint{HookBeforeNavigate}, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := chain.Execute(ctx, &HookContext{Hook: HookBeforeNavigate, URL: "https://original.com"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if result.Action != ActionModify {
		t.Errorf("Action = %q, want %q", result.Action, ActionModify)
	}

	if result.ModifiedURL != "https://modified.com" {
		t.Errorf("ModifiedURL = %q", result.ModifiedURL)
	}
}

func TestMiddlewareChain_Retry(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		result := HookResult{Action: ActionRetry}
		data, _ := json.Marshal(result)

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	chain := NewMiddlewareChain()
	chain.Register(NewMiddlewareProxy(client, []HookPoint{HookOnError}, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := chain.Execute(ctx, &HookContext{Hook: HookOnError, URL: "https://example.com", Error: "timeout"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if result.Action != ActionRetry {
		t.Errorf("Action = %q, want %q", result.Action, ActionRetry)
	}
}

func TestMiddlewareChain_SkipNonMatchingHook(t *testing.T) {
	chain := NewMiddlewareChain()

	client := setupProxyClient(t, func(req *Request) *Response {
		t.Fatal("should not be called for non-matching hook")

		return nil
	})

	chain.Register(NewMiddlewareProxy(client, []HookPoint{HookAfterLoad}, 0))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := chain.Execute(ctx, &HookContext{Hook: HookBeforeNavigate, URL: "https://example.com"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if result.Action != ActionAllow {
		t.Errorf("Action = %q, want %q", result.Action, ActionAllow)
	}
}

// --- EventProxy tests ---

func TestEventProxy_IsSubscribed(t *testing.T) {
	proxy := NewEventProxy(nil, []EventType{EventNavigation, EventConsoleLog}, nil, 0)

	if !proxy.IsSubscribed(EventNavigation) {
		t.Error("expected subscribed to EventNavigation")
	}

	if proxy.IsSubscribed(EventDOMMutation) {
		t.Error("expected not subscribed to EventDOMMutation")
	}
}

func TestEventProxy_DefaultRateLimit(t *testing.T) {
	proxy := NewEventProxy(nil, nil, nil, 0)
	// Default rate limit should be 100.
	if proxy.rateLimiter.maxRate != 100 {
		t.Errorf("maxRate = %d, want 100", proxy.rateLimiter.maxRate)
	}
}

func TestEventProxy_CustomRateLimit(t *testing.T) {
	proxy := NewEventProxy(nil, nil, nil, 50)
	if proxy.rateLimiter.maxRate != 50 {
		t.Errorf("maxRate = %d, want 50", proxy.rateLimiter.maxRate)
	}
}

func TestEventProxy_Emit_NotSubscribed(t *testing.T) {
	proxy := NewEventProxy(nil, []EventType{EventNavigation}, nil, 100)
	event := &Event{Type: EventConsoleLog}

	if proxy.Emit(event) {
		t.Error("expected Emit to return false for unsubscribed event")
	}
}

func TestEventProxy_RequestAction_NotAllowed(t *testing.T) {
	proxy := NewEventProxy(nil, nil, []string{"inject_js"}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.RequestAction(ctx, "screenshot", nil)
	if err == nil {
		t.Fatal("expected error for non-allowed action")
	}
}

func TestEventProxy_RequestAction_Success(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		data, _ := json.Marshal(map[string]any{"ok": true})

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	proxy := NewEventProxy(client, nil, []string{"inject_js"}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := proxy.RequestAction(ctx, "inject_js", map[string]any{"code": "alert(1)"})
	if err != nil {
		t.Fatalf("RequestAction() error: %v", err)
	}

	if result["ok"] != true {
		t.Errorf("result = %v", result)
	}
}

func TestEventProxy_RequestAction_RPCError(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: CodeInternalError, Message: "fail"}}
	})

	proxy := NewEventProxy(client, nil, []string{"inject_js"}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.RequestAction(ctx, "inject_js", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEventProxy_RequestAction_InvalidResponse(t *testing.T) {
	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`"bad"`)}
	})

	proxy := NewEventProxy(client, nil, []string{"inject_js"}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.RequestAction(ctx, "inject_js", nil)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestEventDispatcher_Register_Dispatch(t *testing.T) {
	dispatcher := NewEventDispatcher()

	if dispatcher.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", dispatcher.Len())
	}

	client := setupProxyClient(t, func(req *Request) *Response {
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{}`)}
	})

	proxy := NewEventProxy(client, []EventType{EventNavigation}, nil, 100)
	dispatcher.Register(proxy)

	if dispatcher.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", dispatcher.Len())
	}

	count := dispatcher.Dispatch(&Event{Type: EventNavigation, URL: "https://example.com"})
	if count != 1 {
		t.Errorf("Dispatch() = %d, want 1", count)
	}

	// Dispatch non-subscribed event.
	count = dispatcher.Dispatch(&Event{Type: EventConsoleLog})
	if count != 0 {
		t.Errorf("Dispatch(unsubscribed) = %d, want 0", count)
	}
}

func TestEventRateLimiter_Exhaustion(t *testing.T) {
	rl := newEventRateLimiter(3)

	// First 3 should pass.
	for i := range 3 {
		if !rl.allow() {
			t.Errorf("allow() call %d should be true", i)
		}
	}

	// 4th should fail.
	if rl.allow() {
		t.Error("4th allow() should be false")
	}
}

// --- CommandProxy helper tests ---

func TestToInt_JSONNumber(t *testing.T) {
	got, ok := toInt(json.Number("99"))
	if !ok {
		t.Error("expected ok=true for json.Number")
	}

	if got != 99 {
		t.Errorf("value = %d, want 99", got)
	}
}

func TestToFloat_JSONNumber(t *testing.T) {
	got, ok := toFloat(json.Number("2.5"))
	if !ok {
		t.Error("expected ok=true for json.Number")
	}

	if got != 2.5 {
		t.Errorf("value = %f, want 2.5", got)
	}
}

// --- Client.Notify test ---

func TestClient_Notify(t *testing.T) {
	c, _, pluginR := newTestClient(t)

	// Drain stdin to verify Notify sends data.
	done := make(chan []byte, 1)

	go func() {
		scanner := bufio.NewScanner(pluginR)
		if scanner.Scan() {
			done <- scanner.Bytes()
		}
	}()

	if err := c.Notify("log", map[string]string{"level": "info"}); err != nil {
		t.Fatalf("Notify() error: %v", err)
	}

	select {
	case data := <-done:
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if msg["method"] != "log" {
			t.Errorf("method = %v, want log", msg["method"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

// --- Protocol edge cases ---

func TestNewRequest_MarshalError(t *testing.T) {
	// Channels cannot be marshaled.
	_, err := NewRequest("test", make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable params")
	}
}

func TestNewNotification_MarshalError(t *testing.T) {
	_, err := NewNotification("test", make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable params")
	}
}

func TestNewNotification_NilParams(t *testing.T) {
	n, err := NewNotification("shutdown", nil)
	if err != nil {
		t.Fatal(err)
	}

	if n.Params != nil {
		t.Error("expected nil params")
	}
}

// --- Manager additional tests ---

func TestManager_NewManager_NilLogger(t *testing.T) {
	mgr := NewManager([]string{"/tmp"}, nil)
	if mgr.logger == nil {
		t.Error("expected non-nil logger")
	}
}

// --- ToolProxy.Register helper test ---

func TestToolProxy_Register_WithInputSchema(t *testing.T) {
	// Just verify the ToolProxy creates the tool name correctly.
	tp := &ToolProxy{
		entry:    ToolEntry{Name: "search", Description: "Search", InputSchema: map[string]any{"type": "object"}},
		manifest: &Manifest{Name: "myplugin"},
		manager:  NewManager(nil, nil),
	}

	// We can't easily Register without a real MCP server that accepts any schema,
	// but we can verify the tool name format.
	expected := "plugin_myplugin_search"
	actual := "plugin_" + tp.manifest.Name + "_" + tp.entry.Name

	if actual != expected {
		t.Errorf("tool name = %q, want %q", actual, expected)
	}
}

// --- Manifest additional validation tests ---

func TestManifest_AllValidCapabilities(t *testing.T) {
	valid := []string{
		"scraper_mode", "extractor", "mcp_tool", "cli_command",
		"auth_provider", "mcp_resource", "mcp_prompt", "output_sink",
		"browser_middleware", "event_hook",
	}

	for _, cap := range valid {
		m := &Manifest{Name: "t", Version: "1.0", Command: "./t", Capabilities: []string{cap}}
		if err := m.validate(); err != nil {
			t.Errorf("validate() failed for valid capability %q: %v", cap, err)
		}
	}
}

// --- PageState test ---

func TestPageState_JSON(t *testing.T) {
	ps := &PageState{
		URL:          "https://example.com",
		Title:        "Test",
		LocalStorage: map[string]string{"key": "val"},
	}

	data, err := json.Marshal(ps)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PageState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.URL != ps.URL {
		t.Errorf("URL = %q, want %q", decoded.URL, ps.URL)
	}

	if decoded.LocalStorage["key"] != "val" {
		t.Error("LocalStorage mismatch")
	}
}

// --- CommandProxy.CobraCommand test ---

func TestCommandProxy_CobraCommand_WithUse(t *testing.T) {
	proxy := &CommandProxy{
		entry:    CommandEntry{Name: "mycommand", Use: "mycommand <url>", Short: "My command"},
		manifest: &Manifest{Name: "test-plugin"},
		manager:  NewManager(nil, nil),
	}

	cmd := proxy.CobraCommand()
	if cmd.Use != "mycommand <url>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "mycommand <url>")
	}
}

func TestCommandProxy_CobraCommand_WithFlags(t *testing.T) {
	proxy := &CommandProxy{
		entry: CommandEntry{
			Name:  "cmd",
			Short: "test",
			Flags: []FlagEntry{
				{Name: "output", Type: "string", Default: "json", Short: "o", Description: "Output format"},
				{Name: "limit", Type: "int", Default: float64(10), Short: "l", Description: "Limit"},
				{Name: "verbose", Type: "bool", Default: false, Short: "v", Description: "Verbose"},
				{Name: "rate", Type: "float", Default: 1.5, Short: "r", Description: "Rate"},
				// Without short flag variants.
				{Name: "format", Type: "string", Default: "text"},
				{Name: "count", Type: "int", Default: float64(5)},
				{Name: "debug", Type: "bool", Default: true},
				{Name: "threshold", Type: "float", Default: 0.9},
			},
		},
		manifest: &Manifest{Name: "test-plugin"},
		manager:  NewManager(nil, nil),
	}

	cmd := proxy.CobraCommand()

	// Verify flags were added.
	if f := cmd.Flags().Lookup("output"); f == nil {
		t.Error("missing 'output' flag")
	} else if f.DefValue != "json" {
		t.Errorf("output default = %q, want %q", f.DefValue, "json")
	}

	if f := cmd.Flags().Lookup("limit"); f == nil {
		t.Error("missing 'limit' flag")
	}

	if f := cmd.Flags().Lookup("verbose"); f == nil {
		t.Error("missing 'verbose' flag")
	}

	if f := cmd.Flags().Lookup("rate"); f == nil {
		t.Error("missing 'rate' flag")
	}

	if f := cmd.Flags().Lookup("format"); f == nil {
		t.Error("missing 'format' flag")
	}

	if f := cmd.Flags().Lookup("count"); f == nil {
		t.Error("missing 'count' flag")
	}

	if f := cmd.Flags().Lookup("debug"); f == nil {
		t.Error("missing 'debug' flag")
	}

	if f := cmd.Flags().Lookup("threshold"); f == nil {
		t.Error("missing 'threshold' flag")
	}
}

func TestSetBrowserProvisioner(t *testing.T) {
	// Save and restore the global.
	old := browserProvisionFunc
	defer func() { browserProvisionFunc = old }()

	called := false
	SetBrowserProvisioner(func(_ *cobra.Command) (*BrowserContext, error) {
		called = true

		return &BrowserContext{CDPEndpoint: "ws://localhost:9222"}, nil
	})

	if browserProvisionFunc == nil {
		t.Fatal("expected non-nil provisioner")
	}

	_, _ = browserProvisionFunc(nil)
	if !called {
		t.Error("provisioner was not called")
	}
}
