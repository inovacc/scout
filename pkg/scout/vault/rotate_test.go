package vault

import (
	"path/filepath"
	"testing"
)

func TestRotateRekeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	oldPass := []byte("old-pass")
	newPass := []byte("new-pass")

	v, _ := Create(oldPass, WithPath(path))
	id, _ := v.Set(SecretProfileInput{Name: "x", Secrets: map[string][]byte{"k": []byte("v")}})
	if err := v.Rotate(newPass); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	_ = v.Close()

	if _, err := Open(oldPass, WithPath(path)); err == nil {
		t.Fatal("old passphrase still opens the vault after rotation")
	}
	v2, err := Open(newPass, WithPath(path))
	if err != nil {
		t.Fatalf("Open with new passphrase: %v", err)
	}
	defer func() { _ = v2.Close() }()
	p, err := v2.Get(id)
	if err != nil {
		t.Fatalf("Get after rotate: %v", err)
	}
	defer p.Close()
	if !p.Secrets["k"].Equal([]byte("v")) {
		t.Fatal("secret lost across rotation")
	}
}

func TestRotateAfterCloseRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	v, _ := Create([]byte("p"), WithPath(path))
	_ = v.Close()
	if err := v.Rotate([]byte("new")); err == nil {
		t.Fatal("Rotate after Close should be refused")
	}
}
