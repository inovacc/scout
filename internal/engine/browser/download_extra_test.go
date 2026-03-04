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
