package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

// newTestServer creates a Server with a manually-constructed Provider (no browser needed).
func newTestServer(tools ...Tool) *Server {
	s := &Server{
		provider: &Provider{tools: tools},
		logger:   slog.Default(),
		mux:      http.NewServeMux(),
		limiter:  rate.NewLimiter(rate.Limit(100), 100),
	}
	s.registerRoutes()
	return s
}

func TestServerHealth(t *testing.T) {
	s := newTestServer(
		Tool{Name: "test_tool", Description: "A test tool"},
	)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health status = %d, want 200", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
	if resp.Tools != 1 {
		t.Errorf("tools = %d, want 1", resp.Tools)
	}
}

func TestServerToolsOpenAI(t *testing.T) {
	s := newTestServer(
		Tool{Name: "navigate", Description: "Go to URL", Parameters: emptyParams()},
	)

	req := httptest.NewRequest("GET", "/tools", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /tools status = %d, want 200", w.Code)
	}

	var tools []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Errorf("got %d tools, want 1", len(tools))
	}
}

func TestServerToolsAnthropic(t *testing.T) {
	s := newTestServer(
		Tool{Name: "navigate", Description: "Go to URL", Parameters: emptyParams()},
	)

	req := httptest.NewRequest("GET", "/tools/anthropic", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var tools []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&tools); err != nil {
		t.Fatal(err)
	}
	if tools[0]["name"] != "navigate" {
		t.Errorf("tool name = %v, want navigate", tools[0]["name"])
	}
	if _, ok := tools[0]["input_schema"]; !ok {
		t.Error("Anthropic format should have input_schema")
	}
}

func TestServerCallMissingName(t *testing.T) {
	s := newTestServer()

	body := bytes.NewBufferString(`{"arguments":{}}`)
	req := httptest.NewRequest("POST", "/call", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /call without name: status = %d, want 400", w.Code)
	}
}

func TestServerCallUnknownTool(t *testing.T) {
	s := newTestServer()

	body := bytes.NewBufferString(`{"name":"nonexistent","arguments":{}}`)
	req := httptest.NewRequest("POST", "/call", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("POST /call unknown tool: status = %d, want 404", w.Code)
	}
}

func TestServerCallInvalidJSON(t *testing.T) {
	s := newTestServer()

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("POST", "/call", body)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /call invalid JSON: status = %d, want 400", w.Code)
	}
}

func TestServerCallSuccess(t *testing.T) {
	s := newTestServer(Tool{
		Name:        "echo",
		Description: "Echo back",
		Parameters:  emptyParams(),
		Handler: func(_ context.Context, _ map[string]any) (string, error) {
			return "hello", nil
		},
	})

	body := bytes.NewBufferString(`{"name":"echo","arguments":{}}`)
	req := httptest.NewRequest("POST", "/call", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /call status = %d, want 200", w.Code)
	}

	var resp ToolResult
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello" {
		t.Errorf("content = %q, want hello", resp.Content)
	}
	if resp.IsError {
		t.Error("expected is_error = false")
	}
}

func TestServerCallHandlerError(t *testing.T) {
	s := newTestServer(Tool{
		Name:        "fail",
		Description: "Always fails",
		Parameters:  emptyParams(),
		Handler: func(_ context.Context, _ map[string]any) (string, error) {
			return "", context.DeadlineExceeded
		},
	})

	body := bytes.NewBufferString(`{"name":"fail","arguments":{}}`)
	req := httptest.NewRequest("POST", "/call", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// Provider.Call wraps handler errors as ToolResult with IsError=true and nil error,
	// so the server returns 200 with an error result.
	if w.Code != http.StatusOK {
		t.Errorf("POST /call handler error: status = %d, want 200", w.Code)
	}

	var resp ToolResult
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Error("expected is_error = true for handler error")
	}
	if resp.Content == "" {
		t.Error("expected error message in content")
	}
}

func TestServerToolsSchema(t *testing.T) {
	s := newTestServer(
		Tool{Name: "nav", Description: "Navigate", Parameters: emptyParams()},
	)

	req := httptest.NewRequest("GET", "/tools/schema", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /tools/schema status = %d, want 200", w.Code)
	}

	var schema []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&schema); err != nil {
		t.Fatal(err)
	}
	if len(schema) != 1 {
		t.Errorf("got %d schema entries, want 1", len(schema))
	}
}

