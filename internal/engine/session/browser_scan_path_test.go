package session

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsUnderDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")

	cases := []struct {
		name string
		p    string
		want bool
	}{
		{"direct child", filepath.Join(root, "abc"), true},
		{"nested grandchild", filepath.Join(root, "abc", "data"), true},
		{"root itself", root, true},
		{"sibling outside", filepath.Join(filepath.Dir(root), "other", "data"), false},
		{"unrelated abs", filepath.Join(t.TempDir(), "Chrome", "User Data"), false},
		{"empty path", "", false},
		{"dotdot escape", filepath.Join(root, "abc", "..", "..", "evil"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUnderDir(tc.p, root); got != tc.want {
				t.Fatalf("isUnderDir(%q, %q) = %v, want %v", tc.p, root, got, tc.want)
			}
		})
	}
}

func TestIsUnderDirCaseInsensitiveWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitivity floor only applies on windows")
	}

	root := filepath.Join(t.TempDir(), "Sessions")
	mixed := filepath.Join(filepath.Dir(root), "SESSIONS", "abc", "data")

	if !isUnderDir(mixed, root) {
		t.Fatalf("expected case-insensitive match on windows for %q under %q", mixed, root)
	}
}

func TestIsUnderSessions(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	inside := filepath.Join(dir, "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA", "data")
	if !isUnderSessions(inside) {
		t.Fatalf("isUnderSessions(%q) = false, want true", inside)
	}

	outside := filepath.Join(t.TempDir(), "Chrome", "User Data")
	if isUnderSessions(outside) {
		t.Fatalf("isUnderSessions(%q) = true, want false (real user chrome)", outside)
	}
}

func TestExtractUserDataDir(t *testing.T) {
	cases := []struct {
		name    string
		cmdline string
		want    string
	}{
		{"equals form", `chrome --user-data-dir=/tmp/s/abc/data --headless`, "/tmp/s/abc/data"},
		{"space form", `chrome --user-data-dir /tmp/s/abc/data --headless`, "/tmp/s/abc/data"},
		{"quoted equals", `chrome --user-data-dir="C:\Users\x\AppData\Local\Scout\sessions\abc\data"`, `C:\Users\x\AppData\Local\Scout\sessions\abc\data`},
		{"absent", `chrome --headless --remote-debugging-port=0`, ""},
		{"empty", ``, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractUserDataDir(tc.cmdline); got != tc.want {
				t.Fatalf("extractUserDataDir(%q) = %q, want %q", tc.cmdline, got, tc.want)
			}
		})
	}
}
