package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFetchIndex_Success(t *testing.T) {
	idx := Index{
		Version: "1",
		Plugins: []PluginInfo{
			{Name: "scout-diag", Description: "Diagnostics", Author: "test", Repo: "inovacc/scout", Latest: "v0.1.0"},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(idx)
	}))
	defer ts.Close()

	result, err := FetchIndex(ts.URL)
	if err != nil {
		t.Fatalf("FetchIndex() error: %v", err)
	}

	if len(result.Plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(result.Plugins))
	}

	if result.Plugins[0].Name != "scout-diag" {
		t.Errorf("name = %q", result.Plugins[0].Name)
	}
}

func TestFetchIndex_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := FetchIndex(ts.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestFetchIndex_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer ts.Close()

	_, err := FetchIndex(ts.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchIndex_NetworkError(t *testing.T) {
	_, err := FetchIndex("http://localhost:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestFetchIndex_DefaultURL(t *testing.T) {
	// Just verify it doesn't panic; the actual URL may not be reachable in CI.
	// We test with empty string which should use DefaultIndexURL.
	// This will likely fail due to network, which is fine — we just verify the path.
	_, _ = FetchIndex("")
}

func TestLatestReleaseURL(t *testing.T) {
	url := LatestReleaseURL("inovacc/scout", "scout-diag")
	if url == "" {
		t.Error("empty URL")
	}

	if !containsStr(url, "latest/download") {
		t.Errorf("URL missing 'latest/download': %s", url)
	}

	if !containsStr(url, runtime.GOOS) {
		t.Errorf("URL missing GOOS: %s", url)
	}

	if !containsStr(url, runtime.GOARCH) {
		t.Errorf("URL missing GOARCH: %s", url)
	}
}

func TestArchiveExt(t *testing.T) {
	ext := archiveExt()
	if runtime.GOOS == "windows" {
		if ext != "zip" {
			t.Errorf("archiveExt() = %q on windows, want zip", ext)
		}
	} else {
		if ext != "tar.gz" {
			t.Errorf("archiveExt() = %q on non-windows, want tar.gz", ext)
		}
	}
}

func TestLockFilePath(t *testing.T) {
	path, err := LockFilePath()
	if err != nil {
		t.Fatalf("LockFilePath() error: %v", err)
	}

	if path == "" {
		t.Error("empty path")
	}

	if !containsStr(path, "lock.json") {
		t.Errorf("path missing lock.json: %s", path)
	}
}

func TestLoadLockFile_NonExistent(t *testing.T) {
	// Override HOME to use temp dir so lock file doesn't exist.
	dir := t.TempDir()

	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")

	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	}()

	lf, err := LoadLockFile()
	if err != nil {
		t.Fatalf("LoadLockFile() error: %v", err)
	}

	if len(lf.Plugins) != 0 {
		t.Errorf("expected empty lock file, got %d plugins", len(lf.Plugins))
	}
}

func TestLoadLockFile_ValidFile(t *testing.T) {
	dir := t.TempDir()
	lockDir := filepath.Join(dir, ".scout", "plugins")

	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lf := &LockFile{
		Plugins: []LockedPlugin{
			{Name: "test", Version: "v1.0.0", Checksum: "abc", Installed: "2025-01-01T00:00:00Z"},
		},
	}

	data, _ := json.MarshalIndent(lf, "", "  ")
	if err := os.WriteFile(filepath.Join(lockDir, "lock.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")

	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	}()

	loaded, err := LoadLockFile()
	if err != nil {
		t.Fatalf("LoadLockFile() error: %v", err)
	}

	if len(loaded.Plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(loaded.Plugins))
	}

	if loaded.Plugins[0].Name != "test" {
		t.Errorf("name = %q", loaded.Plugins[0].Name)
	}
}

func TestLoadLockFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	lockDir := filepath.Join(dir, ".scout", "plugins")

	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(lockDir, "lock.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")

	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	}()

	_, err := LoadLockFile()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLockFile_Save(t *testing.T) {
	dir := t.TempDir()

	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")

	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	}()

	lf := &LockFile{}
	lf.Lock("test", "v1.0.0", "abc", "owner/repo")

	if err := lf.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify the file was created.
	path, _ := LockFilePath()

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	var loaded LockFile
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parse saved file: %v", err)
	}

	if len(loaded.Plugins) != 1 {
		t.Errorf("saved %d plugins, want 1", len(loaded.Plugins))
	}
}

func TestFileChecksum_NonExistent(t *testing.T) {
	_, err := FileChecksum("/nonexistent/file")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestVerifyChecksum_NonExistent(t *testing.T) {
	err := VerifyChecksum("/nonexistent/file", "abc")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestIndexSearch_CaseInsensitive(t *testing.T) {
	idx := &Index{
		Plugins: []PluginInfo{
			{Name: "Scout-DIAG", Description: "Network diagnostics", Tags: []string{"PING"}},
		},
	}

	// Search with lowercase should match uppercase name.
	results := idx.Search("scout-diag")
	if len(results) != 1 {
		t.Errorf("case-insensitive name search: got %d", len(results))
	}

	// Tag case-insensitive match.
	results = idx.Search("ping")
	if len(results) != 1 {
		t.Errorf("case-insensitive tag search: got %d", len(results))
	}
}

func TestLockFile_Get_Multiple(t *testing.T) {
	lf := &LockFile{}
	lf.Lock("a", "v1", "c1", "r1")
	lf.Lock("b", "v2", "c2", "r2")
	lf.Lock("c", "v3", "c3", "r3")

	if p := lf.Get("b"); p == nil {
		t.Error("Get(b) = nil")
	} else if p.Version != "v2" {
		t.Errorf("Get(b).Version = %q", p.Version)
	}

	// First and last.
	if p := lf.Get("a"); p == nil || p.Version != "v1" {
		t.Error("Get(a) failed")
	}

	if p := lf.Get("c"); p == nil || p.Version != "v3" {
		t.Error("Get(c) failed")
	}
}
