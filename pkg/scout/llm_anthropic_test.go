package scout

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicProviderName(t *testing.T) {
	p := &AnthropicProvider{}
	if got := p.Name(); got != "anthropic" {
		t.Errorf("Name() = %q, want %q", got, "anthropic")
	}
}

func TestAnthropicProviderRequiresKey(t *testing.T) {
	_, err := NewAnthropicProvider()
	if err == nil {
		t.Fatal("expected error when no API key provided")
	}
}

func TestAnthropicProviderComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q, want %q", r.Header.Get("x-api-key"), "test-key")
		}
		if r.Header.Get("anthropic-version") != AnthropicAPIVersion {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}

		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if req.Model != "claude-test" {
			t.Errorf("model = %q, want %q", req.Model, "claude-test")
		}
		if req.System != "system prompt" {
			t.Errorf("system = %q", req.System)
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
			t.Errorf("messages unexpected: %+v", req.Messages)
		}

		resp := anthropicResponse{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "reviewed output"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewAnthropicProvider(
		WithAnthropicBaseURL(srv.URL),
		WithAnthropicKey("test-key"),
		WithAnthropicModel("claude-test"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := p.Complete(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if result != "reviewed output" {
		t.Errorf("result = %q, want %q", result, "reviewed output")
	}
}

func TestAnthropicProviderHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid key"}}`))
	}))
	defer srv.Close()

	p, err := NewAnthropicProvider(
		WithAnthropicBaseURL(srv.URL),
		WithAnthropicKey("bad-key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error on HTTP 401")
	}
}

func TestAnthropicOptions(t *testing.T) {
	o := defaultAnthropicOptions()

	WithAnthropicBaseURL("https://custom.anthropic.com")(o)
	if o.baseURL != "https://custom.anthropic.com" {
		t.Errorf("baseURL = %q", o.baseURL)
	}

	WithAnthropicKey("my-key")(o)
	if o.apiKey != "my-key" {
		t.Errorf("apiKey = %q", o.apiKey)
	}

	WithAnthropicModel("claude-opus-4-20250514")(o)
	if o.model != "claude-opus-4-20250514" {
		t.Errorf("model = %q", o.model)
	}
}
