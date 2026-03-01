package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newFormTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<form id="login" action="/submit" method="post">
  <input type="text" name="username" required>
  <input type="password" name="password" required>
  <button type="submit">Log In</button>
</form>
</body></html>`))
	})
	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><h1>OK</h1></body></html>`))
	})
	return httptest.NewServer(mux)
}

func TestFormDetectTool(t *testing.T) {
	ts := newFormTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/form")

	// Detect all forms.
	result, err := callTool(ctx, cs, "form_detect", map[string]any{})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("form_detect: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("form_detect error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	var forms []json.RawMessage
	if err := json.Unmarshal([]byte(text), &forms); err != nil {
		t.Fatalf("unmarshal forms: %v", err)
	}
	if len(forms) == 0 {
		t.Error("expected at least one form")
	}

	// Detect specific form by selector.
	result, err = callTool(ctx, cs, "form_detect", map[string]any{"selector": "#login"})
	if err != nil {
		t.Fatalf("form_detect with selector: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		t.Fatalf("form_detect selector error: %s", text)
	}
}

func TestFormFillTool(t *testing.T) {
	ts := newFormTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/form")

	result, err := callTool(ctx, cs, "form_fill", map[string]any{
		"selector": "#login",
		"data":     map[string]any{"username": "admin", "password": "secret"},
	})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("form_fill: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("form_fill error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty confirmation")
	}
}

func TestFormSubmitTool(t *testing.T) {
	ts := newFormTestServer()
	defer ts.Close()

	cs := connectTestClient(t, ServerConfig{Headless: true, Logger: slog.Default()})
	ctx := context.Background()

	navigateHelper(t, ctx, cs, ts.URL+"/form")

	result, err := callTool(ctx, cs, "form_submit", map[string]any{"selector": "#login"})
	if err != nil {
		skipIfNoBrowser(t, err)
		t.Fatalf("form_submit: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcp.TextContent).Text
		skipIfNoBrowser(t, &toolErr{text})
		t.Fatalf("form_submit error: %s", text)
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty confirmation")
	}
}
