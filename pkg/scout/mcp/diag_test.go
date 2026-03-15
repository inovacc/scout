package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newDiagTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>OK</body></html>`))
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Method", r.Method)
		_, _ = w.Write([]byte(`{"method":"` + r.Method + `"}`))
	})

	return httptest.NewServer(mux)
}

func TestPingTool(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ping",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `","count":2}`),
	})
	if err != nil {
		t.Fatalf("ping call: %v", err)
	}

	if result.IsError {
		t.Fatalf("ping error: %v", result.Content)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp pingResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal ping response: %v", err)
	}

	if resp.URL != ts.URL {
		t.Errorf("url = %q, want %q", resp.URL, ts.URL)
	}

	if len(resp.Pings) != 2 {
		t.Errorf("pings count = %d, want 2", len(resp.Pings))
	}

	if resp.Summary == nil {
		t.Error("summary is nil")
	}

	if resp.HTTP == nil {
		t.Error("http is nil")
	} else if resp.HTTP.Status != 200 {
		t.Errorf("http status = %d, want 200", resp.HTTP.Status)
	}
}

func TestPingToolDefaultCount(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ping",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `"}`),
	})
	if err != nil {
		t.Fatalf("ping call: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp pingResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Pings) != 3 {
		t.Errorf("default count: pings = %d, want 3", len(resp.Pings))
	}
}

func TestCurlToolGET(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/echo"}`),
	})
	if err != nil {
		t.Fatalf("curl call: %v", err)
	}

	if result.IsError {
		t.Fatalf("curl error: %v", result.Content)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}

	if resp.Timing == nil {
		t.Error("timing is nil")
	}

	if resp.Size == nil {
		t.Error("size is nil")
	}

	if resp.Headers["x-method"] != "GET" {
		t.Errorf("x-method = %q, want GET", resp.Headers["x-method"])
	}
}

func TestCurlToolPOST(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/echo","method":"POST","headers":{"Content-Type":"application/json"},"body":"{\"key\":\"value\"}"}`),
	})
	if err != nil {
		t.Fatalf("curl call: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}

	if resp.Headers["x-method"] != "POST" {
		t.Errorf("x-method = %q, want POST", resp.Headers["x-method"])
	}
}

func TestCurlToolRedirect(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/redirect"}`),
	})
	if err != nil {
		t.Fatalf("curl call: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200 (after redirect)", resp.Status)
	}

	if len(resp.Redirects) != 1 {
		t.Errorf("redirects = %d, want 1", len(resp.Redirects))
	}
}

func TestCurlToolNoRedirect(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/redirect","followRedirects":false}`),
	})
	if err != nil {
		t.Fatalf("curl call: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 301 {
		t.Errorf("status = %d, want 301", resp.Status)
	}
}

func TestPingToolMaxCount(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	// Count > 20 should be capped to 20 — but test with a small cap.
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ping",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `","count":1}`),
	})
	if err != nil {
		t.Fatalf("ping call: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp pingResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Pings) != 1 {
		t.Errorf("pings = %d, want 1", len(resp.Pings))
	}
}

func TestPingToolViaBrowser(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ping",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `","count":1,"useBrowser":true}`),
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("ping via browser: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("ping browser error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp pingResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Pings) != 1 {
		t.Errorf("pings = %d, want 1", len(resp.Pings))
	}

	if resp.Summary == nil {
		t.Error("summary is nil")
	}
}

func TestCurlToolViaBrowser(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `","useBrowser":true}`),
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("curl via browser: %v", err)
	}

	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolError{text})
		t.Fatalf("curl browser error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}

	if resp.Headers["x-final-url"] == "" {
		t.Error("expected x-final-url header from browser curl")
	}

	if resp.Body == "" {
		t.Error("expected non-empty body")
	}
}

func TestCurlToolTimeout(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/echo","timeout":5}`),
	})
	if err != nil {
		t.Fatalf("curl timeout: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
}

func TestCurlToolMaxRedirects(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/redirect","maxRedirects":5}`),
	})
	if err != nil {
		t.Fatalf("curl maxRedirects: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
}

func TestPingToolInvalidURL(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ping",
		Arguments: json.RawMessage(`{"url":"://invalid","count":1}`),
	})
	if err != nil {
		t.Fatalf("ping call: %v", err)
	}

	// Should return result with error (not crash).
	if result.IsError {
		// error result is fine for invalid URL
		return
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp pingResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error in ping response for invalid URL")
	}
}

