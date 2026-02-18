package scout

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceCreateAndList(t *testing.T) {
	dir := t.TempDir()

	ws, err := NewLLMWorkspace(dir)
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	if ws.Root() != dir {
		t.Errorf("Root() = %q, want %q", ws.Root(), dir)
	}

	// sessions.json and jobs/jobs.json should exist
	if _, err := os.Stat(filepath.Join(dir, "sessions.json")); err != nil {
		t.Errorf("sessions.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jobs", "jobs.json")); err != nil {
		t.Errorf("jobs/jobs.json missing: %v", err)
	}

	// No sessions yet
	sessions, err := ws.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestWorkspaceSessionLifecycle(t *testing.T) {
	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	// Create session
	sess, err := ws.CreateSession("test-session", map[string]string{"env": "test"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if sess.Name != "test-session" {
		t.Errorf("Name = %q", sess.Name)
	}
	if sess.Metadata["env"] != "test" {
		t.Errorf("Metadata[env] = %q", sess.Metadata["env"])
	}

	// Current session should be the one we just created
	cur, err := ws.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if cur.ID != sess.ID {
		t.Errorf("current session ID = %q, want %q", cur.ID, sess.ID)
	}

	// Create second session
	sess2, err := ws.CreateSession("second", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Current should now be second
	cur, err = ws.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if cur.ID != sess2.ID {
		t.Errorf("current = %q, want %q", cur.ID, sess2.ID)
	}

	// Switch back
	if err := ws.SetCurrentSession(sess.ID); err != nil {
		t.Fatalf("SetCurrentSession: %v", err)
	}
	cur, err = ws.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if cur.ID != sess.ID {
		t.Errorf("current = %q, want %q", cur.ID, sess.ID)
	}

	// List
	sessions, err := ws.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	// Get by ID
	got, err := ws.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Name != "test-session" {
		t.Errorf("Name = %q", got.Name)
	}

	// Not found
	_, err = ws.GetSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestWorkspaceJobLifecycle(t *testing.T) {
	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	sess, err := ws.CreateSession("test", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Create job
	job, err := ws.CreateJob(sess.ID, "https://example.com", "extract titles", map[string]string{"tag": "demo"})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if job.Status != JobStatusPending {
		t.Errorf("Status = %q, want %q", job.Status, JobStatusPending)
	}
	if job.URL != "https://example.com" {
		t.Errorf("URL = %q", job.URL)
	}
	if job.Metadata["tag"] != "demo" {
		t.Errorf("Metadata[tag] = %q", job.Metadata["tag"])
	}

	// job.json file should exist
	jobPath := filepath.Join(ws.Root(), "jobs", job.ID, "job.json")
	if _, err := os.Stat(jobPath); err != nil {
		t.Errorf("job.json missing: %v", err)
	}

	// Update job
	job.Status = JobStatusExtracting
	job.ExtractResult = "# Extracted\nSome content"
	if err := ws.UpdateJob(job); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}

	// extract.md should exist
	extractPath := filepath.Join(ws.Root(), "jobs", job.ID, "extract.md")
	if _, err := os.Stat(extractPath); err != nil {
		t.Errorf("extract.md missing: %v", err)
	}

	// Get job
	got, err := ws.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.ExtractResult != "# Extracted\nSome content" {
		t.Errorf("ExtractResult = %q", got.ExtractResult)
	}

	// Current job
	cur, err := ws.CurrentJob()
	if err != nil {
		t.Fatalf("CurrentJob: %v", err)
	}
	if cur.ID != job.ID {
		t.Errorf("current job = %q, want %q", cur.ID, job.ID)
	}

	// List jobs
	refs, err := ws.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 job, got %d", len(refs))
	}

	// List session jobs
	sessRefs, err := ws.ListSessionJobs(sess.ID)
	if err != nil {
		t.Fatalf("ListSessionJobs: %v", err)
	}
	if len(sessRefs) != 1 {
		t.Errorf("expected 1 session job, got %d", len(sessRefs))
	}

	// Not found
	_, err = ws.GetJob("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
}

func TestWorkspaceReviewFiles(t *testing.T) {
	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	sess, err := ws.CreateSession("test", nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	job, err := ws.CreateJob(sess.ID, "https://example.com", "test", nil)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	job.ExtractResult = "extraction output"
	job.ReviewResult = "review output"
	job.Status = JobStatusCompleted
	if err := ws.UpdateJob(job); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}

	// Both files should exist
	extractData, err := os.ReadFile(filepath.Join(ws.Root(), "jobs", job.ID, "extract.md"))
	if err != nil {
		t.Fatalf("read extract.md: %v", err)
	}
	if string(extractData) != "extraction output" {
		t.Errorf("extract.md = %q", extractData)
	}

	reviewData, err := os.ReadFile(filepath.Join(ws.Root(), "jobs", job.ID, "review.md"))
	if err != nil {
		t.Fatalf("read review.md: %v", err)
	}
	if string(reviewData) != "review output" {
		t.Errorf("review.md = %q", reviewData)
	}
}

func TestWorkspaceSetCurrentSessionNotFound(t *testing.T) {
	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	if err := ws.SetCurrentSession("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestWorkspaceNoCurrentSession(t *testing.T) {
	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	_, err = ws.CurrentSession()
	if err == nil {
		t.Fatal("expected error when no current session")
	}
}

func TestWorkspaceNoCurrentJob(t *testing.T) {
	ws, err := NewLLMWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewLLMWorkspace: %v", err)
	}

	_, err = ws.CurrentJob()
	if err == nil {
		t.Fatal("expected error when no current job")
	}
}
