package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/internal/engine/llm"
)

// LLMProvider re-exports llm.Provider from sub-package.
type LLMProvider = llm.Provider
type LLMJobResult = llm.JobResult
type LLMWorkspace = llm.Workspace
type LLMSession = llm.Session
type LLMJob = llm.Job
type JobStatus = llm.JobStatus
type JobRef = llm.JobRef
type JobIndex = llm.JobIndex
type SessionIndex = llm.SessionIndex

// AnthropicProvider re-exports llm.AnthropicProvider from sub-package.
type AnthropicProvider = llm.AnthropicProvider
type AnthropicOption = llm.AnthropicOption
type OllamaProvider = llm.OllamaProvider
type OllamaOption = llm.OllamaOption
type OpenAIProvider = llm.OpenAIProvider
type OpenAIOption = llm.OpenAIOption

// Re-export constants.
const (
	JobStatusPending    = llm.JobStatusPending
	JobStatusExtracting = llm.JobStatusExtracting
	JobStatusReviewing  = llm.JobStatusReviewing
	JobStatusCompleted  = llm.JobStatusCompleted
	JobStatusFailed     = llm.JobStatusFailed

	AnthropicBaseURL    = llm.AnthropicBaseURL
	AnthropicAPIVersion = llm.AnthropicAPIVersion
	OpenAIBaseURL       = llm.OpenAIBaseURL
	OpenRouterBaseURL   = llm.OpenRouterBaseURL
	DeepSeekBaseURL     = llm.DeepSeekBaseURL
	GeminiBaseURL       = llm.GeminiBaseURL
)

// Re-export constructors and option functions.
var (
	NewAnthropicProvider = llm.NewAnthropicProvider
	WithAnthropicBaseURL = llm.WithAnthropicBaseURL
	WithAnthropicKey     = llm.WithAnthropicKey
	WithAnthropicModel   = llm.WithAnthropicModel
	WithAnthropicHTTPClient = llm.WithAnthropicHTTPClient

	NewOllamaProvider    = llm.NewOllamaProvider
	WithOllamaHost       = llm.WithOllamaHost
	WithOllamaModel      = llm.WithOllamaModel
	WithOllamaAutoPull   = llm.WithOllamaAutoPull
	WithOllamaHTTPClient = llm.WithOllamaHTTPClient

	NewOpenAIProvider      = llm.NewOpenAIProvider
	NewOpenRouterProvider  = llm.NewOpenRouterProvider
	NewDeepSeekProvider    = llm.NewDeepSeekProvider
	NewGeminiProvider      = llm.NewGeminiProvider
	WithOpenAIBaseURL      = llm.WithOpenAIBaseURL
	WithOpenAIKey          = llm.WithOpenAIKey
	WithOpenAIModel        = llm.WithOpenAIModel
	WithOpenAIHTTPClient   = llm.WithOpenAIHTTPClient
	WithOpenAIAuthHeader   = llm.WithOpenAIAuthHeader
	WithOpenAIExtraHeaders = llm.WithOpenAIExtraHeaders

	NewLLMWorkspace = llm.NewWorkspace
)

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

// WithLLMReview sets a review provider that validates the extraction output.
func WithLLMReview(provider LLMProvider) LLMOption {
	return func(o *llmOptions) { o.reviewProvider = provider }
}

// WithLLMReviewModel overrides the review provider's default model.
func WithLLMReviewModel(model string) LLMOption {
	return func(o *llmOptions) { o.reviewModel = model }
}

// WithLLMReviewPrompt overrides the default review system prompt.
func WithLLMReviewPrompt(prompt string) LLMOption {
	return func(o *llmOptions) { o.reviewPrompt = prompt }
}

// WithLLMWorkspace sets a workspace for persisting jobs to disk.
func WithLLMWorkspace(ws *LLMWorkspace) LLMOption {
	return func(o *llmOptions) { o.workspace = ws }
}

// WithLLMSessionID sets the session ID for job tracking.
func WithLLMSessionID(id string) LLMOption {
	return func(o *llmOptions) { o.sessionID = id }
}

// WithLLMMetadata adds a key-value metadata pair to the job.
func WithLLMMetadata(key, value string) LLMOption {
	return func(o *llmOptions) {
		if o.metadata == nil {
			o.metadata = make(map[string]string)
		}

		o.metadata[key] = value
	}
}

// pageIntelligenceContext detects the page's framework and render mode and returns
// a short context string suitable for prepending to an LLM system prompt.
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
