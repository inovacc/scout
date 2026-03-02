package runbook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// AIRunbookOption configures AI-assisted runbook generation.
type AIRunbookOption func(*aiRunbookConfig)

type aiRunbookConfig struct {
	provider scout.LLMProvider
	goal     string
	model    string
	timeout  time.Duration
}

func defaultAIConfig() *aiRunbookConfig {
	return &aiRunbookConfig{
		timeout: 60 * time.Second,
	}
}

// WithAI sets the LLM provider for AI-assisted runbook generation.
func WithAI(provider scout.LLMProvider) AIRunbookOption {
	return func(c *aiRunbookConfig) { c.provider = provider }
}

// WithGoal sets a user-specified goal that guides the LLM generation.
func WithGoal(goal string) AIRunbookOption {
	return func(c *aiRunbookConfig) { c.goal = goal }
}

// WithAIModel overrides the provider's default model.
func WithAIModel(model string) AIRunbookOption {
	return func(c *aiRunbookConfig) { c.model = model }
}

// WithAITimeout sets the timeout for the LLM request.
func WithAITimeout(d time.Duration) AIRunbookOption {
	return func(c *aiRunbookConfig) { c.timeout = d }
}

const runbookSchemaPrompt = `You are a web scraping runbook generator. You produce JSON recipes conforming to this schema:

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
- Respond with ONLY the JSON runbook, no explanation or markdown fencing.`

// GenerateWithAI creates a runbook using LLM analysis of the page.
// If the LLM provider is nil or fails, it falls back to rule-based generation.
func GenerateWithAI(browser *scout.Browser, url string, opts ...AIRunbookOption) (*Runbook, error) {
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

	analysisJSON, _ := json.MarshalIndent(analysis, "", "  ") //nolint:errchkjson,musttag

	// Build prompts
	systemPrompt := runbookSchemaPrompt

	var userParts []string

	userParts = append(userParts, fmt.Sprintf("Generate a runbook for: %s", url))
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

	var r Runbook
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
		return nil, fmt.Errorf("runbook: refine-selectors: nil provider")
	}

	selectorsJSON, err := json.Marshal(selectors)
	if err != nil {
		return nil, fmt.Errorf("runbook: refine-selectors: marshal: %w", err)
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
		return nil, fmt.Errorf("runbook: refine-selectors: %w", err)
	}

	response = stripJSONFencing(response)

	var refined map[string]string
	if err := json.Unmarshal([]byte(response), &refined); err != nil {
		return nil, fmt.Errorf("runbook: refine-selectors: parse response: %w", err)
	}

	return refined, nil
}

// generateFallback runs the full rule-based pipeline (analyze + generate).
func generateFallback(browser *scout.Browser, url string) (*Runbook, error) {
	analysis, err := AnalyzeSite(context.Background(), browser, url)
	if err != nil {
		return nil, fmt.Errorf("runbook: ai fallback: analyze: %w", err)
	}

	return fallbackFromAnalysis(analysis)
}

// fallbackFromAnalysis generates a runbook from an existing analysis.
func fallbackFromAnalysis(analysis *SiteAnalysis) (*Runbook, error) {
	return GenerateRunbook(analysis)
}

// stripJSONFencing removes markdown code fences from a JSON response.
func stripJSONFencing(s string) string {
	s = strings.TrimSpace(s)
	if after, ok := strings.CutPrefix(s, "```json"); ok {
		s = after
	} else if after, ok := strings.CutPrefix(s, "```"); ok {
		s = after
	}

	if before, ok := strings.CutSuffix(s, "```"); ok {
		s = before
	}

	return strings.TrimSpace(s)
}
