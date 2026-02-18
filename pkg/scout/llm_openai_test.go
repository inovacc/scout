package scout

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProviderName(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		{OpenAIBaseURL, "openai"},
		{OpenRouterBaseURL, "openrouter"},
		{DeepSeekBaseURL, "deepseek"},
		{GeminiBaseURL, "gemini"},
		{"https://custom.api.com/v1", "openai"},
	}

	for _, tt := range tests {
		p := &OpenAIProvider{baseURL: tt.baseURL}
		if got := p.Name(); got != tt.want {
			t.Errorf("Name() with base %q = %q, want %q", tt.baseURL, got, tt.want)
		}
	}
}

func TestOpenAIProviderRequiresKey(t *testing.T) {
	_, err := NewOpenAIProvider()
	if err == nil {
		t.Fatal("expected error when no API key provided")
	}
}

func TestOpenAIProviderComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q, want %q", r.Header.Get("Authorization"), "Bearer test-key")
		}

		var req openaiChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("model = %q, want %q", req.Model, "test-model")
		}
		if len(req.Messages) != 2 {
			t.Fatalf("messages len = %d, want 2", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("first message role = %q, want %q", req.Messages[0].Role, "system")
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("second message role = %q, want %q", req.Messages[1].Role, "user")
		}

		resp := openaiChatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "extracted data"}},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewOpenAIProvider(
		WithOpenAIBaseURL(srv.URL),
		WithOpenAIKey("test-key"),
		WithOpenAIModel("test-model"),
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	result, err := p.Complete(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if result != "extracted data" {
		t.Errorf("result = %q, want %q", result, "extracted data")
	}
}

func TestOpenAIProviderHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	p, err := NewOpenAIProvider(
		WithOpenAIBaseURL(srv.URL),
		WithOpenAIKey("key"),
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error on HTTP 429")
	}
}

func TestOpenAIProviderExtraHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("missing custom header")
		}

		resp := openaiChatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewOpenAIProvider(
		WithOpenAIBaseURL(srv.URL),
		WithOpenAIKey("key"),
		WithOpenAIExtraHeaders(map[string]string{"X-Custom": "value"}),
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
}

func TestOpenAIOptions(t *testing.T) {
	o := defaultOpenAIOptions()

	WithOpenAIBaseURL("https://custom.api.com")(o)
	if o.baseURL != "https://custom.api.com" {
		t.Errorf("baseURL = %q", o.baseURL)
	}

	WithOpenAIKey("my-key")(o)
	if o.apiKey != "my-key" {
		t.Errorf("apiKey = %q", o.apiKey)
	}

	WithOpenAIModel("gpt-4")(o)
	if o.model != "gpt-4" {
		t.Errorf("model = %q", o.model)
	}

	WithOpenAIAuthHeader("X-API-Key", "")(o)
	if o.authHeader != "X-API-Key" {
		t.Errorf("authHeader = %q", o.authHeader)
	}
}

func TestNewOpenRouterProvider(t *testing.T) {
	p, err := NewOpenRouterProvider("key", "meta-llama/llama-3-8b")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.Name() != "openrouter" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openrouter")
	}
	if p.model != "meta-llama/llama-3-8b" {
		t.Errorf("model = %q", p.model)
	}
}

func TestNewDeepSeekProvider(t *testing.T) {
	p, err := NewDeepSeekProvider("key", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.Name() != "deepseek" {
		t.Errorf("Name() = %q", p.Name())
	}
	if p.model != "deepseek-chat" {
		t.Errorf("model = %q, want %q", p.model, "deepseek-chat")
	}
}

func TestNewGeminiProvider(t *testing.T) {
	p, err := NewGeminiProvider("key", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.Name() != "gemini" {
		t.Errorf("Name() = %q", p.Name())
	}
	if p.model != "gemini-2.0-flash" {
		t.Errorf("model = %q, want %q", p.model, "gemini-2.0-flash")
	}
}
