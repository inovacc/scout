package scout

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	// Existing file should return true.
	tmp := filepath.Join(t.TempDir(), "exists.txt")
	if err := os.WriteFile(tmp, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(tmp) {
		t.Fatal("expected fileExists to return true for existing file")
	}

	// Non-existing file should return false.
	if fileExists(filepath.Join(t.TempDir(), "nope")) {
		t.Fatal("expected fileExists to return false for missing file")
	}

	// Directory should return false.
	if fileExists(t.TempDir()) {
		t.Fatal("expected fileExists to return false for directory")
	}
}

func TestFirstExisting(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "browser.exe")
	if err := os.WriteFile(tmp, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := firstExisting([]string{"/nonexistent/path", tmp}, BrowserBrave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmp {
		t.Fatalf("expected %s, got %s", tmp, got)
	}

	_, err = firstExisting([]string{"/no/such/file"}, BrowserEdge)
	if !errors.Is(err, ErrBrowserNotFound) {
		t.Fatalf("expected ErrBrowserNotFound, got %v", err)
	}
}

func TestLookupBrowserChrome(t *testing.T) {
	// Chrome should always return empty string (rod auto-detect).
	path, err := lookupBrowser(BrowserChrome)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path for chrome, got %q", path)
	}
}

func TestLookupBrowserUnknown(t *testing.T) {
	_, err := lookupBrowser("firefox")
	if !errors.Is(err, ErrBrowserNotFound) {
		t.Fatalf("expected ErrBrowserNotFound, got %v", err)
	}
}

func TestWithBrowserOption(t *testing.T) {
	o := defaults()
	WithBrowser(BrowserEdge)(o)
	if o.browserType != BrowserEdge {
		t.Fatalf("expected browserType edge, got %q", o.browserType)
	}
}
