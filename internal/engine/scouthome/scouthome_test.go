package scouthome

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRootRespectsSCOUT_HOME(t *testing.T) {
	t.Setenv("SCOUT_HOME", `C:\test\scout-home`)

	got, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	if got != `C:\test\scout-home` {
		t.Errorf("Root() = %q, want SCOUT_HOME value", got)
	}
}

func TestRootPlatformDefault(t *testing.T) {
	// Force the platform default by clearing SCOUT_HOME and pointing HOME
	// to a directory that has no legacy ~/.scout sub-entry.
	tmp := t.TempDir()
	t.Setenv("SCOUT_HOME", "")
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))
	}

	if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "openbsd" {
		t.Setenv("XDG_DATA_HOME", "")
	}

	got, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	switch runtime.GOOS {
	case "windows":
		want := filepath.Join(tmp, "AppData", "Local", "Scout")
		if got != want {
			t.Errorf("Root() = %q, want %q", got, want)
		}
	case "darwin":
		want := filepath.Join(tmp, "Library", "Application Support", "Scout")
		if got != want {
			t.Errorf("Root() = %q, want %q", got, want)
		}
	default:
		want := filepath.Join(tmp, ".local", "share", "scout")
		if got != want {
			t.Errorf("Root() = %q, want %q", got, want)
		}
	}
}

func TestRootBackCompatLegacy(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SCOUT_HOME", "")
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	// Create a legacy ~/.scout dir WITH content (sessions subdir) to
	// trigger back-compat. Empty dirs are ignored.
	legacy := filepath.Join(tmp, ".scout")
	if err := os.MkdirAll(filepath.Join(legacy, "sessions"), 0o700); err != nil {
		t.Fatalf("setup legacy dir: %v", err)
	}

	if runtime.GOOS == "windows" {
		// Point LOCALAPPDATA at a path that does NOT exist yet so the
		// resolver prefers the legacy dir.
		t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))
	}

	got, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	if got != legacy {
		t.Errorf("Root() = %q, want legacy %q (back-compat)", got, legacy)
	}
}

func TestSubUsesRoot(t *testing.T) {
	t.Setenv("SCOUT_HOME", `C:\test\scout-home`)

	got, err := Sub("sessions")
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}

	want := filepath.Join(`C:\test\scout-home`, "sessions")
	if got != want {
		t.Errorf("Sub(sessions) = %q, want %q", got, want)
	}
}
