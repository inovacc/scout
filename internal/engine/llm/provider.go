package llm

import "context"

// Provider defines the interface for LLM backends used by ExtractWithLLM.
type Provider interface {
	Name() string
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
