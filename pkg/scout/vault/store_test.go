package vault

import (
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
}
