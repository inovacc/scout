package llm

import (
	"context"
	"errors"
	"testing"

	scoutmcp "github.com/inovacc/scout/pkg/mcp"
)

// TestMCPSamplingProvider_Name verifies the provider identifier.
func TestMCPSamplingProvider_Name(t *testing.T) {
	p := NewMCPSamplingProvider()
	if got := p.Name(); got != "mcp-sampling" {
		t.Errorf("Name() = %q, want %q", got, "mcp-sampling")
	}
}

// TestMCPSamplingProvider_NoSession verifies Complete errors with
// ErrNoMCPSession when no MCP session is wired. This is the degraded path
// that callers can errors.Is-check to fall back to another provider.
func TestMCPSamplingProvider_NoSession(t *testing.T) {
	// Ensure no session is wired.
	scoutmcp.SetSession(nil, nil)

	p := NewMCPSamplingProvider()

	_, err := p.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("Complete with no session: want error, got nil")
	}

	if !errors.Is(err, scoutmcp.ErrNoSession) {
		t.Errorf("Complete err = %v, want errors.Is ErrNoSession", err)
	}
}

// TestMCPSamplingProvider_OptionsApply verifies that options mutate the
// provider's max-token budget.
func TestMCPSamplingProvider_OptionsApply(t *testing.T) {
	p := NewMCPSamplingProvider(WithMCPSamplingMaxTokens(8000))
	if p.maxTokens != 8000 {
		t.Errorf("maxTokens = %d, want 8000", p.maxTokens)
	}
}

// TestMCPSamplingProvider_DefaultMaxTokens verifies the default budget is
// higher than pkg/mcp's 2000 default — extraction needs more headroom.
func TestMCPSamplingProvider_DefaultMaxTokens(t *testing.T) {
	p := NewMCPSamplingProvider()
	if p.maxTokens < 4000 {
		t.Errorf("default maxTokens = %d, want >= 4000", p.maxTokens)
	}
}

// TestIsMCPSamplingAvailable_NoSession verifies the availability probe
// returns false when no session is wired.
func TestIsMCPSamplingAvailable_NoSession(t *testing.T) {
	scoutmcp.SetSession(nil, nil)

	if IsMCPSamplingAvailable() {
		t.Error("IsMCPSamplingAvailable() = true, want false (no session)")
	}
}

// TestMCPSamplingProvider_ImplementsProvider is a compile-time-style assertion
// that MCPSamplingProvider satisfies the Provider interface.
func TestMCPSamplingProvider_ImplementsProvider(t *testing.T) {
	var p Provider = NewMCPSamplingProvider()
	_ = p
}
