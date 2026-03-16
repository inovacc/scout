package hijack

import (
	"encoding/json"
	"testing"
	"time"
)

// --- EventType tests ---

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  EventType
		want string
	}{
		{"EventRequest", EventRequest, "request"},
		{"EventResponse", EventResponse, "response"},
		{"WSSent", WSSent, "ws.sent"},
		{"WSReceived", WSReceived, "ws.received"},
		{"WSOpened", WSOpened, "ws.opened"},
		{"WSClosed", WSClosed, "ws.closed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.got) != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// --- Defaults tests ---

func TestDefaults(t *testing.T) {
	opts := Defaults()
	if opts.ChannelSize != 1024 {
		t.Errorf("ChannelSize = %d, want 1024", opts.ChannelSize)
	}
	if opts.Filter.CaptureBody {
		t.Error("CaptureBody should default to false")
	}
	if len(opts.Filter.URLPatterns) != 0 {
		t.Error("URLPatterns should be empty by default")
	}
}

// --- Option tests ---

func TestWithURLFilter(t *testing.T) {
	opts := Defaults()
	WithURLFilter("*.js", "*.css")(opts)
	if len(opts.Filter.URLPatterns) != 2 {
		t.Fatalf("got %d patterns, want 2", len(opts.Filter.URLPatterns))
	}
	if opts.Filter.URLPatterns[0] != "*.js" {
		t.Errorf("pattern[0] = %q, want *.js", opts.Filter.URLPatterns[0])
	}
	// Append more
	WithURLFilter("*.html")(opts)
	if len(opts.Filter.URLPatterns) != 3 {
		t.Fatalf("got %d patterns, want 3", len(opts.Filter.URLPatterns))
	}
}

func TestWithBodyCapture(t *testing.T) {
	opts := Defaults()
	WithBodyCapture()(opts)
	if !opts.Filter.CaptureBody {
		t.Error("CaptureBody should be true after WithBodyCapture")
	}
}

func TestWithChannelSize(t *testing.T) {
	opts := Defaults()
	WithChannelSize(42)(opts)
	if opts.ChannelSize != 42 {
		t.Errorf("ChannelSize = %d, want 42", opts.ChannelSize)
	}
}

// --- RecorderOption tests ---

func TestRecorderDefaults(t *testing.T) {
	opts := RecorderDefaults()
	if opts.CreatorName != "scout" {
		t.Errorf("CreatorName = %q, want scout", opts.CreatorName)
	}
	if opts.CreatorVersion != "0.1.0" {
		t.Errorf("CreatorVersion = %q, want 0.1.0", opts.CreatorVersion)
	}
	if opts.CaptureBody {
		t.Error("CaptureBody should default to false")
	}
}

func TestWithCaptureBody(t *testing.T) {
	opts := RecorderDefaults()
	WithCaptureBody(true)(opts)
	if !opts.CaptureBody {
		t.Error("CaptureBody should be true")
	}
	WithCaptureBody(false)(opts)
	if opts.CaptureBody {
		t.Error("CaptureBody should be false")
	}
}

func TestWithCreatorName(t *testing.T) {
	opts := RecorderDefaults()
	WithCreatorName("test-tool", "2.0.0")(opts)
	if opts.CreatorName != "test-tool" {
		t.Errorf("CreatorName = %q, want test-tool", opts.CreatorName)
	}
	if opts.CreatorVersion != "2.0.0" {
		t.Errorf("CreatorVersion = %q, want 2.0.0", opts.CreatorVersion)
	}
}

// --- StatusText tests ---

func TestStatusText(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{405, "Method Not Allowed"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{999, "999"}, // unknown code
		{0, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := StatusText(tt.code)
			if got != tt.want {
				t.Errorf("StatusText(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// --- MapToHARHeaders tests ---

func TestMapToHARHeaders(t *testing.T) {
	t.Run("nil map", func(t *testing.T) {
		got := MapToHARHeaders(nil)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		got := MapToHARHeaders(map[string]string{})
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("sorted output", func(t *testing.T) {
		m := map[string]string{
			"Content-Type": "text/html",
			"Accept":       "*/*",
			"Zebra":        "yes",
		}
		got := MapToHARHeaders(m)
		if len(got) != 3 {
			t.Fatalf("got %d headers, want 3", len(got))
		}
		if got[0].Name != "Accept" {
			t.Errorf("first header = %q, want Accept", got[0].Name)
		}
		if got[1].Name != "Content-Type" {
			t.Errorf("second header = %q, want Content-Type", got[1].Name)
		}
		if got[2].Name != "Zebra" {
			t.Errorf("third header = %q, want Zebra", got[2].Name)
		}
	})
}

// --- URLToHARQuery tests ---

func TestURLToHARQuery(t *testing.T) {
	t.Run("no query", func(t *testing.T) {
		got := URLToHARQuery("https://example.com/path")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		got := URLToHARQuery("://bad")
		if got != nil {
			t.Errorf("expected nil for invalid URL, got %v", got)
		}
	})

	t.Run("single param", func(t *testing.T) {
		got := URLToHARQuery("https://example.com?foo=bar")
		if len(got) != 1 {
			t.Fatalf("got %d params, want 1", len(got))
		}
		if got[0].Name != "foo" || got[0].Value != "bar" {
			t.Errorf("got %+v, want foo=bar", got[0])
		}
	})

	t.Run("multiple params", func(t *testing.T) {
		got := URLToHARQuery("https://example.com?a=1&b=2&a=3")
		if len(got) != 3 {
			t.Fatalf("got %d params, want 3", len(got))
		}
	})

	t.Run("empty query value", func(t *testing.T) {
		got := URLToHARQuery("https://example.com?key=")
		if len(got) != 1 {
			t.Fatalf("got %d params, want 1", len(got))
		}
		if got[0].Value != "" {
			t.Errorf("value = %q, want empty", got[0].Value)
		}
	})
}

// --- Recorder tests ---

func TestNewRecorder(t *testing.T) {
	r := NewRecorder()
	if r == nil {
		t.Fatal("NewRecorder returned nil")
	}
	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0", r.Len())
	}
}

func TestRecorderRecordRequest(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "GET",
			URL:       "https://example.com",
			Timestamp: now,
		},
	})

	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1", r.Len())
	}
}

func TestRecorderRecordResponse(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	// Record request first
	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "GET",
			URL:       "https://example.com",
			Timestamp: now,
		},
	})

	// Record matching response
	r.Record(Event{
		Type: EventResponse,
		Response: &CapturedResponse{
			RequestID: "req-1",
			URL:       "https://example.com",
			Status:    200,
			ElapsedMs: 42.5,
			Timestamp: now.Add(42 * time.Millisecond),
		},
	})

	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1", r.Len())
	}
}

