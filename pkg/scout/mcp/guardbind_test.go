package mcp

import "testing"

// TestGuardSSEBind verifies the unauthenticated MCP SSE transport fails closed on
// a non-loopback bind unless the explicit opt-in env is set.
func TestGuardSSEBind(t *testing.T) {
	t.Setenv("SCOUT_MCP_ALLOW_REMOTE", "")

	for _, addr := range []string{"localhost:8080", "127.0.0.1:8080", "[::1]:8080"} {
		if err := guardSSEBind(addr); err != nil {
			t.Errorf("loopback %q rejected: %v", addr, err)
		}
	}

	for _, addr := range []string{"0.0.0.0:8080", ":8080", "192.168.0.10:8080"} {
		if err := guardSSEBind(addr); err == nil {
			t.Errorf("non-loopback %q accepted without opt-in", addr)
		}
	}

	t.Setenv("SCOUT_MCP_ALLOW_REMOTE", "1")
	if err := guardSSEBind("0.0.0.0:8080"); err != nil {
		t.Errorf("opt-in env still rejected: %v", err)
	}
}
