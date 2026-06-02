package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteCreatesFileWithMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.bin")

	if err := atomicWrite(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("atomicWrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("content = %q, want %q", got, "payload")
	}
	// No leftover temp files in the directory.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries, want 1 (leftover temp file?)", len(entries))
	}
}

func TestAtomicWriteReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.bin")
	if err := atomicWrite(path, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("v2-longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "v2-longer" {
		t.Fatalf("content = %q, want v2-longer", got)
	}
}
