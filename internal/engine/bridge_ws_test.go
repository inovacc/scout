package engine

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestBridgeServerStartStop(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	addr := s.Addr()
	if addr == "" {
		t.Fatal("Addr() returned empty")
	}

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Double stop should be safe.
	if err := s.Stop(); err != nil {
		t.Fatalf("double Stop: %v", err)
	}
}

func TestBridgeServerNilSafety(t *testing.T) {
	var s *BridgeServer

	if err := s.Stop(); err != nil {
		t.Fatalf("nil Stop: %v", err)
	}

	if s.Addr() != "" {
		t.Fatal("nil Addr should be empty")
	}

	if s.Clients() != nil {
		t.Fatal("nil Clients should be nil")
	}

	ch := s.Events()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("nil Events channel should be closed")
		}
	default:
		// closed channel returns immediately
	}

	s.OnMessage("test", nil)
	s.Subscribe("test", nil)

	_, err := s.Send("page1", "test", nil)
	if err == nil {
		t.Fatal("nil Send should error")
	}

	if err := s.Broadcast("test", nil); err == nil {
		t.Fatal("nil Broadcast should error")
	}
}

func connectTestClient(t *testing.T, addr, pageID string) *websocket.Conn {
	t.Helper()

	origin := "http://localhost/"
	wsURL := "ws://" + addr + "/bridge"

	conn, err := websocket.Dial(wsURL, "", origin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send registration message.
	reg := BridgeMessage{
		Type:   "register",
		Method: pageID,
	}
	if err := websocket.JSON.Send(conn, reg); err != nil {
		t.Fatalf("send registration: %v", err)
	}

	// Give server a moment to process.
	time.Sleep(50 * time.Millisecond)

	return conn
}

func TestBridgeServerSendReceive(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = s.Stop() }()

	conn := connectTestClient(t, s.Addr(), "page-1")

	defer func() { _ = conn.Close() }()

	// Verify client registered.
	clients := s.Clients()
	if len(clients) != 1 || clients[0] != "page-1" {
		t.Fatalf("expected [page-1], got %v", clients)
	}

	// Send a request from server to client in a goroutine.
	type result struct {
		msg *BridgeMessage
		err error
	}

	ch := make(chan result, 1)

	go func() {
		msg, err := s.Send("page-1", "dom.query", map[string]string{"selector": "h1"})
		ch <- result{msg, err}
	}()

	// Client reads the request and sends response.
	var req BridgeMessage
	if err := websocket.JSON.Receive(conn, &req); err != nil {
		t.Fatalf("client receive: %v", err)
	}

	if req.Method != "dom.query" {
		t.Fatalf("expected method dom.query, got %s", req.Method)
	}

	resp := BridgeMessage{
		ID:     req.ID,
		Type:   "response",
		Result: json.RawMessage(`{"found": true}`),
	}
	if err := websocket.JSON.Send(conn, resp); err != nil {
		t.Fatalf("client send response: %v", err)
	}

	// Server receives the response.
	r := <-ch
	if r.err != nil {
		t.Fatalf("Send error: %v", r.err)
	}

	var data map[string]any
	if err := json.Unmarshal(r.msg.Result, &data); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if data["found"] != true {
		t.Fatalf("expected found=true, got %v", data["found"])
	}
}

func TestBridgeServerSendTimeout(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = s.Stop() }()

	// Send to non-existent client.
	_, err := s.Send("nonexistent", "test", nil)
	if err == nil {
		t.Fatal("expected error for non-existent client")
	}
}

