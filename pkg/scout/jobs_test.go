package scout

import (
	"context"
	"testing"
)

func TestAsyncJobManager_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	m, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager: %v", err)
	}

	id, err := m.Create("batch", map[string]any{"urls": []string{"https://example.com"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	j, err := m.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if j.ID != id {
		t.Errorf("ID = %q, want %q", j.ID, id)
	}

	if j.Type != "batch" {
		t.Errorf("Type = %q, want %q", j.Type, "batch")
	}

	if j.Status != AsyncJobPending {
		t.Errorf("Status = %q, want %q", j.Status, AsyncJobPending)
	}

	if j.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestAsyncJobManager_Lifecycle(t *testing.T) {
	dir := t.TempDir()
	m, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager: %v", err)
	}

	id, err := m.Create("crawl", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Start
	if err := m.Start(id); err != nil {
		t.Fatalf("Start: %v", err)
	}

	j, _ := m.Get(id)
	if j.Status != AsyncJobRunning {
		t.Errorf("Status = %q, want %q", j.Status, AsyncJobRunning)
	}

	if j.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// Update progress
	if err := m.UpdateProgress(id, 5, 1); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}

	j, _ = m.Get(id)
	if j.Progress.Completed != 5 || j.Progress.Failed != 1 {
		t.Errorf("Progress = %+v, want completed=5 failed=1", j.Progress)
	}

	// Complete
	if err := m.Complete(id, "done"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	j, _ = m.Get(id)
	if j.Status != AsyncJobCompleted {
		t.Errorf("Status = %q, want %q", j.Status, AsyncJobCompleted)
	}

	if j.EndedAt == nil {
		t.Error("EndedAt should be set")
	}
}

func TestAsyncJobManager_Cancel(t *testing.T) {
	dir := t.TempDir()
	m, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager: %v", err)
	}

	id, _ := m.Create("batch", nil)
	_ = m.Start(id)

	cancelled := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.RegisterCancel(id, func() {
		cancelled = true
		cancel()
	})

	if err := m.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if !cancelled {
		t.Error("cancel function was not called")
	}

	select {
	case <-ctx.Done():
	default:
		t.Error("context should be cancelled")
	}

	j, _ := m.Get(id)
	if j.Status != AsyncJobCancelled {
		t.Errorf("Status = %q, want %q", j.Status, AsyncJobCancelled)
	}
}

func TestAsyncJobManager_Fail(t *testing.T) {
	dir := t.TempDir()
	m, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager: %v", err)
	}

	id, _ := m.Create("fetch", nil)
	_ = m.Start(id)

	if err := m.Fail(id, "connection timeout"); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	j, _ := m.Get(id)
	if j.Status != AsyncJobFailed {
		t.Errorf("Status = %q, want %q", j.Status, AsyncJobFailed)
	}

	if j.Error != "connection timeout" {
		t.Errorf("Error = %q, want %q", j.Error, "connection timeout")
	}

	if j.EndedAt == nil {
		t.Error("EndedAt should be set")
	}
}

func TestAsyncJobManager_List(t *testing.T) {
	dir := t.TempDir()
	m, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager: %v", err)
	}

	id1, _ := m.Create("batch", nil)
	id2, _ := m.Create("crawl", nil)
	_, _ = m.Create("fetch", nil)
	_ = m.Start(id1)
	_ = m.Start(id2)
	_ = m.Complete(id2, nil)

	// List all
	all := m.List()
	if len(all) != 3 {
		t.Errorf("List() len = %d, want 3", len(all))
	}

	// List by status
	running := m.List(AsyncJobRunning)
	if len(running) != 1 {
		t.Errorf("List(running) len = %d, want 1", len(running))
	}

	completed := m.List(AsyncJobCompleted)
	if len(completed) != 1 {
		t.Errorf("List(completed) len = %d, want 1", len(completed))
	}

	pending := m.List(AsyncJobPending)
	if len(pending) != 1 {
		t.Errorf("List(pending) len = %d, want 1", len(pending))
	}

	// Multiple statuses
	mixed := m.List(AsyncJobRunning, AsyncJobCompleted)
	if len(mixed) != 2 {
		t.Errorf("List(running,completed) len = %d, want 2", len(mixed))
	}
}

func TestAsyncJobManager_Persistence(t *testing.T) {
	dir := t.TempDir()
	m, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager: %v", err)
	}

	id, _ := m.Create("batch", map[string]string{"key": "value"})
	_ = m.Start(id)
	_ = m.UpdateProgress(id, 3, 0)

	// Create new manager from same directory
	m2, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager (reload): %v", err)
	}

	j, err := m2.Get(id)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}

	if j.Status != AsyncJobRunning {
		t.Errorf("Status = %q, want %q", j.Status, AsyncJobRunning)
	}

	if j.Progress.Completed != 3 {
		t.Errorf("Progress.Completed = %d, want 3", j.Progress.Completed)
	}
}

func TestAsyncJobManager_NilCases(t *testing.T) {
	dir := t.TempDir()
	m, err := NewAsyncJobManager(dir)
	if err != nil {
		t.Fatalf("NewAsyncJobManager: %v", err)
	}

	// Get non-existent
	if _, err := m.Get("nonexistent"); err == nil {
		t.Error("expected error for non-existent job")
	}

	// Start non-existent
	if err := m.Start("nonexistent"); err == nil {
		t.Error("expected error for starting non-existent job")
	}

	// Cancel non-existent
	if err := m.Cancel("nonexistent"); err == nil {
		t.Error("expected error for cancelling non-existent job")
	}

	// Cannot start a completed job
	id, _ := m.Create("batch", nil)
	_ = m.Start(id)
	_ = m.Complete(id, nil)

	if err := m.Start(id); err == nil {
		t.Error("expected error starting completed job")
	}

	// Cannot cancel a completed job
	if err := m.Cancel(id); err == nil {
		t.Error("expected error cancelling completed job")
	}

	// List empty filter returns all
	all := m.List()
	if len(all) != 1 {
		t.Errorf("List() len = %d, want 1", len(all))
	}
}
