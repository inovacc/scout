//go:build windows

package session

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRmdirLowLevelWindowsReadOnly proves that rmdirLowLevel removes a dir
// tree that contains a read-only file — the exact case that defeats a plain
// os.RemoveAll when Windows AV or OneDrive has set FILE_ATTRIBUTE_READONLY.
func TestRmdirLowLevelWindowsReadOnly(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "sess-rmdir")

	// Build a small tree:
	//   sess-rmdir/
	//     sub/
	//       readonly.txt   (FILE_ATTRIBUTE_READONLY)
	//     normal.txt
	sub := filepath.Join(target, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}

	roFile := filepath.Join(sub, "readonly.txt")
	if err := os.WriteFile(roFile, []byte("locked"), 0o444); err != nil {
		t.Fatalf("write readonly.txt: %v", err)
	}

	normalFile := filepath.Join(target, "normal.txt")
	if err := os.WriteFile(normalFile, []byte("normal"), 0o600); err != nil {
		t.Fatalf("write normal.txt: %v", err)
	}

	// Confirm the read-only file is not removable with os.Remove directly.
	if err := os.Remove(roFile); err == nil {
		t.Skip("read-only attribute not honoured on this system; skipping test")
	}

	if err := rmdirLowLevel(target); err != nil {
		t.Fatalf("rmdirLowLevel returned error: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists after rmdirLowLevel; stat err=%v", err)
	}
}
