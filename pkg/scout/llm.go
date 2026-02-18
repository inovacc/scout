package scout

import (
	"context"
	"encoding/json"
	"fmt"
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

// ExtractWithLLM sends the page content to an LLM with the given prompt and returns the response.
func (p *Page) ExtractWithLLM(prompt string, opts ...LLMOption) (string, error) {
	o := defaultLLMOptions()
	for _, fn := range opts {
		fn(o)
	}

	if o.provider == nil {
		return "", fmt.Errorf("scout: extract-llm: no LLM provider set (use WithLLMProvider)")
	}

	var md string
	var err error
	if o.mainOnly {
		md, err = p.MarkdownContent()
	} else {
		md, err = p.Markdown()
	}
	if err != nil {
		return "", fmt.Errorf("scout: extract-llm: get markdown: %w", err)
	}

	userPrompt := prompt + "\n\n---\n\n" + md

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	result, err := o.provider.Complete(ctx, o.systemPrompt, userPrompt)
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
