package recipe

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// mockLLMProvider implements scout.LLMProvider for testing.
type mockLLMProvider struct {
	name     string
	response string
	err      error
}

func (m *mockLLMProvider) Name() string { return m.name }
func (m *mockLLMProvider) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestAIRecipeOptions(t *testing.T) {
	cfg := defaultAIConfig()

	if cfg.provider != nil {
		t.Fatal("expected nil provider by default")
	}
	if cfg.goal != "" {
		t.Fatal("expected empty goal by default")
	}
	if cfg.model != "" {
		t.Fatal("expected empty model by default")
	}
	if cfg.timeout != 60*time.Second {
		t.Fatalf("expected 60s timeout, got %v", cfg.timeout)
	}

	p := &mockLLMProvider{name: "test"}
	opts := []AIRecipeOption{
		WithAI(p),
		WithGoal("extract product prices"),
		WithAIModel("gpt-4"),
		WithAITimeout(30 * time.Second),
	}

	for _, fn := range opts {
		fn(cfg)
	}

	if cfg.provider != p {
		t.Fatal("WithAI did not set provider")
	}
	if cfg.goal != "extract product prices" {
		t.Fatalf("WithGoal did not set goal, got %q", cfg.goal)
	}
	if cfg.model != "gpt-4" {
		t.Fatalf("WithAIModel did not set model, got %q", cfg.model)
	}
	if cfg.timeout != 30*time.Second {
		t.Fatalf("WithAITimeout did not set timeout, got %v", cfg.timeout)
	}
}

func TestRefineSelectors(t *testing.T) {
	expected := map[string]string{
		"title": "[data-testid=\"product-title\"]",
		"price": "[data-testid=\"product-price\"]",
	}

	responseJSON, _ := json.Marshal(expected)
	provider := &mockLLMProvider{
		name:     "mock",
		response: string(responseJSON),
	}

	html := `<div data-testid="product-title">Widget</div><span data-testid="product-price">$9.99</span>`
	selectors := map[string]string{
		"title": "h2",
		"price": ".price",
	}

	refined, err := RefineSelectors(provider, html, selectors)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for k, v := range expected {
		if refined[k] != v {
			t.Errorf("field %q: expected %q, got %q", k, v, refined[k])
		}
	}
}

func TestRefineSelectors_NilProvider(t *testing.T) {
	_, err := RefineSelectors(nil, "<html></html>", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestRefineSelectors_ProviderError(t *testing.T) {
	provider := &mockLLMProvider{
		name: "fail",
		err:  fmt.Errorf("network timeout"),
	}

	_, err := RefineSelectors(provider, "<html></html>", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error when provider fails")
	}
}

func TestRefineSelectors_InvalidJSON(t *testing.T) {
	provider := &mockLLMProvider{
		name:     "bad",
		response: "not json at all",
	}

	_, err := RefineSelectors(provider, "<html></html>", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestRefineSelectors_MarkdownFencing(t *testing.T) {
	expected := map[string]string{"title": "#heading"}
	responseJSON, _ := json.Marshal(expected)
	provider := &mockLLMProvider{
		name:     "fenced",
		response: "```json\n" + string(responseJSON) + "\n```",
	}

	refined, err := RefineSelectors(provider, "<h1 id=\"heading\">Hi</h1>", map[string]string{"title": "h1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refined["title"] != "#heading" {
		t.Errorf("expected %q, got %q", "#heading", refined["title"])
	}
}

func TestStripJSONFencing(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{`{"a":"b"}`, `{"a":"b"}`},
		{"```json\n{\"a\":\"b\"}\n```", `{"a":"b"}`},
		{"```\n{\"a\":\"b\"}\n```", `{"a":"b"}`},
		{"  ```json\n{}\n```  ", `{}`},
	}

	for _, tt := range tests {
		got := stripJSONFencing(tt.input)
		if got != tt.want {
			t.Errorf("stripJSONFencing(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
