package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// fakeWS is an in-memory WebSocketable. Sent frames are captured; Read pulls
// from a queue of server->client frames and blocks until one is available or
// the socket is closed.
type fakeWS struct {
	mu       sync.Mutex
	sent     [][]byte
	incoming chan []byte
	closed   chan struct{}
	sentSig  chan struct{}
	closeOne sync.Once

	sendErr error
}

func newFakeWS() *fakeWS {
	return &fakeWS{
		incoming: make(chan []byte, 16),
		closed:   make(chan struct{}),
		sentSig:  make(chan struct{}, 64),
	}
}

func (f *fakeWS) Send(data []byte) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	f.mu.Lock()
	f.sent = append(f.sent, cp)
	f.mu.Unlock()
	select {
	case f.sentSig <- struct{}{}:
	default:
	}
	return nil
}

func (f *fakeWS) Read() ([]byte, error) {
	select {
	case b := <-f.incoming:
		return b, nil
	case <-f.closed:
		return nil, io.EOF
	}
}

func (f *fakeWS) push(b []byte) { f.incoming <- b }

func (f *fakeWS) close() {
	f.closeOne.Do(func() { close(f.closed) })
}

func (f *fakeWS) lastSent() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return nil
	}
	return f.sent[len(f.sent)-1]
}

func TestNewClientDefaults(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.Event() == nil {
		t.Fatal("Event() channel must be non-nil")
	}
	if c.logger == nil {
		t.Fatal("default logger must be set")
	}
}

func TestClientLoggerOverride(t *testing.T) {
	c := New()
	var lines int
	custom := loggerFunc(func(_ ...any) { lines++ })

	ret := c.Logger(custom)
	if ret != c {
		t.Fatal("Logger should return the same client for chaining")
	}
	c.logger.Println("hi")
	if lines != 1 {
		t.Fatalf("custom logger not invoked; lines=%d", lines)
	}
}

type loggerFunc func(vs ...any)

func (l loggerFunc) Println(vs ...any) { l(vs...) }

// TestCallSuccess drives a full request/response correlation: Call sends a
// request, the fake transport echoes a matching response by ID, and Call
// returns the result payload.
func TestCallSuccess(t *testing.T) {
	ws := newFakeWS()
	c := New().Logger(loggerFunc(func(...any) {}))
	c.Start(ws)
	defer ws.close()

	// Decode the request the client sends to learn its assigned ID, then reply.
	go func() {
		raw := waitForSend(t, ws)
		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("bad request json: %v", err)
			return
		}
		resp, err := json.Marshal(Response{ID: req.ID, Result: json.RawMessage(`{"value":42}`)})
		if err != nil {
			t.Errorf("marshal response: %v", err)
			return
		}
		ws.push(resp)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := c.Call(ctx, "", "Runtime.evaluate", map[string]string{"expression": "1+1"})
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if string(out) != `{"value":42}` {
		t.Fatalf("Call result = %s, want {\"value\":42}", out)
	}

	// The first request must carry ID 1 and the method/params we passed.
	var req Request
	if err := json.Unmarshal(ws.lastSent(), &req); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if req.ID != 1 || req.Method != "Runtime.evaluate" {
		t.Fatalf("sent request = %+v, want ID 1 method Runtime.evaluate", req)
	}
}

