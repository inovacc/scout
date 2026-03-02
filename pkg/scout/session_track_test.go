package scout

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionTracker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.json")

	// Load from non-existent file.
	tracker, err := LoadTrackerFrom(path)
	if err != nil {
		t.Fatalf("LoadTrackerFrom: %v", err)
	}
	if len(tracker.Sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(tracker.Sessions))
	}

	// Register a session.
	entry := SessionEntry{
		ID:        "test-id-1",
		DataDir:   filepath.Join(dir, "data1"),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Browser:   "chrome",
		Headless:  true,
		Reusable:  true,
	}
	if err := os.MkdirAll(entry.DataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Register(entry); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Reload and verify persistence.
	tracker2, err := LoadTrackerFrom(path)
	if err != nil {
		t.Fatalf("LoadTrackerFrom reload: %v", err)
	}
	if len(tracker2.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(tracker2.Sessions))
	}
	if tracker2.Sessions[0].ID != "test-id-1" {
		t.Fatalf("expected id test-id-1, got %s", tracker2.Sessions[0].ID)
	}

	// FindReusable.
	found := tracker2.FindReusable("chrome", true)
	if found == nil {
		t.Fatal("expected to find reusable session")
	}
	notFound := tracker2.FindReusable("edge", true)
	if notFound != nil {
		t.Fatal("expected no match for edge")
	}

	// Update.
	if err := tracker2.Update("test-id-1", func(e *SessionEntry) {
		e.URLs = []string{"https://example.com"}
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Prune — data dir exists, so nothing pruned.
	pruned, err := tracker2.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("expected 0 pruned, got %d", pruned)
	}

	// Remove data dir, then prune.
	if err := os.RemoveAll(entry.DataDir); err != nil {
		t.Fatal(err)
	}
	pruned, err = tracker2.Prune()
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned, got %d", pruned)
	}
	if len(tracker2.Sessions) != 0 {
		t.Fatalf("expected 0 sessions after prune, got %d", len(tracker2.Sessions))
	}
}

func TestSessionTrackerRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.json")

	tracker, err := LoadTrackerFrom(path)
	if err != nil {
		t.Fatal(err)
	}

	_ = tracker.Register(SessionEntry{ID: "a", DataDir: "/tmp/a", Browser: "chrome"})
	_ = tracker.Register(SessionEntry{ID: "b", DataDir: "/tmp/b", Browser: "chrome"})

	if err := tracker.Remove("a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(tracker.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(tracker.Sessions))
	}
	if tracker.Sessions[0].ID != "b" {
		t.Fatalf("expected remaining session b, got %s", tracker.Sessions[0].ID)
	}
}
