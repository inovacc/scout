package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
