package hijack

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"sync"
	"time"
)

// wsConnection tracks a single WebSocket connection during recording.
type wsConnection struct {
	url      string
	openedAt time.Time
	messages []HARWebSocketMessage
}

// Recorder collects Events and exports them as a HAR 1.2 log.
// Attach it to a SessionHijacker to build a HAR archive from live traffic.
type Recorder struct {
	mu        sync.Mutex
	requests  map[string]*CapturedRequest  // requestID -> request
	responses map[string]*CapturedResponse // requestID -> response
	bodies    map[string]string            // requestID -> body (from body-capture events)
	order     []string                     // requestIDs in order of first appearance
	wsConns   map[string]*wsConnection     // requestID -> WebSocket connection state
}

// NewRecorder creates a recorder that can be fed Events.
func NewRecorder() *Recorder {
	return &Recorder{
		requests:  make(map[string]*CapturedRequest),
		responses: make(map[string]*CapturedResponse),
		bodies:    make(map[string]string),
		wsConns:   make(map[string]*wsConnection),
	}
}

// Record processes a single Event. Call this for each event from SessionHijacker.Events().
func (r *Recorder) Record(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch ev.Type { //nolint:exhaustive
	case EventRequest:
		if ev.Request != nil {
			id := ev.Request.RequestID
			if _, exists := r.requests[id]; !exists {
				r.order = append(r.order, id)
			}

			r.requests[id] = ev.Request
		}
	case EventResponse:
		if ev.Response != nil {
			id := ev.Response.RequestID
			if ev.Response.Status > 0 {
				r.responses[id] = ev.Response
			}

			if ev.Response.Body != "" {
				r.bodies[id] = ev.Response.Body
			}
		}
	case WSOpened:
		if ev.Frame != nil {
			r.wsConns[ev.Frame.RequestID] = &wsConnection{
				url:      ev.Frame.URL,
				openedAt: ev.Frame.Timestamp,
			}
		}
	case WSSent, WSReceived:
		if ev.Frame != nil {
			conn, ok := r.wsConns[ev.Frame.RequestID]
			if !ok {
				// Connection opened before recording started; create ad-hoc entry.
				conn = &wsConnection{url: ev.Frame.URL, openedAt: ev.Frame.Timestamp}
				r.wsConns[ev.Frame.RequestID] = conn
			}

			dir := "receive"
			if ev.Type == WSSent {
				dir = "send"
			}

			elapsed := ev.Frame.Timestamp.Sub(conn.openedAt).Seconds() * 1000
			conn.messages = append(conn.messages, HARWebSocketMessage{
				Time:   elapsed,
				Opcode: int(ev.Frame.Opcode),
				Data:   ev.Frame.Payload,
				Type:   dir,
			})
		}
	case WSClosed:
		// Keep the connection data — it will be exported in ExportHAR.
	}
}

// RecordAll drains all events from the channel until it closes and records them.
func (r *Recorder) RecordAll(events <-chan Event) {
	for ev := range events {
		r.Record(ev)
	}
}

// Len returns the number of recorded request/response pairs.
func (r *Recorder) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.order)
}

// ExportHAR returns the recorded traffic as a HAR 1.2 JSON document.
func (r *Recorder) ExportHAR() ([]byte, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := make([]HAREntry, 0, len(r.order))

	for _, id := range r.order {
		req, hasReq := r.requests[id]
		if !hasReq {
			continue
		}

		entry := HAREntry{
			StartedDateTime: req.Timestamp.UTC().Format(time.RFC3339Nano),
			Request: HARRequest{
				Method:      req.Method,
				URL:         req.URL,
				HTTPVersion: "HTTP/1.1",
				Headers:     MapToHARHeaders(req.Headers),
				QueryString: URLToHARQuery(req.URL),
				HeadersSize: -1,
				BodySize:    len(req.Body),
			},
		}

		if req.Body != "" {
			ct := req.Headers["content-type"]
			if ct == "" {
				ct = req.Headers["Content-Type"]
			}

			entry.Request.PostData = &HARPost{
				MimeType: ct,
				Text:     req.Body,
			}
		}

		resp, hasResp := r.responses[id]
		if hasResp {
			entry.Response = HARResponse{
				Status:      resp.Status,
				StatusText:  StatusText(resp.Status),
				HTTPVersion: "HTTP/1.1",
				Headers:     MapToHARHeaders(resp.Headers),
				Content: HARContent{
					MimeType: resp.MimeType,
				},
				HeadersSize: -1,
				BodySize:    -1,
			}
			entry.Time = resp.ElapsedMs

			entry.Timings = HARTimings{
				Blocked: -1,
				DNS:     -1,
				Connect: -1,
				Send:    0,
				Wait:    resp.ElapsedMs,
				Receive: 0,
				SSL:     -1,
			}
		} else {
			entry.Response = HARResponse{
				Status:      0,
				StatusText:  "pending",
				HTTPVersion: "HTTP/1.1",
				Content:     HARContent{MimeType: "x-unknown"},
				HeadersSize: -1,
				BodySize:    -1,
			}
		}

		if body, hasBody := r.bodies[id]; hasBody {
			entry.Response.Content.Text = body
			entry.Response.Content.Size = len(body)
		}

		entries = append(entries, entry)
	}

	// Add WebSocket connections as HAR entries with _webSocketMessages extension.
	for _, conn := range r.wsConns {
		entry := HAREntry{
			StartedDateTime: conn.openedAt.UTC().Format(time.RFC3339Nano),
			Time:            0,
			Request: HARRequest{
				Method:      "GET",
				URL:         conn.url,
				HTTPVersion: "HTTP/1.1",
				HeadersSize: -1,
				BodySize:    0,
				Headers:     []HARHeader{{Name: "Upgrade", Value: "websocket"}},
			},
			Response: HARResponse{
				Status:      101,
				StatusText:  "Switching Protocols",
				HTTPVersion: "HTTP/1.1",
				Headers:     []HARHeader{{Name: "Upgrade", Value: "websocket"}},
				Content:     HARContent{MimeType: "x-unknown"},
				HeadersSize: -1,
				BodySize:    -1,
			},
			WebSocket: &HARWebSocket{
				URL:      conn.url,
				Status:   101,
				Messages: conn.messages,
			},
		}
		entries = append(entries, entry)
	}

	log := struct {
		Log HARLog `json:"log"`
	}{
		Log: HARLog{
			Version: "1.2",
			Creator: HARCreator{
				Name:    "scout-hijack",
				Version: "1.0.0",
			},
			Entries: entries,
		},
	}

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, 0, fmt.Errorf("scout: hijack export har: %w", err)
	}

	return data, len(entries), nil
}

