package hijack

import "time"

// EventType identifies the kind of intercepted network event.
type EventType string

const (
	EventRequest  EventType = "request"
	EventResponse EventType = "response"
	WSSent        EventType = "ws.sent"
	WSReceived    EventType = "ws.received"
	WSOpened      EventType = "ws.opened"
	WSClosed      EventType = "ws.closed"
)

// CapturedRequest describes an intercepted HTTP request.
type CapturedRequest struct {
	RequestID    string            `json:"request_id"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         string            `json:"body,omitempty"`
	ResourceType string            `json:"resource_type,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// CapturedResponse describes an intercepted HTTP response.
type CapturedResponse struct {
	RequestID string            `json:"request_id"`
	URL       string            `json:"url"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	MimeType  string            `json:"mime_type,omitempty"`
	ElapsedMs float64           `json:"elapsed_ms"`
	Timestamp time.Time         `json:"timestamp"`
}

// WebSocketFrame describes an intercepted WebSocket message.
type WebSocketFrame struct {
	RequestID string    `json:"request_id"`
	URL       string    `json:"url"`
	Direction string    `json:"direction"` // "sent" or "received"
	Opcode    float64   `json:"opcode"`
	Payload   string    `json:"payload"`
	Masked    bool      `json:"masked"`
	Timestamp time.Time `json:"timestamp"`
}

// Event is a discriminated union of intercepted network events.
type Event struct {
	Type     EventType         `json:"type"`
	Request  *CapturedRequest  `json:"request,omitempty"`
	Response *CapturedResponse `json:"response,omitempty"`
	Frame    *WebSocketFrame   `json:"frame,omitempty"`
}

// Filter controls which network events are captured.
type Filter struct {
	URLPatterns   []string `json:"url_patterns,omitempty"`
	ResourceTypes []string `json:"resource_types,omitempty"`
	CaptureBody   bool     `json:"capture_body,omitempty"`
}

// Option configures a SessionHijacker.
type Option func(*Options)

// Options holds hijack configuration.
type Options struct {
	Filter      Filter
	ChannelSize int
}

// Defaults returns default hijack options.
func Defaults() *Options {
	return &Options{
		ChannelSize: 1024,
	}
}

// WithURLFilter adds URL glob patterns to the hijack filter.
func WithURLFilter(patterns ...string) Option {
	return func(o *Options) {
		o.Filter.URLPatterns = append(o.Filter.URLPatterns, patterns...)
	}
}

// WithBodyCapture enables response body capture.
func WithBodyCapture() Option {
	return func(o *Options) { o.Filter.CaptureBody = true }
}

// WithChannelSize sets the event channel buffer size. Default: 1024.
func WithChannelSize(n int) Option {
	return func(o *Options) { o.ChannelSize = n }
}