// TestCallErrorResponse verifies that a protocol error in the response is
// surfaced as the *Error and matchable via errors.Is.
func TestCallErrorResponse(t *testing.T) {
	ws := newFakeWS()
	c := New().Logger(loggerFunc(func(...any) {}))
	c.Start(ws)
	defer ws.close()

	go func() {
		raw := waitForSend(t, ws)
		var req Request
		_ = json.Unmarshal(raw, &req)
		resp, err := json.Marshal(Response{
			ID:    req.ID,
			Error: &Error{Code: -32000, Message: "Could not find object with given id"},
		})
		if err != nil {
			t.Errorf("marshal response: %v", err)
			return
		}
		ws.push(resp)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Call(ctx, "session-1", "DOM.describeNode", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrObjNotFound) {
		t.Fatalf("error = %v, want ErrObjNotFound", err)
	}
}

// TestCallSendError verifies a transport send failure is returned directly and
// no goroutine leaks waiting on the response.
func TestCallSendError(t *testing.T) {
	ws := newFakeWS()
	ws.sendErr = errors.New("socket broken")
	c := New().Logger(loggerFunc(func(...any) {}))
	c.Start(ws)
	defer ws.close()

	_, err := c.Call(context.Background(), "", "Page.enable", nil)
	if err == nil || err.Error() != "socket broken" {
		t.Fatalf("Call error = %v, want socket broken", err)
	}
}

// TestCallContextCanceled verifies Call respects a canceled context and returns
// the context error rather than blocking forever.
func TestCallContextCanceled(t *testing.T) {
	ws := newFakeWS()
	c := New().Logger(loggerFunc(func(...any) {}))
	c.Start(ws)
	defer ws.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before any response is pushed

	_, err := c.Call(ctx, "", "Page.enable", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Call error = %v, want context.Canceled", err)
	}
}

// TestEventDispatch verifies that a frame with id==0 is decoded as an Event and
// delivered on the Event channel.
func TestEventDispatch(t *testing.T) {
	ws := newFakeWS()
	c := New().Logger(loggerFunc(func(...any) {}))
	c.Start(ws)
	defer ws.close()

	evt, err := json.Marshal(Event{
		SessionID: "S1",
		Method:    "Target.targetCreated",
		Params:    json.RawMessage(`{"targetId":"t1"}`),
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	ws.push(evt)

	select {
	case got := <-c.Event():
		if got.Method != "Target.targetCreated" {
			t.Fatalf("event method = %q, want Target.targetCreated", got.Method)
		}
		if string(got.Params) != `{"targetId":"t1"}` {
			t.Fatalf("event params = %s", got.Params)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

// TestConsumeMessagesClosesEventOnReadError verifies that when the transport
// returns a read error, the Event channel is closed (signaling shutdown).
func TestConsumeMessagesClosesEventOnReadError(t *testing.T) {
	ws := newFakeWS()
	c := New().Logger(loggerFunc(func(...any) {}))
	c.Start(ws)

	ws.close() // forces Read to return io.EOF

	select {
	case _, ok := <-c.Event():
		if ok {
			t.Fatal("expected Event channel to be closed after read error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Event channel close")
	}
}

// TestUnknownResponseIDIgnored verifies a response whose ID has no pending
// caller is silently dropped and does not crash the consume loop, which then
// still services a subsequent valid call.
func TestUnknownResponseIDIgnored(t *testing.T) {
	ws := newFakeWS()
	c := New().Logger(loggerFunc(func(...any) {}))
	c.Start(ws)
	defer ws.close()

	// Push a response for an ID nobody is waiting on.
	orphan, err := json.Marshal(Response{ID: 9999, Result: json.RawMessage(`"x"`)})
	if err != nil {
		t.Fatalf("marshal orphan: %v", err)
	}
	ws.push(orphan)

	go func() {
		raw := waitForSend(t, ws)
		var req Request
		_ = json.Unmarshal(raw, &req)
		resp, err := json.Marshal(Response{ID: req.ID, Result: json.RawMessage(`"ok"`)})
		if err != nil {
			t.Errorf("marshal response: %v", err)
			return
		}
		ws.push(resp)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := c.Call(ctx, "", "Page.enable", nil)
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if string(out) != `"ok"` {
		t.Fatalf("result = %s, want \"ok\"", out)
	}
}

// waitForSend blocks until a frame has been sent or times out.
func waitForSend(t *testing.T, ws *fakeWS) []byte {
	t.Helper()
	select {
	case <-ws.sentSig:
		return ws.lastSent()
	case <-time.After(2 * time.Second):
		t.Error("no frame was sent before timeout")
		return nil
	}
}
