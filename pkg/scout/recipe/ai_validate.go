package recipe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// LLMValidation holds the result of an LLM-based recipe review.
type LLMValidation struct {
	Valid            bool     `json:"valid"`
	Suggestions      []string `json:"suggestions,omitempty"`
	MissingFields    []string `json:"missing_fields,omitempty"`
	FragileSelectors []string `json:"fragile_selectors,omitempty"`
}

const validateSystemPrompt = `You are a web scraping recipe quality reviewer. You will receive a recipe JSON and optionally sample extracted items.

Evaluate the recipe for:
1. Selector quality — are the CSS selectors stable (prefer data attributes, IDs, ARIA labels, semantic tags) or fragile (positional, deeply nested classes)?
2. Missing fields — based on the sample data and recipe URL, are there obvious fields that should be extracted but are not (e.g. price, image, date)?
3. Completeness — does the recipe capture enough data to be useful?
4. Suggestions — any improvements (pagination, wait_for, better selectors)?

Respond with ONLY a JSON object in this exact format:
{
  "valid": true/false,
  "suggestions": ["suggestion 1", "suggestion 2"],
  "missing_fields": ["field1", "field2"],
  "fragile_selectors": ["selector1", "selector2"]
}

If the recipe looks good, set "valid": true with empty arrays. Do not include any text outside the JSON.`

// ValidateWithLLM sends a recipe and optional sample items to an LLM for review.
func ValidateWithLLM(provider scout.LLMProvider, r *Recipe, sampleItems []map[string]any) (*LLMValidation, error) {
	if provider == nil {
		return nil, fmt.Errorf("recipe: validate-llm: nil provider")
	}

	if r == nil {
		return nil, fmt.Errorf("recipe: validate-llm: nil recipe")
	}

	recipeJSON, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("recipe: validate-llm: marshal recipe: %w", err)
	}

	var userParts []string
	userParts = append(userParts, fmt.Sprintf("Recipe to review:\n%s", string(recipeJSON)))

	if len(sampleItems) > 0 {
		// Limit to first 5 sample items to keep prompt size reasonable.
		items := sampleItems
		if len(items) > 5 {
			items = items[:5]
		}
		sampleJSON, _ := json.MarshalIndent(items, "", "  ")
		userParts = append(userParts, fmt.Sprintf("\n\nSample extracted items (%d shown):\n%s", len(items), string(sampleJSON)))
	}

	userPrompt := strings.Join(userParts, "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, err := provider.Complete(ctx, validateSystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("recipe: validate-llm: %w", err)
	}

	response = stripJSONFencing(response)

	var result LLMValidation
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Fallback: treat entire response as a single suggestion.
		return &LLMValidation{
			Valid:       false,
			Suggestions: []string{strings.TrimSpace(response)},
		}, nil
	}

	return &result, nil
}
