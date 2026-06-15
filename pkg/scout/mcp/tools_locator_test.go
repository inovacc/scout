package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const locatorTestHTML = `<!DOCTYPE html><html><head><title>Loc Test</title></head><body>
<h1>Welcome</h1>
<button id="b">Submit</button>
<label for="user">Username</label><input id="user" type="text">
<div data-testid="greeting">Hello Scout</div>
<ul><li>one</li><li>two</li><li>three</li></ul>
</body></html>`

func locatorTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(locatorTestHTML))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// resultText returns the first text content of a tool result.
func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatalf("tool result had no content")
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool result content was not text: %T", r.Content[0])
	}
	return tc.Text
}

func TestLocatorMCPTools(t *testing.T) {
	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()
	srv := locatorTestServer(t)
	navigateHelper(t, ctx, cs, srv.URL)

	t.Run("locator_text by testid", func(t *testing.T) {
		r, err := callTool(ctx, cs, "locator_text", map[string]any{"strategy": "testid", "value": "greeting"})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if r.IsError {
			t.Fatalf("error: %s", resultText(t, r))
		}
		if got := resultText(t, r); got != "Hello Scout" {
			t.Fatalf("text=%q want %q", got, "Hello Scout")
		}
	})

	t.Run("locator_count listitem", func(t *testing.T) {
		r, err := callTool(ctx, cs, "locator_count", map[string]any{"strategy": "role", "value": "listitem"})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if got := resultText(t, r); got != "3" {
			t.Fatalf("count=%q want 3", got)
		}
	})

	t.Run("locator_fill then expect", func(t *testing.T) {
		if _, err := callTool(ctx, cs, "locator_fill", map[string]any{"strategy": "label", "value": "Username", "text": "alice"}); err != nil {
			t.Fatalf("fill: %v", err)
		}
		// Verify via eval that the value stuck.
		r, err := callTool(ctx, cs, "eval", map[string]any{"expression": "() => document.getElementById('user').value"})
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if got := resultText(t, r); got != "alice" {
			t.Fatalf("input value=%q want alice", got)
		}
	})

	t.Run("expect_visible", func(t *testing.T) {
		r, err := callTool(ctx, cs, "expect_visible", map[string]any{"strategy": "role", "value": "button", "name": "Submit"})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if r.IsError {
			t.Fatalf("expect_visible error: %s", resultText(t, r))
		}
	})

	t.Run("expect_text contains", func(t *testing.T) {
		r, err := callTool(ctx, cs, "expect_text", map[string]any{"strategy": "role", "value": "heading", "text": "Welcome", "contains": true})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if r.IsError {
			t.Fatalf("expect_text error: %s", resultText(t, r))
		}
	})

	t.Run("expect_visible negate on missing element", func(t *testing.T) {
		r, err := callTool(ctx, cs, "expect_visible", map[string]any{"strategy": "text", "value": "does-not-exist", "negate": true, "timeout_ms": 1000})
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		if r.IsError {
			t.Fatalf("negated expect_visible should pass for missing element: %s", resultText(t, r))
		}
	})
}
