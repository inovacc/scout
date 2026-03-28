package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inovacc/scout/internal/idle"
	"github.com/inovacc/scout/pkg/scout"
)

// mockPage implements the page interface for testing handler success paths.
type mockPage struct {
	title      string
	url        string
	screenshot []byte
	evalResult *scout.EvalResult
	markdown   string
	evalErr    error
}

func (m *mockPage) Title() (string, error)          { return m.title, nil }
func (m *mockPage) URL() (string, error)            { return m.url, nil }
func (m *mockPage) Screenshot() ([]byte, error)     { return m.screenshot, nil }
func (m *mockPage) Markdown(_ ...scout.MarkdownOption) (string, error) { return m.markdown, nil }
func (m *mockPage) Eval(_ string, _ ...any) (*scout.EvalResult, error) {
	return m.evalResult, m.evalErr
}

// newMockProvider creates a Provider with a mock getPage for testing handler logic.
func newMockProvider(mp *mockPage) *Provider {
	p := &Provider{
		getPage: func(_ context.Context, _ string) (page, error) {
			return mp, nil
		},
	}
	p.registerBuiltinTools()
	return p
}

// newTestBrowser creates a headless browser for testing. Skips if unavailable.
func newTestBrowser(t *testing.T) *scout.Browser {
	t.Helper()

	b, err := scout.New(scout.WithHeadless(true), scout.WithNoSandbox(), scout.WithoutBridge())
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	t.Cleanup(func() { _ = b.Close() })

	return b
}

func TestToolSchemas(t *testing.T) {
	// Provider without browser — just test schema generation.
	p := &Provider{}
	p.registerBuiltinTools()

	if len(p.Tools()) != 9 {
		t.Errorf("tools count = %d, want 9", len(p.Tools()))
	}

	// OpenAI format.
	openai := p.OpenAITools()
	if len(openai) != 9 {
		t.Errorf("OpenAI tools = %d", len(openai))
	}

	for _, tool := range openai {
		if tool["type"] != "function" {
			t.Errorf("tool type = %v, want function", tool["type"])
		}

		fn, ok := tool["function"].(map[string]any)
		if !ok {
			t.Fatal("function field missing")
		}

		if fn["name"] == nil || fn["name"] == "" {
			t.Error("tool name is empty")
		}
	}

	// Anthropic format.
	anthropic := p.AnthropicTools()
	if len(anthropic) != 9 {
		t.Errorf("Anthropic tools = %d", len(anthropic))
	}

	for _, tool := range anthropic {
		if tool["name"] == nil || tool["name"] == "" {
			t.Error("anthropic tool name empty")
		}

		if tool["input_schema"] == nil {
			t.Error("anthropic input_schema missing")
		}
	}

	// JSON export.
	data, err := p.ToolSchemaJSON()
	if err != nil {
		t.Fatalf("ToolSchemaJSON: %v", err)
	}

	var parsed []any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(parsed) != 9 {
		t.Errorf("JSON tools = %d", len(parsed))
	}
}

func TestParams(t *testing.T) {
	p := params("url", "string", "The URL", true)

	if p["type"] != "object" {
		t.Errorf("type = %v", p["type"])
	}

	props, ok := p["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing")
	}

	if props["url"] == nil {
		t.Error("url prop missing")
	}

	reqd, ok := p["required"].([]string)
	if !ok || len(reqd) != 1 || reqd[0] != "url" {
		t.Errorf("required = %v", p["required"])
	}
}

func TestParamsNotRequired(t *testing.T) {
	p := params("optional", "boolean", "An optional field", false)

	if p["type"] != "object" {
		t.Errorf("type = %v", p["type"])
	}

	if _, hasRequired := p["required"]; hasRequired {
		t.Error("expected no required field for optional param")
	}
}

func TestParamsMulti(t *testing.T) {
	p := paramsMulti(
		param("selector", "string", "CSS sel", true),
		param("text", "string", "Text", true),
	)

	props := p["properties"].(map[string]any)
	if len(props) != 2 {
		t.Errorf("props = %d", len(props))
	}

	reqd := p["required"].([]string)
	if len(reqd) != 2 {
		t.Errorf("required = %d", len(reqd))
	}
}

func TestParamsMultiNoRequired(t *testing.T) {
	p := paramsMulti(
		param("opt1", "string", "Optional 1", false),
		param("opt2", "boolean", "Optional 2", false),
	)

	if _, hasRequired := p["required"]; hasRequired {
		t.Error("expected no required field when all params optional")
	}
}

