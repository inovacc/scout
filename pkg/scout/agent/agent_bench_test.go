package agent

import (
	"context"
	"testing"
)

func BenchmarkProviderCall(b *testing.B) {
	p := &Provider{
		tools: []Tool{
			{
				Name: "echo",
				Handler: func(_ context.Context, _ map[string]any) (string, error) {
					return "ok", nil
				},
			},
		},
	}

	ctx := context.Background()
	args := map[string]any{"input": "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.Call(ctx, "echo", args)
	}
}

func BenchmarkOpenAITools(b *testing.B) {
	p := &Provider{}
	p.getPage = p.ensurePage
	p.registerBuiltinTools()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.OpenAITools()
	}
}

func BenchmarkAnthropicTools(b *testing.B) {
	p := &Provider{}
	p.getPage = p.ensurePage
	p.registerBuiltinTools()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.AnthropicTools()
	}
}

func BenchmarkToolSchemaJSON(b *testing.B) {
	p := &Provider{}
	p.getPage = p.ensurePage
	p.registerBuiltinTools()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.ToolSchemaJSON()
	}
}