// MapToHARHeaders converts a string map to HAR headers.
func MapToHARHeaders(m map[string]string) []HARHeader {
	if len(m) == 0 {
		return nil
	}

	headers := make([]HARHeader, 0, len(m))
	for k, v := range m {
		headers = append(headers, HARHeader{Name: k, Value: v})
	}

	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Name < headers[j].Name
	})

	return headers
}

// URLToHARQuery parses a URL and returns HAR query string pairs.
func URLToHARQuery(rawURL string) []HARQuery {
	u, err := url.Parse(rawURL)
	if err != nil || u.RawQuery == "" {
		return nil
	}

	params := u.Query()

	qs := make([]HARQuery, 0, len(params))
	for k, vals := range params {
		for _, v := range vals {
			qs = append(qs, HARQuery{Name: k, Value: v})
		}
	}

	return qs
}

// StatusText returns the standard HTTP status text for a status code.
func StatusText(code int) string {
	texts := map[int]string{
		200: "OK", 201: "Created", 204: "No Content",
		301: "Moved Permanently", 302: "Found", 304: "Not Modified",
		400: "Bad Request", 401: "Unauthorized", 403: "Forbidden",
		404: "Not Found", 405: "Method Not Allowed", 429: "Too Many Requests",
		500: "Internal Server Error", 502: "Bad Gateway", 503: "Service Unavailable",
	}
	if t, ok := texts[code]; ok {
		return t
	}

	return fmt.Sprintf("%d", code)
}

// ExportWebSocketHAR returns only WebSocket entries as HAR JSON.
func (r *Recorder) ExportWebSocketHAR() ([]byte, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := make([]HAREntry, 0, len(r.wsConns))

	for _, conn := range r.wsConns {
		entry := HAREntry{
			StartedDateTime: conn.openedAt.UTC().Format(time.RFC3339Nano),
			Request: HARRequest{
				Method:      "GET",
				URL:         conn.url,
				HTTPVersion: "HTTP/1.1",
				HeadersSize: -1,
				Headers:     []HARHeader{{Name: "Upgrade", Value: "websocket"}},
			},
			Response: HARResponse{
				Status:      101,
				StatusText:  "Switching Protocols",
				HTTPVersion: "HTTP/1.1",
				Headers:     []HARHeader{{Name: "Upgrade", Value: "websocket"}},
				Content:     HARContent{MimeType: "x-unknown"},
				HeadersSize: -1,
				BodySize:    -1,
			},
			WebSocket: &HARWebSocket{
				URL:      conn.url,
				Status:   101,
				Messages: conn.messages,
			},
		}
		entries = append(entries, entry)
	}

	log := struct {
		Log HARLog `json:"log"`
	}{
		Log: HARLog{
			Version: "1.2",
			Creator: HARCreator{Name: "scout-hijack", Version: "1.0.0"},
			Entries: entries,
		},
	}

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, 0, fmt.Errorf("scout: hijack export ws har: %w", err)
	}

	return data, len(entries), nil
}

// WebSocketCount returns the number of recorded WebSocket connections.
func (r *Recorder) WebSocketCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.wsConns)
}

// WebSocketMessageCount returns the total number of recorded WebSocket messages.
func (r *Recorder) WebSocketMessageCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, conn := range r.wsConns {
		count += len(conn.messages)
	}
	return count
}
