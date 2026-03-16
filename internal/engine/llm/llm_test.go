package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJobStatus_Constants(t *testing.T) {
	tests := []struct {
		status JobStatus
		want   string
	}{
		{JobStatusPending, "pending"},
		{JobStatusExtracting, "extracting"},
		{JobStatusReviewing, "reviewing"},
		{JobStatusCompleted, "completed"},
		{JobStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("JobStatus = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestJob_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	job := Job{
		ID:              "test-id",
		SessionID:       "sess-1",
		Status:          JobStatusCompleted,
		URL:             "https://example.com",
		Prompt:          "extract titles",
		ExtractProvider: "openai",
		ExtractModel:    "gpt-4o-mini",
		ExtractResult:   "# Titles\n- Title 1",
		ReviewProvider:  "anthropic",
		ReviewModel:     "claude-sonnet-4-20250514",
		ReviewPrompt:    "check accuracy",
		ReviewResult:    "Looks good",
		CreatedAt:       now,
		UpdatedAt:       now,
		Metadata:        map[string]string{"env": "test"},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Job
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != job.ID {
		t.Errorf("ID = %q, want %q", got.ID, job.ID)
	}

	if got.Status != job.Status {
		t.Errorf("Status = %q, want %q", got.Status, job.Status)
	}

	if got.ExtractResult != job.ExtractResult {
		t.Errorf("ExtractResult = %q, want %q", got.ExtractResult, job.ExtractResult)
	}

	if got.ReviewResult != job.ReviewResult {
		t.Errorf("ReviewResult = %q, want %q", got.ReviewResult, job.ReviewResult)
	}

	if got.Metadata["env"] != "test" {
		t.Errorf("Metadata[env] = %q, want %q", got.Metadata["env"], "test")
	}
}

func TestJob_OmitEmpty(t *testing.T) {
	job := Job{
		ID:        "id",
		SessionID: "sess",
		Status:    JobStatusPending,
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// These should be omitted when empty
	for _, field := range []string{"extract_result", "review_provider", "review_model", "review_prompt", "review_result", "metadata", "error"} {
		if _, ok := m[field]; ok {
			t.Errorf("field %q should be omitted when empty", field)
		}
	}
}

func TestJobResult_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input JobResult
	}{
		{
			name: "full",
			input: JobResult{
				JobID:         "j-1",
				ExtractResult: "extracted data",
				ReviewResult:  "reviewed data",
				Reviewed:      true,
			},
		},
		{
			name: "no_review",
			input: JobResult{
				ExtractResult: "data only",
				Reviewed:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var got JobResult
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got != tt.input {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tt.input)
			}
		})
	}
}

func TestJobRef_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ref := JobRef{
		ID:        "ref-1",
		SessionID: "sess-1",
		Status:    JobStatusExtracting,
		URL:       "https://example.com",
		CreatedAt: now,
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got JobRef
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != ref.ID || got.Status != ref.Status || got.URL != ref.URL {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, ref)
	}
}

func TestJobIndex_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	idx := JobIndex{
		Jobs: []JobRef{
			{ID: "j-1", SessionID: "s-1", Status: JobStatusCompleted, CreatedAt: now},
			{ID: "j-2", SessionID: "s-1", Status: JobStatusPending, CreatedAt: now},
		},
		Current: "j-2",
	}

	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got JobIndex
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Current != "j-2" {
		t.Errorf("Current = %q, want %q", got.Current, "j-2")
	}

	if len(got.Jobs) != 2 {
		t.Errorf("Jobs len = %d, want 2", len(got.Jobs))
	}
}

func TestDefaultReviewPrompt_NotEmpty(t *testing.T) {
	if DefaultReviewPrompt == "" {
		t.Fatal("DefaultReviewPrompt should not be empty")
	}
}

func TestDefaultReviewPrompt_ContainsKeyInstructions(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
	}{
		{"accuracy", "accuracy"},
		{"completeness", "completeness"},
		{"hallucinations", "hallucinations"},
		{"formatting", "ormatting"},
		{"corrections", "corrections"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(strings.ToLower(DefaultReviewPrompt), strings.ToLower(tt.keyword)) {
				t.Errorf("DefaultReviewPrompt missing keyword %q", tt.keyword)
			}
		})
	}
}

func TestSessionIndex_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	idx := SessionIndex{
		Sessions: []Session{
			{ID: "s-1", Name: "test", CreatedAt: now, UpdatedAt: now, Metadata: map[string]string{"k": "v"}},
		},
		Current: "s-1",
	}

	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got SessionIndex
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Current != "s-1" {
		t.Errorf("Current = %q, want %q", got.Current, "s-1")
	}

	if len(got.Sessions) != 1 {
		t.Fatalf("Sessions len = %d, want 1", len(got.Sessions))
	}

	if got.Sessions[0].Name != "test" {
		t.Errorf("Name = %q", got.Sessions[0].Name)
	}
}

func TestOpenAIProvider_APIErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := openaiChatResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "model not found"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewOpenAIProvider(
		WithOpenAIBaseURL(srv.URL),
		WithOpenAIKey("key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error = %q, want to contain 'model not found'", err.Error())
	}
}

