package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

func TestProfileShow_LegacySecretsCarryDeprecationNote(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.scoutprofile")
	if err := scout.SaveProfile(&scout.UserProfile{
		Version: 1, Name: "legacy",
		Cookies: []scout.Cookie{{Name: "sid", Value: "x", Domain: ".e.com", Path: "/"}},
	}, legacy); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	var out bytes.Buffer
	cmd := profileShowCmd
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.RunE(cmd, []string{legacy}); err != nil {
		t.Fatalf("profile show: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("vault")) {
		t.Errorf("legacy secret profile show should point users to the vault; got:\n%s", out.String())
	}
}

func TestProfileShow_NonSecretOmitsSecretSections(t *testing.T) {
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.scoutprofile")
	if err := scout.SaveProfile(&scout.UserProfile{
		Version: 1, Name: "clean",
		Identity: scout.ProfileIdentity{UserAgent: "UA/1"},
	}, clean); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	var out bytes.Buffer
	cmd := profileShowCmd
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.RunE(cmd, []string{clean}); err != nil {
		t.Fatalf("profile show: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("Cookies:")) {
		t.Errorf("non-secret profile must not print a Cookies section; got:\n%s", out.String())
	}
}
