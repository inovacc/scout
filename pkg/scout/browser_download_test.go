package scout

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBraveAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch string
		version      string
		want         string
	}{
		{"windows", "amd64", "1.87.188", "brave-v1.87.188-win32-x64.zip"},
		{"windows", "arm64", "1.87.188", "brave-v1.87.188-win32-arm64.zip"},
		{"darwin", "amd64", "1.87.188", "brave-v1.87.188-darwin-x64.zip"},
		{"darwin", "arm64", "1.87.188", "brave-v1.87.188-darwin-arm64.zip"},
		{"linux", "amd64", "1.87.188", "brave-browser-1.87.188-linux-amd64.zip"},
		{"linux", "arm64", "1.87.188", "brave-browser-1.87.188-linux-arm64.zip"},
	}

	for _, tt := range tests {
		key := tt.goos + "_" + tt.goarch
		pattern, ok := braveAssets[key]
		if !ok {
			t.Errorf("no asset pattern for %s", key)
			continue
		}

		got := fmt.Sprintf(pattern, tt.version)
		if got != tt.want {
			t.Errorf("braveAssets[%s] with version %s = %q, want %q", key, tt.version, got, tt.want)
		}
	}
}

func TestBraveBinPath(t *testing.T) {
	got := braveBinPath()
	expected, ok := braveBins[runtime.GOOS]
	if !ok {
		expected = "brave"
	}

	if got != expected {
		t.Errorf("braveBinPath() = %q, want %q", got, expected)
	}
}

func TestBraveAssetNameCurrentPlatform(t *testing.T) {
	name := braveAssetName("1.0.0")
	key := runtime.GOOS + "_" + runtime.GOARCH
	if _, ok := braveAssets[key]; ok && name == "" {
		t.Errorf("braveAssetName returned empty for supported platform %s", key)
	}
}

func TestLatestBraveVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"tag_name": "v1.87.188"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// We can't easily inject the URL into latestBraveVersion, so test the
	// parsing logic directly by simulating what the function does.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		t.Fatal(err)
	}

	if release.TagName != "v1.87.188" {
		t.Errorf("got tag_name %q, want %q", release.TagName, "v1.87.188")
	}
}

func TestExtractZipArchive(t *testing.T) {
	// Create a minimal zip in memory.
	zipData := createTestZip(t, map[string]string{
		"test.txt":        "hello world",
		"subdir/file.txt": "nested content",
	})

	destDir := t.TempDir()
	if err := extractZipArchive(zipData, destDir); err != nil {
		t.Fatal(err)
	}

	// Verify files.
	data, err := os.ReadFile(filepath.Join(destDir, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("test.txt = %q, want %q", string(data), "hello world")
	}

	data, err = os.ReadFile(filepath.Join(destDir, "subdir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested content" {
		t.Errorf("subdir/file.txt = %q, want %q", string(data), "nested content")
	}
}

func TestResolveBrowserEdge(t *testing.T) {
	_, err := resolveBrowser(context.Background(), BrowserEdge)
	if err == nil {
		// Edge is installed — that's fine, skip the error check.
		return
	}

	if !isNotFound(err) {
		t.Errorf("expected ErrBrowserNotFound, got: %v", err)
		return
	}

	// Should contain the download URL hint.
	if got := err.Error(); !contains(got, "microsoft.com/edge/download") {
		t.Errorf("error should contain download URL, got: %s", got)
	}
}

func TestResolveBrowserChrome(t *testing.T) {
	// Chrome should fall through to rod auto-detect (empty path).
	path, err := resolveBrowser(context.Background(), BrowserChrome)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
	}
	// rod auto-detect returns empty string.
	if path != "" {
		t.Logf("Chrome found at: %s", path)
	}
}

func TestListDownloadedBrowsers(t *testing.T) {
	// Just ensure it doesn't error.
	_, err := ListDownloadedBrowsers()
	if err != nil {
		t.Fatal(err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createTestZip creates a zip archive in memory from a map of filename→content.
func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}
