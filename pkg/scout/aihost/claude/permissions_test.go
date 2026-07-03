package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestPatchUnpatchPermissions runs against an isolated temp HOME so it never touches
// the real ~/.claude/settings.json. It proves the patch is additive + idempotent and
// the unpatch removes exactly scout's entries, preserving user-added ones.
func TestPatchUnpatchPermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)        // unix os.UserHomeDir
	t.Setenv("USERPROFILE", home) // windows os.UserHomeDir

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed a pre-existing user permission to prove we never clobber it.
	if err := os.WriteFile(settingsPath, []byte(`{"permissions":{"allow":["Bash(git status)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Patch: adds every scout entry, keeps the user's.
	if err := PatchPermissions(); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if present, total := PermissionsAllowCount(); present != total {
		t.Errorf("after patch: %d/%d present, want all", present, total)
	}
	allow := readAllow(t, settingsPath)
	if !allowContains(allow, "Bash(git status)") {
		t.Error("patch dropped the pre-existing user permission")
	}
	if !allowContains(allow, "mcp__scout__navigate") {
		t.Error("patch did not add mcp__scout__navigate")
	}

	// Idempotent: a second patch must not duplicate entries.
	if err := PatchPermissions(); err != nil {
		t.Fatal(err)
	}
	if allow2 := readAllow(t, settingsPath); len(allow2) != len(allow) {
		t.Errorf("patch not idempotent: %d -> %d entries", len(allow), len(allow2))
	}

	// Unpatch: removes exactly scout's entries, keeps the user's.
	if err := UnpatchPermissions(); err != nil {
		t.Fatalf("unpatch: %v", err)
	}
	allow3 := readAllow(t, settingsPath)
	if !allowContains(allow3, "Bash(git status)") {
		t.Error("unpatch dropped the user's permission")
	}
	if allowContains(allow3, "mcp__scout__navigate") {
		t.Error("unpatch left a scout permission behind")
	}
	if present, _ := PermissionsAllowCount(); present != 0 {
		t.Errorf("after unpatch: %d scout entries present, want 0", present)
	}
}

func readAllow(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Permissions.Allow
}

func allowContains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
