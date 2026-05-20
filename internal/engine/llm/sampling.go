package llm

import (
	"context"
	"errors"
	"fmt"

	scoutmcp "github.com/inovacc/scout/pkg/mcp"
)

// MCPSamplingProvider implements Provider via MCP sampling reverse-RPC. When
// Scout runs as an MCP server (e.g. under Claude Code), this provider asks
// the connected MCP HOST to perform the LLM completion instead of calling an
// external API. Eliminates the need for users to configure Ollama / OpenAI /
// Anthropic credentials when Scout is already running inside an LLM host.
//
// Failure modes:
//   - No MCP session wired (Scout not running as MCP server, or sampling
//     capability not negotiated): Complete returns an error wrapping
//     scoutmcp.ErrNoSession. Callers may errors.Is-check it and fall back
//     to a different provider.
//   - Host returns non-text content (image, audio): returns an error
//     wrapping scoutmcp.ErrNonText.
//
// See pkg/mcp/doc.go for the design rationale and reverse-RPC pattern.
type MCPSamplingProvider struct {
	maxTokens int64
}

// MCPSamplingOption configures the MCP sampling provider.
type MCPSamplingOption func(*MCPSamplingProvider)

// WithMCPSamplingMaxTokens overrides the output token budget for sampling
// requests. Default is 4000 — higher than pkg/mcp's 2000 default because
// structured extraction routinely produces larger payloads. Pass <= 0 to
// fall back to the pkg/mcp default.
func WithMCPSamplingMaxTokens(n int64) MCPSamplingOption {
	return func(p *MCPSamplingProvider) { p.maxTokens = n }
}

// NewMCPSamplingProvider creates a Provider that delegates completions to
// the connected MCP host via sampling/createMessage. Construction never
// fails; the actual session check happens at Complete time so a Provider
// can be wired up before the MCP session is established.
func NewMCPSamplingProvider(opts ...MCPSamplingOption) *MCPSamplingProvider {
	p := &MCPSamplingProvider{maxTokens: 4000}
	for _, o := range opts {
		o(p)
	}

	return p
}

// Name returns the provider identifier.
func (p *MCPSamplingProvider) Name() string {
	return "mcp-sampling"
}

// Complete delegates to the connected MCP host. The system prompt is
// concatenated with the user prompt (current pkg/mcp.Sample interface
// supports one message role only — "user"). Future enhancement: when
// pkg/mcp exposes SystemPrompt in its params, wire it through.
func (p *MCPSamplingProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !scoutmcp.HasSession() {
		return "", fmt.Errorf("llm: %w", scoutmcp.ErrNoSession)
	}

	prompt := userPrompt
	if systemPrompt != "" {
		prompt = systemPrompt + "\n\n" + userPrompt
	}

	out, err := scoutmcp.SampleWithMaxTokens(ctx, prompt, p.maxTokens)
	if err != nil {
		return "", fmt.Errorf("llm: mcp sampling: %w", err)
	}

	return string(out), nil
}

// IsMCPSamplingAvailable reports whether the MCP sampling session is
// currently wired. Useful for default-provider auto-selection — callers can
// pick MCPSamplingProvider when this returns true and fall back to
// configured providers (Ollama/OpenAI/Anthropic) otherwise.
func IsMCPSamplingAvailable() bool {
	return scoutmcp.HasSession()
}

// ErrNoMCPSession is re-exported for callers that need to errors.Is-check
// without importing pkg/mcp directly.
var ErrNoMCPSession = scoutmcp.ErrNoSession

// Compile-time assertions.
var (
	_ Provider           = (*MCPSamplingProvider)(nil)
	_ = errors.Is
)