func TestOpenAIProvider_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openaiChatResponse{})
	}))
	defer srv.Close()

	p, err := NewOpenAIProvider(
		WithOpenAIBaseURL(srv.URL),
		WithOpenAIKey("key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}

	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error = %q, want to contain 'no choices'", err.Error())
	}
}

func TestAnthropicProvider_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(anthropicResponse{})
	}))
	defer srv.Close()

	p, err := NewAnthropicProvider(
		WithAnthropicBaseURL(srv.URL),
		WithAnthropicKey("key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error for empty content")
	}

	if !strings.Contains(err.Error(), "no text content") {
		t.Errorf("error = %q, want to contain 'no text content'", err.Error())
	}
}

func TestAnthropicProvider_APIErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := anthropicResponse{
			Error: &struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}{Type: "invalid_request_error", Message: "max_tokens too large"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewAnthropicProvider(
		WithAnthropicBaseURL(srv.URL),
		WithAnthropicKey("key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "max_tokens too large") {
		t.Errorf("error = %q, want to contain 'max_tokens too large'", err.Error())
	}
}

func TestAnthropicProvider_MultipleTextBlocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := anthropicResponse{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hello "},
				{Type: "text", Text: "World"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewAnthropicProvider(
		WithAnthropicBaseURL(srv.URL),
		WithAnthropicKey("key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := p.Complete(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if result != "Hello World" {
		t.Errorf("result = %q, want %q", result, "Hello World")
	}
}

func TestAnthropicProvider_DefaultModel(t *testing.T) {
	o := defaultAnthropicOptions()
	if o.model != "claude-sonnet-4-20250514" {
		t.Errorf("default model = %q, want %q", o.model, "claude-sonnet-4-20250514")
	}

	if o.baseURL != AnthropicBaseURL {
		t.Errorf("default baseURL = %q, want %q", o.baseURL, AnthropicBaseURL)
	}
}

func TestOpenAIProvider_DefaultOptions(t *testing.T) {
	o := defaultOpenAIOptions()
	if o.model != "gpt-4o-mini" {
		t.Errorf("default model = %q, want %q", o.model, "gpt-4o-mini")
	}

	if o.baseURL != OpenAIBaseURL {
		t.Errorf("default baseURL = %q, want %q", o.baseURL, OpenAIBaseURL)
	}

	if o.authHeader != "Authorization" {
		t.Errorf("default authHeader = %q, want %q", o.authHeader, "Authorization")
	}

	if o.authPrefix != "Bearer " {
		t.Errorf("default authPrefix = %q, want %q", o.authPrefix, "Bearer ")
	}
}

func TestNewOllamaProvider_WithHost(t *testing.T) {
	p, err := NewOllamaProvider(WithOllamaHost("http://localhost:11434"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", p.Name(), "ollama")
	}

	if p.model != "llama3.2" {
		t.Errorf("model = %q, want %q", p.model, "llama3.2")
	}
}

func TestNewOllamaProvider_InvalidHost(t *testing.T) {
	_, err := NewOllamaProvider(WithOllamaHost("://invalid"))
	if err == nil {
		t.Fatal("expected error for invalid host URL")
	}
}

func TestNewOllamaProvider_CustomModel(t *testing.T) {
	p, err := NewOllamaProvider(
		WithOllamaHost("http://localhost:11434"),
		WithOllamaModel("codellama"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.model != "codellama" {
		t.Errorf("model = %q, want %q", p.model, "codellama")
	}
}

func TestOpenAIProvider_CustomAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "mykey" {
			t.Errorf("custom auth header = %q, want %q", r.Header.Get("X-API-Key"), "mykey")
		}

		resp := openaiChatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "ok"}},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewOpenAIProvider(
		WithOpenAIBaseURL(srv.URL),
		WithOpenAIKey("mykey"),
		WithOpenAIAuthHeader("X-API-Key", ""),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
}

func TestOpenAIProvider_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	p, err := NewOpenAIProvider(
		WithOpenAIBaseURL(srv.URL),
		WithOpenAIKey("key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestAnthropicProvider_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	p, err := NewAnthropicProvider(
		WithAnthropicBaseURL(srv.URL),
		WithAnthropicKey("key"),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = p.Complete(context.Background(), "sys", "usr")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestAnthropicProvider_CustomHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	p, err := NewAnthropicProvider(
		WithAnthropicKey("key"),
		WithAnthropicHTTPClient(customClient),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.client != customClient {
		t.Error("custom HTTP client not set")
	}
}

func TestOpenAIProvider_CustomHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	p, err := NewOpenAIProvider(
		WithOpenAIKey("key"),
		WithOpenAIHTTPClient(customClient),
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.client != customClient {
		t.Error("custom HTTP client not set")
	}
}

func TestBaseURLConstants(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"OpenAI", OpenAIBaseURL},
		{"OpenRouter", OpenRouterBaseURL},
		{"DeepSeek", DeepSeekBaseURL},
		{"Gemini", GeminiBaseURL},
		{"Anthropic", AnthropicBaseURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.url == "" {
				t.Error("base URL should not be empty")
			}

			if !strings.HasPrefix(tt.url, "https://") {
				t.Errorf("base URL %q should use HTTPS", tt.url)
			}
		})
	}
}
