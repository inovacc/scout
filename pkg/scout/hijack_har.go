package scout

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"sync"
	"time"
)

// HijackRecorder collects HijackEvents and exports them as a HAR 1.2 log.
// Attach it to a SessionHijacker to build a HAR archive from live traffic.
type HijackRecorder struct {
	mu        sync.Mutex
	requests  map[string]*CapturedRequest  // requestID -> request
	responses map[string]*CapturedResponse // requestID -> response
	bodies    map[string]string            // requestID -> body (from body-capture events)
	order     []string                     // requestIDs in order of first appearance
}

// NewHijackRecorder creates a recorder that can be fed HijackEvents.
func NewHijackRecorder() *HijackRecorder {
	return &HijackRecorder{
		requests:  make(map[string]*CapturedRequest),
		responses: make(map[string]*CapturedResponse),
		bodies:    make(map[string]string),
	}
}

// Record processes a single HijackEvent. Call this for each event from SessionHijacker.Events().
func (r *HijackRecorder) Record(ev HijackEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch ev.Type {
	case HijackEventRequest:
		if ev.Request != nil {
			id := ev.Request.RequestID
			if _, exists := r.requests[id]; !exists {
				r.order = append(r.order, id)
			}
			r.requests[id] = ev.Request
		}
	case HijackEventResponse:
		if ev.Response != nil {
			id := ev.Response.RequestID
			if ev.Response.Status > 0 {
				r.responses[id] = ev.Response
			}
			if ev.Response.Body != "" {
				r.bodies[id] = ev.Response.Body
			}
		}
	}
}

// RecordAll drains all events from the channel until it closes and records them.
func (r *HijackRecorder) RecordAll(events <-chan HijackEvent) {
	for ev := range events {
		r.Record(ev)
	}
}

// Len returns the number of recorded request/response pairs.
func (r *HijackRecorder) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.order)
}

// ExportHAR returns the recorded traffic as a HAR 1.2 JSON document.
func (r *HijackRecorder) ExportHAR() ([]byte, int, error) {
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
				Headers:     mapToHARHeaders(req.Headers),
				QueryString: urlToHARQuery(req.URL),
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
				StatusText:  statusText(resp.Status),
				HTTPVersion: "HTTP/1.1",
				Headers:     mapToHARHeaders(resp.Headers),
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

func mapToHARHeaders(m map[string]string) []HARHeader {
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

func urlToHARQuery(rawURL string) []HARQuery {
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

func statusText(code int) string {
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
