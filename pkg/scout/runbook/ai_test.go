package runbook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// mockLLMProvider implements scout.LLMProvider for testing.
type mockLLMProvider struct {
	name      string
	responses map[string]string // keyword in user prompt -> response
	err       error
}

func (m *mockLLMProvider) Name() string { return m.name }

func (m *mockLLMProvider) Complete(_ context.Context, _, userPrompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	for keyword, response := range m.responses {
		if strings.Contains(userPrompt, keyword) {
			return response, nil
		}
	}
	// Default: return first response if any.
	for _, response := range m.responses {
		return response, nil
	}
	return "", fmt.Errorf("no mock response configured")
}

func TestStripJSONFencing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no fencing", `{"name":"test"}`, `{"name":"test"}`},
		{"json fencing", "```json\n{\"name\":\"test\"}\n```", `{"name":"test"}`},
		{"plain fencing", "```\n{\"name\":\"test\"}\n```", `{"name":"test"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripJSONFencing(tt.input)
			if got != tt.want {
				t.Errorf("stripJSONFencing() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRefineSelectors_Mock(t *testing.T) {
	refined := map[string]string{
		"title": "[data-testid=\"title\"]",
		"price": "[data-testid=\"price\"]",
	}
	refinedJSON, _ := json.Marshal(refined)

	provider := &mockLLMProvider{
		name:      "mock",
		responses: map[string]string{"Current selectors": string(refinedJSON)},
	}

	result, err := RefineSelectors(provider, "<div><h1>Title</h1><span>$9.99</span></div>", map[string]string{
		"title": "h1",
		"price": "span",
	})
	if err != nil {
		t.Fatalf("RefineSelectors() error: %v", err)
	}

	if result["title"] != "[data-testid=\"title\"]" {
		t.Errorf("title selector = %q, want %q", result["title"], "[data-testid=\"title\"]")
	}
	if result["price"] != "[data-testid=\"price\"]" {
		t.Errorf("price selector = %q, want %q", result["price"], "[data-testid=\"price\"]")
	}
}

func TestRefineSelectors_NilProvider(t *testing.T) {
	_, err := RefineSelectors(nil, "<div></div>", map[string]string{"title": "h1"})
	if err == nil || !strings.Contains(err.Error(), "nil provider") {
		t.Errorf("expected nil provider error, got %v", err)
	}
}

func TestRefineSelectors_ProviderError(t *testing.T) {
	provider := &mockLLMProvider{
		name: "mock",
		err:  fmt.Errorf("api timeout"),
	}

	_, err := RefineSelectors(provider, "<div></div>", map[string]string{"title": "h1"})
	if err == nil || !strings.Contains(err.Error(), "api timeout") {
		t.Errorf("expected api timeout error, got %v", err)
	}
}

func TestRefineSelectors_InvalidJSON(t *testing.T) {
	provider := &mockLLMProvider{
		name:      "mock",
		responses: map[string]string{"Current selectors": "not valid json"},
	}

	_, err := RefineSelectors(provider, "<div></div>", map[string]string{"title": "h1"})
	if err == nil || !strings.Contains(err.Error(), "parse response") {
		t.Errorf("expected parse error, got %v", err)
	}
}
