package recipe

import (
	"fmt"
	"testing"
)

func TestValidateWithLLM_ValidRecipe(t *testing.T) {
	provider := &mockLLMProvider{
		name:      "mock",
		responses: map[string]string{"": `{"valid": true, "suggestions": [], "missing_fields": [], "fragile_selectors": []}`},
	}

	r := &Recipe{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		URL:     "http://example.com",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}

	result, err := ValidateWithLLM(provider, r, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid=true")
	}
	if len(result.Suggestions) != 0 {
		t.Errorf("expected no suggestions, got %d", len(result.Suggestions))
	}
}

func TestValidateWithLLM_WithSuggestions(t *testing.T) {
	provider := &mockLLMProvider{
		name: "mock",
		responses: map[string]string{"": `{
			"valid": false,
			"suggestions": ["Add pagination support", "Use data-testid selectors"],
			"missing_fields": ["price", "image"],
			"fragile_selectors": [".item > div:nth-child(2) > span"]
		}`},
	}

	r := &Recipe{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		URL:     "http://example.com",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}

	samples := []map[string]any{
		{"title": "Product 1"},
		{"title": "Product 2"},
	}

	result, err := ValidateWithLLM(provider, r, samples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false")
	}
	if len(result.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(result.Suggestions))
	}
	if len(result.MissingFields) != 2 {
		t.Errorf("expected 2 missing fields, got %d", len(result.MissingFields))
	}
	if len(result.FragileSelectors) != 1 {
		t.Errorf("expected 1 fragile selector, got %d", len(result.FragileSelectors))
	}
}

func TestValidateWithLLM_NilProvider(t *testing.T) {
	r := &Recipe{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		URL:     "http://example.com",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}

	_, err := ValidateWithLLM(nil, r, nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
	if got := err.Error(); got != "recipe: validate-llm: nil provider" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestValidateWithLLM_InvalidJSON(t *testing.T) {
	provider := &mockLLMProvider{
		name:      "mock",
		responses: map[string]string{"": "The recipe looks mostly good but could use pagination."},
	}

	r := &Recipe{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		URL:     "http://example.com",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}

	result, err := ValidateWithLLM(provider, r, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for fallback")
	}
	if len(result.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion from fallback, got %d", len(result.Suggestions))
	}
	if result.Suggestions[0] != "The recipe looks mostly good but could use pagination." {
		t.Errorf("unexpected suggestion: %s", result.Suggestions[0])
	}
}

func TestValidateWithLLM_NilRecipe(t *testing.T) {
	provider := &mockLLMProvider{name: "mock", responses: map[string]string{"": "{}"}}

	_, err := ValidateWithLLM(provider, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil recipe")
	}
	if got := err.Error(); got != "recipe: validate-llm: nil recipe" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestValidateWithLLM_ProviderError(t *testing.T) {
	provider := &mockLLMProvider{
		name: "mock",
		err:  fmt.Errorf("connection refused"),
	}

	r := &Recipe{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		URL:     "http://example.com",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}

	_, err := ValidateWithLLM(provider, r, nil)
	if err == nil {
		t.Fatal("expected error from provider")
	}
}
