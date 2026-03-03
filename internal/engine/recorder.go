package engine

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/hijack"
	"github.com/inovacc/scout/pkg/scout/rod/lib/proto"
)

// Re-export recorder option types.
type RecorderOption = hijack.RecorderOption

var (
	WithCaptureBody = hijack.WithCaptureBody
	WithCreatorName = hijack.WithCreatorName
)

// NetworkRecorder captures HTTP traffic from a Page via CDP events and exports HAR 1.2 logs.
type NetworkRecorder struct {
	mu         sync.Mutex
	entries    []HAREntry
	pending    map[proto.NetworkRequestID]*HAREntry
	startTimes map[proto.NetworkRequestID]time.Time
	opts       *hijack.RecorderOptions
	page       *Page
	stopCh     chan struct{}
	stopped    bool
}

// NewNetworkRecorder creates a recorder that immediately begins capturing network traffic
// from the given page. Call Stop() to end recording.
func NewNetworkRecorder(page *Page, opts ...RecorderOption) *NetworkRecorder {
	if page == nil {
		return nil
	}

	o := hijack.RecorderDefaults()
	for _, fn := range opts {
		fn(o)
	}

	r := &NetworkRecorder{
		pending:    make(map[proto.NetworkRequestID]*HAREntry),
		startTimes: make(map[proto.NetworkRequestID]time.Time),
		opts:       o,
		page:       page,
		stopCh:     make(chan struct{}),
	}
	r.start()

	return r
}

func (r *NetworkRecorder) start() {
	rodPage := r.page.RodPage()

	go rodPage.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			now := time.Now()
			headers := parseNetworkHeaders(e.Request.Headers)

			entry := &HAREntry{
				StartedDateTime: now.UTC().Format(time.RFC3339Nano),
				Request: HARRequest{
					Method:      e.Request.Method,
					URL:         e.Request.URL,
					HTTPVersion: "HTTP/1.1",
					Headers:     headers,
					HeadersSize: -1,
					BodySize:    -1,
				},
			}

			if e.Request.HasPostData {
				entry.Request.PostData = &HARPost{
					Text: e.Request.PostData,
				}
				entry.Request.BodySize = len(e.Request.PostData)
			}

			r.mu.Lock()
			r.pending[e.RequestID] = entry
			r.startTimes[e.RequestID] = now
			r.mu.Unlock()
		},

		func(e *proto.NetworkResponseReceived) {
			r.mu.Lock()
			entry, ok := r.pending[e.RequestID]
			startTime := r.startTimes[e.RequestID]
			r.mu.Unlock()

			if !ok {
				return
			}

			respHeaders := parseNetworkHeaders(e.Response.Headers)

			entry.Response = HARResponse{
				Status:      e.Response.Status,
				StatusText:  e.Response.StatusText,
				HTTPVersion: e.Response.Protocol,
				Headers:     respHeaders,
				Content: HARContent{
					MimeType: e.Response.MIMEType,
				},
				HeadersSize: -1,
				BodySize:    -1,
			}
			entry.ServerIPAddress = e.Response.RemoteIPAddress

			if t := e.Response.Timing; t != nil {
				entry.Timings = HARTimings{
					Blocked: t.DNSStart,
					DNS:     t.DNSEnd - t.DNSStart,
					Connect: t.ConnectEnd - t.ConnectStart,
					SSL:     t.SslEnd - t.SslStart,
					Send:    t.SendEnd - t.SendStart,
					Wait:    t.ReceiveHeadersEnd - t.SendEnd,
					Receive: 0, // filled on LoadingFinished
				}
			}

			entry.Time = float64(time.Since(startTime).Milliseconds())
		},

		func(e *proto.NetworkLoadingFinished) {
			r.mu.Lock()
			entry, ok := r.pending[e.RequestID]

			startTime := r.startTimes[e.RequestID]
			if ok {
				delete(r.pending, e.RequestID)
				delete(r.startTimes, e.RequestID)
			}

			r.mu.Unlock()

			if !ok {
				return
			}

			entry.Time = float64(time.Since(startTime).Milliseconds())

			if r.opts.CaptureBody {
				body, err := proto.NetworkGetResponseBody{
					RequestID: e.RequestID,
				}.Call(r.page.RodPage())
				if err == nil && body != nil {
					entry.Response.Content.Text = body.Body

					entry.Response.Content.Size = len(body.Body)
					if body.Base64Encoded {
						entry.Response.Content.Encoding = "base64"
					}
				}
			}

			r.mu.Lock()
			r.entries = append(r.entries, *entry)
			r.mu.Unlock()
		},

		func(e *proto.NetworkLoadingFailed) {
			r.mu.Lock()

			entry, ok := r.pending[e.RequestID]
			if ok {
				delete(r.pending, e.RequestID)
				delete(r.startTimes, e.RequestID)
			}

			r.mu.Unlock()

			if !ok {
				return
			}

			entry.Response = HARResponse{
				Status:     0,
				StatusText: e.ErrorText,
				Content: HARContent{
					MimeType: "x-unknown",
				},
			}

			r.mu.Lock()
			r.entries = append(r.entries, *entry)
			r.mu.Unlock()
		},
	)()
}

// Stop ends the recording. It is safe to call multiple times.
func (r *NetworkRecorder) Stop() {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.stopped {
		r.stopped = true
		close(r.stopCh)
	}
}

// ExportHAR returns the recorded traffic as a HAR 1.2 JSON document.
// The second return value is the number of entries.
func (r *NetworkRecorder) ExportHAR() ([]byte, int, error) {
	if r == nil {
		return nil, 0, fmt.Errorf("scout: export har: nil recorder")
	}

	r.mu.Lock()
	entries := make([]HAREntry, len(r.entries))
	copy(entries, r.entries)
	r.mu.Unlock()

	log := struct {
		Log HARLog `json:"log"`
	}{
		Log: HARLog{
			Version: "1.2",
			Creator: HARCreator{
				Name:    r.opts.CreatorName,
				Version: r.opts.CreatorVersion,
			},
			Entries: entries,
		},
	}

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, 0, fmt.Errorf("scout: export har: %w", err)
	}

	return data, len(entries), nil
}

// Entries returns a copy of the recorded HAR entries.
func (r *NetworkRecorder) Entries() []HAREntry {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	entries := make([]HAREntry, len(r.entries))
	copy(entries, r.entries)
	r.mu.Unlock()

	return entries
}

// Clear removes all recorded entries.
func (r *NetworkRecorder) Clear() {
	if r == nil {
		return
	}

	r.mu.Lock()
	r.entries = nil
	r.pending = make(map[proto.NetworkRequestID]*HAREntry)
	r.startTimes = make(map[proto.NetworkRequestID]time.Time)
	r.mu.Unlock()
}

// parseNetworkHeaders converts CDP NetworkHeaders to a HAR header slice.
func parseNetworkHeaders(h proto.NetworkHeaders) []HARHeader {
	if h == nil {
		return nil
	}

	headers := make([]HARHeader, 0, len(h))
	for k, v := range h {
		headers = append(headers, HARHeader{Name: k, Value: v.String()})
	}

	return headers
}
