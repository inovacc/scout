package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckUpdatesWithLock(t *testing.T) {
	lock := &LockFile{
		Plugins: []LockedPlugin{
			{Name: "scout-diag", Version: "v0.1.0", Checksum: "abc", Repo: "inovacc/scout-diag"},
			{Name: "scout-search", Version: "v0.2.0", Checksum: "def", Repo: "inovacc/scout-search"},
			{Name: "scout-forms", Version: "v0.3.0", Checksum: "ghi", Repo: "inovacc/scout-forms"},
		},
	}

	index := &Index{
		Version: "1",
		Plugins: []PluginInfo{
			{Name: "scout-diag", Latest: "v0.2.0", Repo: "inovacc/scout-diag"},       // update available
			{Name: "scout-search", Latest: "v0.2.0", Repo: "inovacc/scout-search"},    // up to date
			{Name: "scout-forms", Latest: "v0.4.0", Repo: "inovacc/scout-forms"},      // update available
			{Name: "scout-unknown", Latest: "v1.0.0", Repo: "inovacc/scout-unknown"},  // not installed
		},
	}

	updates := CheckUpdatesWithLock(lock, index)

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d: %+v", len(updates), updates)
	}

	// Verify the two expected updates are present.
	found := map[string]bool{}
	for _, u := range updates {
		found[u.Name] = true

		switch u.Name {
		case "scout-diag":
			if u.CurrentVersion != "v0.1.0" || u.LatestVersion != "v0.2.0" {
				t.Errorf("scout-diag: got %s -> %s", u.CurrentVersion, u.LatestVersion)
			}
		case "scout-forms":
			if u.CurrentVersion != "v0.3.0" || u.LatestVersion != "v0.4.0" {
				t.Errorf("scout-forms: got %s -> %s", u.CurrentVersion, u.LatestVersion)
			}
		default:
			t.Errorf("unexpected update: %s", u.Name)
		}
	}

	if !found["scout-diag"] {
		t.Error("missing update for scout-diag")
	}

	if !found["scout-forms"] {
		t.Error("missing update for scout-forms")
	}
}

func TestCheckUpdatesWithLockEmpty(t *testing.T) {
	lock := &LockFile{}
	index := &Index{
		Plugins: []PluginInfo{
			{Name: "scout-diag", Latest: "v1.0.0"},
		},
	}

	updates := CheckUpdatesWithLock(lock, index)
	if len(updates) != 0 {
		t.Errorf("expected 0 updates for empty lock, got %d", len(updates))
	}
}

func TestCheckUpdatesWithLockAllUpToDate(t *testing.T) {
	lock := &LockFile{
		Plugins: []LockedPlugin{
			{Name: "scout-diag", Version: "v1.0.0"},
		},
	}

	index := &Index{
		Plugins: []PluginInfo{
			{Name: "scout-diag", Latest: "v1.0.0"},
		},
	}

	updates := CheckUpdatesWithLock(lock, index)
	if len(updates) != 0 {
		t.Errorf("expected 0 updates when all up to date, got %d", len(updates))
	}
}

func TestCheckUpdatesHTTP(t *testing.T) {
	index := &Index{
		Version: "1",
		Plugins: []PluginInfo{
			{Name: "scout-diag", Latest: "v0.3.0", Repo: "inovacc/scout-diag"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(index)
	}))

	defer srv.Close()

	// FetchIndex with test server URL should work.
	fetched, err := FetchIndex(srv.URL)
	if err != nil {
		t.Fatalf("FetchIndex: %v", err)
	}

	if len(fetched.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(fetched.Plugins))
	}

	if fetched.Plugins[0].Latest != "v0.3.0" {
		t.Errorf("latest = %q, want v0.3.0", fetched.Plugins[0].Latest)
	}
}

func TestShouldCheckNeverChecked(t *testing.T) {
	// Point to a temp dir so we don't find any existing file.
	origHome := os.Getenv("HOME")
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// On Windows, also set USERPROFILE.
	if origHome == "" {
		t.Setenv("USERPROFILE", dir)
	}

	if !ShouldCheck(24 * time.Hour) {
		t.Error("ShouldCheck should return true when never checked")
	}
}

func TestShouldCheckRecent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	pluginDir := filepath.Join(dir, ".scout", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a recent timestamp.
	recent := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	checkFile := filepath.Join(pluginDir, ".last-update-check")

	if err := os.WriteFile(checkFile, []byte(recent), 0o644); err != nil {
		t.Fatal(err)
	}

	if ShouldCheck(24 * time.Hour) {
		t.Error("ShouldCheck should return false when checked 1 hour ago with 24h interval")
	}
}

func TestShouldCheckOld(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	pluginDir := filepath.Join(dir, ".scout", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write an old timestamp (2 days ago).
	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	checkFile := filepath.Join(pluginDir, ".last-update-check")

	if err := os.WriteFile(checkFile, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	if !ShouldCheck(24 * time.Hour) {
		t.Error("ShouldCheck should return true when checked 48 hours ago with 24h interval")
	}
}

func TestMarkChecked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	if err := MarkChecked(); err != nil {
		t.Fatalf("MarkChecked: %v", err)
	}

	path := filepath.Join(dir, ".scout", "plugins", ".last-update-check")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read check file: %v", err)
	}

	ts, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		t.Fatalf("parse timestamp: %v (raw: %q)", err, string(data))
	}

	if time.Since(ts) > 5*time.Second {
		t.Errorf("timestamp too old: %v", ts)
	}
}

func TestShouldCheckInvalidContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	pluginDir := filepath.Join(dir, ".scout", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	checkFile := filepath.Join(pluginDir, ".last-update-check")
	if err := os.WriteFile(checkFile, []byte("not-a-date"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !ShouldCheck(24 * time.Hour) {
		t.Error("ShouldCheck should return true when file contains invalid content")
	}
}