func TestRecorderIgnoresNilPayloads(t *testing.T) {
	r := NewRecorder()
	r.Record(Event{Type: EventRequest, Request: nil})
	r.Record(Event{Type: EventResponse, Response: nil})
	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0", r.Len())
	}
}

func TestRecorderIgnoresZeroStatusResponse(t *testing.T) {
	r := NewRecorder()
	r.Record(Event{
		Type: EventResponse,
		Response: &CapturedResponse{
			RequestID: "req-1",
			Status:    0,
		},
	})
	// Response with status 0 is not stored in responses map
	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if data == nil {
		t.Error("data should not be nil")
	}
}

func TestRecorderResponseBodyStored(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "GET",
			URL:       "https://example.com",
			Timestamp: now,
		},
	})

	r.Record(Event{
		Type: EventResponse,
		Response: &CapturedResponse{
			RequestID: "req-1",
			Status:    200,
			Body:      "hello world",
			MimeType:  "text/plain",
			Timestamp: now,
		},
	})

	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	var harDoc struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &harDoc); err != nil {
		t.Fatal(err)
	}
	if harDoc.Log.Entries[0].Response.Content.Text != "hello world" {
		t.Errorf("body = %q, want hello world", harDoc.Log.Entries[0].Response.Content.Text)
	}
	if harDoc.Log.Entries[0].Response.Content.Size != 11 {
		t.Errorf("size = %d, want 11", harDoc.Log.Entries[0].Response.Content.Size)
	}
}

