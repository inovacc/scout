package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewJob(t *testing.T) {
	urls := []string{"https://example.com", "https://other.com"}
	job := NewJob("scrape", urls, "scout scrape")

	if job.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	if job.Type != "scrape" {
		t.Fatalf("expected type scrape, got %s", job.Type)
	}

	if job.Status != JobPending {
		t.Fatalf("expected status pending, got %s", job.Status)
	}

	if len(job.TargetURLs) != 2 {
		t.Fatalf("expected 2 target URLs, got %d", len(job.TargetURLs))
	}

	if job.Command != "scout scrape" {
		t.Fatalf("expected command 'scout scrape', got %s", job.Command)
	}

	if job.StartedAt.IsZero() {
		t.Fatal("expected non-zero StartedAt")
	}

	if job.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero UpdatedAt")
	}
}

func TestWriteReadJob(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	sessionID := "test-session-job"
	job := NewJob("crawl", []string{"https://example.com"}, "scout crawl")

	if err := WriteJob(sessionID, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	// Verify file exists.
	jobPath := filepath.Join(dir, sessionID, jobFile)
	if _, err := os.Stat(jobPath); err != nil {
		t.Fatalf("job.json not found: %v", err)
	}

	// Read back.
	got, err := ReadJob(sessionID)
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}

	if got.ID != job.ID {
		t.Fatalf("ID mismatch: %s vs %s", got.ID, job.ID)
	}

	if got.Type != "crawl" {
		t.Fatalf("Type mismatch: %s", got.Type)
	}

	if got.Status != JobPending {
		t.Fatalf("Status mismatch: %s", got.Status)
	}

	if got.Command != "scout crawl" {
		t.Fatalf("Command mismatch: %s", got.Command)
	}

	if len(got.TargetURLs) != 1 || got.TargetURLs[0] != "https://example.com" {
		t.Fatalf("TargetURLs mismatch: %v", got.TargetURLs)
	}
}

func TestAddJobStep(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	sessionID := "test-steps"
	job := NewJob("scrape", nil, "")

	if err := WriteJob(sessionID, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	now := time.Now()
	step := JobStep{
		Name:        "navigate",
		Description: "Navigate to target URL",
		StartedAt:   now,
		CompletedAt: now.Add(2 * time.Second),
	}

	if err := AddJobStep(sessionID, step); err != nil {
		t.Fatalf("AddJobStep: %v", err)
	}

	got, err := ReadJob(sessionID)
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}

	if len(got.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(got.Steps))
	}

	if got.Steps[0].Name != "navigate" {
		t.Fatalf("step name mismatch: %s", got.Steps[0].Name)
	}

	if got.Progress == nil {
		t.Fatal("expected non-nil progress after AddJobStep")
	}

	if got.Progress.CurrentStep != 1 {
		t.Fatalf("expected current_step=1, got %d", got.Progress.CurrentStep)
	}

	if got.Progress.Message != "navigate" {
		t.Fatalf("expected progress message 'navigate', got %s", got.Progress.Message)
	}
}

func TestStartJob(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	sessionID := "test-start"
	job := NewJob("scrape", nil, "")

	if err := WriteJob(sessionID, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	if err := StartJob(sessionID); err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	got, err := ReadJob(sessionID)
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}

	if got.Status != JobRunning {
		t.Fatalf("expected status running, got %s", got.Status)
	}
}

func TestCompleteJob(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	sessionID := "test-complete"
	job := NewJob("scrape", nil, "")

	if err := WriteJob(sessionID, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	if err := StartJob(sessionID); err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	if err := CompleteJob(sessionID, "42 items scraped"); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	got, err := ReadJob(sessionID)
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}

	if got.Status != JobCompleted {
		t.Fatalf("expected status completed, got %s", got.Status)
	}

	if got.Output != "42 items scraped" {
		t.Fatalf("output mismatch: %s", got.Output)
	}

	if got.CompletedAt.IsZero() {
		t.Fatal("expected non-zero CompletedAt")
	}
}

func TestFailJob(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	sessionID := "test-fail"
	job := NewJob("scrape", nil, "")

	if err := WriteJob(sessionID, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	if err := StartJob(sessionID); err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	if err := FailJob(sessionID, "connection timeout"); err != nil {
		t.Fatalf("FailJob: %v", err)
	}

	got, err := ReadJob(sessionID)
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}

	if got.Status != JobFailed {
		t.Fatalf("expected status failed, got %s", got.Status)
	}

	if got.Error != "connection timeout" {
		t.Fatalf("error mismatch: %s", got.Error)
	}

	if got.CompletedAt.IsZero() {
		t.Fatal("expected non-zero CompletedAt")
	}
}

func TestUpdateJobProgress(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	sessionID := "test-progress"
	job := NewJob("crawl", nil, "")

	if err := WriteJob(sessionID, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	if err := UpdateJobProgress(sessionID, 3, 10, "processing page 3"); err != nil {
		t.Fatalf("UpdateJobProgress: %v", err)
	}

	got, err := ReadJob(sessionID)
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}

	if got.Progress == nil {
		t.Fatal("expected non-nil progress")
	}

	if got.Progress.CurrentStep != 3 {
		t.Fatalf("expected current_step=3, got %d", got.Progress.CurrentStep)
	}

	if got.Progress.TotalSteps != 10 {
		t.Fatalf("expected total_steps=10, got %d", got.Progress.TotalSteps)
	}

	if got.Progress.Percentage != 30.0 {
		t.Fatalf("expected percentage=30.0, got %f", got.Progress.Percentage)
	}

	if got.Progress.Message != "processing page 3" {
		t.Fatalf("expected message 'processing page 3', got %s", got.Progress.Message)
	}
}

func TestRemoveJob(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	sessionID := "test-remove"
	job := NewJob("scrape", nil, "")

	if err := WriteJob(sessionID, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	if err := RemoveJob(sessionID); err != nil {
		t.Fatalf("RemoveJob: %v", err)
	}

	jobPath := filepath.Join(dir, sessionID, jobFile)
	if _, err := os.Stat(jobPath); !os.IsNotExist(err) {
		t.Fatalf("expected job.json to be removed, got: %v", err)
	}
}

func TestReadJobNotFound(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	_, err := ReadJob("nonexistent-session")
	if err == nil {
		t.Fatal("expected error for missing job.json")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got: %v", err)
	}
}
