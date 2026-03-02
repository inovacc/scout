package scout

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/scout/rod/lib/proto"
)

// HijackEventType identifies the kind of intercepted network event.
type HijackEventType string

const (
	HijackEventRequest  HijackEventType = "request"
	HijackEventResponse HijackEventType = "response"
	HijackWSSent        HijackEventType = "ws.sent"
	HijackWSReceived    HijackEventType = "ws.received"
	HijackWSOpened      HijackEventType = "ws.opened"
	HijackWSClosed      HijackEventType = "ws.closed"
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

// HijackEvent is a discriminated union of intercepted network events.
type HijackEvent struct {
	Type     HijackEventType   `json:"type"`
	Request  *CapturedRequest  `json:"request,omitempty"`
	Response *CapturedResponse `json:"response,omitempty"`
	Frame    *WebSocketFrame   `json:"frame,omitempty"`
}

// HijackFilter controls which network events are captured.
type HijackFilter struct {
	URLPatterns   []string `json:"url_patterns,omitempty"`
	ResourceTypes []string `json:"resource_types,omitempty"`
	CaptureBody   bool     `json:"capture_body,omitempty"`
}

// HijackOption configures a SessionHijacker.
type HijackOption func(*hijackOptions)

type hijackOptions struct {
	filter      HijackFilter
	channelSize int
}

func hijackDefaults() *hijackOptions {
	return &hijackOptions{
		channelSize: 1024,
	}
}

// WithHijackURLFilter adds URL glob patterns to the hijack filter.
func WithHijackURLFilter(patterns ...string) HijackOption {
	return func(o *hijackOptions) {
		o.filter.URLPatterns = append(o.filter.URLPatterns, patterns...)
	}
}

// WithHijackBodyCapture enables response body capture.
func WithHijackBodyCapture() HijackOption {
	return func(o *hijackOptions) { o.filter.CaptureBody = true }
}

// WithHijackChannelSize sets the event channel buffer size. Default: 1024.
func WithHijackChannelSize(n int) HijackOption {
	return func(o *hijackOptions) { o.channelSize = n }
}

// SessionHijacker captures real-time network traffic (HTTP + WebSocket) from a Page via CDP events.
type SessionHijacker struct {
	page       *Page
	events     chan HijackEvent
	stopCh     chan struct{}
	stopped    bool
	mu         sync.Mutex
	opts       *hijackOptions
	pendingWS  map[proto.NetworkRequestID]string // requestID -> URL
	startTimes map[proto.NetworkRequestID]time.Time
}

// NewSessionHijacker creates a hijacker that immediately begins capturing network traffic.
func (p *Page) NewSessionHijacker(opts ...HijackOption) (*SessionHijacker, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: session hijacker: nil page")
	}

	o := hijackDefaults()
	for _, fn := range opts {
		fn(o)
	}

	h := &SessionHijacker{
		page:       p,
		events:     make(chan HijackEvent, o.channelSize),
		stopCh:     make(chan struct{}),
		opts:       o,
		pendingWS:  make(map[proto.NetworkRequestID]string),
		startTimes: make(map[proto.NetworkRequestID]time.Time),
	}

	h.startCDP()
	return h, nil
}

// Events returns a read-only channel of hijacked network events.
func (h *SessionHijacker) Events() <-chan HijackEvent {
	return h.events
}

// Stop ends the hijacking session. It is safe to call multiple times.
func (h *SessionHijacker) Stop() {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.stopped {
		h.stopped = true
		close(h.stopCh)
		close(h.events)
	}
}

func (h *SessionHijacker) emit(ev HijackEvent) {
	select {
	case <-h.stopCh:
		return
	default:
	}

	select {
	case h.events <- ev:
	default: // drop if consumer is slow
	}
}

func (h *SessionHijacker) matchFilter(url string) bool {
	patterns := h.opts.filter.URLPatterns
	if len(patterns) == 0 {
		return true
	}

	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, url); matched {
			return true
		}
		// Also try contains for simple glob patterns like "*api*"
		if strings.Contains(pattern, "*") {
			clean := strings.ReplaceAll(pattern, "*", "")
			if clean != "" && strings.Contains(url, clean) {
				return true
			}
		}
	}
	return false
}

