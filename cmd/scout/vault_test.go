package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/vault"
)

func TestVaultCLIInitSetListRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.bin")
	t.Setenv("SCOUT_VAULT_PASSPHRASE", "cli-pass")

	if _, err := vault.Create([]byte("cli-pass"), vault.WithPath(path)); err != nil {
		t.Fatalf("seed vault: %v", err)
	}
	v, err := vault.Open([]byte("cli-pass"), vault.WithPath(path))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	id, err := v.Set(vault.SecretProfileInput{Name: "svc", Secrets: map[string][]byte{"token": []byte("abc")}})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	_ = v.Close()

	out := renderVaultList([]vault.ProfileMeta{{ID: id, Name: "svc", SecretKeys: []string{"token"}}})
	if !strings.Contains(out, id) || !strings.Contains(out, "svc") || !strings.Contains(out, "token") {
		t.Fatalf("list output missing fields: %s", out)
	}
	if strings.Contains(out, "abc") {
		t.Fatalf("list output leaked secret value: %s", out)
	}
}

func TestParseSecretArgsHappyPath(t *testing.T) {
	got, err := parseSecretArgs([]string{"api_key=sk-1", "token=t2"})
	if err != nil {
		t.Fatalf("parseSecretArgs: %v", err)
	}
	if string(got["api_key"]) != "sk-1" || string(got["token"]) != "t2" {
		t.Fatalf("parsed = %v", got)
	}
}

func TestParseSecretArgsMalformedDoesNotLeakValue(t *testing.T) {
	// A user who forgets '=' would pass the secret as one token; the error must
	// NOT echo it.
	secret := "superSecretValue123"
	_, err := parseSecretArgs([]string{secret})
	if err == nil {
		t.Fatal("expected error for malformed arg")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked the secret value: %v", err)
	}
	if strings.Contains(err.Error(), "Secret") {
		t.Fatalf("error leaked part of the secret value: %v", err)
	}
}
