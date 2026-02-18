package scout

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Known base URLs for OpenAI-compatible providers.
const (
	OpenAIBaseURL     = "https://api.openai.com/v1"
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	DeepSeekBaseURL   = "https://api.deepseek.com/v1"
	GeminiBaseURL     = "https://generativelanguage.googleapis.com/v1beta/openai"
)

// OpenAIProvider implements LLMProvider using any OpenAI-compatible chat completions API.
// Works with OpenAI, OpenRouter, DeepSeek, Gemini, Kivi, and any other compatible endpoint.
type OpenAIProvider struct {
	baseURL      string
	apiKey       string
	model        string
	client       *http.Client
	authHeader   string // header name for API key (default: "Authorization")
	authPrefix   string // prefix before API key (default: "Bearer ")
	extraHeaders map[string]string
}

// OpenAIOption configures the OpenAI-compatible provider.
type OpenAIOption func(*openaiOptions)

type openaiOptions struct {
	baseURL      string
	apiKey       string
	model        string
	httpClient   *http.Client
	authHeader   string
	authPrefix   string
	extraHeaders map[string]string
}

func defaultOpenAIOptions() *openaiOptions {
	return &openaiOptions{
		baseURL:    OpenAIBaseURL,
		model:      "gpt-4o-mini",
		authHeader: "Authorization",
		authPrefix: "Bearer ",
	}
}

// WithOpenAIBaseURL sets the API base URL.
func WithOpenAIBaseURL(url string) OpenAIOption {
	return func(o *openaiOptions) { o.baseURL = url }
}

// WithOpenAIKey sets the API key.
func WithOpenAIKey(key string) OpenAIOption {
	return func(o *openaiOptions) { o.apiKey = key }
}

// WithOpenAIModel sets the default model.
func WithOpenAIModel(model string) OpenAIOption {
	return func(o *openaiOptions) { o.model = model }
}

// WithOpenAIHTTPClient sets a custom HTTP client.
func WithOpenAIHTTPClient(c *http.Client) OpenAIOption {
	return func(o *openaiOptions) { o.httpClient = c }
}

// WithOpenAIAuthHeader sets a custom authorization header name and prefix.
func WithOpenAIAuthHeader(header, prefix string) OpenAIOption {
	return func(o *openaiOptions) {
		o.authHeader = header
		o.authPrefix = prefix
	}
}

// WithOpenAIExtraHeaders adds extra HTTP headers to every request.
func WithOpenAIExtraHeaders(headers map[string]string) OpenAIOption {
	return func(o *openaiOptions) { o.extraHeaders = headers }
}

// NewOpenAIProvider creates a provider for any OpenAI-compatible API.
func NewOpenAIProvider(opts ...OpenAIOption) (*OpenAIProvider, error) {
	o := defaultOpenAIOptions()
	for _, fn := range opts {
		fn(o)
	}

	if o.apiKey == "" {
		return nil, fmt.Errorf("scout: openai: API key is required (use WithOpenAIKey)")
	}

	client := http.DefaultClient
	if o.httpClient != nil {
		client = o.httpClient
	}

	return &OpenAIProvider{
		baseURL:      o.baseURL,
		apiKey:       o.apiKey,
		model:        o.model,
		client:       client,
		authHeader:   o.authHeader,
		authPrefix:   o.authPrefix,
		extraHeaders: o.extraHeaders,
	}, nil
}

// NewOpenRouterProvider creates a provider for OpenRouter.
func NewOpenRouterProvider(apiKey, model string, opts ...OpenAIOption) (*OpenAIProvider, error) {
	defaults := []OpenAIOption{
		WithOpenAIBaseURL(OpenRouterBaseURL),
		WithOpenAIKey(apiKey),
		WithOpenAIModel(model),
	}
	return NewOpenAIProvider(append(defaults, opts...)...)
}

// NewDeepSeekProvider creates a provider for DeepSeek.
func NewDeepSeekProvider(apiKey, model string, opts ...OpenAIOption) (*OpenAIProvider, error) {
	if model == "" {
		model = "deepseek-chat"
	}
	defaults := []OpenAIOption{
		WithOpenAIBaseURL(DeepSeekBaseURL),
		WithOpenAIKey(apiKey),
		WithOpenAIModel(model),
	}
	return NewOpenAIProvider(append(defaults, opts...)...)
}

// NewGeminiProvider creates a provider for Google Gemini via its OpenAI-compatible endpoint.
func NewGeminiProvider(apiKey, model string, opts ...OpenAIOption) (*OpenAIProvider, error) {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	defaults := []OpenAIOption{
		WithOpenAIBaseURL(GeminiBaseURL),
		WithOpenAIKey(apiKey),
		WithOpenAIModel(model),
	}
	return NewOpenAIProvider(append(defaults, opts...)...)
}

// Name returns the provider name based on base URL.
func (p *OpenAIProvider) Name() string {
	switch p.baseURL {
	case OpenRouterBaseURL:
		return "openrouter"
	case DeepSeekBaseURL:
		return "deepseek"
	case GeminiBaseURL:
		return "gemini"
	default:
		return "openai"
	}
}

type openaiChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openaiChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

type openaiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a chat completion request and returns the response content.
func (p *OpenAIProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []openaiChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := openaiChatRequest{
		Model:    p.model,
		Messages: messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("scout: %s: marshal request: %w", p.Name(), err)
	}

	url := p.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("scout: %s: create request: %w", p.Name(), err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(p.authHeader, p.authPrefix+p.apiKey)
	for k, v := range p.extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("scout: %s: request: %w", p.Name(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scout: %s: read response: %w", p.Name(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scout: %s: HTTP %d: %s", p.Name(), resp.StatusCode, string(respBody))
	}

	var chatResp openaiChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("scout: %s: decode response: %w", p.Name(), err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("scout: %s: API error: %s", p.Name(), chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("scout: %s: no choices in response", p.Name())
	}

	return chatResp.Choices[0].Message.Content, nil
}
