package browser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/archive"
)

func TestBraveAssetName(t *testing.T) {
	name := braveAssetName("1.87.188")
	if name == "" {
		t.Skip("no brave asset for current platform")
	}

	if !strings.Contains(name, "1.87.188") {
		t.Errorf("braveAssetName(1.87.188) = %q, expected version in name", name)
	}

	if !strings.HasSuffix(name, ".zip") {
		t.Errorf("braveAssetName(1.87.188) = %q, expected .zip suffix", name)
	}
}

func TestBraveBinPath(t *testing.T) {
	got := braveBinPath()
	if got == "" {
		t.Skip("no brave binary path for current platform")
	}

	t.Logf("braveBinPath() = %q", got)
}

func TestBraveDownloadURL(t *testing.T) {
	m := LoadManifest()
	url := m.Brave.DownloadURL("1.87.188")
	if url == "" {
		t.Skip("no brave download URL for current platform")
	}

	if !strings.Contains(url, "1.87.188") {
		t.Errorf("Brave.DownloadURL(1.87.188) = %q, expected version in URL", url)
	}

	if !strings.HasPrefix(url, "https://") {
		t.Errorf("Brave.DownloadURL should start with https://, got %q", url)
	}
}

func TestLatestBraveVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"tag_name": "v1.87.188"}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

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
	zipData := createTestZip(t, map[string]string{
		"test.txt":        "hello world",
		"subdir/file.txt": "nested content",
	})

	destDir := t.TempDir()
	if err := archive.Extract(zipData, "test.zip", destDir); err != nil {
		t.Fatal(err)
	}

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
	_, err := Resolve(context.Background(), Edge)
	if err == nil {
		return
	}

	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got: %v", err)
		return
	}

	if got := err.Error(); !strings.Contains(got, "microsoft.com/edge/download") {
		t.Errorf("error should contain download URL, got: %s", got)
	}
}

func TestResolveBrowserChrome(t *testing.T) {
	path, err := Resolve(context.Background(), Chrome)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
	}
	if path != "" {
		t.Logf("Chrome found at: %s", path)
	}
}

func TestListDownloaded(t *testing.T) {
	_, err := ListDownloaded()
	if err != nil {
		t.Fatal(err)
	}
}

func TestResolveCached(t *testing.T) {
	got, err := ResolveCached(context.Background(), Brave)
	if err != nil {
		t.Fatalf("ResolveCached(brave) error: %v", err)
	}

	if !FileExists(got) {
		t.Errorf("ResolveCached(brave) = %q but file doesn't exist", got)
	}
}

func TestBestCached(t *testing.T) {
	got, err := BestCached()
	if err != nil {
		t.Fatalf("BestCached() error: %v", err)
	}

	if !FileExists(got) {
		t.Errorf("BestCached() = %q but file doesn't exist", got)
	}
}

func TestBrowserRegistry(t *testing.T) {
	cacheDir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}

	fakeDir := filepath.Join(cacheDir, "test-browser", "1.0.0")
	fakeBin := filepath.Join(fakeDir, "test.exe")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeBin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(filepath.Join(cacheDir, "test-browser")) }()

	RegisterBrowser("test-browser", "1.0.0", fakeBin)

	got := LookupRegistryBrowser("test-browser")
	if got != fakeBin {
		t.Errorf("LookupRegistryBrowser(test-browser) = %q, want %q", got, fakeBin)
	}

	// Re-register (should be no-op).
	RegisterBrowser("test-browser", "1.0.0", fakeBin)
	entries, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if e.Name == "test-browser" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 test-browser entry, got %d", count)
	}

	// Clean up the test entry from registry.
	var cleaned []BrowserEntry
	for _, e := range entries {
		if e.Name != "test-browser" {
			cleaned = append(cleaned, e)
		}
	}
	_ = SaveRegistry(cleaned)
}

func TestLatestCachedBin(t *testing.T) {
	dir := t.TempDir()

	if got := LatestCachedBin(dir, "chrome.exe"); got != "" {
		t.Errorf("LatestCachedBin(empty) = %q, want empty", got)
	}

	v1 := filepath.Join(dir, "100")
	v2 := filepath.Join(dir, "200")
	_ = os.MkdirAll(v1, 0o755)
	_ = os.MkdirAll(v2, 0o755)

	bin1 := filepath.Join(v1, "chrome.exe")
	_ = os.WriteFile(bin1, []byte("v1"), 0o755)

	got := LatestCachedBin(dir, "chrome.exe")
	if got != bin1 {
		t.Errorf("LatestCachedBin = %q, want %q", got, bin1)
	}

	bin2 := filepath.Join(v2, "chrome.exe")
	_ = os.WriteFile(bin2, []byte("v2"), 0o755)

	got = LatestCachedBin(dir, "chrome.exe")
	if got != bin2 {
		t.Errorf("LatestCachedBin = %q, want %q (should prefer later version)", got, bin2)
	}
}

func TestDownloadAllBrowsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser download test in short mode")
	}

	ctx := context.Background()

	t.Run("chrome", func(t *testing.T) {
		path, err := DownloadChrome(ctx)
		if err != nil {
			t.Fatalf("DownloadChrome: %v", err)
		}

		if !FileExists(path) {
			t.Fatalf("chrome binary not found at %s", path)
		}

		t.Logf("Chrome downloaded to: %s", path)
	})

	t.Run("chromium", func(t *testing.T) {
		path, err := DownloadChromium(ctx, ChromiumRevisionDefault)
		if err != nil {
			t.Fatalf("DownloadChromium: %v", err)
		}

		if !FileExists(path) {
			t.Fatalf("chromium binary not found at %s", path)
		}

		t.Logf("Chromium downloaded to: %s", path)
	})

	t.Run("brave", func(t *testing.T) {
		path, err := DownloadBrave(ctx)
		if err != nil {
			t.Fatalf("DownloadBrave: %v", err)
		}

		if !FileExists(path) {
			t.Fatalf("brave binary not found at %s", path)
		}

		t.Logf("Brave downloaded to: %s", path)
	})

	t.Run("edge", func(t *testing.T) {
		path, err := DownloadEdge(ctx)
		if err != nil {
			t.Fatalf("DownloadEdge: %v", err)
		}

		if !FileExists(path) {
			t.Fatalf("edge binary not found at %s", path)
		}

		t.Logf("Edge downloaded to: %s", path)
	})

	browsers, err := ListDownloaded()
	if err != nil {
		t.Fatalf("ListDownloaded: %v", err)
	}

	found := make(map[string]bool)
	for _, b := range browsers {
		found[b.Name] = true
		t.Logf("Cached: %s versions=%v", b.Name, b.Versions)
	}

	for _, name := range []string{"chrome", "chromium", "brave", "edge"} {
		if !found[name] {
			t.Errorf("expected %s in cached browsers", name)
		}
	}

	for _, bt := range []BrowserType{Chrome, Brave, Edge} {
		path, err := ResolveCached(ctx, bt)
		if err != nil {
			t.Errorf("ResolveCached(%s): %v", bt, err)
			continue
		}

		if !FileExists(path) {
			t.Errorf("ResolveCached(%s) = %q but file doesn't exist", bt, path)
		}

		t.Logf("ResolveCached(%s) = %s", bt, path)
	}

	best, err := BestCached()
	if err != nil {
		t.Fatalf("BestCached: %v", err)
	}

	t.Logf("BestCached = %s", best)
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
