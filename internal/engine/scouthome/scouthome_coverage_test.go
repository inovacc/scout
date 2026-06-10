package scouthome

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setHomeEnv points every home-resolving env var at tmp and clears
// SCOUT_HOME so resolution falls through to platformDefault/legacy.
func setHomeEnv(t *testing.T, tmp string) {
	t.Helper()
	t.Setenv("SCOUT_HOME", "")
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", filepath.Join(tmp, "AppData", "Local"))
	case "linux", "freebsd", "openbsd":
		t.Setenv("XDG_DATA_HOME", "")
	}
}

// wantPlatformDefault mirrors platformDefault() so tests can assert the
// resolved path without duplicating the switch in every test body.
func wantPlatformDefault(t *testing.T, tmp string) string {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(tmp, "AppData", "Local", "Scout")
	case "darwin":
		return filepath.Join(tmp, "Library", "Application Support", "Scout")
	default:
		return filepath.Join(tmp, ".local", "share", "scout")
	}
}

func TestLegacyHasContent(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "missing dir returns false",
			setup: func(t *testing.T, dir string) { t.Helper() },
			want:  false,
		},
		{
			name: "empty dir returns false",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			},
			want: false,
		},
		{
			name: "dir with a file returns true",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, "marker"), []byte("x"), 0o600); err != nil {
					t.Fatalf("write: %v", err)
				}
			},
			want: true,
		},
		{
			name: "dir with a subdir returns true",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o700); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "legacy")
			tt.setup(t, dir)

			if got := legacyHasContent(dir); got != tt.want {
				t.Errorf("legacyHasContent(%q) = %v, want %v", dir, got, tt.want)
			}
		})
	}
}

func TestPlatformDefault(t *testing.T) {
	tmp := t.TempDir()
	setHomeEnv(t, tmp)

	got, err := platformDefault()
	if err != nil {
		t.Fatalf("platformDefault: %v", err)
	}

	want := wantPlatformDefault(t, tmp)
	if got != want {
		t.Errorf("platformDefault() = %q, want %q", got, want)
	}
}

func TestPlatformDefaultLinuxXDG(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skipf("XDG_DATA_HOME path only applies to linux/BSD; GOOS=%s", runtime.GOOS)
	}

	tmp := t.TempDir()
	xdg := filepath.Join(tmp, "xdg-data")
	t.Setenv("SCOUT_HOME", "")
	t.Setenv("XDG_DATA_HOME", xdg)

	got, err := platformDefault()
	if err != nil {
		t.Fatalf("platformDefault: %v", err)
	}

	want := filepath.Join(xdg, "scout")
	if got != want {
		t.Errorf("platformDefault() = %q, want %q (XDG override)", got, want)
	}
}

// TestPlatformDefaultErrorWindows exercises the error branch on Windows:
// when LOCALAPPDATA is unset there is no conventional path to resolve.
func TestPlatformDefaultErrorWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("LOCALAPPDATA-empty error path is Windows-only; GOOS=%s", runtime.GOOS)
	}

	t.Setenv("LOCALAPPDATA", "")

	got, err := platformDefault()
	if err == nil {
		t.Fatalf("platformDefault() = %q, want error when LOCALAPPDATA empty", got)
	}
	if !strings.Contains(err.Error(), "scout:") {
		t.Errorf("error %q missing scout: prefix", err.Error())
	}
}

func TestLegacyDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	got, err := legacyDir()
	if err != nil {
		t.Fatalf("legacyDir: %v", err)
	}

	want := filepath.Join(tmp, ".scout")
	if got != want {
		t.Errorf("legacyDir() = %q, want %q", got, want)
	}
}

// TestRootPrefersPlatformWhenBothExist proves the resolver does NOT fall
// back to legacy when the platform-default dir already exists, even if the
// legacy ~/.scout has content. This is the key branch at line 53-55.
func TestRootPrefersPlatformWhenBothExist(t *testing.T) {
	tmp := t.TempDir()
	setHomeEnv(t, tmp)

	// Legacy dir with content.
	legacy := filepath.Join(tmp, ".scout")
	if err := os.MkdirAll(filepath.Join(legacy, "sessions"), 0o700); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	// Platform default also already exists -> must win.
	preferred := wantPlatformDefault(t, tmp)
	if err := os.MkdirAll(preferred, 0o700); err != nil {
		t.Fatalf("setup preferred: %v", err)
	}

	got, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	if got != preferred {
		t.Errorf("Root() = %q, want preferred %q (platform default wins when it exists)", got, preferred)
	}
}

// TestRootEmptyLegacyIgnored proves an empty legacy dir does not trigger
// back-compat: resolution falls through to the platform default.
func TestRootEmptyLegacyIgnored(t *testing.T) {
	tmp := t.TempDir()
	setHomeEnv(t, tmp)

	// Empty legacy dir (no entries) -> ignored.
	if err := os.MkdirAll(filepath.Join(tmp, ".scout"), 0o700); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	got, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	want := wantPlatformDefault(t, tmp)
	if got != want {
		t.Errorf("Root() = %q, want platform default %q (empty legacy ignored)", got, want)
	}
}

// TestRootSCOUTHomeBeatsEverything proves SCOUT_HOME short-circuits before
// any legacy or platform logic runs, even when legacy content exists.
func TestRootSCOUTHomeBeatsEverything(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	if err := os.MkdirAll(filepath.Join(tmp, ".scout", "sessions"), 0o700); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	override := filepath.Join(tmp, "explicit-home")
	t.Setenv("SCOUT_HOME", override)

	got, err := Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}

	if got != override {
		t.Errorf("Root() = %q, want SCOUT_HOME override %q", got, override)
	}
}

func TestMustRoot(t *testing.T) {
	override := filepath.Join(t.TempDir(), "must-root")
	t.Setenv("SCOUT_HOME", override)

	got := MustRoot()
	if got != override {
		t.Errorf("MustRoot() = %q, want %q", got, override)
	}
}

func TestMustRootPanicsOnWindowsNoLocalAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("MustRoot panic path requires Windows LOCALAPPDATA-empty error; GOOS=%s", runtime.GOOS)
	}

	tmp := t.TempDir()
	t.Setenv("SCOUT_HOME", "")
	t.Setenv("LOCALAPPDATA", "")
	// No legacy content, so back-compat does not rescue resolution.
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("MustRoot() did not panic on unresolvable home")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "scout: cannot resolve home directory") {
			t.Errorf("panic value = %v, want scout home-resolution message", r)
		}
	}()

	_ = MustRoot()
}

func TestSub(t *testing.T) {
	override := filepath.Join(t.TempDir(), "sub-root")
	t.Setenv("SCOUT_HOME", override)

	tests := []struct {
		name   string
		subdir string
	}{
		{name: "sessions", subdir: "sessions"},
		{name: "nested", subdir: filepath.Join("plugins", "lock")},
		{name: "empty subdir joins to root", subdir: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sub(tt.subdir)
			if err != nil {
				t.Fatalf("Sub(%q): %v", tt.subdir, err)
			}

			want := filepath.Join(override, tt.subdir)
			if got != want {
				t.Errorf("Sub(%q) = %q, want %q", tt.subdir, got, want)
			}
		})
	}
}

func TestMustSub(t *testing.T) {
	override := filepath.Join(t.TempDir(), "mustsub-root")
	t.Setenv("SCOUT_HOME", override)

	got := MustSub("browsers")
	want := filepath.Join(override, "browsers")
	if got != want {
		t.Errorf("MustSub(browsers) = %q, want %q", got, want)
	}
}