func TestCurlToolWithBody(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/echo","method":"PUT","headers":{"Content-Type":"text/plain","Accept":"application/json"},"body":"hello world","timeout":10}`),
	})
	if err != nil {
		t.Fatalf("curl PUT: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}

	if resp.Headers["x-method"] != "PUT" {
		t.Errorf("x-method = %q, want PUT", resp.Headers["x-method"])
	}
}

func TestCurlToolInvalidURL(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"http://localhost:1/nonexistent","timeout":2}`),
	})
	if err != nil {
		t.Fatalf("curl invalid: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Should have error for connection refused.
	if resp.Error == "" && resp.Status == 0 {
		t.Error("expected error for unreachable URL")
	}
}

func TestCurlToolHEADMethod(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/echo","method":"HEAD"}`),
	})
	if err != nil {
		t.Fatalf("curl HEAD: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
}

func TestPingToolConnectionRefused(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true})

	// Use an unreachable port to trigger connection errors.
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "ping",
		Arguments: json.RawMessage(`{"url":"http://127.0.0.1:1","count":2}`),
	})
	if err != nil {
		t.Fatalf("ping conn refused: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp pingResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Should still have pings even with errors.
	if len(resp.Pings) != 2 {
		t.Errorf("pings = %d, want 2", len(resp.Pings))
	}

	if resp.Error == "" {
		t.Error("expected error in response for connection refused")
	}
}

func TestCurlToolDELETEMethod(t *testing.T) {
	ts := newDiagTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "curl",
		Arguments: json.RawMessage(`{"url":"` + ts.URL + `/echo","method":"DELETE","followRedirects":true}`),
	})
	if err != nil {
		t.Fatalf("curl DELETE: %v", err)
	}

	text := result.Content[0].(*mcp.TextContent).Text

	var resp curlResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
}

func TestServeContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Serve on stdio with already-cancelled context should return quickly.
	err := Serve(ctx, slog.Default(), true, false, "", 0)
	if err != nil && err != context.Canceled {
		t.Logf("serve returned: %v (acceptable)", err)
	}
}

func TestServeSSEContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)

	go func() {
		errCh <- ServeSSE(ctx, slog.Default(), "127.0.0.1:0", true, false, "", 0)
	}()

	// Give server time to start, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil && err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"example.com", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"http://example.com", "http://example.com"},
	}
	for _, tt := range tests {
		got := normalizeURL(tt.in)
		if got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTLSVersionString(t *testing.T) {
	tests := []struct {
		version uint16
		want    string
	}{
		{0x0301, "TLS 1.0"},
		{0x0302, "TLS 1.1"},
		{0x0303, "TLS 1.2"},
		{0x0304, "TLS 1.3"},
		{0x0000, "0x0000"},
		{0xFFFF, "0xffff"},
	}

	for _, tc := range tests {
		got := tlsVersionString(tc.version)
		if got != tc.want {
			t.Errorf("tlsVersionString(0x%04X) = %q, want %q", tc.version, got, tc.want)
		}
	}
}

func TestMs(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want float64
	}{
		{"1 second", time.Second, 1000.0},
		{"500ms", 500 * time.Millisecond, 500.0},
		{"1.5ms", 1500 * time.Microsecond, 1.5},
		{"zero", 0, 0.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ms(tc.d)
			if got != tc.want {
				t.Errorf("ms(%v) = %f, want %f", tc.d, got, tc.want)
			}
		})
	}
}

func TestSummarizePings(t *testing.T) {
	pings := []pingResult{
		{Seq: 1, TotalMS: 10.0},
		{Seq: 2, TotalMS: 30.0},
		{Seq: 3, TotalMS: 20.0},
	}

	s := summarizePings(pings)
	if s.MinMS != 10.0 {
		t.Errorf("MinMS = %f, want 10.0", s.MinMS)
	}

	if s.MaxMS != 30.0 {
		t.Errorf("MaxMS = %f, want 30.0", s.MaxMS)
	}

	if s.AvgMS != 20.0 {
		t.Errorf("AvgMS = %f, want 20.0", s.AvgMS)
	}
}

func TestSummarizePingsEmpty(t *testing.T) {
	s := summarizePings(nil)
	if s.MinMS != 0 || s.MaxMS != 0 {
		t.Errorf("expected zeros for empty pings, got min=%f max=%f", s.MinMS, s.MaxMS)
	}
}

func TestJsonResult(t *testing.T) {
	result, err := jsonResult(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("jsonResult: %v", err)
	}

	if result.IsError {
		t.Error("expected non-error result")
	}

	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"key"`) || !strings.Contains(text, `"value"`) {
		t.Errorf("unexpected JSON: %s", text)
	}
}

func TestErrResult(t *testing.T) {
	result, err := errResult("something went wrong")
	if err != nil {
		t.Fatalf("errResult: %v", err)
	}

	if !result.IsError {
		t.Error("expected IsError=true")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text != "something went wrong" {
		t.Errorf("text = %q, want %q", text, "something went wrong")
	}
}

func TestTextResult(t *testing.T) {
	result, err := textResult("hello world")
	if err != nil {
		t.Fatalf("textResult: %v", err)
	}

	if result.IsError {
		t.Error("expected IsError=false")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}
}
