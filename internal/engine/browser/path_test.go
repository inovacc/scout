package browser

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "exists.txt")
	if err := os.WriteFile(tmp, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !FileExists(tmp) {
		t.Fatal("expected FileExists to return true for existing file")
	}

	if FileExists(filepath.Join(t.TempDir(), "nope")) {
		t.Fatal("expected FileExists to return false for missing file")
	}

	if FileExists(t.TempDir()) {
		t.Fatal("expected FileExists to return false for directory")
	}
}

func TestFirstExisting(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "browser.exe")
	if err := os.WriteFile(tmp, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := firstExisting([]string{"/nonexistent/path", tmp}, Brave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != tmp {
		t.Fatalf("expected %s, got %s", tmp, got)
	}

	_, err = firstExisting([]string{"/no/such/file"}, Edge)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLookupBrowserChrome(t *testing.T) {
	path, err := lookupBrowser(Chrome)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != "" {
		t.Fatalf("expected empty path for chrome, got %q", path)
	}
}

func TestLookupBrowserUnknown(t *testing.T) {
	_, err := lookupBrowser("firefox")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
