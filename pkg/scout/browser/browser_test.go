package browser

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.logger == nil {
		t.Error("default logger should not be nil")
	}
}

func TestManagerOptions(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	m := NewManager(WithCacheDir(dir), WithLogger(logger))

	if m.cacheDir != dir {
		t.Errorf("cacheDir = %q, want %q", m.cacheDir, dir)
	}

	if m.logger != logger {
		t.Error("logger was not set correctly")
	}
}

func TestBrowserInfo(t *testing.T) {
	info := BrowserInfo{
		Name:       "Google Chrome",
		Type:       TypeChrome,
		Path:       "/usr/bin/google-chrome",
		Version:    "120.0.6099.109",
		Downloaded: false,
	}

	if info.Name != "Google Chrome" {
		t.Errorf("Name = %q", info.Name)
	}

	if info.Type != TypeChrome {
		t.Errorf("Type = %q", info.Type)
	}

	if info.Downloaded {
		t.Error("Downloaded should be false")
	}
}

func TestManagerList(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(WithCacheDir(dir))

	browsers, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// May be empty on CI, but should not error.
	if browsers == nil {
		// nil is acceptable (no browsers found), just ensure no panic.
		_ = browsers
	}
}

func TestManagerClean_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(WithCacheDir(dir))

	if err := m.Clean(); err != nil {
		t.Fatalf("Clean() on empty dir: %v", err)
	}
}

func TestManagerClean_WithContent(t *testing.T) {
	dir := t.TempDir()

	subdir := filepath.Join(dir, "brave-1.0.0")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(WithCacheDir(dir))

	if err := m.Clean(); err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected empty dir after Clean, got %d entries", len(entries))
	}
}

func TestDetect(t *testing.T) {
	browsers, err := Detect()
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	// May be empty on CI. Just verify it returns without error.
	_ = browsers
}

func TestResolve_Unknown(t *testing.T) {
	m := NewManager(WithCacheDir(t.TempDir()))

	_, err := m.Resolve("firefox")
	if err == nil {
		t.Fatal("expected error for unknown browser type")
	}

	if !errors.Is(err, ErrUnknownType) {
		t.Errorf("expected ErrUnknownType, got: %v", err)
	}
}

func TestDownload_Unknown(t *testing.T) {
	m := NewManager(WithCacheDir(t.TempDir()))

	_, err := m.Download("opera")
	if err == nil {
		t.Fatal("expected error for unknown browser type")
	}

	if !errors.Is(err, ErrUnknownType) {
		t.Errorf("expected ErrUnknownType, got: %v", err)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Google Chrome 120.0.6099.109", "120.0.6099.109"},
		{"Brave Browser 1.62.156 Chromium: 121.0.6167.85", "1.62.156"},
		{"Microsoft Edge 120.0.2210.144", "120.0.2210.144"},
		{"no version here", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := ParseVersion(tt.input)
		if got != tt.want {
			t.Errorf("ParseVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDetectByType_NotFound(t *testing.T) {
	// This may or may not find a browser depending on the system.
	// We just verify no panic and correct error type if not found.
	_, err := DetectByType("edge")
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestBest(t *testing.T) {
	// May return error on CI with no browsers. Just verify no panic.
	info, err := Best()
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("unexpected error type: %v", err)
		}

		return
	}

	if info.Path == "" {
		t.Error("Best() returned empty path")
	}
}
