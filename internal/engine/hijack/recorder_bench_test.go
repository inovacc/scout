package hijack

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkRecorderRecord(b *testing.B) {
	r := NewRecorder()
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Record(Event{
			Type: EventRequest,
			Request: &CapturedRequest{
				RequestID: fmt.Sprintf("req-%d", i%1000),
				Method:    "GET",
				URL:       "https://example.com/page",
				Timestamp: now,
			},
		})
	}
}

func BenchmarkRecorderExportHAR(b *testing.B) {
	r := NewRecorder()
	now := time.Now()

	// Pre-populate with 100 request/response pairs.
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("req-%d", i)
		r.Record(Event{Type: EventRequest, Request: &CapturedRequest{
			RequestID: id, Method: "GET", URL: "https://example.com/page",
			Headers: map[string]string{"Accept": "text/html"}, Timestamp: now,
		}})
		r.Record(Event{Type: EventResponse, Response: &CapturedResponse{
			RequestID: id, URL: "https://example.com/page", Status: 200,
			Headers: map[string]string{"Content-Type": "text/html"}, ElapsedMs: 50,
		}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = r.ExportHAR()
	}
}

func BenchmarkRecorderWebSocket(b *testing.B) {
	r := NewRecorder()
	now := time.Now()

	r.Record(Event{Type: WSOpened, Frame: &WebSocketFrame{
		RequestID: "ws-1", URL: "wss://example.com/ws", Timestamp: now,
	}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Record(Event{
			Type: WSSent,
			Frame: &WebSocketFrame{
				RequestID: "ws-1", URL: "wss://example.com/ws",
				Direction: "sent", Opcode: 1, Payload: `{"msg":"hello"}`,
				Timestamp: now.Add(time.Duration(i) * time.Millisecond),
			},
		})
	}
}

func BenchmarkMapToHARHeaders(b *testing.B) {
	headers := map[string]string{
		"Content-Type":    "text/html",
		"Content-Length":  "1234",
		"Cache-Control":   "no-cache",
		"Accept-Encoding": "gzip",
		"X-Request-Id":    "abc-123",
		"Authorization":   "Bearer xxx",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MapToHARHeaders(headers)
	}
}