func TestBridgeServerEventChannel(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = s.Stop() }()

	conn := connectTestClient(t, s.Addr(), "page-evt")

	defer func() { _ = conn.Close() }()

	// Client sends an event.
	evt := BridgeMessage{
		Type:   "event",
		Method: "dom.mutation",
		Params: json.RawMessage(`{"selector": "body", "count": 3}`),
	}
	if err := websocket.JSON.Send(conn, evt); err != nil {
		t.Fatalf("send event: %v", err)
	}

	// Read from events channel.
	select {
	case e := <-s.Events():
		if e.Type != "dom.mutation" {
			t.Fatalf("expected dom.mutation, got %s", e.Type)
		}

		if e.PageID != "page-evt" {
			t.Fatalf("expected page-evt, got %s", e.PageID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBridgeServerSubscribe(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = s.Stop() }()

	conn := connectTestClient(t, s.Addr(), "page-sub")

	defer func() { _ = conn.Close() }()

	var (
		mu       sync.Mutex
		received []BridgeEvent
	)

	// Subscribe only to user.click events.

	s.Subscribe("user.click", func(e BridgeEvent) {
		mu.Lock()

		received = append(received, e)
		mu.Unlock()
	})

	// Send a user.click event.
	click := BridgeMessage{
		Type:   "event",
		Method: "user.click",
		Params: json.RawMessage(`{"x": 100, "y": 200}`),
	}
	if err := websocket.JSON.Send(conn, click); err != nil {
		t.Fatalf("send click: %v", err)
	}

	// Send a dom.mutation event (should not be received by subscriber).
	mutation := BridgeMessage{
		Type:   "event",
		Method: "dom.mutation",
		Params: json.RawMessage(`{}`),
	}
	if err := websocket.JSON.Send(conn, mutation); err != nil {
		t.Fatalf("send mutation: %v", err)
	}

	// Drain both events from channel.
	for range 2 {
		select {
		case <-s.Events():
		case <-time.After(2 * time.Second):
			t.Fatal("timeout draining events")
		}
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 subscribed event, got %d", len(received))
	}

	if received[0].Type != "user.click" {
		t.Fatalf("expected user.click, got %s", received[0].Type)
	}
}

func TestBridgeServerConcurrentClients(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = s.Stop() }()

	const numClients = 5

	conns := make([]*websocket.Conn, numClients)
	for i := range numClients {
		conns[i] = connectTestClient(t, s.Addr(), fmt.Sprintf("page-%d", i))
	}

	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	clients := s.Clients()
	if len(clients) != numClients {
		t.Fatalf("expected %d clients, got %d", numClients, len(clients))
	}

	// Broadcast to all.
	if err := s.Broadcast("ping", map[string]string{"msg": "hello"}); err != nil {
		t.Fatalf("Broadcast: %v", err)
	}

	// Each client should receive the broadcast.
	for i, conn := range conns {
		var msg BridgeMessage
		if err := websocket.JSON.Receive(conn, &msg); err != nil {
			t.Fatalf("client %d receive: %v", i, err)
		}

		if msg.Method != "ping" {
			t.Fatalf("client %d expected ping, got %s", i, msg.Method)
		}
	}
}

func TestBridgeServerBroadcast(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = s.Stop() }()

	conn1 := connectTestClient(t, s.Addr(), "p1")
	conn2 := connectTestClient(t, s.Addr(), "p2")

	defer func() { _ = conn1.Close() }()
	defer func() { _ = conn2.Close() }()

	if err := s.Broadcast("notify", map[string]int{"value": 42}); err != nil {
		t.Fatalf("Broadcast: %v", err)
	}

	for _, conn := range []*websocket.Conn{conn1, conn2} {
		var msg BridgeMessage
		if err := websocket.JSON.Receive(conn, &msg); err != nil {
			t.Fatalf("receive: %v", err)
		}

		if msg.Method != "notify" {
			t.Fatalf("expected notify, got %s", msg.Method)
		}
	}
}

func TestBridgeServerOnMessage(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")

	// Register handler before starting.
	s.OnMessage("tab.list", func(msg BridgeMessage) (any, error) {
		return map[string]any{"tabs": []string{"tab1", "tab2"}}, nil
	})

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = s.Stop() }()

	conn := connectTestClient(t, s.Addr(), "page-handler")

	defer func() { _ = conn.Close() }()

	// Client sends a request to the server.
	req := BridgeMessage{
		ID:     "client-req-1",
		Type:   "request",
		Method: "tab.list",
	}
	if err := websocket.JSON.Send(conn, req); err != nil {
		t.Fatalf("send request: %v", err)
	}

	// Client receives the response.
	var resp BridgeMessage
	if err := websocket.JSON.Receive(conn, &resp); err != nil {
		t.Fatalf("receive response: %v", err)
	}

	if resp.ID != "client-req-1" {
		t.Fatalf("expected id client-req-1, got %s", resp.ID)
	}

	if resp.Type != "response" {
		t.Fatalf("expected type response, got %s", resp.Type)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	tabs, ok := result["tabs"]
	if !ok {
		t.Fatal("expected tabs in result")
	}

	tabList := tabs.([]any)
	if len(tabList) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(tabList))
	}
}
