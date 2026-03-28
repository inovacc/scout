package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inovacc/scout/internal/metrics"
)

func TestServerMetricsOnCall(t *testing.T) {
	metrics.Reset()

	s := &Server{
		provider: &Provider{tools: []Tool{
			{
				Name:        "test_nav",
				Description: "test",
				Parameters:  emptyParams(),
				Handler: func(_ context.Context, _ map[string]any) (string, error) {
					return "ok", nil
				},
			},
		}},
		mux:    http.NewServeMux(),
		logger: slog.Default(),
	}
	s.registerRoutes()

	body := bytes.NewBufferString(`{"name":"test_nav","arguments":{}}`)
	req := httptest.NewRequest("POST", "/call", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if got := metrics.Get().ToolCallsTotal.Load(); got != 1 {
		t.Errorf("ToolCallsTotal = %d, want 1", got)
	}
}

func TestServerMetricsOnError(t *testing.T) {
	metrics.Reset()

	s := &Server{
		provider: &Provider{tools: []Tool{
			{
				Name:        "fail_tool",
				Description: "always fails",
				Parameters:  emptyParams(),
				Handler: func(_ context.Context, _ map[string]any) (string, error) {
					return "", fmt.Errorf("boom")
				},
			},
		}},
		mux:    http.NewServeMux(),
		logger: slog.Default(),
	}
	s.registerRoutes()

	body := bytes.NewBufferString(`{"name":"fail_tool","arguments":{}}`)
	req := httptest.NewRequest("POST", "/call", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// provider.Call wraps handler errors into ToolResult{IsError: true} (not a Go error),
	// so handleCall reaches the success path and increments ToolCallsTotal.
	if got := metrics.Get().ToolCallsTotal.Load(); got != 1 {
		t.Errorf("ToolCallsTotal = %d after error, want 1", got)
	}
	if got := metrics.Get().ErrorsTotal.Load(); got != 1 {
		t.Errorf("ErrorsTotal = %d after error, want 1", got)
	}
}

func TestServerMetricsEndpoint(t *testing.T) {
	metrics.Reset()
	metrics.Get().ToolCallsTotal.Add(42)

	s := &Server{
		provider: &Provider{},
		mux:      http.NewServeMux(),
	}
	s.registerRoutes()

	// Test Prometheus endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /metrics status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "scout_tool_calls_total 42") {
		t.Errorf("Prometheus output missing tool_calls_total 42:\n%s", w.Body.String())
	}

	// Test JSON endpoint
	req = httptest.NewRequest("GET", "/metrics/json", nil)
	w = httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /metrics/json status = %d", w.Code)
	}

	var data map[string]any
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if v := data["tool_calls_total"].(float64); v != 42 {
		t.Errorf("JSON tool_calls_total = %v, want 42", v)
	}
}

func TestServerMultipleCallsAccumulate(t *testing.T) {
	metrics.Reset()

	s := &Server{
		provider: &Provider{tools: []Tool{
			{
				Name:       "ping",
				Parameters: emptyParams(),
				Handler: func(_ context.Context, _ map[string]any) (string, error) {
					return "pong", nil
				},
			},
		}},
		mux:    http.NewServeMux(),
		logger: slog.Default(),
	}
	s.registerRoutes()

	for i := 0; i < 5; i++ {
		body := bytes.NewBufferString(`{"name":"ping","arguments":{}}`)
		req := httptest.NewRequest("POST", "/call", body)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
	}

	if got := metrics.Get().ToolCallsTotal.Load(); got != 5 {
		t.Errorf("ToolCallsTotal after 5 calls = %d, want 5", got)
	}
}