func (h *SessionHijacker) startCDP() {
	rodPage := h.page.RodPage()

	go rodPage.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			if !h.matchFilter(e.Request.URL) {
				return
			}

			h.mu.Lock()
			h.startTimes[e.RequestID] = time.Now()
			h.mu.Unlock()

			headers := make(map[string]string)
			for k, v := range e.Request.Headers {
				headers[k] = v.String()
			}

			h.emit(HijackEvent{
				Type: HijackEventRequest,
				Request: &CapturedRequest{
					RequestID:    string(e.RequestID),
					Method:       e.Request.Method,
					URL:          e.Request.URL,
					Headers:      headers,
					Body:         e.Request.PostData,
					ResourceType: string(e.Type),
					Timestamp:    time.Now(),
				},
			})
		},

		func(e *proto.NetworkResponseReceived) {
			if !h.matchFilter(e.Response.URL) {
				return
			}

			h.mu.Lock()
			startTime, hasStart := h.startTimes[e.RequestID]
			h.mu.Unlock()

			var elapsedMs float64
			if hasStart {
				elapsedMs = float64(time.Since(startTime).Milliseconds())
			}

			headers := make(map[string]string)
			for k, v := range e.Response.Headers {
				headers[k] = v.String()
			}

			h.emit(HijackEvent{
				Type: HijackEventResponse,
				Response: &CapturedResponse{
					RequestID: string(e.RequestID),
					URL:       e.Response.URL,
					Status:    e.Response.Status,
					Headers:   headers,
					MimeType:  e.Response.MIMEType,
					ElapsedMs: elapsedMs,
					Timestamp: time.Now(),
				},
			})
		},

		func(e *proto.NetworkLoadingFinished) {
			if !h.opts.filter.CaptureBody {
				h.mu.Lock()
				delete(h.startTimes, e.RequestID)
				h.mu.Unlock()
				return
			}

			body, err := proto.NetworkGetResponseBody{
				RequestID: e.RequestID,
			}.Call(h.page.RodPage())

			h.mu.Lock()
			delete(h.startTimes, e.RequestID)
			h.mu.Unlock()

			if err != nil || body == nil {
				return
			}

			// Emit a response event with body populated.
			h.emit(HijackEvent{
				Type: HijackEventResponse,
				Response: &CapturedResponse{
					RequestID: string(e.RequestID),
					Body:      body.Body,
					Timestamp: time.Now(),
				},
			})
		},

		func(e *proto.NetworkWebSocketCreated) {
			h.mu.Lock()
			h.pendingWS[e.RequestID] = e.URL
			h.mu.Unlock()

			if !h.matchFilter(e.URL) {
				return
			}

			h.emit(HijackEvent{
				Type: HijackWSOpened,
				Frame: &WebSocketFrame{
					RequestID: string(e.RequestID),
					URL:       e.URL,
					Direction: "opened",
					Timestamp: time.Now(),
				},
			})
		},

		func(e *proto.NetworkWebSocketFrameSent) {
			h.mu.Lock()
			url := h.pendingWS[e.RequestID]
			h.mu.Unlock()

			if !h.matchFilter(url) {
				return
			}

			var payload string
			var opcode float64
			var masked bool
			if e.Response != nil {
				payload = e.Response.PayloadData
				opcode = e.Response.Opcode
				masked = e.Response.Mask
			}

			h.emit(HijackEvent{
				Type: HijackWSSent,
				Frame: &WebSocketFrame{
					RequestID: string(e.RequestID),
					URL:       url,
					Direction: "sent",
					Opcode:    opcode,
					Payload:   payload,
					Masked:    masked,
					Timestamp: time.Now(),
				},
			})
		},

		func(e *proto.NetworkWebSocketFrameReceived) {
			h.mu.Lock()
			url := h.pendingWS[e.RequestID]
			h.mu.Unlock()

			if !h.matchFilter(url) {
				return
			}

			var payload string
			var opcode float64
			var masked bool
			if e.Response != nil {
				payload = e.Response.PayloadData
				opcode = e.Response.Opcode
				masked = e.Response.Mask
			}

			h.emit(HijackEvent{
				Type: HijackWSReceived,
				Frame: &WebSocketFrame{
					RequestID: string(e.RequestID),
					URL:       url,
					Direction: "received",
					Opcode:    opcode,
					Payload:   payload,
					Masked:    masked,
					Timestamp: time.Now(),
				},
			})
		},

		func(e *proto.NetworkWebSocketClosed) {
			h.mu.Lock()
			url := h.pendingWS[e.RequestID]
			delete(h.pendingWS, e.RequestID)
			h.mu.Unlock()

			if !h.matchFilter(url) {
				return
			}

			h.emit(HijackEvent{
				Type: HijackWSClosed,
				Frame: &WebSocketFrame{
					RequestID: string(e.RequestID),
					URL:       url,
					Direction: "closed",
					Timestamp: time.Now(),
				},
			})
		},
	)()
}
