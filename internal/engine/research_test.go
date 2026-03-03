package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// researchMockLLM is a test double for LLMProvider.
type researchMockLLM struct {
	name     string
	response string
	err      error
	calls    []researchMockCall
}

type researchMockCall struct {
	SystemPrompt string
	UserPrompt   string
}

func (m *researchMockLLM) Name() string { return m.name }

func (m *researchMockLLM) Complete(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	m.calls = append(m.calls, researchMockCall{SystemPrompt: systemPrompt, UserPrompt: userPrompt})
	return m.response, m.err
}

func TestResearchAgent_NilBrowser(t *testing.T) {
	mock := &researchMockLLM{name: "mock", response: "{}"}
	agent := NewResearchAgent(nil, mock)

	_, err := agent.Research(context.Background(), "test query")
	if err == nil {
		t.Fatal("expected error for nil browser")
	}

	if !strings.Contains(err.Error(), "browser is nil") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.DeepResearch(context.Background(), "test query")
	if err == nil {
		t.Fatal("expected error for nil browser in DeepResearch")
	}
}

func TestResearchAgent_NilProvider(t *testing.T) {
	// Use a non-nil browser placeholder — Research will fail before using it
	b := &Browser{}
	agent := NewResearchAgent(b, nil)

	_, err := agent.Research(context.Background(), "test query")
	if err == nil {
		t.Fatal("expected error for nil provider")
	}

	if !strings.Contains(err.Error(), "LLM provider is nil") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = agent.DeepResearch(context.Background(), "test query")
	if err == nil {
		t.Fatal("expected error for nil provider in DeepResearch")
	}
}

func TestResearchAgent_Options(t *testing.T) {
	agent := NewResearchAgent(nil, nil,
		WithResearchMaxSources(10),
		WithResearchDepth(3),
		WithResearchTimeout(5*time.Minute),
		WithResearchFetchMode("text"),
		WithResearchConcurrency(5),
		WithResearchEngine(Bing),
		WithResearchMainContent(false),
	)

	if agent.opts.maxSources != 10 {
		t.Errorf("maxSources = %d, want 10", agent.opts.maxSources)
	}

	if agent.opts.maxDepth != 3 {
		t.Errorf("maxDepth = %d, want 3", agent.opts.maxDepth)
	}

	if agent.opts.timeout != 5*time.Minute {
		t.Errorf("timeout = %v, want 5m", agent.opts.timeout)
	}

	if agent.opts.fetchMode != "text" {
		t.Errorf("fetchMode = %q, want text", agent.opts.fetchMode)
	}

	if agent.opts.concurrency != 5 {
		t.Errorf("concurrency = %d, want 5", agent.opts.concurrency)
	}

	if agent.opts.engine != Bing {
		t.Errorf("engine = %v, want Bing", agent.opts.engine)
	}

	if agent.opts.mainOnly {
		t.Error("mainOnly should be false")
	}
}

func TestResearchAgent_Defaults(t *testing.T) {
	agent := NewResearchAgent(nil, nil)

	if agent.opts.maxSources != 5 {
		t.Errorf("default maxSources = %d, want 5", agent.opts.maxSources)
	}

	if agent.opts.maxDepth != 1 {
		t.Errorf("default maxDepth = %d, want 1", agent.opts.maxDepth)
	}

	if agent.opts.fetchMode != "markdown" {
		t.Errorf("default fetchMode = %q, want markdown", agent.opts.fetchMode)
	}

	if agent.opts.timeout != 2*time.Minute {
		t.Errorf("default timeout = %v, want 2m", agent.opts.timeout)
	}

	if !agent.opts.mainOnly {
		t.Error("default mainOnly should be true")
	}
}

