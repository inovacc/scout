package vault

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	pass := []byte("store-pass")

	in := &vaultData{Version: 1, Profiles: []storedProfile{{
		ID:        "p1",
		Name:      "demo",
		Secrets:   map[string][]byte{"api_key": []byte("sk-abc")},
		Cookies:   []Cookie{{Name: "sid", Value: "v", Domain: "example.com"}},
		Headers:   map[string][]byte{"Authorization": []byte("Bearer t")},
		Storage:   map[string]OriginStore{"https://example.com": {LocalStorage: map[string]string{"k": "v"}}},
		CreatedAt: time.Unix(1, 0).UTC(),
		UpdatedAt: time.Unix(2, 0).UTC(),
	}}}

	if err := saveVault(path, in, pass); err != nil {
		t.Fatalf("saveVault: %v", err)
	}
	out, err := loadVault(path, pass)
	if err != nil {
		t.Fatalf("loadVault: %v", err)
	}
	if len(out.Profiles) != 1 || out.Profiles[0].ID != "p1" {
		t.Fatalf("profiles = %+v", out.Profiles)
	}
	if string(out.Profiles[0].Secrets["api_key"]) != "sk-abc" {
		t.Fatalf("secret = %q", out.Profiles[0].Secrets["api_key"])
	}
	if out.Profiles[0].Storage["https://example.com"].LocalStorage["k"] != "v" {
		t.Fatalf("storage round-trip failed: %+v", out.Profiles[0].Storage)
	}
}

func TestLoadVaultWrongPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	if err := saveVault(path, &vaultData{Version: 1}, []byte("right")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadVault(path, []byte("wrong")); err == nil {
		t.Fatal("loadVault succeeded with wrong passphrase")
	}
}

func TestLoadVaultMissingFile(t *testing.T) {
	_, err := loadVault(filepath.Join(t.TempDir(), "nope.bin"), []byte("p"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, ErrVaultNotFound) {
		t.Fatalf("want ErrVaultNotFound, got %v", err)
	}
}

func TestLoadVaultCorruptFileNotMistakenForMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	// Write garbage that exists but is not a valid encrypted vault blob.
	if err := os.WriteFile(path, []byte("not-a-valid-vault-blob"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadVault(path, []byte("p"))
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
	if errors.Is(err, ErrVaultNotFound) {
		t.Fatal("corrupt existing file must NOT report ErrVaultNotFound (would let Create clobber a real vault)")
	}
}
