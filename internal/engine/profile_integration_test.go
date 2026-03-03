package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveExtensions_Found(t *testing.T) {
	baseDir := t.TempDir()

	// Create fake extension directories.
	extID := "cjpalhdlnbpafiamejdnhcphjbkeiagm"

	extPath := filepath.Join(baseDir, extID)
	if err := os.MkdirAll(extPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	prof := &UserProfile{
		Version:    1,
		Name:       "test",
		Extensions: []string{extID},
	}

	resolved := ResolveExtensionsWithBase(prof, baseDir)

	if len(resolved) != 1 {
		t.Fatalf("resolved len = %d, want 1", len(resolved))
	}

	if resolved[0] != extPath {
		t.Errorf("resolved[0] = %q, want %q", resolved[0], extPath)
	}
}

func TestResolveExtensions_AbsolutePath(t *testing.T) {
	// Create a real directory to use as an absolute extension path.
	extDir := t.TempDir()

	prof := &UserProfile{
		Version:    1,
		Name:       "test",
		Extensions: []string{extDir},
	}

	resolved := ResolveExtensionsWithBase(prof, t.TempDir())

	if len(resolved) != 1 {
		t.Fatalf("resolved len = %d, want 1", len(resolved))
	}

	if resolved[0] != extDir {
		t.Errorf("resolved[0] = %q, want %q", resolved[0], extDir)
	}
}

func TestResolveExtensions_NotFound(t *testing.T) {
	baseDir := t.TempDir()

	prof := &UserProfile{
		Version:    1,
		Name:       "test",
		Extensions: []string{"nonexistent-extension-id"},
	}

	resolved := ResolveExtensionsWithBase(prof, baseDir)

	if len(resolved) != 0 {
		t.Errorf("resolved len = %d, want 0 (missing extension should be skipped)", len(resolved))
	}
}

func TestResolveExtensions_EmptyProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile *UserProfile
	}{
		{"nil profile", nil},
		{"empty extensions", &UserProfile{Version: 1, Name: "test"}},
		{"nil extensions", &UserProfile{Version: 1, Name: "test", Extensions: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := ResolveExtensionsWithBase(tt.profile, t.TempDir())
			if len(resolved) != 0 {
				t.Errorf("resolved len = %d, want 0", len(resolved))
			}
		})
	}
}

