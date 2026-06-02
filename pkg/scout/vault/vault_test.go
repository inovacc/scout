package vault

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestVaultSetGetListRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	pass := []byte("vault-pass")

	v, err := Create(pass, WithPath(path))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	id, err := v.Set(SecretProfileInput{
		Name:    "openai",
		Secrets: map[string][]byte{"api_key": []byte("sk-live-1")},
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if id == "" {
		t.Fatal("Set returned empty id")
	}
	if err := v.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen from disk and verify persistence + metadata hygiene.
	v2, err := Open(pass, WithPath(path))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = v2.Close() }()

	metas := v2.List()
	if len(metas) != 1 || metas[0].ID != id || metas[0].Name != "openai" {
		t.Fatalf("List = %+v", metas)
	}
	if len(metas[0].SecretKeys) != 1 || metas[0].SecretKeys[0] != "api_key" {
		t.Fatalf("SecretKeys = %v", metas[0].SecretKeys)
	}

	p, err := v2.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !p.Secrets["api_key"].Equal([]byte("sk-live-1")) {
		t.Fatal("secret mismatch after reopen")
	}
	p.Close()

	if err := v2.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(v2.List()) != 0 {
		t.Fatal("profile not removed")
	}
}

func TestVaultSetUpdatesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	v, _ := Create([]byte("p"), WithPath(path))
	defer func() { _ = v.Close() }()

	id, _ := v.Set(SecretProfileInput{Name: "a", Secrets: map[string][]byte{"k": []byte("1")}})
	id2, err := v.Set(SecretProfileInput{ID: id, Name: "a2", Secrets: map[string][]byte{"k": []byte("2")}})
	if err != nil || id2 != id {
		t.Fatalf("update returned id=%q err=%v, want %q", id2, err, id)
	}
	if len(v.List()) != 1 {
		t.Fatal("update created a duplicate profile")
	}
	p, _ := v.Get(id)
	defer p.Close()
	if !p.Secrets["k"].Equal([]byte("2")) {
		t.Fatal("value not updated")
	}
}

func TestGetUnknownID(t *testing.T) {
	v, _ := Create([]byte("p"), WithPath(filepath.Join(t.TempDir(), "vault.bin")))
	defer func() { _ = v.Close() }()
	if _, err := v.Get("does-not-exist"); err == nil {
		t.Fatal("Get succeeded for unknown id")
	}
}

func TestVaultUseAfterCloseDoesNotCorruptDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	pass := []byte("real-pass")

	v, err := Create(pass, WithPath(path))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := v.Set(SecretProfileInput{Name: "a", Secrets: map[string][]byte{"k": []byte("v")}}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := v.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// All ops must now refuse with ErrVaultClosed and must NOT touch disk.
	if _, err := v.Set(SecretProfileInput{Name: "b"}); !errors.Is(err, ErrVaultClosed) {
		t.Fatalf("Set after Close = %v, want ErrVaultClosed", err)
	}
	if _, err := v.Get("anything"); !errors.Is(err, ErrVaultClosed) {
		t.Fatalf("Get after Close = %v, want ErrVaultClosed", err)
	}
	if err := v.Remove("anything"); !errors.Is(err, ErrVaultClosed) {
		t.Fatalf("Remove after Close = %v, want ErrVaultClosed", err)
	}
	if v.List() != nil {
		t.Fatal("List after Close should return nil")
	}
	// Second Close is a no-op.
	if err := v.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// The on-disk vault must STILL open with the real passphrase and retain data.
	v2, err := Open(pass, WithPath(path))
	if err != nil {
		t.Fatalf("Open after Close: %v (disk corrupted?)", err)
	}
	defer func() { _ = v2.Close() }()
	if len(v2.List()) != 1 {
		t.Fatalf("expected 1 profile after reopen, got %d", len(v2.List()))
	}
}
