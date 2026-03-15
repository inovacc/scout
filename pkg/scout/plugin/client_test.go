package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// newTestClient creates a Client wired to pipes for testing without a real subprocess.
// Returns the client, a writer (plugin's stdout → client reads), and a reader (client writes → plugin's stdin).
func newTestClient(t *testing.T) (*Client, io.Writer, io.Reader) {
	t.Helper()

	// pluginStdout: test writes responses here, client reads them via scanner.
	stdoutR, stdoutW := io.Pipe()
	// pluginStdin: client writes requests here, test reads them.
	stdinR, stdinW := io.Pipe()

	c := &Client{
		manifest: &Manifest{Name: "test-plugin", Version: "1.0.0", Command: "./test"},
		logger:   slog.Default(),
		encoder:  json.NewEncoder(stdinW),
		scanner:  bufio.NewScanner(stdoutR),
		pending:  make(map[int64]chan *Response),
		notify:   make(chan *Notification, 64),
		done:     make(chan struct{}),
		started:  true,
	}
	c.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	go c.readLoop()

	t.Cleanup(func() {
		_ = stdoutW.Close()
		_ = stdinW.Close()
	})

	return c, stdoutW, stdinR
}

// mockResponder reads JSON-RPC requests from r and writes responses to w using the handler function.
func mockResponder(t *testing.T, r io.Reader, w io.Writer, handler func(req *Request) *Response) {
	t.Helper()

	scanner := bufio.NewScanner(r)
	encoder := json.NewEncoder(w)

	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		resp := handler(&req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return
			}
		}
	}
}

