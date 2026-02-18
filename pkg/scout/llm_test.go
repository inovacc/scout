package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// mockProvider is a test LLM provider that returns canned responses.
type mockProvider struct {
	name     string
	response string
	err      error
	lastSys  string
	lastUser string
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Complete(_ context.Context, sys, user string) (string, error) {
	m.lastSys = sys
	m.lastUser = user
	return m.response, m.err
}

func TestExtractWithLLM(t *testing.T) {
	ts := newTestServer()
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/markdown")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	mock := &mockProvider{name: "mock", response: "Test output"}

	result, err := page.ExtractWithLLM("Summarize this page", WithLLMProvider(mock))
	if err != nil {
		t.Fatalf("ExtractWithLLM: %v", err)
	}

	if result != "Test output" {
		t.Errorf("got %q, want %q", result, "Test output")
	}

	if mock.lastSys == "" {
		t.Error("system prompt was empty")
	}
	if mock.lastUser == "" {
		t.Error("user prompt was empty")
	}
	if len(mock.lastUser) < len("Summarize this page") {
		t.Error("user prompt should contain the original prompt plus page content")
	}
}

func TestExtractWithLLMJSON(t *testing.T) {
	ts := newTestServer()
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/markdown")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	type Result struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}

	mock := &mockProvider{
		name:     "mock",
		response: `{"title":"Test","summary":"A test page"}`,
	}

	var got Result
	err = page.ExtractWithLLMJSON("Extract title and summary", &got, WithLLMProvider(mock))
	if err != nil {
		t.Fatalf("ExtractWithLLMJSON: %v", err)
	}

	if got.Title != "Test" {
		t.Errorf("title = %q, want %q", got.Title, "Test")
	}
	if got.Summary != "A test page" {
		t.Errorf("summary = %q, want %q", got.Summary, "A test page")
	}
}

func TestExtractWithLLMSchemaValidation(t *testing.T) {
	ts := newTestServer()
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/markdown")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	mock := &mockProvider{
		name:     "mock",
		response: "this is not json",
	}

	schema := json.RawMessage(`{"type":"object"}`)
	_, err = page.ExtractWithLLM("Extract data", WithLLMProvider(mock), WithLLMSchema(schema))
	if err == nil {
		t.Fatal("expected error for invalid JSON with schema validation")
	}
}

func TestExtractWithLLMNoProvider(t *testing.T) {
	ts := newTestServer()
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/markdown")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	_, err = page.ExtractWithLLM("Summarize")
	if err == nil {
		t.Fatal("expected error when no provider set")
	}
}

func TestExtractWithLLMProviderError(t *testing.T) {
	ts := newTestServer()
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/markdown")
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	mock := &mockProvider{
		name: "mock",
		err:  fmt.Errorf("connection refused"),
	}

	_, err = page.ExtractWithLLM("Summarize", WithLLMProvider(mock))
	if err == nil {
		t.Fatal("expected error from provider")
	}
}

func TestLLMOptions(t *testing.T) {
	o := defaultLLMOptions()

	WithLLMModel("gpt-4")(o)
	if o.model != "gpt-4" {
		t.Errorf("model = %q, want %q", o.model, "gpt-4")
	}

	WithLLMTemperature(0.7)(o)
	if o.temperature != 0.7 {
		t.Errorf("temperature = %v, want %v", o.temperature, 0.7)
	}

	WithLLMMaxTokens(1000)(o)
	if o.maxTokens != 1000 {
		t.Errorf("maxTokens = %d, want %d", o.maxTokens, 1000)
	}

	WithLLMTimeout(30 * time.Second)(o)
	if o.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want %v", o.timeout, 30*time.Second)
	}

	WithLLMSystemPrompt("custom")(o)
	if o.systemPrompt != "custom" {
		t.Errorf("systemPrompt = %q, want %q", o.systemPrompt, "custom")
	}

	schema := json.RawMessage(`{"type":"object"}`)
	WithLLMSchema(schema)(o)
	if string(o.schema) != `{"type":"object"}` {
		t.Errorf("schema = %s, want %s", o.schema, schema)
	}
}

func TestLLMDefaultSystemPrompt(t *testing.T) {
	o := defaultLLMOptions()
	if o.systemPrompt == "" {
		t.Error("default system prompt should not be empty")
	}
}

func TestLLMMainContentOnly(t *testing.T) {
	o := defaultLLMOptions()
	if !o.mainOnly {
		t.Error("mainOnly should default to true")
	}

	o.mainOnly = false
	WithLLMMainContent()(o)
	if !o.mainOnly {
		t.Error("WithLLMMainContent should set mainOnly to true")
	}
}
