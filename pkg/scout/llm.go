package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LLMProvider defines the interface for LLM backends used by ExtractWithLLM.
type LLMProvider interface {
	Name() string
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// LLMOption configures LLM extraction behavior.
type LLMOption func(*llmOptions)

type llmOptions struct {
	provider     LLMProvider
	model        string
	temperature  float64
	maxTokens    int
	schema       json.RawMessage
	systemPrompt string
	timeout      time.Duration
	mainOnly     bool

	// Review phase
	reviewProvider LLMProvider
	reviewModel    string
	reviewPrompt   string

	// Workspace persistence
	workspace *LLMWorkspace
	sessionID string
	metadata  map[string]string
}

func defaultLLMOptions() *llmOptions {
	return &llmOptions{
		temperature: 0.0,
		timeout:     60 * time.Second,
		mainOnly:    true,
		systemPrompt: "Extract structured data from the following web page content. " +
			"Respond only with the extracted information, no explanations.",
	}
}

// WithLLMProvider sets the LLM provider to use.
func WithLLMProvider(p LLMProvider) LLMOption {
	return func(o *llmOptions) { o.provider = p }
}

// WithLLMModel overrides the provider's default model.
func WithLLMModel(model string) LLMOption {
	return func(o *llmOptions) { o.model = model }
}

// WithLLMTemperature sets the sampling temperature (0.0–1.0).
func WithLLMTemperature(t float64) LLMOption {
	return func(o *llmOptions) { o.temperature = t }
}

// WithLLMMaxTokens sets the maximum number of tokens in the response.
func WithLLMMaxTokens(n int) LLMOption {
	return func(o *llmOptions) { o.maxTokens = n }
}

// WithLLMSchema sets a JSON schema for response validation.
func WithLLMSchema(schema json.RawMessage) LLMOption {
	return func(o *llmOptions) { o.schema = schema }
}

// WithLLMSystemPrompt overrides the default system prompt.
func WithLLMSystemPrompt(s string) LLMOption {
	return func(o *llmOptions) { o.systemPrompt = s }
}

// WithLLMTimeout sets the timeout for the LLM request.
func WithLLMTimeout(d time.Duration) LLMOption {
	return func(o *llmOptions) { o.timeout = d }
}

// WithLLMMainContent uses MarkdownContent() (main content only) instead of Markdown().
func WithLLMMainContent() LLMOption {
	return func(o *llmOptions) { o.mainOnly = true }
}

// pageIntelligenceContext detects the page's framework and render mode and returns
// a short context string suitable for prepending to an LLM system prompt.
// Returns an empty string if detection fails or yields no useful info.
func (p *Page) pageIntelligenceContext() string {
	var parts []string

	if fw, err := p.DetectFramework(); err == nil && fw != nil {
		desc := fw.Name
		if fw.Version != "" {
			desc += " " + fw.Version
		}

		if fw.SPA {
			desc += " (SPA)"
		}

		parts = append(parts, "Framework: "+desc)
	}

	if ri, err := p.DetectRenderMode(); err == nil && ri.Mode != RenderUnknown {
		desc := string(ri.Mode)
		if ri.Hydrated {
			desc += ", hydrated"
		}

		parts = append(parts, "Render mode: "+desc)
	}

	if len(parts) == 0 {
		return ""
	}

	return "Page intelligence: " + fmt.Sprintf("%s.", joinStrings(parts, "; "))
}

// joinStrings joins a slice with a separator. Avoids importing strings for one call.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString(parts[0])
	for _, p := range parts[1:] {
		result.WriteString(sep + p)
	}

	return result.String()
}

// ExtractWithLLM sends the page content to an LLM with the given prompt and returns the response.
func (p *Page) ExtractWithLLM(prompt string, opts ...LLMOption) (string, error) {
	o := defaultLLMOptions()
	for _, fn := range opts {
		fn(o)
	}

	if o.provider == nil {
		return "", fmt.Errorf("scout: extract-llm: no LLM provider set (use WithLLMProvider)")
	}

	var (
		md  string
		err error
	)

	if o.mainOnly {
		md, err = p.MarkdownContent()
	} else {
		md, err = p.Markdown()
	}

	if err != nil {
		return "", fmt.Errorf("scout: extract-llm: get markdown: %w", err)
	}

	// Enrich system prompt with page intelligence
	systemPrompt := o.systemPrompt
	if intel := p.pageIntelligenceContext(); intel != "" {
		systemPrompt = intel + "\n\n" + systemPrompt
	}

	userPrompt := prompt + "\n\n---\n\n" + md

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	result, err := o.provider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("scout: extract-llm: %s: %w", o.provider.Name(), err)
	}

	if len(o.schema) > 0 {
		if !json.Valid([]byte(result)) {
			return "", fmt.Errorf("scout: extract-llm: response is not valid JSON")
		}
	}

	return result, nil
}

// ExtractWithLLMJSON sends the page content to an LLM and decodes the JSON response into target.
func (p *Page) ExtractWithLLMJSON(prompt string, target any, opts ...LLMOption) error {
	result, err := p.ExtractWithLLM(prompt, opts...)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(result), target); err != nil {
		return fmt.Errorf("scout: extract-llm: decode JSON: %w", err)
	}

	return nil
}