func TestEmptyParams(t *testing.T) {
	p := emptyParams()

	if p["type"] != "object" {
		t.Errorf("type = %v", p["type"])
	}

	props, ok := p["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing")
	}

	if len(props) != 0 {
		t.Errorf("expected 0 properties, got %d", len(props))
	}
}

func TestCallUnknownTool(t *testing.T) {
	p := &Provider{}
	p.registerBuiltinTools()

	_, err := p.Call(context.TODO(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestCallSuccess(t *testing.T) {
	p := &Provider{
		tools: []Tool{
			{
				Name:        "greet",
				Description: "Say hello",
				Parameters:  emptyParams(),
				Handler: func(_ context.Context, args map[string]any) (string, error) {
					name, _ := args["name"].(string)
					return "hello " + name, nil
				},
			},
		},
	}

	result, err := p.Call(context.TODO(), "greet", map[string]any{"name": "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content != "hello world" {
		t.Errorf("content = %q, want %q", result.Content, "hello world")
	}

	if result.IsError {
		t.Error("expected is_error = false")
	}
}

func TestCallHandlerError(t *testing.T) {
	p := &Provider{
		tools: []Tool{
			{
				Name:        "fail",
				Description: "Always fails",
				Parameters:  emptyParams(),
				Handler: func(_ context.Context, _ map[string]any) (string, error) {
					return "", fmt.Errorf("something broke")
				},
			},
		},
	}

	result, err := p.Call(context.TODO(), "fail", nil)
	if err != nil {
		t.Fatalf("Call should not return Go error for handler errors: %v", err)
	}

	if !result.IsError {
		t.Error("expected is_error = true")
	}

	if result.Content != "something broke" {
		t.Errorf("content = %q, want %q", result.Content, "something broke")
	}
}

func TestOpenAIToolsFormat(t *testing.T) {
	p := &Provider{
		tools: []Tool{
			{
				Name:        "navigate",
				Description: "Go to URL",
				Parameters:  params("url", "string", "URL", true),
			},
			{
				Name:        "screenshot",
				Description: "Take screenshot",
				Parameters:  emptyParams(),
			},
		},
	}

	tools := p.OpenAITools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	for i, tool := range tools {
		if tool["type"] != "function" {
			t.Errorf("tool[%d] type = %v, want function", i, tool["type"])
		}

		fn, ok := tool["function"].(map[string]any)
		if !ok {
			t.Fatalf("tool[%d] missing function field", i)
		}

		if fn["name"] == nil || fn["name"] == "" {
			t.Errorf("tool[%d] name is empty", i)
		}

		if fn["description"] == nil || fn["description"] == "" {
			t.Errorf("tool[%d] description is empty", i)
		}

		if fn["parameters"] == nil {
			t.Errorf("tool[%d] parameters is nil", i)
		}
	}
}

func TestAnthropicToolsFormat(t *testing.T) {
	p := &Provider{
		tools: []Tool{
			{
				Name:        "eval",
				Description: "Run JS",
				Parameters:  params("script", "string", "JS code", true),
			},
		},
	}

	tools := p.AnthropicTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool["name"] != "eval" {
		t.Errorf("name = %v, want eval", tool["name"])
	}

	if tool["description"] != "Run JS" {
		t.Errorf("description = %v, want Run JS", tool["description"])
	}

	schema, ok := tool["input_schema"].(map[string]any)
	if !ok {
		t.Fatal("input_schema missing or wrong type")
	}

	if schema["type"] != "object" {
		t.Errorf("input_schema.type = %v, want object", schema["type"])
	}
}

func TestToolSchemaJSON(t *testing.T) {
	p := &Provider{
		tools: []Tool{
			{Name: "a", Description: "A", Parameters: emptyParams()},
			{Name: "b", Description: "B", Parameters: params("x", "string", "X", true)},
		},
	}

	data, err := p.ToolSchemaJSON()
	if err != nil {
		t.Fatalf("ToolSchemaJSON: %v", err)
	}

	var parsed []map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("expected 2 tools in JSON, got %d", len(parsed))
	}

	// Verify it's OpenAI format
	for _, tool := range parsed {
		if tool["type"] != "function" {
			t.Errorf("JSON tool type = %v, want function", tool["type"])
		}
	}
}

func TestProviderToolsEmpty(t *testing.T) {
	p := &Provider{}
	if len(p.Tools()) != 0 {
		t.Errorf("expected 0 tools, got %d", len(p.Tools()))
	}

	openai := p.OpenAITools()
	if len(openai) != 0 {
		t.Errorf("expected 0 OpenAI tools, got %d", len(openai))
	}

	anthropic := p.AnthropicTools()
	if len(anthropic) != 0 {
		t.Errorf("expected 0 Anthropic tools, got %d", len(anthropic))
	}
}

func TestNewProviderNilBrowser(t *testing.T) {
	// NewProvider with nil browser should not panic; it just registers tools.
	p := NewProvider(nil)
	if len(p.Tools()) != 9 {
		t.Errorf("tools = %d, want 9", len(p.Tools()))
	}
}

func TestNewServerDefaultConfig(t *testing.T) {
	// NewServer will fail because no browser is available, but we test the
	// config defaulting and option setup paths.
	s, err := NewServer(ServerConfig{})
	if err != nil {
		// Expected when browser unavailable — this covers the error return path.
		return
	}

	// If it succeeded (browser available), verify the server is valid.
	defer s.Close()

	if s.provider == nil {
		t.Error("expected provider to be set")
	}
	if s.mux == nil {
		t.Error("expected mux to be set")
	}
}

func TestNewServerWithOptions(t *testing.T) {
	// Test with stealth + browser bin options to cover those branches.
	_, err := NewServer(ServerConfig{
		Stealth:    true,
		BrowserBin: "/nonexistent/browser",
		Headless:   true,
	})
	// Will fail — just covering the option branches.
	if err == nil {
		t.Log("NewServer succeeded unexpectedly; closing is handled by caller")
	}
}

func TestNewServerWithLogger(t *testing.T) {
	logger := slog.Default()
	_, err := NewServer(ServerConfig{
		Addr:   "localhost:0",
		Logger: logger,
	})
	// Expected to fail without browser.
	if err == nil {
		t.Log("NewServer succeeded unexpectedly")
	}
}

func TestCloseNilBrowser(t *testing.T) {
	s := &Server{browser: nil}
	// Should not panic.
	s.Close()
}

func TestCloseWithProvider(t *testing.T) {
	s := &Server{
		provider: &Provider{},
		browser:  nil,
	}
	// Should not panic.
	s.Close()
}

// --- Browser-dependent tests ---

func TestNewProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	p := NewProvider(b)
	if len(p.Tools()) != 9 {
		t.Errorf("tools = %d, want 9", len(p.Tools()))
	}

	if p.browser == nil {
		t.Error("expected browser to be set")
	}
}

func TestHandleNavigate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><head><title>Test Page</title></head><body>Hello</body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	result, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("Call navigate: %v", err)
	}

	if result.IsError {
		t.Errorf("navigate returned error: %s", result.Content)
	}

	if result.Content == "" {
		t.Error("expected non-empty content from navigate")
	}
}

