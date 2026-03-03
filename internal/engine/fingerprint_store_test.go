package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintStore_SaveLoadDelete(t *testing.T) {
	dir := t.TempDir()

	store, err := NewFingerprintStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	fp := GenerateFingerprint(WithFingerprintOS("windows"))

	sf, err := store.Save(fp, "test")
	if err != nil {
		t.Fatal(err)
	}

	if sf.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	if sf.UseCount != 0 {
		t.Fatalf("expected use count 0, got %d", sf.UseCount)
	}

	if len(sf.Tags) != 1 || sf.Tags[0] != "test" {
		t.Fatalf("unexpected tags: %v", sf.Tags)
	}

	// Load.
	loaded, err := store.Load(sf.ID)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Fingerprint.Platform != "Win32" {
		t.Fatalf("expected Win32, got %s", loaded.Fingerprint.Platform)
	}

	// MarkUsed.
	if err := store.MarkUsed(sf.ID, "example.com"); err != nil {
		t.Fatal(err)
	}

	updated, _ := store.Load(sf.ID)
	if updated.UseCount != 1 {
		t.Fatalf("expected use count 1, got %d", updated.UseCount)
	}

	if len(updated.Domains) != 1 || updated.Domains[0] != "example.com" {
		t.Fatalf("unexpected domains: %v", updated.Domains)
	}

	// Delete.
	if err := store.Delete(sf.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Load(sf.ID); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestFingerprintStore_List(t *testing.T) {
	dir := t.TempDir()

	store, err := NewFingerprintStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if _, err := store.Generate(); err != nil {
			t.Fatal(err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 fingerprints, got %d", len(list))
	}
}

func TestFingerprintStore_DefaultDir(t *testing.T) {
	// Ensure the default store creation doesn't fail.
	store, err := NewFingerprintStore("")
	if err != nil {
		t.Fatal(err)
	}

	// Verify dir was created under home.
	home, _ := os.UserHomeDir()

	expected := filepath.Join(home, ".scout", "fingerprints")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected default dir %s to exist: %v", expected, err)
	}

	_ = store
}
