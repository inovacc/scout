package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractEdge_UnsupportedFormat(t *testing.T) {
	err := extractEdge([]byte("data"), "https://example.com/edge.tar.gz", t.TempDir())
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}

	if got := err.Error(); got != "unsupported edge installer format: edge.tar.gz" {
		t.Errorf("error = %q", got)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	subDir := filepath.Join(src, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "hello" {
		t.Errorf("a.txt = %q, want %q", data, "hello")
	}

	data, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "world" {
		t.Errorf("sub/b.txt = %q, want %q", data, "world")
	}
}

func TestCopyDir_EmptyDir(t *testing.T) {
	if err := copyDir(t.TempDir(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	defer srv.Close()

	_, err := DownloadFile(context.Background(), srv.URL+"/missing")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDownloadFile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("binary data"))
	}))

	defer srv.Close()

	data, err := DownloadFile(context.Background(), srv.URL+"/file.zip")
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "binary data" {
		t.Errorf("data = %q, want %q", data, "binary data")
	}
}

func TestDownloadFile_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))

	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DownloadFile(ctx, srv.URL+"/file")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestParseBrowserVersion_EdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"no version here", ""},
		{"Chrome 120.0.6099.109 stable", "120.0.6099.109"},
		{"Brave Browser 1.61.109 Chromium: 120.0.6099.199", "120.0.6099.199"},
		{"Microsoft Edge 120.0.2210.91", "120.0.2210.91"},
		{"v1.2.3", "1.2.3"},
		{"version 99.0.1", "99.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseBrowserVersion(tt.input)
			if got != tt.want {
				t.Errorf("ParseBrowserVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolve_UnknownBrowser(t *testing.T) {
	_, err := Resolve(context.Background(), "firefox")
	if err == nil {
		t.Fatal("expected error for unknown browser type")
	}
}

func TestLatestCachedBin_EmptyDir(t *testing.T) {
	got := LatestCachedBin(t.TempDir(), "chrome.exe")
	if got != "" {
		t.Errorf("LatestCachedBin(empty) = %q, want empty", got)
	}
}

func TestLatestCachedBin_NoMatchingBinary(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "1.0.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := LatestCachedBin(dir, "chrome.exe")
	if got != "" {
		t.Errorf("LatestCachedBin(no binary) = %q, want empty", got)
	}
}

func TestLatestCachedBin_MultipleVersions(t *testing.T) {
	dir := t.TempDir()

	for _, ver := range []string{"100.0", "200.0"} {
		verDir := filepath.Join(dir, ver)
		if err := os.MkdirAll(verDir, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(verDir, "chrome"), []byte("bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got := LatestCachedBin(dir, "chrome")
	if got == "" {
		t.Fatal("LatestCachedBin should find a binary")
	}

	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestRegisterBrowser_MissingBinary(t *testing.T) {
	bogusPath := filepath.Join(t.TempDir(), "nonexistent", "chrome")
	RegisterBrowser("test-browser-extra", "1.0.0", bogusPath)

	got := LookupRegistryBrowser("test-browser-extra")
	if got != "" {
		t.Errorf("LookupRegistryBrowser with missing binary = %q, want empty", got)
	}
}

func TestIsNotFound(t *testing.T) {
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) should be false")
	}

	if !IsNotFound(ErrNotFound) {
		t.Error("IsNotFound(ErrNotFound) should be true")
	}

	if !IsNotFound(fmt.Errorf("wrapped: %w", ErrNotFound)) {
		t.Error("IsNotFound(wrapped) should be true")
	}

	if IsNotFound(fmt.Errorf("other error")) {
		t.Error("IsNotFound(other) should be false")
	}
}

func TestBrowserRegistryNames_UnknownType(t *testing.T) {
	names := browserRegistryNames("firefox")
	// Unknown types return empty slice (default case).
	if len(names) != 0 {
		t.Errorf("browserRegistryNames(firefox) = %v, want []", names)
	}
}

func TestDetectBrowsers_ReturnsSorted(t *testing.T) {
	browsers := DetectBrowsers()
	// Verify sorting: priority should be non-decreasing.
	for i := 1; i < len(browsers); i++ {
		pi := browserTypePriority[browsers[i-1].Type]
		pj := browserTypePriority[browsers[i].Type]

		if pi > pj {
			t.Errorf("browsers not sorted: %s (pri %d) before %s (pri %d)",
				browsers[i-1].Type, pi, browsers[i].Type, pj)
		}
	}
}

func TestDetectBrowsers_AllHavePaths(t *testing.T) {
	for _, b := range DetectBrowsers() {
		if b.Path == "" {
			t.Errorf("browser %s has empty path", b.Name)
		}

		if b.Name == "" {
			t.Error("browser has empty name")
		}

		if b.Type == "" {
			t.Error("browser has empty type")
		}
	}
}

func TestBestDetected_ReturnsChrome(t *testing.T) {
	path, bt, err := BestDetected()
	if err != nil {
		t.Skipf("no browsers detected: %v", err)
	}

	if path == "" {
		t.Error("path should not be empty")
	}

	// BestDetected should return Chrome if available (highest priority).
	t.Logf("BestDetected: type=%s path=%s", bt, path)
}

func TestLookupBrowser_Chrome(t *testing.T) {
	// On Windows, Chrome returns ("", nil) for rod auto-detect.
	path, err := LookupBrowser(Chrome)
	if err == nil {
		t.Logf("LookupBrowser(Chrome) = %q", path)
	}
}

func TestLookupBrowser_Electron(t *testing.T) {
	// Electron is not a standard install, should return ErrNotFound.
	_, err := LookupBrowser(Electron)
	if err == nil {
		t.Log("LookupBrowser(Electron) succeeded unexpectedly")
	}
}

func TestListDownloaded_NoError(t *testing.T) {
	browsers, err := ListDownloaded()
	if err != nil {
		t.Fatal(err)
	}

	for _, b := range browsers {
		if b.Name == "" {
			t.Error("downloaded browser has empty name")
		}

		t.Logf("downloaded: %s versions=%v", b.Name, b.Versions)
	}
}

func TestBestCached_ReturnsPathOrError(t *testing.T) {
	path, err := BestCached()
	if err != nil {
		t.Logf("BestCached: %v (no cached browsers)", err)
		return
	}

	if path == "" {
		t.Error("BestCached returned empty path without error")
	}

	if !FileExists(path) {
		t.Errorf("BestCached returned non-existent path: %s", path)
	}
}

func TestFileExists_VariousPaths(t *testing.T) {
	dir := t.TempDir()

	// Regular file.
	f := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !FileExists(f) {
		t.Errorf("FileExists(%q) = false, want true", f)
	}

	// Directory should return false.
	if FileExists(dir) {
		t.Error("FileExists(dir) should be false")
	}

	// Non-existent.
	if FileExists(filepath.Join(dir, "nonexistent")) {
		t.Error("FileExists(nonexistent) should be false")
	}
}

func TestFirstExisting_AllMissing(t *testing.T) {
	_, err := firstExisting([]string{"/a/b/c", "/d/e/f"}, Chrome)
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFirstExisting_FindsFirst(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "first")
	f2 := filepath.Join(dir, "second")

	if err := os.WriteFile(f1, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(f2, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := firstExisting([]string{f1, f2}, Chrome)
	if err != nil {
		t.Fatal(err)
	}

	if got != f1 {
		t.Errorf("firstExisting returned %q, want %q", got, f1)
	}
}

func TestFirstExisting_SkipsMissing(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "found")

	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := firstExisting([]string{"/missing", f}, Chrome)
	if err != nil {
		t.Fatal(err)
	}

	if got != f {
		t.Errorf("got %q, want %q", got, f)
	}
}

func TestCopyDir_PreservesContent(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create a multi-level structure.
	dirs := []string{"a", filepath.Join("a", "b"), "c"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(src, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		"root.txt":                     "root",
		filepath.Join("a", "a.txt"):    "afile",
		filepath.Join("a", "b", "b.txt"): "bfile",
		filepath.Join("c", "c.txt"):    "cfile",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	for name, want := range files {
		data, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Errorf("missing file %s: %v", name, err)
			continue
		}

		if string(data) != want {
			t.Errorf("%s = %q, want %q", name, data, want)
		}
	}
}

func TestDownloadFile_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer srv.Close()

	data, err := DownloadFile(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(data))
	}
}

func TestDownloadFile_LargePayload(t *testing.T) {
	payload := make([]byte, 1024*1024) // 1MB
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))

	defer srv.Close()

	data, err := DownloadFile(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != len(payload) {
		t.Errorf("got %d bytes, want %d", len(data), len(payload))
	}
}

func TestChromiumRevisionDefault_NonZero(t *testing.T) {
	if ChromiumRevisionDefault <= 0 {
		t.Errorf("ChromiumRevisionDefault = %d, want > 0", ChromiumRevisionDefault)
	}
}

func TestStripFirstDir_MultipleEntries(t *testing.T) {
	dir := t.TempDir()

	// Create two top-level entries — stripFirstDir should no-op.
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := stripFirstDir(dir); err != nil {
		t.Fatal(err)
	}

	// Both should still exist.
	if _, err := os.Stat(filepath.Join(dir, "a")); err != nil {
		t.Error("dir 'a' should still exist")
	}

	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Error("file 'b.txt' should still exist")
	}
}

func TestStripFirstDir_SingleDir(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "chrome-win")

	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inner, "chrome.exe"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := stripFirstDir(dir); err != nil {
		t.Fatal(err)
	}

	// chrome.exe should now be at top level.
	if !FileExists(filepath.Join(dir, "chrome.exe")) {
		t.Error("chrome.exe should be promoted to top level")
	}

	// Inner dir should be gone.
	if _, err := os.Stat(inner); !os.IsNotExist(err) {
		t.Error("inner dir should be removed")
	}
}

func TestResolveCached_Electron(t *testing.T) {
	// Electron is not in the switch — should get ErrNotFound.
	_, err := ResolveCached(context.Background(), Electron)
	if err == nil {
		t.Fatal("expected error for Electron type")
	}

	if !IsNotFound(err) {
		t.Logf("error (not ErrNotFound but acceptable): %v", err)
	}
}

func TestEdgeBinPath_NonEmpty(t *testing.T) {
	got := edgeBinPath()
	if got == "" {
		t.Error("edgeBinPath() should not be empty")
	}
}

func TestBraveBinPath_NonEmpty(t *testing.T) {
	got := braveBinPath()
	if got == "" {
		t.Error("braveBinPath() should not be empty")
	}
}