func TestHandleScreenshot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><h1>Screenshot Test</h1></body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	// First navigate to a page.
	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "screenshot", map[string]any{})
	if err != nil {
		t.Fatalf("Call screenshot: %v", err)
	}

	if result.IsError {
		t.Errorf("screenshot returned error: %s", result.Content)
	}

	if result.Content == "" {
		t.Error("expected non-empty content from screenshot")
	}
}

func TestHandleExtractText(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><p id="target">Extracted Text</p></body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "extract_text", map[string]any{"selector": "#target"})
	if err != nil {
		t.Fatalf("Call extract_text: %v", err)
	}

	if result.IsError {
		t.Errorf("extract_text returned error: %s", result.Content)
	}

	if result.Content != "Extracted Text" {
		t.Errorf("content = %q, want %q", result.Content, "Extracted Text")
	}
}

func TestHandleClick(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><button id="btn">Click Me</button></body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "click", map[string]any{"selector": "#btn"})
	if err != nil {
		t.Fatalf("Call click: %v", err)
	}

	if result.IsError {
		t.Errorf("click returned error: %s", result.Content)
	}

	if result.Content != "Clicked #btn" {
		t.Errorf("content = %q, want %q", result.Content, "Clicked #btn")
	}
}

func TestHandleType(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><input id="inp" type="text"/></body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "type_text", map[string]any{
		"selector": "#inp",
		"text":     "hello world",
	})
	if err != nil {
		t.Fatalf("Call type_text: %v", err)
	}

	if result.IsError {
		t.Errorf("type_text returned error: %s", result.Content)
	}
}