func TestClient_Call_Success(t *testing.T) {
	c, pluginW, pluginR := newTestClient(t)

	go mockResponder(t, pluginR, pluginW, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{"greeting":"hello"}`),
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := c.Call(ctx, "greet", map[string]string{"name": "world"})
	if err != nil {
		t.Fatalf("Call() error: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if got["greeting"] != "hello" {
		t.Errorf("greeting = %q, want %q", got["greeting"], "hello")
	}
}

func TestClient_Call_RPCError(t *testing.T) {
	c, pluginW, pluginR := newTestClient(t)

	go mockResponder(t, pluginR, pluginW, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: CodeMethodNotFound, Message: "method not found"},
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Call(ctx, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}

	if rpcErr.Code != CodeMethodNotFound {
		t.Errorf("code = %d, want %d", rpcErr.Code, CodeMethodNotFound)
	}
}

func TestClient_Call_ContextCanceled(t *testing.T) {
	c, _, pluginR := newTestClient(t)

	// Drain stdin so encoder.Encode doesn't block.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := pluginR.Read(buf); err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.Call(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestClient_Call_ProcessExited(t *testing.T) {
	c, pluginW, pluginR := newTestClient(t)

	// Drain stdin so encoder.Encode doesn't block.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := pluginR.Read(buf); err != nil {
				return
			}
		}
	}()

	// Close plugin stdout to make readLoop exit → closes c.done.
	_ = pluginW.(io.Closer).Close()

	// Wait for done to be closed.
	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for done")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Call(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error on exited process")
	}
}

func TestClient_ReadLoop_Notification(t *testing.T) {
	c, pluginW, _ := newTestClient(t)

	notif := Notification{
		JSONRPC: "2.0",
		Method:  "result",
		Params:  json.RawMessage(`{"type":"post"}`),
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = fmt.Fprintf(pluginW, "%s\n", data)

	select {
	case got := <-c.Notifications():
		if got.Method != "result" {
			t.Errorf("method = %q, want %q", got.Method, "result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestClient_ReadLoop_InvalidJSON(t *testing.T) {
	c, pluginW, _ := newTestClient(t)

	// Write invalid JSON — should be silently skipped.
	_, _ = fmt.Fprintf(pluginW, "not-json\n")

	// Write a valid notification after.
	notif := Notification{
		JSONRPC: "2.0",
		Method:  "ping",
		Params:  json.RawMessage(`{}`),
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = fmt.Fprintf(pluginW, "%s\n", data)

	select {
	case got := <-c.Notifications():
		if got.Method != "ping" {
			t.Errorf("method = %q, want %q", got.Method, "ping")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification after invalid JSON")
	}
}

func TestClient_ReadLoop_EmptyLine(t *testing.T) {
	c, pluginW, _ := newTestClient(t)

	// Write empty line then valid notification.
	_, _ = fmt.Fprintf(pluginW, "\n")
	notif := Notification{JSONRPC: "2.0", Method: "test"}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = fmt.Fprintf(pluginW, "%s\n", data)

	select {
	case got := <-c.Notifications():
		if got.Method != "test" {
			t.Errorf("method = %q, want %q", got.Method, "test")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestClient_Initialize(t *testing.T) {
	c, pluginW, pluginR := newTestClient(t)

	go mockResponder(t, pluginR, pluginW, func(req *Request) *Response {
		if req.Method != "initialize" {
			t.Errorf("method = %q, want %q", req.Method, "initialize")
		}

		return &Response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}
}

func TestClient_Initialize_Error(t *testing.T) {
	c, pluginW, pluginR := newTestClient(t)

	go mockResponder(t, pluginR, pluginW, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: CodeInternalError, Message: "init failed"},
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Initialize(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Call_MultipleConcurrent(t *testing.T) {
	c, pluginW, pluginR := newTestClient(t)

	go mockResponder(t, pluginR, pluginW, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(fmt.Sprintf(`{"id":%d}`, req.ID)),
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			result, err := c.Call(ctx, "test", map[string]int{"i": i})
			if err != nil {
				t.Errorf("Call(%d) error: %v", i, err)
				return
			}

			if result == nil {
				t.Errorf("Call(%d) returned nil result", i)
			}
		}(i)
	}

	wg.Wait()
}

func TestClient_NewClient_NilLogger(t *testing.T) {
	c := NewClient(&Manifest{Name: "test"}, nil)
	if c.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestClient_Notifications_Channel(t *testing.T) {
	c := NewClient(&Manifest{Name: "test"}, nil)

	ch := c.Notifications()
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestClient_Done_Channel(t *testing.T) {
	c := NewClient(&Manifest{Name: "test"}, nil)

	ch := c.Done()
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestClient_Start_AlreadyStarted(t *testing.T) {
	c := &Client{
		manifest: &Manifest{Name: "test"},
		started:  true,
		pending:  make(map[int64]chan *Response),
		notify:   make(chan *Notification, 64),
		done:     make(chan struct{}),
	}

	err := c.Start(context.Background())
	if err != nil {
		t.Errorf("Start() on already started client should return nil, got: %v", err)
	}
}

func TestClient_Shutdown_NotStarted(t *testing.T) {
	c := &Client{
		manifest: &Manifest{Name: "test"},
		started:  false,
	}

	err := c.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() on not-started client should return nil, got: %v", err)
	}
}

func TestClient_ReadLoop_NotificationBufferFull(t *testing.T) {
	// Create client with tiny notification buffer.
	stdoutR, stdoutW := io.Pipe()

	c := &Client{
		manifest: &Manifest{Name: "test-plugin"},
		logger:   slog.Default(),
		pending:  make(map[int64]chan *Response),
		notify:   make(chan *Notification, 1), // buffer of 1
		done:     make(chan struct{}),
		scanner:  bufio.NewScanner(stdoutR),
		started:  true,
	}
	c.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	go c.readLoop()

	t.Cleanup(func() {
		_ = stdoutW.Close()
	})

	// Send 3 notifications — the buffer is 1, so at least one should be dropped.
	for range 3 {
		notif := Notification{JSONRPC: "2.0", Method: "test", Params: json.RawMessage(`{}`)}

		data, err := json.Marshal(notif)
		if err != nil {
			t.Fatal(err)
		}

		_, _ = fmt.Fprintf(stdoutW, "%s\n", data)
	}

	// Read the one that fits.
	select {
	case <-c.Notifications():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestClient_ReadLoop_ResponseWithoutPending(t *testing.T) {
	c, pluginW, _ := newTestClient(t)

	// Send a response with an ID that nobody is waiting for.
	resp := Response{JSONRPC: "2.0", ID: 99999, Result: json.RawMessage(`{}`)}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = fmt.Fprintf(pluginW, "%s\n", data)

	// Send a notification to verify the readLoop is still working.
	notif := Notification{JSONRPC: "2.0", Method: "alive"}

	data, err = json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = fmt.Fprintf(pluginW, "%s\n", data)

	select {
	case got := <-c.Notifications():
		if got.Method != "alive" {
			t.Errorf("method = %q, want %q", got.Method, "alive")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout — readLoop may have crashed on unmatched response")
	}
}
