package hijack

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRecorderWebSocketEvents(t *testing.T) {
	r := NewRecorder()

	now := time.Now()

	// Open a WS connection
	r.Record(Event{
		Type: WSOpened,
		Frame: &WebSocketFrame{
			RequestID: "ws-1",
			URL:       "wss://example.com/ws",
			Direction: "opened",
			Timestamp: now,
		},
	})

	// Send a message
	r.Record(Event{
		Type: WSSent,
		Frame: &WebSocketFrame{
			RequestID: "ws-1",
			URL:       "wss://example.com/ws",
			Direction: "sent",
			Opcode:    1,
			Payload:   `{"action":"subscribe"}`,
			Timestamp: now.Add(100 * time.Millisecond),
		},
	})

	// Receive a message
	r.Record(Event{
		Type: WSReceived,
		Frame: &WebSocketFrame{
			RequestID: "ws-1",
			URL:       "wss://example.com/ws",
			Direction: "received",
			Opcode:    1,
			Payload:   `{"status":"subscribed"}`,
			Timestamp: now.Add(200 * time.Millisecond),
		},
	})

	// Close
	r.Record(Event{
		Type: WSClosed,
		Frame: &WebSocketFrame{
			RequestID: "ws-1",
			URL:       "wss://example.com/ws",
			Direction: "closed",
			Timestamp: now.Add(5 * time.Second),
		},
	})

	// Verify counts
	if got := r.WebSocketCount(); got != 1 {
		t.Errorf("WebSocketCount() = %d, want 1", got)
	}
	if got := r.WebSocketMessageCount(); got != 2 {
		t.Errorf("WebSocketMessageCount() = %d, want 2", got)
	}

	// Test ExportHAR includes WS entry
	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 { // 0 HTTP + 1 WS
		t.Errorf("ExportHAR count = %d, want 1", count)
	}

	// Verify JSON structure
	var har struct {
		Log struct {
			Entries []json.RawMessage `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		t.Fatal(err)
	}
	if len(har.Log.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(har.Log.Entries))
	}

	// Verify WS entry has _webSocketMessages
	var entry struct {
		Request struct {
			URL string `json:"url"`
		} `json:"request"`
		WebSocket *struct {
			URL      string `json:"url"`
			Messages []struct {
				Type string `json:"type"`
				Data string `json:"data"`
			} `json:"messages"`
		} `json:"_webSocketMessages"`
	}
	if err := json.Unmarshal(har.Log.Entries[0], &entry); err != nil {
		t.Fatal(err)
	}
	if entry.WebSocket == nil {
		t.Fatal("expected _webSocketMessages in WS HAR entry")
	}
	if len(entry.WebSocket.Messages) != 2 {
		t.Errorf("got %d WS messages, want 2", len(entry.WebSocket.Messages))
	}
}

func TestRecorderWebSocketWithoutOpen(t *testing.T) {
	// Test that WS messages before WSOpened still get recorded (ad-hoc connection)
	r := NewRecorder()

	r.Record(Event{
		Type: WSSent,
		Frame: &WebSocketFrame{
			RequestID: "ws-orphan",
			URL:       "wss://example.com/ws",
			Direction: "sent",
			Opcode:    1,
			Payload:   "hello",
			Timestamp: time.Now(),
		},
	})

	if got := r.WebSocketCount(); got != 1 {
		t.Errorf("WebSocketCount() = %d, want 1", got)
	}
}

func TestRecorderExportWebSocketHAR(t *testing.T) {
	r := NewRecorder()

	// Add an HTTP request
	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "http-1",
			Method:    "GET",
			URL:       "https://example.com",
			Timestamp: time.Now(),
		},
	})

	// Add a WS connection
	r.Record(Event{
		Type: WSOpened,
		Frame: &WebSocketFrame{
			RequestID: "ws-1",
			URL:       "wss://example.com/ws",
			Timestamp: time.Now(),
		},
	})

	// ExportWebSocketHAR should only have WS
	data, count, err := r.ExportWebSocketHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("ExportWebSocketHAR count = %d, want 1", count)
	}

	// Verify it's valid JSON
	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("ExportWebSocketHAR returned invalid JSON: %v", err)
	}

	// ExportHAR should have both
	_, fullCount, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if fullCount != 2 { // 1 HTTP + 1 WS
		t.Errorf("ExportHAR count = %d, want 2", fullCount)
	}
}

func TestRecorderMixedHTTPAndWS(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	r.Record(Event{Type: EventRequest, Request: &CapturedRequest{RequestID: "r1", Method: "GET", URL: "https://example.com", Timestamp: now}})
	r.Record(Event{Type: WSOpened, Frame: &WebSocketFrame{RequestID: "ws1", URL: "wss://example.com/ws", Timestamp: now}})
	r.Record(Event{Type: EventResponse, Response: &CapturedResponse{RequestID: "r1", URL: "https://example.com", Status: 200, ElapsedMs: 50}})
	r.Record(Event{Type: WSSent, Frame: &WebSocketFrame{RequestID: "ws1", URL: "wss://example.com/ws", Opcode: 1, Payload: "ping", Timestamp: now.Add(time.Second)}})

	if r.Len() != 1 {
		t.Errorf("HTTP Len = %d, want 1", r.Len())
	}
	if r.WebSocketCount() != 1 {
		t.Errorf("WS count = %d, want 1", r.WebSocketCount())
	}
	if r.WebSocketMessageCount() != 1 {
		t.Errorf("WS msg count = %d, want 1", r.WebSocketMessageCount())
	}
}

func TestRecorderWebSocketMessageDirections(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	r.Record(Event{
		Type: WSOpened,
		Frame: &WebSocketFrame{
			RequestID: "ws-dir",
			URL:       "wss://example.com/ws",
			Timestamp: now,
		},
	})

	r.Record(Event{
		Type: WSSent,
		Frame: &WebSocketFrame{
			RequestID: "ws-dir",
			URL:       "wss://example.com/ws",
			Opcode:    1,
			Payload:   "outbound",
			Timestamp: now.Add(50 * time.Millisecond),
		},
	})

	r.Record(Event{
		Type: WSReceived,
		Frame: &WebSocketFrame{
			RequestID: "ws-dir",
			URL:       "wss://example.com/ws",
			Opcode:    1,
			Payload:   "inbound",
			Timestamp: now.Add(100 * time.Millisecond),
		},
	})

	data, _, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}

	var har struct {
		Log struct {
			Entries []struct {
				WebSocket *HARWebSocket `json:"_webSocketMessages"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		t.Fatal(err)
	}

	if len(har.Log.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(har.Log.Entries))
	}

	ws := har.Log.Entries[0].WebSocket
	if ws == nil {
		t.Fatal("expected WebSocket data")
	}

	if len(ws.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(ws.Messages))
	}

	if ws.Messages[0].Type != "send" {
		t.Errorf("message[0] type = %q, want send", ws.Messages[0].Type)
	}
	if ws.Messages[1].Type != "receive" {
		t.Errorf("message[1] type = %q, want receive", ws.Messages[1].Type)
	}
	if ws.Messages[0].Data != "outbound" {
		t.Errorf("message[0] data = %q, want outbound", ws.Messages[0].Data)
	}
	if ws.Messages[1].Data != "inbound" {
		t.Errorf("message[1] data = %q, want inbound", ws.Messages[1].Data)
	}
}

func TestRecorderEmptyExport(t *testing.T) {
	r := NewRecorder()

	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("empty recorder ExportHAR count = %d, want 0", count)
	}

	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("empty ExportHAR invalid JSON: %v", err)
	}

	wsData, wsCount, err := r.ExportWebSocketHAR()
	if err != nil {
		t.Fatal(err)
	}
	if wsCount != 0 {
		t.Errorf("empty recorder ExportWebSocketHAR count = %d, want 0", wsCount)
	}

	if err := json.Unmarshal(wsData, &check); err != nil {
		t.Fatalf("empty ExportWebSocketHAR invalid JSON: %v", err)
	}
}
