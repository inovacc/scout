package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIndexSearch(t *testing.T) {
	idx := &Index{
		Plugins: []PluginInfo{
			{Name: "scout-diag", Description: "Network diagnostics", Tags: []string{"ping", "curl"}},
			{Name: "scout-search", Description: "Web search", Tags: []string{"google", "fetch"}},
			{Name: "scout-forms", Description: "Form automation", Tags: []string{"forms"}},
		},
	}

	// Empty query returns all.
	if results := idx.Search(""); len(results) != 3 {
		t.Errorf("empty search: got %d, want 3", len(results))
	}

	// Name match.
	if results := idx.Search("diag"); len(results) != 1 || results[0].Name != "scout-diag" {
		t.Errorf("name search 'diag': got %v", results)
	}

	// Description match.
	if results := idx.Search("web"); len(results) != 1 {
		t.Errorf("desc search 'web': got %d", len(results))
	}

	// Tag match.
	if results := idx.Search("ping"); len(results) != 1 {
		t.Errorf("tag search 'ping': got %d", len(results))
	}

	// No match.
	if results := idx.Search("nonexistent"); len(results) != 0 {
		t.Errorf("no-match search: got %d", len(results))
	}
}

func TestLockFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock.json")

	// Override lock path for test.
	lf := &LockFile{}
	lf.Lock("scout-diag", "v0.68.0", "abc123", "inovacc/scout")
	lf.Lock("scout-search", "v0.68.0", "def456", "inovacc/scout")

	// Save manually to test path.
	data, _ := json.MarshalIndent(lf, "", "  ")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Verify.
	if len(lf.Plugins) != 2 {
		t.Fatalf("plugins count = %d, want 2", len(lf.Plugins))
	}

	if p := lf.Get("scout-diag"); p == nil {
		t.Error("Get(scout-diag) = nil")
	} else if p.Checksum != "abc123" {
		t.Errorf("checksum = %q", p.Checksum)
	}

	if p := lf.Get("nonexistent"); p != nil {
		t.Error("Get(nonexistent) should be nil")
	}

	// Update existing.
	lf.Lock("scout-diag", "v0.69.0", "xyz789", "inovacc/scout")

	if len(lf.Plugins) != 2 {
		t.Errorf("after update: plugins count = %d, want 2", len(lf.Plugins))
	}

	if p := lf.Get("scout-diag"); p.Version != "v0.69.0" {
		t.Errorf("version after update = %q", p.Version)
	}
}

func TestFileChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")

	if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}

	checksum, err := FileChecksum(path)
	if err != nil {
		t.Fatalf("FileChecksum: %v", err)
	}

	// SHA256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if checksum != expected {
		t.Errorf("checksum = %q, want %q", checksum, expected)
	}

	// Verify should pass.
	if err := VerifyChecksum(path, expected); err != nil {
		t.Errorf("VerifyChecksum: %v", err)
	}

	// Wrong checksum should fail.
	if err := VerifyChecksum(path, "wrong"); err == nil {
		t.Error("expected error for wrong checksum")
	}
}

func TestReleaseURL(t *testing.T) {
	url := ReleaseURL("inovacc/scout", "v0.68.0", "scout-diag")
	if url == "" {
		t.Error("empty URL")
	}

	if !containsStr(url, "inovacc/scout") || !containsStr(url, "v0.68.0") || !containsStr(url, "scout-diag") {
		t.Errorf("unexpected URL: %s", url)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
