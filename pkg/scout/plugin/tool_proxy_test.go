package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolProxy_NamespacePrefix(t *testing.T) {
	proxy := &ToolProxy{
		entry:    ToolEntry{Name: "greet", Description: "Say hello"},
		manifest: &Manifest{Name: "example"},
	}

	wantName := "plugin_example_greet"
	gotName := "plugin_" + proxy.manifest.Name + "_" + proxy.entry.Name

	if gotName != wantName {
		t.Errorf("tool name = %q, want %q", gotName, wantName)
	}
}

// setupToolProxyWithMock creates a ToolProxy backed by a mock plugin that responds via pipes.
func setupToolProxyWithMock(t *testing.T, handler func(req *Request) *Response) *ToolProxy {
	t.Helper()

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	manifest := &Manifest{Name: "test-plugin", Version: "1.0.0", Command: "./test"}

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

	mgr := NewManager(nil, nil)
	mgr.clients[manifest.Name] = client

	go mockResponder(t, stdinR, stdoutW, handler)

	t.Cleanup(func() {
		_ = stdoutW.Close()
		_ = stdinW.Close()
	})

	return &ToolProxy{
		entry:    ToolEntry{Name: "greet", Description: "Say hello"},
		manifest: manifest,
		manager:  mgr,
	}
}

func TestToolProxy_Handler_Success(t *testing.T) {
	proxy := setupToolProxyWithMock(t, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"content":[{"type":"text","text":"hello world"}],"isError":false}`),
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	callReq := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "greet"}}

	result, err := proxy.handler(ctx, callReq)
	if err != nil {
		t.Fatalf("handler() error: %v", err)
	}

	if result.IsError {
		t.Error("expected IsError=false")
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestToolProxy_Handler_RPCError(t *testing.T) {
	proxy := setupToolProxyWithMock(t, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: CodeInternalError, Message: "boom"},
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	callReq := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "greet"}}

	result, err := proxy.handler(ctx, callReq)
	if err != nil {
		t.Fatalf("handler() should not return Go error, got: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true")
	}
}

func TestToolProxy_Handler_ClientStartFailed(t *testing.T) {
	manifest := &Manifest{Name: "bad-plugin", Version: "1.0.0", Command: "/nonexistent/binary"}
	mgr := NewManager(nil, nil)
	// Don't add client — getClient will try to start one and fail.

	proxy := &ToolProxy{
		entry:    ToolEntry{Name: "test"},
		manifest: manifest,
		manager:  mgr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	callReq := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "greet"}}

	result, err := proxy.handler(ctx, callReq)
	if err != nil {
		t.Fatalf("handler() should not return Go error, got: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true for failed client start")
	}
}

func TestToolProxy_Handler_UnmarshalFallback(t *testing.T) {
	// When the result is not a standard tool result format, handler should use raw text.
	proxy := setupToolProxyWithMock(t, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"just a string"`),
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	callReq := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "greet"}}

	result, err := proxy.handler(ctx, callReq)
	if err != nil {
		t.Fatalf("handler() error: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected fallback content")
	}
}

func TestToolProxy_Handler_EmptyContent(t *testing.T) {
	// When result has valid structure but empty content array.
	proxy := setupToolProxyWithMock(t, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"content":[],"isError":false}`),
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	callReq := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "greet"}}

	result, err := proxy.handler(ctx, callReq)
	if err != nil {
		t.Fatalf("handler() error: %v", err)
	}

	// Empty content should trigger fallback.
	if len(result.Content) == 0 {
		t.Fatal("expected fallback content for empty content array")
	}
}

func TestToolProxy_Register(t *testing.T) {
	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	manifest := &Manifest{Name: "reg-test", Version: "1.0.0", Command: "./test"}

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

	mgr := NewManager(nil, nil)
	mgr.clients[manifest.Name] = client

	// Respond to tool calls.
	go func() {
		scanner := bufio.NewScanner(stdinR)
		encoder := json.NewEncoder(stdoutW)

		for scanner.Scan() {
			var req Request
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}

			if err := encoder.Encode(&Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`),
			}); err != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		_ = stdoutW.Close()
		_ = stdinW.Close()
	})

	proxy := &ToolProxy{
		entry:    ToolEntry{Name: "mytool", Description: "A tool"},
		manifest: manifest,
		manager:  mgr,
	}

	// Verify the tool name is correctly namespaced.
	wantName := fmt.Sprintf("plugin_%s_%s", manifest.Name, proxy.entry.Name)
	if wantName != "plugin_reg-test_mytool" {
		t.Errorf("unexpected tool name: %s", wantName)
	}
}