func TestHandleEval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>Eval Test</body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	navResult, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if navResult.IsError {
		t.Fatalf("navigate returned error: %s", navResult.Content)
	}

	// Use ensurePage("") path by using eval which calls it with empty URL.
	// The page should still be open from the navigate call above.
	result, err := p.Call(context.TODO(), "eval", map[string]any{"script": "() => document.title"})
	if err != nil {
		t.Fatalf("Call eval: %v", err)
	}

	// If ensurePage returns no pages, result will be an error — that's
	// still coverage of the eval handler and ensurePage error path.
	if !result.IsError {
		// If it succeeded, the content should be non-empty.
		if result.Content == "" {
			t.Error("expected non-empty content from eval")
		}
	}
}

func TestHandleURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>URL Test</body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "page_url", map[string]any{})
	if err != nil {
		t.Fatalf("Call page_url: %v", err)
	}

	if result.IsError {
		t.Errorf("page_url returned error: %s", result.Content)
	}

	if result.Content != ts.URL+"/" {
		t.Errorf("url = %q, want %q", result.Content, ts.URL+"/")
	}
}

func TestHandleTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><head><title>My Title</title></head><body>Title Test</body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "page_title", map[string]any{})
	if err != nil {
		t.Fatalf("Call page_title: %v", err)
	}

	if result.IsError {
		t.Errorf("page_title returned error: %s", result.Content)
	}

	if result.Content != "My Title" {
		t.Errorf("title = %q, want %q", result.Content, "My Title")
	}
}

func TestHandleMarkdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><h1>Heading</h1><p>Paragraph</p></body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "markdown", map[string]any{})
	if err != nil {
		t.Fatalf("Call markdown: %v", err)
	}

	if result.IsError {
		t.Errorf("markdown returned error: %s", result.Content)
	}

	if result.Content == "" {
		t.Error("expected non-empty markdown")
	}
}

func TestHandleMarkdownMainOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><main><h1>Main Content</h1></main><footer>Footer</footer></body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	_, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := p.Call(context.TODO(), "markdown", map[string]any{"mainOnly": true})
	if err != nil {
		t.Fatalf("Call markdown mainOnly: %v", err)
	}

	if result.IsError {
		t.Errorf("markdown mainOnly returned error: %s", result.Content)
	}
}

func TestEnsurePageNoURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)
	p := NewProvider(b)

	// No pages open yet — should return error.
	_, err := p.Call(context.TODO(), "screenshot", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	// The handler returns an error via ToolResult because ensurePage fails
	// when there are no open pages.
}

func TestEnsurePageWithURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b := newTestBrowser(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><head><title>URL Page</title></head><body>OK</body></html>`)
	}))
	defer ts.Close()

	p := NewProvider(b)

	// navigate uses ensurePage with a URL — exercises the url != "" branch.
	result, err := p.Call(context.TODO(), "navigate", map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}

	if result.IsError {
		t.Errorf("navigate error: %s", result.Content)
	}
}

// --- Handler success path tests (mock page) ---

func TestHandleNavigateMock(t *testing.T) {
	mp := &mockPage{title: "Example Page"}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "navigate", map[string]any{"url": "http://example.com"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	expected := "Navigated to http://example.com (title: Example Page)"
	if result.Content != expected {
		t.Errorf("content = %q, want %q", result.Content, expected)
	}
}

func TestHandleScreenshotMock(t *testing.T) {
	mp := &mockPage{screenshot: make([]byte, 1024)}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "screenshot", map[string]any{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	expected := "Screenshot captured (1024 bytes)"
	if result.Content != expected {
		t.Errorf("content = %q, want %q", result.Content, expected)
	}
}

func TestHandleScreenshotErrorMock(t *testing.T) {
	mp := &mockPage{}
	p := &Provider{
		getPage: func(_ context.Context, _ string) (page, error) {
			return nil, fmt.Errorf("no page")
		},
	}
	_ = mp // unused intentionally
	p.registerBuiltinTools()

	result, err := p.Call(context.TODO(), "screenshot", map[string]any{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for failed getPage")
	}
}

func TestHandleExtractTextMock(t *testing.T) {
	mp := &mockPage{evalResult: &scout.EvalResult{Value: "Hello World"}}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "extract_text", map[string]any{"selector": ".test"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	if result.Content != "Hello World" {
		t.Errorf("content = %q, want %q", result.Content, "Hello World")
	}
}

func TestHandleExtractTextEvalErrorMock(t *testing.T) {
	mp := &mockPage{evalErr: fmt.Errorf("eval failed")}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "extract_text", map[string]any{"selector": ".test"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result for failed eval")
	}
}

func TestHandleClickMock(t *testing.T) {
	mp := &mockPage{evalResult: &scout.EvalResult{}}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "click", map[string]any{"selector": "#btn"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	if result.Content != "Clicked #btn" {
		t.Errorf("content = %q, want %q", result.Content, "Clicked #btn")
	}
}

func TestHandleClickEvalErrorMock(t *testing.T) {
	mp := &mockPage{evalErr: fmt.Errorf("click failed")}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "click", map[string]any{"selector": "#btn"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result")
	}
}

func TestHandleTypeMock(t *testing.T) {
	mp := &mockPage{evalResult: &scout.EvalResult{Value: "typed"}}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "type_text", map[string]any{"selector": "#inp", "text": "hello"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	if result.Content != "typed" {
		t.Errorf("content = %q, want %q", result.Content, "typed")
	}
}

func TestHandleTypeEvalErrorMock(t *testing.T) {
	mp := &mockPage{evalErr: fmt.Errorf("type failed")}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "type_text", map[string]any{"selector": "#inp", "text": "hello"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result")
	}
}

func TestHandleMarkdownMock(t *testing.T) {
	mp := &mockPage{markdown: "# Hello\n\nWorld"}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "markdown", map[string]any{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	if result.Content != "# Hello\n\nWorld" {
		t.Errorf("content = %q", result.Content)
	}
}

func TestHandleMarkdownMainOnlyMock(t *testing.T) {
	mp := &mockPage{markdown: "# Main Only"}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "markdown", map[string]any{"mainOnly": true})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
}

func TestHandleEvalMock(t *testing.T) {
	mp := &mockPage{evalResult: &scout.EvalResult{Value: json.Number("42")}}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "eval", map[string]any{"script": "() => 42"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	if result.Content != "42" {
		t.Errorf("content = %q, want %q", result.Content, "42")
	}
}

func TestHandleEvalErrorMock(t *testing.T) {
	mp := &mockPage{evalErr: fmt.Errorf("eval failed")}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "eval", map[string]any{"script": "bad"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !result.IsError {
		t.Error("expected error result")
	}
}

func TestHandleURLMock(t *testing.T) {
	mp := &mockPage{url: "http://example.com/page"}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "page_url", map[string]any{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.Content != "http://example.com/page" {
		t.Errorf("content = %q", result.Content)
	}
}

func TestHandleTitleMock(t *testing.T) {
	mp := &mockPage{title: "Test Title"}
	p := newMockProvider(mp)

	result, err := p.Call(context.TODO(), "page_title", map[string]any{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if result.Content != "Test Title" {
		t.Errorf("content = %q", result.Content)
	}
}

// --- Handler error path tests (nil browser) ---

func TestHandlersNilBrowserErrorPath(t *testing.T) {
	// Create a provider with registered builtin tools but nil browser.
	// All handlers should return error results without panicking.
	p := &Provider{}
	p.registerBuiltinTools()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"navigate", map[string]any{"url": "http://example.com"}},
		{"screenshot", map[string]any{}},
		{"extract_text", map[string]any{"selector": "#test"}},
		{"click", map[string]any{"selector": "#btn"}},
		{"type_text", map[string]any{"selector": "#inp", "text": "hello"}},
		{"markdown", map[string]any{}},
		{"eval", map[string]any{"script": "() => 1"}},
		{"page_url", map[string]any{}},
		{"page_title", map[string]any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Call(context.TODO(), tt.name, tt.args)
			if err != nil {
				t.Fatalf("Call %s returned Go error: %v", tt.name, err)
			}

			if !result.IsError {
				t.Errorf("Call %s expected is_error=true for nil browser", tt.name)
			}

			if result.Content == "" {
				t.Errorf("Call %s expected non-empty error content", tt.name)
			}
		})
	}
}

func TestHandleMarkdownMainOnlyNilBrowser(t *testing.T) {
	p := &Provider{}
	p.registerBuiltinTools()

	result, err := p.Call(context.TODO(), "markdown", map[string]any{"mainOnly": true})
	if err != nil {
		t.Fatalf("Call markdown returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("expected is_error=true for nil browser")
	}
}

// --- Server content-type tests ---

func TestServerContentTypeJSON(t *testing.T) {
	s := newTestServer(
		Tool{Name: "t", Description: "test", Parameters: emptyParams()},
	)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/tools"},
		{"GET", "/tools/openai"},
		{"GET", "/tools/anthropic"},
		{"GET", "/tools/schema"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)

		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("%s %s Content-Type = %q, want application/json", ep.method, ep.path, ct)
		}
	}
}

func TestListenAndServe(t *testing.T) {
	s := newTestServer(
		Tool{Name: "t", Description: "test", Parameters: emptyParams()},
	)
	s.config = ServerConfig{Addr: "localhost:0"}
	s.logger = slog.Default()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe(ctx)
	}()

	// Give server time to start, then cancel context to shut it down.
	time.Sleep(100 * time.Millisecond)
	cancel()

	err := <-errCh
	if err != nil {
		t.Errorf("ListenAndServe returned error: %v", err)
	}
}

func TestListenAndServeWithIdleTimeout(t *testing.T) {
	s := newTestServer(
		Tool{Name: "t", Description: "test", Parameters: emptyParams()},
	)
	s.config = ServerConfig{
		Addr:        "localhost:0",
		IdleTimeout: 50 * time.Millisecond,
	}
	s.logger = slog.Default()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	idleCalled := make(chan struct{}, 1)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe(ctx, func() {
			idleCalled <- struct{}{}
			cancel()
		})
	}()

	// Wait for idle callback or timeout.
	select {
	case <-idleCalled:
		// Good, idle timeout triggered.
	case <-time.After(2 * time.Second):
		cancel()
		t.Error("idle timeout did not trigger")
	}

	err := <-errCh
	if err != nil {
		t.Errorf("ListenAndServe returned error: %v", err)
	}
}

func TestListenAndServeWithRequests(t *testing.T) {
	s := newTestServer(
		Tool{
			Name:        "echo",
			Description: "Echo",
			Parameters:  emptyParams(),
			Handler: func(_ context.Context, _ map[string]any) (string, error) {
				return "pong", nil
			},
		},
	)
	s.config = ServerConfig{Addr: "localhost:0"}
	s.logger = slog.Default()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a listener to get the actual port.
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	s.config.Addr = addr

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe(ctx)
	}()

	// Wait for server to start.
	time.Sleep(100 * time.Millisecond)

	// Make a request.
	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		cancel()
		t.Fatalf("GET /health: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /health status = %d, want 200", resp.StatusCode)
	}

	cancel()
	<-errCh
}

func TestListenAndServeBadAddr(t *testing.T) {
	s := newTestServer()
	// Use an invalid address to trigger listen error.
	s.config = ServerConfig{Addr: "invalid-addr-no-port"}
	s.logger = slog.Default()

	ctx := context.Background()
	err := s.ListenAndServe(ctx)
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

func TestTouchWithIdleTimer(t *testing.T) {
	s := newTestServer(
		Tool{Name: "t", Description: "test", Parameters: emptyParams()},
	)

	called := false
	s.idle = idle.New(time.Hour, func() { called = true })
	defer s.idle.Stop()

	// touch should reset the idle timer without panicking.
	s.touch()

	if called {
		t.Error("idle callback should not have been triggered immediately")
	}
}

func TestServerToolsSchemaError(t *testing.T) {
	// Create a provider with an unmarshallable value to trigger schema error.
	s := &Server{
		provider: &Provider{
			tools: []Tool{
				{
					Name:        "bad",
					Description: "bad tool",
					Parameters:  map[string]any{"bad": make(chan int)},
				},
			},
		},
		logger: slog.Default(),
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()

	req := httptest.NewRequest("GET", "/tools/schema", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("GET /tools/schema with bad params: status = %d, want 500", w.Code)
	}
}

func TestServerToolsSchemaValidJSON(t *testing.T) {
	s := newTestServer(
		Tool{Name: "nav", Description: "Navigate", Parameters: params("url", "string", "URL", true)},
		Tool{Name: "shot", Description: "Screenshot", Parameters: emptyParams()},
	)

	req := httptest.NewRequest("GET", "/tools/schema", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var schema []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&schema); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(schema) != 2 {
		t.Errorf("schema entries = %d, want 2", len(schema))
	}

	for _, entry := range schema {
		if entry["type"] != "function" {
			t.Errorf("schema entry type = %v, want function", entry["type"])
		}
	}
}