func TestRecorderDuplicateRequestIDNotDuplicated(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "GET",
			URL:       "https://example.com/v1",
			Timestamp: now,
		},
	})

	// Same request ID again (updates the request, does not add to order)
	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "POST",
			URL:       "https://example.com/v2",
			Timestamp: now,
		},
	})

	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1 (duplicate should not add)", r.Len())
	}
}

func TestRecorderWebSocketEventsIgnored(t *testing.T) {
	r := NewRecorder()
	r.Record(Event{
		Type: WSSent,
		Frame: &WebSocketFrame{
			RequestID: "ws-1",
			Payload:   "hello",
		},
	})
	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0 (ws events don't create entries)", r.Len())
	}
}

func TestRecorderRecordAll(t *testing.T) {
	r := NewRecorder()
	ch := make(chan Event, 3)
	now := time.Now()

	ch <- Event{Type: EventRequest, Request: &CapturedRequest{RequestID: "r1", Method: "GET", URL: "https://a.com", Timestamp: now}}
	ch <- Event{Type: EventRequest, Request: &CapturedRequest{RequestID: "r2", Method: "GET", URL: "https://b.com", Timestamp: now}}
	ch <- Event{Type: EventResponse, Response: &CapturedResponse{RequestID: "r1", Status: 200}}
	close(ch)

	r.RecordAll(ch)
	if r.Len() != 2 {
		t.Errorf("Len() = %d, want 2", r.Len())
	}
}

// --- ExportHAR tests ---

func TestExportHAREmpty(t *testing.T) {
	r := NewRecorder()
	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	var harDoc struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &harDoc); err != nil {
		t.Fatal(err)
	}
	if harDoc.Log.Version != "1.2" {
		t.Errorf("version = %q, want 1.2", harDoc.Log.Version)
	}
	if harDoc.Log.Creator.Name != "scout-hijack" {
		t.Errorf("creator = %q, want scout-hijack", harDoc.Log.Creator.Name)
	}
	if len(harDoc.Log.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(harDoc.Log.Entries))
	}
}

func TestExportHARWithRequestAndResponse(t *testing.T) {
	r := NewRecorder()
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "POST",
			URL:       "https://example.com/api?q=test",
			Headers:   map[string]string{"Content-Type": "application/json"},
			Body:      `{"key":"value"}`,
			Timestamp: now,
		},
	})

	r.Record(Event{
		Type: EventResponse,
		Response: &CapturedResponse{
			RequestID: "req-1",
			URL:       "https://example.com/api?q=test",
			Status:    201,
			Headers:   map[string]string{"X-Custom": "yes"},
			MimeType:  "application/json",
			ElapsedMs: 100.5,
			Timestamp: now.Add(100 * time.Millisecond),
		},
	})

	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	var harDoc struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &harDoc); err != nil {
		t.Fatal(err)
	}

	entry := harDoc.Log.Entries[0]

	// Request checks
	if entry.Request.Method != "POST" {
		t.Errorf("method = %q, want POST", entry.Request.Method)
	}
	if entry.Request.HTTPVersion != "HTTP/1.1" {
		t.Errorf("httpVersion = %q, want HTTP/1.1", entry.Request.HTTPVersion)
	}
	if entry.Request.PostData == nil {
		t.Fatal("PostData should not be nil")
	}
	if entry.Request.PostData.MimeType != "application/json" {
		t.Errorf("PostData.MimeType = %q, want application/json", entry.Request.PostData.MimeType)
	}
	if entry.Request.PostData.Text != `{"key":"value"}` {
		t.Errorf("PostData.Text = %q", entry.Request.PostData.Text)
	}
	if entry.Request.BodySize != 15 {
		t.Errorf("BodySize = %d, want 15", entry.Request.BodySize)
	}

	// Query string
	if len(entry.Request.QueryString) != 1 {
		t.Fatalf("QueryString len = %d, want 1", len(entry.Request.QueryString))
	}
	if entry.Request.QueryString[0].Name != "q" || entry.Request.QueryString[0].Value != "test" {
		t.Errorf("QueryString[0] = %+v, want q=test", entry.Request.QueryString[0])
	}

	// Response checks
	if entry.Response.Status != 201 {
		t.Errorf("status = %d, want 201", entry.Response.Status)
	}
	if entry.Response.StatusText != "Created" {
		t.Errorf("statusText = %q, want Created", entry.Response.StatusText)
	}
	if entry.Time != 100.5 {
		t.Errorf("time = %f, want 100.5", entry.Time)
	}

	// Timings
	if entry.Timings.Wait != 100.5 {
		t.Errorf("timings.wait = %f, want 100.5", entry.Timings.Wait)
	}
	if entry.Timings.Blocked != -1 {
		t.Errorf("timings.blocked = %f, want -1", entry.Timings.Blocked)
	}
}

