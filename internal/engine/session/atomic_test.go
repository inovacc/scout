package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteFileAtomicReplacesExisting verifies the atomic write replaces an
// existing file with new contents and leaves no .tmp residue.
func TestWriteFileAtomicReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scout.pid")

	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := writeFileAtomic(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// TestWriteFileAtomicPreservesOriginalOnFailure verifies that if the temp
// file cannot be created (e.g. parent dir unwritable), the original file is
// untouched.
func TestWriteFileAtomicPreservesOriginalOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scout.pid")

	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Point at a non-existent parent dir; CreateTemp should fail.
	bogus := filepath.Join(dir, "does", "not", "exist", "scout.pid")
	if err := writeFileAtomic(bogus, []byte("new"), 0o644); err == nil {
		t.Fatalf("writeFileAtomic to bogus path: want error, got nil")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(got) != "original" {
		t.Errorf("original mutated: contents = %q", got)
	}
}

// TestWriteInfoAtomic verifies that WriteInfo round-trips through the atomic
// write path and the result is readable by ReadInfo.
func TestWriteInfoAtomic(t *testing.T) {
	dir := t.TempDir()

	origSessionsDir := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = origSessionsDir })

	info := &SessionInfo{
		ScoutPID:   12345,
		BrowserPID: 67890,
		Reusable:   true,
		Browser:    "chrome",
		Headless:   true,
	}

	if err := WriteInfo("sess-1", info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	got, err := ReadInfo("sess-1")
	if err != nil {
		t.Fatalf("ReadInfo: %v", err)
	}

	if got.ScoutPID != info.ScoutPID || got.BrowserPID != info.BrowserPID ||
		got.Reusable != info.Reusable || got.Browser != info.Browser ||
		got.Headless != info.Headless {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, info)
	}

	// Verify no leftover .tmp files in session dir.
	entries, err := os.ReadDir(filepath.Join(dir, "sess-1"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file left in session dir: %s", e.Name())
		}
	}
}
