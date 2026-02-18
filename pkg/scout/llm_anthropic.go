package scout

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// AnthropicBaseURL is the default Anthropic API endpoint.
	AnthropicBaseURL = "https://api.anthropic.com/v1"
	// AnthropicAPIVersion is the API version header value.
	AnthropicAPIVersion = "2023-06-01"
)

// AnthropicProvider implements LLMProvider using the Anthropic Messages API.
type AnthropicProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// AnthropicOption configures the Anthropic provider.
type AnthropicOption func(*anthropicOptions)

type anthropicOptions struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func defaultAnthropicOptions() *anthropicOptions {
	return &anthropicOptions{
		baseURL: AnthropicBaseURL,
		model:   "claude-sonnet-4-20250514",
	}
}

// WithAnthropicBaseURL sets the API base URL.
func WithAnthropicBaseURL(url string) AnthropicOption {
	return func(o *anthropicOptions) { o.baseURL = url }
}

// WithAnthropicKey sets the API key.
func WithAnthropicKey(key string) AnthropicOption {
	return func(o *anthropicOptions) { o.apiKey = key }
}

// WithAnthropicModel sets the default model.
func WithAnthropicModel(model string) AnthropicOption {
	return func(o *anthropicOptions) { o.model = model }
}

// WithAnthropicHTTPClient sets a custom HTTP client.
func WithAnthropicHTTPClient(c *http.Client) AnthropicOption {
	return func(o *anthropicOptions) { o.httpClient = c }
}

// NewAnthropicProvider creates a new Anthropic LLM provider.
func NewAnthropicProvider(opts ...AnthropicOption) (*AnthropicProvider, error) {
	o := defaultAnthropicOptions()
	for _, fn := range opts {
		fn(o)
	}

	if o.apiKey == "" {
		return nil, fmt.Errorf("scout: anthropic: API key is required (use WithAnthropicKey)")
	}

	client := http.DefaultClient
	if o.httpClient != nil {
		client = o.httpClient
	}

	return &AnthropicProvider{
		baseURL: o.baseURL,
		apiKey:  o.apiKey,
		model:   o.model,
		client:  client,
	}, nil
}

// Name returns "anthropic".
func (p *AnthropicProvider) Name() string { return "anthropic" }

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a messages request to Anthropic and returns the response text.
func (p *AnthropicProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     p.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("scout: anthropic: marshal request: %w", err)
	}

	url := p.baseURL + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("scout: anthropic: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", AnthropicAPIVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("scout: anthropic: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scout: anthropic: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scout: anthropic: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp anthropicResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("scout: anthropic: decode response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("scout: anthropic: API error: %s", chatResp.Error.Message)
	}

	var result string
	for _, c := range chatResp.Content {
		if c.Type == "text" {
			result += c.Text
		}
	}

	if result == "" {
		return "", fmt.Errorf("scout: anthropic: no text content in response")
	}

	return result, nil
}
