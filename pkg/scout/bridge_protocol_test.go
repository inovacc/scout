package scout

import (
	"encoding/json"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestBridgeProtocol_JSONRPCSerialization(t *testing.T) {
	tests := []struct {
		name string
		msg  BridgeMessage
	}{
		{
			name: "request",
			msg: BridgeMessage{
				ID:     "req-1",
				Type:   "request",
				Method: "dom.query",
				Params: json.RawMessage(`{"selector":"h1"}`),
			},
		},
		{
			name: "response",
			msg: BridgeMessage{
				ID:     "req-1",
				Type:   "response",
				Result: json.RawMessage(`{"found":true}`),
			},
		},
		{
			name: "response with error",
			msg: BridgeMessage{
				ID:    "req-2",
				Type:  "response",
				Error: "element not found",
			},
		},
		{
			name: "event",
			msg: BridgeMessage{
				Type:   "event",
				Method: "user.click",
				Params: json.RawMessage(`{"selector":"#btn","x":10,"y":20}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var decoded BridgeMessage
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if decoded.ID != tt.msg.ID {
				t.Fatalf("ID mismatch: %q != %q", decoded.ID, tt.msg.ID)
			}
			if decoded.Type != tt.msg.Type {
				t.Fatalf("Type mismatch: %q != %q", decoded.Type, tt.msg.Type)
			}
			if decoded.Method != tt.msg.Method {
				t.Fatalf("Method mismatch: %q != %q", decoded.Method, tt.msg.Method)
			}
			if decoded.Error != tt.msg.Error {
				t.Fatalf("Error mismatch: %q != %q", decoded.Error, tt.msg.Error)
			}
			if string(decoded.Params) != string(tt.msg.Params) {
				t.Fatalf("Params mismatch: %s != %s", decoded.Params, tt.msg.Params)
			}
			if string(decoded.Result) != string(tt.msg.Result) {
				t.Fatalf("Result mismatch: %s != %s", decoded.Result, tt.msg.Result)
			}
		})
	}
}

func TestBridgeProtocol_Registration(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := connectTestClient(t, s.Addr(), "page-reg-1")
	defer func() { _ = conn.Close() }()

	clients := s.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0] != "page-reg-1" {
		t.Fatalf("expected page-reg-1, got %s", clients[0])
	}
}

func TestBridgeProtocol_Reconnect(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	// Connect first time.
	conn1 := connectTestClient(t, s.Addr(), "page-recon")
	clients := s.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}

	// Disconnect.
	_ = conn1.Close()
	time.Sleep(100 * time.Millisecond)

	// After disconnect, client list should be empty.
	clients = s.Clients()
	if len(clients) != 0 {
		t.Fatalf("expected 0 clients after disconnect, got %d", len(clients))
	}

	// Reconnect with same page ID.
	conn2 := connectTestClient(t, s.Addr(), "page-recon")
	defer func() { _ = conn2.Close() }()

	clients = s.Clients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client after reconnect, got %d", len(clients))
	}
	if clients[0] != "page-recon" {
		t.Fatalf("expected page-recon, got %s", clients[0])
	}
}

func TestBridgeProtocol_MessageRouting(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	// Connect two clients.
	conn1 := connectTestClient(t, s.Addr(), "page-route-1")
	conn2 := connectTestClient(t, s.Addr(), "page-route-2")
	defer func() { _ = conn1.Close() }()
	defer func() { _ = conn2.Close() }()

	// Send to page-route-1 only.
	go func() {
		_, _ = s.Send("page-route-1", "ping", map[string]string{"msg": "hello"})
	}()

	// conn1 should receive the message.
	var msg1 BridgeMessage
	if err := websocket.JSON.Receive(conn1, &msg1); err != nil {
		t.Fatalf("conn1 receive: %v", err)
	}
	if msg1.Method != "ping" {
		t.Fatalf("expected ping, got %s", msg1.Method)
	}

	// conn2 should NOT receive anything (with short timeout).
	_ = conn2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var msg2 BridgeMessage
	err := websocket.JSON.Receive(conn2, &msg2)
	if err == nil {
		t.Fatal("conn2 should not have received a message")
	}
}

func TestBridgeProtocol_ErrorHandling(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := connectTestClient(t, s.Addr(), "page-err-proto")
	defer func() { _ = conn.Close() }()

	// Server sends request, client responds with error.
	type result struct {
		msg *BridgeMessage
		err error
	}
	ch := make(chan result, 1)
	go func() {
		msg, err := s.Send("page-err-proto", "dom.query", map[string]string{"selector": "#missing"})
		ch <- result{msg, err}
	}()

	// Client reads request and sends error response.
	var req BridgeMessage
	if err := websocket.JSON.Receive(conn, &req); err != nil {
		t.Fatalf("receive: %v", err)
	}

	resp := BridgeMessage{
		ID:    req.ID,
		Type:  "response",
		Error: "element not found: #missing",
	}
	if err := websocket.JSON.Send(conn, resp); err != nil {
		t.Fatalf("send response: %v", err)
	}

	r := <-ch
	if r.err == nil {
		t.Fatal("expected error from Send")
	}
	if r.msg == nil {
		t.Fatal("expected non-nil message even with error")
	}
	if r.msg.Error != "element not found: #missing" {
		t.Fatalf("expected error message, got %q", r.msg.Error)
	}
}