func TestExportHARRequestWithoutResponse(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "GET",
			URL:       "https://example.com",
			Timestamp: now,
		},
	})

	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	var harDoc struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &harDoc); err != nil {
		t.Fatal(err)
	}

	entry := harDoc.Log.Entries[0]
	if entry.Response.StatusText != "pending" {
		t.Errorf("statusText = %q, want pending", entry.Response.StatusText)
	}
	if entry.Response.Content.MimeType != "x-unknown" {
		t.Errorf("mimeType = %q, want x-unknown", entry.Response.Content.MimeType)
	}
}

func TestExportHARRequestWithPostDataContentTypeFallback(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	// Use lowercase content-type header
	r.Record(Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "req-1",
			Method:    "POST",
			URL:       "https://example.com",
			Headers:   map[string]string{"content-type": "text/plain"},
			Body:      "hello",
			Timestamp: now,
		},
	})

	data, _, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}

	var harDoc struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &harDoc); err != nil {
		t.Fatal(err)
	}

	if harDoc.Log.Entries[0].Request.PostData.MimeType != "text/plain" {
		t.Errorf("PostData.MimeType = %q, want text/plain", harDoc.Log.Entries[0].Request.PostData.MimeType)
	}
}

func TestExportHARPreservesOrder(t *testing.T) {
	r := NewRecorder()
	now := time.Now()

	for i, url := range []string{"https://first.com", "https://second.com", "https://third.com"} {
		r.Record(Event{
			Type: EventRequest,
			Request: &CapturedRequest{
				RequestID: string(rune('a' + i)),
				Method:    "GET",
				URL:       url,
				Timestamp: now.Add(time.Duration(i) * time.Second),
			},
		})
	}

	data, count, err := r.ExportHAR()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	var harDoc struct {
		Log HARLog `json:"log"`
	}
	if err := json.Unmarshal(data, &harDoc); err != nil {
		t.Fatal(err)
	}

	urls := []string{
		harDoc.Log.Entries[0].Request.URL,
		harDoc.Log.Entries[1].Request.URL,
		harDoc.Log.Entries[2].Request.URL,
	}
	want := []string{"https://first.com", "https://second.com", "https://third.com"}
	for i := range want {
		if urls[i] != want[i] {
			t.Errorf("entry[%d].URL = %q, want %q", i, urls[i], want[i])
		}
	}
}

// --- Event JSON serialization ---

func TestEventJSONRoundTrip(t *testing.T) {
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	ev := Event{
		Type: EventRequest,
		Request: &CapturedRequest{
			RequestID: "r1",
			Method:    "GET",
			URL:       "https://test.com",
			Timestamp: now,
		},
	}

	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Type != EventRequest {
		t.Errorf("Type = %q, want request", decoded.Type)
	}
	if decoded.Request.RequestID != "r1" {
		t.Errorf("RequestID = %q, want r1", decoded.Request.RequestID)
	}
	if decoded.Response != nil {
		t.Error("Response should be nil")
	}
	if decoded.Frame != nil {
		t.Error("Frame should be nil")
	}
}

// --- Filter struct ---

func TestFilterJSON(t *testing.T) {
	f := Filter{
		URLPatterns:   []string{"*.js"},
		ResourceTypes: []string{"script"},
		CaptureBody:   true,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Filter
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.URLPatterns) != 1 || decoded.URLPatterns[0] != "*.js" {
		t.Errorf("URLPatterns = %v", decoded.URLPatterns)
	}
	if !decoded.CaptureBody {
		t.Error("CaptureBody should be true")
	}
}