func TestResearchResult_Fields(t *testing.T) {
	r := &ResearchResult{
		Query:   "test query",
		Summary: "Test summary",
		Sources: []ResearchSource{
			{URL: "https://example.com", Title: "Example", Content: "content", Relevance: 0.9},
		},
		FollowUpQuestions: []string{"q1", "q2"},
		Duration:          5 * time.Second,
		Depth:             2,
	}

	if r.Query != "test query" {
		t.Errorf("Query = %q, want 'test query'", r.Query)
	}

	if r.Summary != "Test summary" {
		t.Errorf("Summary = %q, want 'Test summary'", r.Summary)
	}

	if len(r.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(r.Sources))
	}

	if r.Sources[0].URL != "https://example.com" {
		t.Errorf("Source URL = %q", r.Sources[0].URL)
	}

	if r.Sources[0].Relevance != 0.9 {
		t.Errorf("Source Relevance = %f, want 0.9", r.Sources[0].Relevance)
	}

	if len(r.FollowUpQuestions) != 2 {
		t.Errorf("FollowUpQuestions len = %d, want 2", len(r.FollowUpQuestions))
	}

	if r.Depth != 2 {
		t.Errorf("Depth = %d, want 2", r.Depth)
	}

	// JSON serialization roundtrip
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded ResearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.Query != r.Query {
		t.Errorf("decoded Query = %q", decoded.Query)
	}
}

func TestResearchAgent_BuildPrompt(t *testing.T) {
	mock := &researchMockLLM{name: "mock"}
	agent := NewResearchAgent(nil, mock)

	sources := []ResearchSource{
		{URL: "https://a.com", Title: "Source A", Content: "Content about topic A"},
		{URL: "https://b.com", Title: "Source B", Content: "Content about topic B"},
	}

	prompt := agent.BuildPrompt("my research query", sources)

	if !strings.Contains(prompt, "my research query") {
		t.Error("prompt should contain the query")
	}

	if !strings.Contains(prompt, "https://a.com") {
		t.Error("prompt should contain source A URL")
	}

	if !strings.Contains(prompt, "https://b.com") {
		t.Error("prompt should contain source B URL")
	}

	if !strings.Contains(prompt, "Content about topic A") {
		t.Error("prompt should contain source A content")
	}

	if !strings.Contains(prompt, "Content about topic B") {
		t.Error("prompt should contain source B content")
	}

	if !strings.Contains(prompt, "Source A") {
		t.Error("prompt should contain source A title")
	}

	if !strings.Contains(prompt, "Sources (2)") {
		t.Error("prompt should contain source count")
	}
}

func TestResearchAgent_BuildPrompt_Truncation(t *testing.T) {
	mock := &researchMockLLM{name: "mock"}
	agent := NewResearchAgent(nil, mock)

	longContent := strings.Repeat("x", 10000)
	sources := []ResearchSource{
		{URL: "https://long.com", Title: "Long", Content: longContent},
	}

	prompt := agent.BuildPrompt("query", sources)
	if strings.Contains(prompt, strings.Repeat("x", 10000)) {
		t.Error("prompt should truncate very long content")
	}

	if !strings.Contains(prompt, "[... truncated]") {
		t.Error("prompt should indicate truncation")
	}
}

func TestResearchAgent_ParseResponse(t *testing.T) {
	agent := NewResearchAgent(nil, nil)
	sources := []ResearchSource{
		{URL: "https://a.com", Title: "A", Relevance: 0.5},
	}

	// Valid JSON response
	resp := `{"summary":"Good answer","follow_up_questions":["q1","q2","q3"],"source_relevance":[{"url":"https://a.com","relevance":0.95}]}`
	result := agent.parseResponse("query", resp, sources)

	if result.Summary != "Good answer" {
		t.Errorf("Summary = %q", result.Summary)
	}

	if len(result.FollowUpQuestions) != 3 {
		t.Errorf("FollowUpQuestions len = %d, want 3", len(result.FollowUpQuestions))
	}

	if result.Sources[0].Relevance != 0.95 {
		t.Errorf("Relevance = %f, want 0.95", result.Sources[0].Relevance)
	}

	// Invalid JSON fallback
	result2 := agent.parseResponse("query", "plain text answer", sources)
	if result2.Summary != "plain text answer" {
		t.Errorf("fallback Summary = %q", result2.Summary)
	}
}

func TestDeduplicateSources(t *testing.T) {
	sources := []ResearchSource{
		{URL: "https://a.com", Title: "A"},
		{URL: "https://b.com", Title: "B"},
		{URL: "https://a.com", Title: "A duplicate"},
	}

	deduped := deduplicateSources(sources)
	if len(deduped) != 2 {
		t.Fatalf("deduped len = %d, want 2", len(deduped))
	}

	if deduped[0].Title != "A" {
		t.Errorf("first should be A, got %q", deduped[0].Title)
	}
}
