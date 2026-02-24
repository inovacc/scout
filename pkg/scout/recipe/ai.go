package recipe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// AIRecipeOption configures AI-assisted recipe generation.
type AIRecipeOption func(*aiRecipeConfig)

type aiRecipeConfig struct {
	provider scout.LLMProvider
	goal     string
	model    string
	timeout  time.Duration
}

func defaultAIConfig() *aiRecipeConfig {
	return &aiRecipeConfig{
		timeout: 60 * time.Second,
	}
}

// WithAI sets the LLM provider for AI-assisted recipe generation.
func WithAI(provider scout.LLMProvider) AIRecipeOption {
	return func(c *aiRecipeConfig) { c.provider = provider }
}

// WithGoal sets a user-specified goal that guides the LLM generation.
func WithGoal(goal string) AIRecipeOption {
	return func(c *aiRecipeConfig) { c.goal = goal }
}

// WithAIModel overrides the provider's default model.
func WithAIModel(model string) AIRecipeOption {
	return func(c *aiRecipeConfig) { c.model = model }
}

// WithAITimeout sets the timeout for the LLM request.
func WithAITimeout(d time.Duration) AIRecipeOption {
	return func(c *aiRecipeConfig) { c.timeout = d }
}

const recipeSchemaPrompt = `You are a web scraping recipe generator. You produce JSON recipes conforming to this schema:

{
  "version": "1",
  "name": "<short-kebab-case-name>",
  "type": "extract" or "automate",
  "url": "<target-url>",
  "wait_for": "<css-selector-to-wait-for>",
  "selectors": { "<name>": "<css-selector>" },
  "items": {
    "container": "<css-selector-for-repeating-element>",
    "fields": { "<field-name>": "<css-selector-or-selector@attr>" }
  },
  "pagination": {
    "strategy": "click" or "url" or "scroll" or "load_more",
    "next_selector": "<css-selector>",
    "max_pages": <int>
  },
  "steps": [
    { "action": "navigate|click|type|screenshot|extract|eval|wait|key", "url": "...", "selector": "...", "text": "..." }
  ],
  "output": { "format": "json" }
}

Rules:
- For "extract" recipes: include "url", "items" (with "container" and "fields"), optionally "pagination". Do NOT include "steps".
- For "automate" recipes: include "steps". Do NOT include "items".
- Use semantic field names (e.g. "title", "price", "author", not "text_0", "text_1").
- Prefer stable selectors: data attributes, IDs, semantic tags, ARIA labels over positional or class-based selectors.
- Respond with ONLY the JSON recipe, no explanation or markdown fencing.`

// GenerateWithAI creates a recipe using LLM analysis of the page.
// If the LLM provider is nil or fails, it falls back to rule-based generation.
func GenerateWithAI(browser *scout.Browser, url string, opts ...AIRecipeOption) (*Recipe, error) {
	cfg := defaultAIConfig()
	for _, fn := range opts {
		fn(cfg)
	}

	// If no provider, fall back immediately
	if cfg.provider == nil {
		return generateFallback(browser, url)
	}

	// Navigate and extract page content
	page, err := browser.NewPage(url)
	if err != nil {
		return generateFallback(browser, url)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return generateFallback(browser, url)
	}

	// Get a sample of the page HTML (first 5000 chars of body innerHTML)
	htmlResult, err := page.Eval(`() => {
		const body = document.body;
		if (!body) return "";
		const html = body.innerHTML;
		return html.substring(0, 5000);
	}`)
	if err != nil {
		return generateFallback(browser, url)
	}
	htmlSample := fmt.Sprintf("%v", htmlResult.Value)

	// Get rule-based analysis as context
	analysis, err := AnalyzeSite(context.Background(), browser, url)
	if err != nil {
		return generateFallback(browser, url)
	}

	analysisJSON, _ := json.MarshalIndent(analysis, "", "  ")

	// Build prompts
	systemPrompt := recipeSchemaPrompt

	var userParts []string
	userParts = append(userParts, fmt.Sprintf("Generate a recipe for: %s", url))
	if cfg.goal != "" {
		userParts = append(userParts, fmt.Sprintf("\nGoal: %s", cfg.goal))
	}
	userParts = append(userParts, fmt.Sprintf("\n\nRule-based analysis of the page:\n%s", string(analysisJSON)))
	userParts = append(userParts, fmt.Sprintf("\n\nHTML sample (first 5000 chars):\n%s", htmlSample))

	userPrompt := strings.Join(userParts, "")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	response, err := cfg.provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fallbackFromAnalysis(analysis)
	}

	// Strip markdown fencing if present
	response = stripJSONFencing(response)

	var r Recipe
	if err := json.Unmarshal([]byte(response), &r); err != nil {
		return fallbackFromAnalysis(analysis)
	}

	if err := r.Validate(); err != nil {
		return fallbackFromAnalysis(analysis)
	}

	return &r, nil
}

// RefineSelectors asks the LLM to suggest more stable selectors.
func RefineSelectors(provider scout.LLMProvider, html string, selectors map[string]string) (map[string]string, error) {
	if provider == nil {
		return nil, fmt.Errorf("recipe: refine-selectors: nil provider")
	}

	selectorsJSON, err := json.Marshal(selectors)
	if err != nil {
		return nil, fmt.Errorf("recipe: refine-selectors: marshal: %w", err)
	}

	systemPrompt := `You are a CSS selector optimizer. Given an HTML snippet and a map of field names to CSS selectors,
suggest more stable selectors using data attributes, IDs, ARIA labels, or semantic HTML tags.
Respond with ONLY a JSON object mapping field names to improved CSS selectors, no explanation.`

	// Truncate HTML to a reasonable size
	if len(html) > 8000 {
		html = html[:8000]
	}

	userPrompt := fmt.Sprintf("Current selectors:\n%s\n\nHTML context:\n%s", string(selectorsJSON), html)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, err := provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("recipe: refine-selectors: %w", err)
	}

	response = stripJSONFencing(response)

	var refined map[string]string
	if err := json.Unmarshal([]byte(response), &refined); err != nil {
		return nil, fmt.Errorf("recipe: refine-selectors: parse response: %w", err)
	}

	return refined, nil
}

// generateFallback runs the full rule-based pipeline (analyze + generate).
func generateFallback(browser *scout.Browser, url string) (*Recipe, error) {
	analysis, err := AnalyzeSite(context.Background(), browser, url)
	if err != nil {
		return nil, fmt.Errorf("recipe: ai fallback: analyze: %w", err)
	}
	return fallbackFromAnalysis(analysis)
}

// fallbackFromAnalysis generates a recipe from an existing analysis.
func fallbackFromAnalysis(analysis *SiteAnalysis) (*Recipe, error) {
	return GenerateRecipe(analysis)
}

// stripJSONFencing removes markdown code fences from a JSON response.
func stripJSONFencing(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
