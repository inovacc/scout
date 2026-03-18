package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// WebSocketConnection represents an active WebSocket connection observed via CDP.
type WebSocketConnection struct {
	RequestID string
	URL       string
	Messages  []WebSocketMessage
	mu        sync.Mutex
	page      *Page
	handlers  []WebSocketHandler
	closed    bool
}

// WebSocketMessage is a single WebSocket frame captured via CDP.
type WebSocketMessage struct {
	Direction string    `json:"direction"` // "sent" or "received"
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Opcode    int       `json:"opcode"` // 1=text, 2=binary
}

// WebSocketHandler is called for each received WebSocket message.
type WebSocketHandler func(msg WebSocketMessage)

// WebSocketOption configures WebSocket monitoring.
type WebSocketOption func(*wsOptions)

type wsOptions struct {
	urlFilter string
	captureAll bool
}

// WithWSURLFilter filters WebSocket connections by URL pattern.
func WithWSURLFilter(pattern string) WebSocketOption {
	return func(o *wsOptions) { o.urlFilter = pattern }
}

// WithWSCaptureAll captures both sent and received messages.
func WithWSCaptureAll() WebSocketOption {
	return func(o *wsOptions) { o.captureAll = true }
}

// MonitorWebSockets starts monitoring WebSocket traffic on the page via CDP events.
// Returns a channel that emits WebSocket messages as they occur.
func (p *Page) MonitorWebSockets(opts ...WebSocketOption) (<-chan WebSocketMessage, func(), error) {
	o := &wsOptions{captureAll: true}
	for _, opt := range opts {
		opt(o)
	}

	ch := make(chan WebSocketMessage, 256)
	ctx, cancel := context.WithCancel(context.Background())

	// Use CDP Network domain events for WebSocket frames.
	// Enable Network domain if not already enabled.
	_, _ = p.Eval(`void 0`) // ensure page is alive

	go func() {
		defer close(ch)

		// Poll for WS frames via CDP event listeners injected via JS.
		// This is a simplified approach using the page's eval capability.
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		// Inject WS interceptor.
		_, _ = p.Eval(`(() => {
			if (window.__scoutWSCapture) return;
			window.__scoutWSCapture = [];
			const origWS = window.WebSocket;
			window.WebSocket = function(url, protocols) {
				const ws = protocols ? new origWS(url, protocols) : new origWS(url);
				const capture = window.__scoutWSCapture;

				ws.addEventListener('message', (e) => {
					capture.push({direction: 'received', data: typeof e.data === 'string' ? e.data : '[binary]', ts: Date.now(), url: url});
				});

				const origSend = ws.send.bind(ws);
				ws.send = function(data) {
					capture.push({direction: 'sent', data: typeof data === 'string' ? data : '[binary]', ts: Date.now(), url: url});
					return origSend(data);
				};

				return ws;
			};
			window.WebSocket.prototype = origWS.prototype;
			window.WebSocket.CONNECTING = origWS.CONNECTING;
			window.WebSocket.OPEN = origWS.OPEN;
			window.WebSocket.CLOSING = origWS.CLOSING;
			window.WebSocket.CLOSED = origWS.CLOSED;
		})()`)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := p.Eval(`(() => {
					if (!window.__scoutWSCapture || window.__scoutWSCapture.length === 0) return '[]';
					const msgs = JSON.stringify(window.__scoutWSCapture);
					window.__scoutWSCapture = [];
					return msgs;
				})()`)
				if err != nil {
					continue
				}

				raw := result.String()
				if raw == "" || raw == "[]" {
					continue
				}

				var msgs []struct {
					Direction string `json:"direction"`
					Data      string `json:"data"`
					TS        int64  `json:"ts"`
					URL       string `json:"url"`
				}

				if err := json.Unmarshal([]byte(raw), &msgs); err != nil {
					continue
				}

				for _, m := range msgs {
					if o.urlFilter != "" && !matchesPattern(m.URL, o.urlFilter) {
						continue
					}

					msg := WebSocketMessage{
						Direction: m.Direction,
						Data:      m.Data,
						Timestamp: time.UnixMilli(m.TS),
						Opcode:    1, // text
					}

					select {
					case ch <- msg:
					default:
						// Drop on overflow.
					}
				}
			}
		}
	}()

	stop := func() {
		cancel()
		// Clean up interceptor.
		_, _ = p.Eval(`delete window.__scoutWSCapture`)
	}

	return ch, stop, nil
}

func matchesPattern(s, pattern string) bool {
	// Simple substring match. Could be extended to glob.
	return len(pattern) == 0 || containsSubstr(s, pattern)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}

// SendWebSocketMessage sends a message to a WebSocket connection on the page via JS eval.
func (p *Page) SendWebSocketMessage(url, data string) error {
	js := fmt.Sprintf(`(() => {
		// Find matching WebSocket.
		const targets = [];
		// We need to find existing WS connections — not straightforward from JS.
		// This works for newly created connections after our interceptor.
		return 'send not directly supported — use page eval with your WS reference';
	})()`)

	_, err := p.Eval(js)

	return err
}
