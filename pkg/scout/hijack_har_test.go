package scout

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestHijackRecorderEmpty(t *testing.T) {
	r := NewHijackRecorder()

	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected 0 entries, got %d", count)
	}

	var har struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		t.Fatal(err)
	}

	if har.Log.Version != "1.2" {
		t.Fatalf("expected HAR version 1.2, got %s", har.Log.Version)
	}
}

func TestHijackRecorderRequestResponse(t *testing.T) {
	r := NewHijackRecorder()
	now := time.Now()

	r.Record(HijackEvent{
		Type: HijackEventRequest,
		Request: &CapturedRequest{
			RequestID: "1",
			Method:    "GET",
			URL:       "https://example.com/api?q=test",
			Headers:   map[string]string{"Accept": "application/json"},
			Timestamp: now,
		},
	})

	r.Record(HijackEvent{
		Type: HijackEventResponse,
		Response: &CapturedResponse{
			RequestID: "1",
			URL:       "https://example.com/api?q=test",
			Status:    200,
			Headers:   map[string]string{"Content-Type": "application/json"},
			MimeType:  "application/json",
			ElapsedMs: 42.5,
			Timestamp: now.Add(42 * time.Millisecond),
		},
	})

	// Body capture event
	r.Record(HijackEvent{
		Type: HijackEventResponse,
		Response: &CapturedResponse{
			RequestID: "1",
			Body:      `{"result":"ok"}`,
			Timestamp: now.Add(43 * time.Millisecond),
		},
	})

	if r.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", r.Len())
	}

	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected 1 entry, got %d", count)
	}

	var har struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		t.Fatal(err)
	}

	entry := har.Log.Entries[0]
	if entry.Request.Method != http.MethodGet {
		t.Fatalf("expected GET, got %s", entry.Request.Method)
	}

	if entry.Response.Status != 200 {
		t.Fatalf("expected 200, got %d", entry.Response.Status)
	}

	if entry.Response.Content.Text != `{"result":"ok"}` {
		t.Fatalf("expected body, got %q", entry.Response.Content.Text)
	}

	if len(entry.Request.QueryString) != 1 || entry.Request.QueryString[0].Name != "q" {
		t.Fatalf("expected query string parsed, got %v", entry.Request.QueryString)
	}
}

func TestHijackRecorderWebSocketIgnored(t *testing.T) {
	r := NewHijackRecorder()

	// WebSocket events should not create HAR entries.
	r.Record(HijackEvent{
		Type: HijackWSOpened,
		Frame: &WebSocketFrame{
			RequestID: "ws1",
			URL:       "wss://example.com/ws",
			Direction: "opened",
			Timestamp: time.Now(),
		},
	})

	if r.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", r.Len())
	}
}

func TestStatusText(t *testing.T) {
	if got := statusText(200); got != "OK" {
		t.Fatalf("expected OK, got %s", got)
	}

	if got := statusText(404); got != "Not Found" {
		t.Fatalf("expected Not Found, got %s", got)
	}

	if got := statusText(999); got != "999" {
		t.Fatalf("expected 999, got %s", got)
	}
}
