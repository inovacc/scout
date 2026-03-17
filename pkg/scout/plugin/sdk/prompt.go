package sdk

import "context"

// PromptHandler handles prompt requests from Scout.
type PromptHandler interface {
	Get(ctx context.Context, name string, arguments map[string]string) ([]PromptMessage, error)
}

// PromptHandlerFunc adapts a function to PromptHandler.
type PromptHandlerFunc func(ctx context.Context, name string, arguments map[string]string) ([]PromptMessage, error)

func (f PromptHandlerFunc) Get(ctx context.Context, name string, arguments map[string]string) ([]PromptMessage, error) {
	return f(ctx, name, arguments)
}

// PromptMessage is a message in a prompt response.
type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