func TestResolveExtensions_MixedFoundAndMissing(t *testing.T) {
	baseDir := t.TempDir()

	// Create only one of two extensions.
	extID := "found-ext"
	if err := os.MkdirAll(filepath.Join(baseDir, extID), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	prof := &UserProfile{
		Version:    1,
		Name:       "test",
		Extensions: []string{extID, "missing-ext"},
	}

	resolved := ResolveExtensionsWithBase(prof, baseDir)

	if len(resolved) != 1 {
		t.Fatalf("resolved len = %d, want 1", len(resolved))
	}

	if resolved[0] != filepath.Join(baseDir, extID) {
		t.Errorf("resolved[0] = %q, want %q", resolved[0], filepath.Join(baseDir, extID))
	}
}

func TestProfileApplyWithExtensions(t *testing.T) {
	baseDir := t.TempDir()

	extID := "test-ext-abc"

	extPath := filepath.Join(baseDir, extID)
	if err := os.MkdirAll(extPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	prof := &UserProfile{
		Version: 1,
		Name:    "ext-apply-test",
		Identity: ProfileIdentity{
			UserAgent: "TestAgent/3.0",
		},
		Extensions: []string{extID},
	}

	// Use ResolveExtensionsWithBase to simulate what applyProfileToOptions does
	// but with a custom base dir (since we can't control ~/.scout/extensions/ in tests).
	resolved := ResolveExtensionsWithBase(prof, baseDir)

	// Manually apply to options to verify the resolved paths land correctly.
	o := defaults()
	o.userAgent = prof.Identity.UserAgent
	o.extensions = append(o.extensions, resolved...)
	o.profile = prof

	if o.userAgent != "TestAgent/3.0" {
		t.Errorf("userAgent = %q, want %q", o.userAgent, "TestAgent/3.0")
	}

	if len(o.extensions) != 1 {
		t.Fatalf("extensions len = %d, want 1", len(o.extensions))
	}

	if o.extensions[0] != extPath {
		t.Errorf("extensions[0] = %q, want %q", o.extensions[0], extPath)
	}
}

func TestProfileFullWorkflow(t *testing.T) {
	// Create a profile, save to temp file, load back, verify round-trip.
	now := time.Now().Truncate(time.Second)

	original := &UserProfile{
		Version:   1,
		Name:      "workflow-test",
		CreatedAt: now,
		UpdatedAt: now,
		Browser: ProfileBrowser{
			Type:     "chrome",
			WindowW:  1280,
			WindowH:  720,
			Platform: "linux",
			Arch:     "amd64",
		},
		Identity: ProfileIdentity{
			UserAgent: "WorkflowAgent/1.0",
			Language:  "en-US",
			Timezone:  "America/Chicago",
			Locale:    "en-US",
		},
		Cookies: []Cookie{
			{Name: "session", Value: "s123", Domain: ".example.com", Path: "/", Secure: true},
			{Name: "pref", Value: "dark", Domain: ".example.com", Path: "/"},
		},
		Storage: map[string]ProfileOriginStorage{
			"https://example.com": {
				LocalStorage:   map[string]string{"theme": "dark", "lang": "en"},
				SessionStorage: map[string]string{"cart": "[]"},
			},
		},
		Headers:    map[string]string{"X-Custom": "workflow", "Accept-Language": "en-US"},
		Extensions: []string{"ext-a", "ext-b"},
		Proxy:      "socks5://127.0.0.1:9050",
		Notes:      "full workflow test",
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.scoutprofile")

	// Save.
	if err := SaveProfile(original, path); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	// Load.
	loaded, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}

	// Verify all fields round-trip.
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Name", loaded.Name, original.Name},
		{"Browser.Type", loaded.Browser.Type, original.Browser.Type},
		{"Browser.Platform", loaded.Browser.Platform, original.Browser.Platform},
		{"Identity.UserAgent", loaded.Identity.UserAgent, original.Identity.UserAgent},
		{"Identity.Language", loaded.Identity.Language, original.Identity.Language},
		{"Identity.Timezone", loaded.Identity.Timezone, original.Identity.Timezone},
		{"Proxy", loaded.Proxy, original.Proxy},
		{"Notes", loaded.Notes, original.Notes},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}

	if loaded.Browser.WindowW != 1280 || loaded.Browser.WindowH != 720 {
		t.Errorf("Window = %dx%d, want 1280x720", loaded.Browser.WindowW, loaded.Browser.WindowH)
	}

	if len(loaded.Cookies) != 2 {
		t.Errorf("Cookies len = %d, want 2", len(loaded.Cookies))
	}

	if len(loaded.Extensions) != 2 {
		t.Errorf("Extensions len = %d, want 2", len(loaded.Extensions))
	}

	if len(loaded.Headers) != 2 {
		t.Errorf("Headers len = %d, want 2", len(loaded.Headers))
	}

	if loaded.Storage["https://example.com"].LocalStorage["theme"] != "dark" {
		t.Error("localStorage 'theme' not preserved")
	}

	if loaded.Storage["https://example.com"].SessionStorage["cart"] != "[]" {
		t.Error("sessionStorage 'cart' not preserved")
	}

	// Validate.
	if err := loaded.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestProfileFullWorkflowEncrypted(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	original := &UserProfile{
		Version:   1,
		Name:      "encrypted-workflow",
		CreatedAt: now,
		UpdatedAt: now,
		Identity:  ProfileIdentity{UserAgent: "EncAgent/1.0"},
		Cookies:   []Cookie{{Name: "tok", Value: "abc", Domain: ".test.com"}},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "enc.scoutprofile")
	passphrase := "test-p@ssphrase-123"

	if err := SaveProfileEncrypted(original, path, passphrase); err != nil {
		t.Fatalf("SaveProfileEncrypted: %v", err)
	}

	// Verify raw file is not valid JSON (encrypted).
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var dummy map[string]any
	if json.Unmarshal(raw, &dummy) == nil {
		t.Error("encrypted file should not be valid JSON")
	}

	// Load back.
	loaded, err := LoadProfileEncrypted(path, passphrase)
	if err != nil {
		t.Fatalf("LoadProfileEncrypted: %v", err)
	}

	if loaded.Name != "encrypted-workflow" {
		t.Errorf("Name = %q, want %q", loaded.Name, "encrypted-workflow")
	}

	if loaded.Identity.UserAgent != "EncAgent/1.0" {
		t.Errorf("UserAgent = %q, want %q", loaded.Identity.UserAgent, "EncAgent/1.0")
	}
}
