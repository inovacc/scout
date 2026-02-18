package scout

import (
	"context"
	"fmt"
	"testing"
)

func TestExtractWithLLMReview(t *testing.T) {
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

	extractor := &mockProvider{name: "extractor", response: "extracted content"}
	reviewer := &mockProvider{name: "reviewer", response: "review: looks good"}

	result, err := page.ExtractWithLLMReview("Summarize this page",
		WithLLMProvider(extractor),
		WithLLMReview(reviewer),
	)
	if err != nil {
		t.Fatalf("ExtractWithLLMReview: %v", err)
	}

	if result.ExtractResult != "extracted content" {
		t.Errorf("ExtractResult = %q, want %q", result.ExtractResult, "extracted content")
	}
	if result.ReviewResult != "review: looks good" {
		t.Errorf("ReviewResult = %q, want %q", result.ReviewResult, "review: looks good")
	}
	if !result.Reviewed {
		t.Error("Reviewed should be true")
	}
}

func TestExtractWithLLMReviewNoReviewer(t *testing.T) {
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

	extractor := &mockProvider{name: "extractor", response: "just extraction"}

	result, err := page.ExtractWithLLMReview("Summarize",
		WithLLMProvider(extractor),
	)
	if err != nil {
		t.Fatalf("ExtractWithLLMReview: %v", err)
	}

	if result.ExtractResult != "just extraction" {
		t.Errorf("ExtractResult = %q", result.ExtractResult)
	}
	if result.Reviewed {
		t.Error("Reviewed should be false without reviewer")
	}
	if result.ReviewResult != "" {
		t.Error("ReviewResult should be empty")
	}
}

func TestExtractWithLLMReviewWithWorkspace(t *testing.T) {
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

	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	extractor := &mockProvider{name: "extractor", response: "data"}
	reviewer := &mockProvider{name: "reviewer", response: "confirmed"}

	result, err := page.ExtractWithLLMReview("Extract",
		WithLLMProvider(extractor),
		WithLLMReview(reviewer),
		WithLLMWorkspace(ws),
		WithLLMMetadata("source", "test"),
	)
	if err != nil {
		t.Fatalf("ExtractWithLLMReview: %v", err)
	}

	if result.JobID == "" {
		t.Error("JobID should be set when workspace is provided")
	}

	// Verify job persisted
	job, err := ws.GetJob(result.JobID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}

	if job.Status != JobStatusCompleted {
		t.Errorf("job Status = %q, want %q", job.Status, JobStatusCompleted)
	}
	if job.ExtractResult != "data" {
		t.Errorf("job ExtractResult = %q", job.ExtractResult)
	}
	if job.ReviewResult != "confirmed" {
		t.Errorf("job ReviewResult = %q", job.ReviewResult)
	}
	if job.ExtractProvider != "extractor" {
		t.Errorf("job ExtractProvider = %q", job.ExtractProvider)
	}
	if job.ReviewProvider != "reviewer" {
		t.Errorf("job ReviewProvider = %q", job.ReviewProvider)
	}
	if job.Metadata["source"] != "test" {
		t.Errorf("job Metadata[source] = %q", job.Metadata["source"])
	}
}

func TestExtractWithLLMReviewExtractError(t *testing.T) {
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

	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	extractor := &mockProvider{name: "extractor", err: fmt.Errorf("connection failed")}

	_, err = page.ExtractWithLLMReview("Extract",
		WithLLMProvider(extractor),
		WithLLMWorkspace(ws),
	)
	if err == nil {
		t.Fatal("expected error from extractor")
	}

	// Job should be marked failed
	job, err := ws.CurrentJob()
	if err != nil {
		t.Fatalf("CurrentJob: %v", err)
	}
	if job.Status != JobStatusFailed {
		t.Errorf("job Status = %q, want %q", job.Status, JobStatusFailed)
	}
}

func TestExtractWithLLMReviewReviewError(t *testing.T) {
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

	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	extractor := &mockProvider{name: "extractor", response: "data"}
	reviewer := &mockProvider{name: "reviewer", err: fmt.Errorf("review failed")}

	_, err = page.ExtractWithLLMReview("Extract",
		WithLLMProvider(extractor),
		WithLLMReview(reviewer),
		WithLLMWorkspace(ws),
	)
	if err == nil {
		t.Fatal("expected error from reviewer")
	}

	job, err := ws.CurrentJob()
	if err != nil {
		t.Fatalf("CurrentJob: %v", err)
	}
	if job.Status != JobStatusFailed {
		t.Errorf("job Status = %q, want %q", job.Status, JobStatusFailed)
	}
	// Extract result should still be saved
	if job.ExtractResult != "data" {
		t.Errorf("job ExtractResult = %q", job.ExtractResult)
	}
}

func TestExtractWithLLMReviewNoProvider(t *testing.T) {
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

	_, err = page.ExtractWithLLMReview("Summarize")
	if err == nil {
		t.Fatal("expected error when no provider set")
	}
}

func TestReviewOptions(t *testing.T) {
	o := defaultLLMOptions()

	mock := &mockProvider{name: "reviewer"}
	WithLLMReview(mock)(o)
	if o.reviewProvider == nil {
		t.Error("reviewProvider should be set")
	}

	WithLLMReviewModel("gpt-4o")(o)
	if o.reviewModel != "gpt-4o" {
		t.Errorf("reviewModel = %q", o.reviewModel)
	}

	WithLLMReviewPrompt("custom review")(o)
	if o.reviewPrompt != "custom review" {
		t.Errorf("reviewPrompt = %q", o.reviewPrompt)
	}

	WithLLMSessionID("sess-123")(o)
	if o.sessionID != "sess-123" {
		t.Errorf("sessionID = %q", o.sessionID)
	}

	WithLLMMetadata("key1", "val1")(o)
	WithLLMMetadata("key2", "val2")(o)
	if o.metadata["key1"] != "val1" || o.metadata["key2"] != "val2" {
		t.Errorf("metadata = %v", o.metadata)
	}
}

// verifyMockReviewPrompt checks the reviewer received proper context.
func TestReviewPromptContainsContext(t *testing.T) {
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

	extractor := &mockProvider{name: "ext", response: "extracted stuff"}
	reviewer := &mockReviewCapture{mockProvider: mockProvider{name: "rev", response: "ok"}}

	_, err = page.ExtractWithLLMReview("My prompt",
		WithLLMProvider(extractor),
		WithLLMReview(reviewer),
	)
	if err != nil {
		t.Fatalf("ExtractWithLLMReview: %v", err)
	}

	// The reviewer's user prompt should contain:
	// 1. Original prompt
	// 2. Source page content
	// 3. Extraction result
	if reviewer.lastUser == "" {
		t.Fatal("reviewer user prompt empty")
	}

	contains := func(s, sub string) bool {
		return len(s) > 0 && len(sub) > 0 && containsStr(s, sub)
	}

	if !contains(reviewer.lastUser, "My prompt") {
		t.Error("reviewer prompt missing original prompt")
	}
	if !contains(reviewer.lastUser, "extracted stuff") {
		t.Error("reviewer prompt missing extraction result")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type mockReviewCapture struct {
	mockProvider
}

func (m *mockReviewCapture) Complete(_ context.Context, sys, user string) (string, error) {
	m.lastSys = sys
	m.lastUser = user
	return m.response, m.err
}