func TestServerHealthMultipleTools(t *testing.T) {
	s := newTestServer(
		Tool{Name: "a", Description: "Tool A"},
		Tool{Name: "b", Description: "Tool B"},
		Tool{Name: "c", Description: "Tool C"},
	)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Tools != 3 {
		t.Errorf("tools = %d, want 3", resp.Tools)
	}
}

func TestCORSHeaders(t *testing.T) {
	s := newTestServer(Tool{Name: "t", Description: "test"})
	handler := corsMiddleware(s.rateLimitMiddleware(s.mux))

	t.Run("origin echoed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
			t.Errorf("Access-Control-Allow-Origin = %q, want https://example.com", got)
		}
		if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
			t.Errorf("Access-Control-Allow-Methods = %q, want 'GET, POST, OPTIONS'", got)
		}
	})

	t.Run("no origin no headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin should be empty without Origin, got %q", got)
		}
	})

	t.Run("preflight OPTIONS returns 204", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/call", nil)
		req.Header.Set("Origin", "https://app.test")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("OPTIONS status = %d, want 204", w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.test" {
			t.Errorf("Access-Control-Allow-Origin = %q, want https://app.test", got)
		}
	})
}

func TestRateLimiting(t *testing.T) {
	s := &Server{
		provider: &Provider{tools: []Tool{{Name: "t", Description: "test"}}},
		logger:   slog.Default(),
		mux:      http.NewServeMux(),
		limiter:  rate.NewLimiter(rate.Limit(5), 5), // 5 req/s, burst 5
	}
	s.registerRoutes()
	handler := corsMiddleware(s.rateLimitMiddleware(s.mux))

	// Use up all burst tokens.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, w.Code)
		}
	}

	// Next request should be rate limited.
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit status = %d, want 429", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "rate limit exceeded" {
		t.Errorf("error = %q, want 'rate limit exceeded'", resp["error"])
	}
}

// newTestServerWithAPIKey creates a test server with an API key configured.
func newTestServerWithAPIKey(apiKey string, tools ...Tool) *Server {
	s := &Server{
		provider: &Provider{tools: tools},
		config:   ServerConfig{APIKey: apiKey},
		logger:   slog.Default(),
		mux:      http.NewServeMux(),
		limiter:  rate.NewLimiter(rate.Limit(100), 100),
	}
	s.registerRoutes()
	return s
}

func TestAuthMiddlewareNoKey(t *testing.T) {
	s := newTestServer(Tool{Name: "t", Description: "test"})
	handler := corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux)))

	req := httptest.NewRequest("GET", "/tools", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no API key configured: status = %d, want 200", w.Code)
	}
}

func TestAuthMiddlewareValidKey(t *testing.T) {
	s := newTestServerWithAPIKey("secret-key-123", Tool{Name: "t", Description: "test", Parameters: emptyParams()})
	handler := corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux)))

	req := httptest.NewRequest("GET", "/tools", nil)
	req.Header.Set("Authorization", "Bearer secret-key-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid API key: status = %d, want 200", w.Code)
	}
}

func TestAuthMiddlewareInvalidKey(t *testing.T) {
	s := newTestServerWithAPIKey("secret-key-123", Tool{Name: "t", Description: "test"})
	handler := corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux)))

	req := httptest.NewRequest("GET", "/tools", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid API key: status = %d, want 401", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "invalid API key" {
		t.Errorf("error = %q, want 'invalid API key'", resp["error"])
	}
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	s := newTestServerWithAPIKey("secret-key-123", Tool{Name: "t", Description: "test"})
	handler := corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux)))

	req := httptest.NewRequest("GET", "/tools", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing header: status = %d, want 401", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "missing Authorization header" {
		t.Errorf("error = %q, want 'missing Authorization header'", resp["error"])
	}
}

func TestAuthMiddlewareHealthBypass(t *testing.T) {
	s := newTestServerWithAPIKey("secret-key-123", Tool{Name: "t", Description: "test"})
	handler := corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux)))

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/health bypass: status = %d, want 200", w.Code)
	}
}

func TestAuthMiddlewareMetricsBypass(t *testing.T) {
	s := newTestServerWithAPIKey("secret-key-123", Tool{Name: "t", Description: "test"})
	handler := corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux)))

	for _, path := range []string{"/metrics", "/metrics/json"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s bypass: status = %d, want 200", path, w.Code)
		}
	}
}
