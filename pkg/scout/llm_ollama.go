package scout

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ollama/ollama/api"
)

// OllamaProvider implements LLMProvider using a local or remote Ollama server.
type OllamaProvider struct {
	client *api.Client
	model  string
}

// OllamaOption configures the Ollama provider.
type OllamaOption func(*ollamaOptions)

type ollamaOptions struct {
	host       string
	model      string
	httpClient *http.Client
	autoPull   bool
}

func defaultOllamaOptions() *ollamaOptions {
	return &ollamaOptions{
		model: "llama3.2",
	}
}

// WithOllamaHost sets the Ollama server URL (default: env OLLAMA_HOST or http://localhost:11434).
func WithOllamaHost(host string) OllamaOption {
	return func(o *ollamaOptions) { o.host = host }
}

// WithOllamaModel sets the default model name.
func WithOllamaModel(model string) OllamaOption {
	return func(o *ollamaOptions) { o.model = model }
}

// WithOllamaAutoPull enables automatic model pulling if the model is not available locally.
func WithOllamaAutoPull() OllamaOption {
	return func(o *ollamaOptions) { o.autoPull = true }
}

// WithOllamaHTTPClient sets a custom HTTP client for the Ollama API.
func WithOllamaHTTPClient(c *http.Client) OllamaOption {
	return func(o *ollamaOptions) { o.httpClient = c }
}

// NewOllamaProvider creates a new Ollama LLM provider.
func NewOllamaProvider(opts ...OllamaOption) (*OllamaProvider, error) {
	o := defaultOllamaOptions()
	for _, fn := range opts {
		fn(o)
	}

	var client *api.Client

	if o.host != "" {
		base, err := url.Parse(o.host)
		if err != nil {
			return nil, fmt.Errorf("scout: ollama: parse host: %w", err)
		}
		httpC := http.DefaultClient
		if o.httpClient != nil {
			httpC = o.httpClient
		}
		client = api.NewClient(base, httpC)
	} else {
		var err error
		client, err = api.ClientFromEnvironment()
		if err != nil {
			return nil, fmt.Errorf("scout: ollama: create client: %w", err)
		}
	}

	return &OllamaProvider{
		client: client,
		model:  o.model,
	}, nil
}

// Name returns "ollama".
func (o *OllamaProvider) Name() string { return "ollama" }

// Complete sends a prompt to the Ollama server and returns the full response.
func (o *OllamaProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	stream := false
	req := &api.GenerateRequest{
		Model:  o.model,
		System: systemPrompt,
		Prompt: userPrompt,
		Stream: &stream,
	}

	var result strings.Builder
	err := o.client.Generate(ctx, req, func(resp api.GenerateResponse) error {
		result.WriteString(resp.Response)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scout: ollama: generate: %w", err)
	}

	return result.String(), nil
}

// ListModels returns the names of all locally available models.
func (o *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	resp, err := o.client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("scout: ollama: list models: %w", err)
	}

	names := make([]string, 0, len(resp.Models))
	for _, m := range resp.Models {
		names = append(names, m.Name)
	}

	return names, nil
}

// PullModel downloads a model from the Ollama registry.
func (o *OllamaProvider) PullModel(ctx context.Context, model string, progress func(status string, completed, total int64)) error {
	stream := true
	req := &api.PullRequest{
		Model:  model,
		Stream: &stream,
	}

	return o.client.Pull(ctx, req, func(resp api.ProgressResponse) error {
		if progress != nil {
			progress(resp.Status, resp.Completed, resp.Total)
		}
		return nil
	})
}

// HasModel checks whether a model is available locally.
func (o *OllamaProvider) HasModel(ctx context.Context, model string) (bool, error) {
	models, err := o.ListModels(ctx)
	if err != nil {
		return false, err
	}

	for _, m := range models {
		if m == model || strings.HasPrefix(m, model+":") {
			return true, nil
		}
	}

	return false, nil
}
