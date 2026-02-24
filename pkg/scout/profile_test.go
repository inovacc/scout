package scout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveLoadProfile(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	p := &UserProfile{
		Version:   1,
		Name:      "test-profile",
		CreatedAt: now,
		UpdatedAt: now,
		Browser: ProfileBrowser{
			Type:     "chrome",
			WindowW:  1920,
			WindowH:  1080,
			Platform: "linux",
			Arch:     "amd64",
		},
		Identity: ProfileIdentity{
			UserAgent: "Mozilla/5.0 Test",
			Language:  "en-US",
			Timezone:  "America/New_York",
			Locale:    "en-US",
		},
		Cookies: []Cookie{
			{Name: "session", Value: "abc123", Domain: ".example.com", Secure: true},
		},
		Storage: map[string]ProfileOriginStorage{
			"https://example.com": {
				LocalStorage:   map[string]string{"key1": "val1"},
				SessionStorage: map[string]string{"key2": "val2"},
			},
		},
		Headers:    map[string]string{"X-Custom": "value"},
		Extensions: []string{"/path/to/ext"},
		Proxy:      "socks5://127.0.0.1:1080",
		Notes:      "test notes",
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.scoutprofile")

	if err := SaveProfile(p, path); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	loaded, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}

	if loaded.Name != "test-profile" {
		t.Errorf("Name = %q, want %q", loaded.Name, "test-profile")
	}

	if loaded.Browser.Type != "chrome" {
		t.Errorf("Browser.Type = %q, want %q", loaded.Browser.Type, "chrome")
	}

	if loaded.Identity.UserAgent != "Mozilla/5.0 Test" {
		t.Errorf("Identity.UserAgent = %q, want %q", loaded.Identity.UserAgent, "Mozilla/5.0 Test")
	}

	if loaded.Identity.Timezone != "America/New_York" {
		t.Errorf("Identity.Timezone = %q, want %q", loaded.Identity.Timezone, "America/New_York")
	}

	if len(loaded.Cookies) != 1 {
		t.Fatalf("Cookies len = %d, want 1", len(loaded.Cookies))
	}

	if loaded.Cookies[0].Name != "session" {
		t.Errorf("Cookie.Name = %q, want %q", loaded.Cookies[0].Name, "session")
	}

	if loaded.Storage["https://example.com"].LocalStorage["key1"] != "val1" {
		t.Error("LocalStorage key1 mismatch")
	}

	if loaded.Proxy != "socks5://127.0.0.1:1080" {
		t.Errorf("Proxy = %q, want %q", loaded.Proxy, "socks5://127.0.0.1:1080")
	}

	if loaded.Notes != "test notes" {
		t.Errorf("Notes = %q, want %q", loaded.Notes, "test notes")
	}

	// Verify file permissions (skip on Windows where 0600 is not enforced).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if perm := info.Mode().Perm(); perm&0o077 != 0 && perm != 0o666 {
		// On Unix, expect 0600. On Windows, permissions are different.
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestCaptureProfile_NilPage(t *testing.T) {
	_, err := CaptureProfile(nil)
	if err == nil {
		t.Fatal("expected error for nil page")
	}

	_, err = CaptureProfile(&Page{})
	if err == nil {
		t.Fatal("expected error for nil inner page")
	}
}

func TestProfileDefaults(t *testing.T) {
	before := time.Now()

	p := &UserProfile{
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if p.Version != 1 {
		t.Errorf("Version = %d, want 1", p.Version)
	}

	if p.CreatedAt.Before(before) {
		t.Error("CreatedAt should be >= test start time")
	}
}

func TestProfileWithName(t *testing.T) {
	cfg := &profileConfig{}
	WithProfileName("my-profile")(cfg)

	if cfg.name != "my-profile" {
		t.Errorf("name = %q, want %q", cfg.name, "my-profile")
	}
}

func TestLoadProfile_NotFound(t *testing.T) {
	_, err := LoadProfile("/nonexistent/path/profile.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadProfile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{invalid json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadProfile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveProfile_Nil(t *testing.T) {
	err := SaveProfile(nil, "/tmp/test.json")
	if err == nil {
		t.Fatal("expected error for nil profile")
	}
}

func TestOriginFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/path?q=1", "https://example.com"},
		{"http://localhost:8080/foo", "http://localhost:8080"},
		{"", ""},
		{"not-a-url", ""},
		{"about:blank", ""},
	}

	for _, tt := range tests {
		got := originFromURL(tt.input)
		if got != tt.want {
			t.Errorf("originFromURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWithProfileData(t *testing.T) {
	p := &UserProfile{
		Identity: ProfileIdentity{
			UserAgent: "TestAgent/1.0",
		},
		Proxy: "http://proxy:8080",
		Browser: ProfileBrowser{
			WindowW: 1280,
			WindowH: 720,
			Type:    "brave",
		},
		Extensions: []string{"/ext/one"},
	}

	o := defaults()
	WithProfileData(p)(o)

	if o.userAgent != "TestAgent/1.0" {
		t.Errorf("userAgent = %q, want %q", o.userAgent, "TestAgent/1.0")
	}

	if o.proxy != "http://proxy:8080" {
		t.Errorf("proxy = %q, want %q", o.proxy, "http://proxy:8080")
	}

	if o.windowW != 1280 || o.windowH != 720 {
		t.Errorf("window = %dx%d, want 1280x720", o.windowW, o.windowH)
	}

	if o.browserType != BrowserBrave {
		t.Errorf("browserType = %q, want %q", o.browserType, BrowserBrave)
	}

	if o.profile != p {
		t.Error("profile not stored on options")
	}
}

func TestWithProfileData_Nil(t *testing.T) {
	o := defaults()
	WithProfileData(nil)(o)

	if o.profile != nil {
		t.Error("expected nil profile on options")
	}
}

func TestApplyProfile_NilPage(t *testing.T) {
	p := &UserProfile{}

	err := (*Page)(nil).ApplyProfile(p)
	if err == nil {
		t.Fatal("expected error for nil page")
	}
}

func TestApplyProfile_NilProfile(t *testing.T) {
	page := &Page{page: nil}

	// nil inner page
	err := page.ApplyProfile(&UserProfile{})
	if err == nil {
		t.Fatal("expected error for nil inner page")
	}
}

func TestSaveLoadProfileEncrypted(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	p := &UserProfile{
		Version:   1,
		Name:      "encrypted-test",
		CreatedAt: now,
		UpdatedAt: now,
		Identity:  ProfileIdentity{UserAgent: "TestAgent/2.0"},
		Cookies:   []Cookie{{Name: "tok", Value: "xyz", Domain: ".example.com"}},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "enc.scoutprofile")

	if err := SaveProfileEncrypted(p, path, "s3cret"); err != nil {
		t.Fatalf("SaveProfileEncrypted: %v", err)
	}

	loaded, err := LoadProfileEncrypted(path, "s3cret")
	if err != nil {
		t.Fatalf("LoadProfileEncrypted: %v", err)
	}

	if loaded.Name != "encrypted-test" {
		t.Errorf("Name = %q, want %q", loaded.Name, "encrypted-test")
	}

	if loaded.Identity.UserAgent != "TestAgent/2.0" {
		t.Errorf("UserAgent = %q, want %q", loaded.Identity.UserAgent, "TestAgent/2.0")
	}

	if len(loaded.Cookies) != 1 || loaded.Cookies[0].Name != "tok" {
		t.Errorf("Cookies mismatch")
	}
}

func TestLoadProfileEncrypted_WrongPassphrase(t *testing.T) {
	p := &UserProfile{Version: 1, Name: "test", CreatedAt: time.Now(), UpdatedAt: time.Now()}

	dir := t.TempDir()
	path := filepath.Join(dir, "enc.scoutprofile")

	if err := SaveProfileEncrypted(p, path, "correct"); err != nil {
		t.Fatalf("SaveProfileEncrypted: %v", err)
	}

	_, err := LoadProfileEncrypted(path, "wrong")
	if err == nil {
		t.Fatal("expected error for wrong passphrase")
	}
}

func TestMergeProfiles(t *testing.T) {
	base := &UserProfile{
		Version:   1,
		Name:      "base",
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Identity:  ProfileIdentity{UserAgent: "BaseAgent", Language: "en-US"},
		Browser:   ProfileBrowser{Type: "chrome", WindowW: 1920, WindowH: 1080},
		Headers:   map[string]string{"X-Base": "1", "X-Shared": "base"},
		Extensions: []string{"/ext/a", "/ext/b"},
		Proxy:     "socks5://base:1080",
		Notes:     "base notes",
	}

	overlay := &UserProfile{
		Version:   1,
		Name:      "overlay",
		CreatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		Identity:  ProfileIdentity{UserAgent: "OverlayAgent"},
		Browser:   ProfileBrowser{Type: "brave"},
		Headers:   map[string]string{"X-Overlay": "2", "X-Shared": "overlay"},
		Extensions: []string{"/ext/b", "/ext/c"},
	}

	merged := MergeProfiles(base, overlay)

	// Overlay wins on scalar fields.
	if merged.Name != "overlay" {
		t.Errorf("Name = %q, want %q", merged.Name, "overlay")
	}
	if merged.Identity.UserAgent != "OverlayAgent" {
		t.Errorf("UserAgent = %q, want %q", merged.Identity.UserAgent, "OverlayAgent")
	}
	// Base kept when overlay empty.
	if merged.Identity.Language != "en-US" {
		t.Errorf("Language = %q, want %q", merged.Identity.Language, "en-US")
	}
	if merged.Browser.Type != "brave" {
		t.Errorf("Browser.Type = %q, want %q", merged.Browser.Type, "brave")
	}
	if merged.Proxy != "socks5://base:1080" {
		t.Errorf("Proxy = %q, want base proxy", merged.Proxy)
	}
	if merged.Notes != "base notes" {
		t.Errorf("Notes = %q, want base notes", merged.Notes)
	}

	// Headers merged, overlay wins on conflict.
	if merged.Headers["X-Base"] != "1" {
		t.Error("X-Base header lost")
	}
	if merged.Headers["X-Shared"] != "overlay" {
		t.Errorf("X-Shared = %q, want overlay", merged.Headers["X-Shared"])
	}
	if merged.Headers["X-Overlay"] != "2" {
		t.Error("X-Overlay header lost")
	}

	// Extensions union.
	if len(merged.Extensions) != 3 {
		t.Errorf("Extensions len = %d, want 3", len(merged.Extensions))
	}

	// Timestamps.
	if !merged.CreatedAt.Equal(base.CreatedAt) {
		t.Errorf("CreatedAt = %v, want earliest (base)", merged.CreatedAt)
	}
	if !merged.UpdatedAt.Equal(overlay.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want latest (overlay)", merged.UpdatedAt)
	}
}

func TestMergeProfiles_Cookies(t *testing.T) {
	base := &UserProfile{
		Version: 1,
		Cookies: []Cookie{
			{Name: "shared", Value: "base-val", Domain: ".example.com", Path: "/"},
			{Name: "base-only", Value: "b", Domain: ".example.com", Path: "/"},
		},
	}

	overlay := &UserProfile{
		Version: 1,
		Cookies: []Cookie{
			{Name: "shared", Value: "overlay-val", Domain: ".example.com", Path: "/"},
			{Name: "new", Value: "n", Domain: ".other.com", Path: "/"},
		},
	}

	merged := MergeProfiles(base, overlay)

	if len(merged.Cookies) != 3 {
		t.Fatalf("Cookies len = %d, want 3", len(merged.Cookies))
	}

	cookieByName := make(map[string]Cookie)
	for _, c := range merged.Cookies {
		cookieByName[c.Name] = c
	}

	if cookieByName["shared"].Value != "overlay-val" {
		t.Errorf("shared cookie = %q, want overlay-val", cookieByName["shared"].Value)
	}
	if cookieByName["base-only"].Value != "b" {
		t.Error("base-only cookie lost")
	}
	if cookieByName["new"].Value != "n" {
		t.Error("new cookie lost")
	}
}

func TestDiffProfiles(t *testing.T) {
	a := &UserProfile{
		Version:  1,
		Name:     "alpha",
		Identity: ProfileIdentity{UserAgent: "A"},
		Browser:  ProfileBrowser{Type: "chrome"},
		Cookies: []Cookie{
			{Name: "kept", Value: "v1", Domain: ".example.com", Path: "/"},
			{Name: "removed", Value: "r", Domain: ".example.com", Path: "/"},
			{Name: "modified", Value: "old", Domain: ".example.com", Path: "/"},
		},
		Storage:    map[string]ProfileOriginStorage{"https://a.com": {}},
		Headers:    map[string]string{"X-A": "1"},
		Extensions: []string{"/ext/a", "/ext/shared"},
	}

	b := &UserProfile{
		Version:  1,
		Name:     "beta",
		Identity: ProfileIdentity{UserAgent: "B"},
		Browser:  ProfileBrowser{Type: "brave"},
		Cookies: []Cookie{
			{Name: "kept", Value: "v1", Domain: ".example.com", Path: "/"},
			{Name: "added", Value: "a", Domain: ".other.com", Path: "/"},
			{Name: "modified", Value: "new", Domain: ".example.com", Path: "/"},
		},
		Storage:    map[string]ProfileOriginStorage{"https://b.com": {}},
		Headers:    map[string]string{"X-B": "2"},
		Extensions: []string{"/ext/b", "/ext/shared"},
	}

	d := DiffProfiles(a, b)

	if !d.NameChanged {
		t.Error("NameChanged should be true")
	}
	if !d.IdentityChanged {
		t.Error("IdentityChanged should be true")
	}
	if !d.BrowserChanged {
		t.Error("BrowserChanged should be true")
	}
	if d.CookiesAdded != 1 {
		t.Errorf("CookiesAdded = %d, want 1", d.CookiesAdded)
	}
	if d.CookiesRemoved != 1 {
		t.Errorf("CookiesRemoved = %d, want 1", d.CookiesRemoved)
	}
	if d.CookiesModified != 1 {
		t.Errorf("CookiesModified = %d, want 1", d.CookiesModified)
	}
	if d.StorageOriginsAdded != 1 {
		t.Errorf("StorageOriginsAdded = %d, want 1", d.StorageOriginsAdded)
	}
	if d.StorageOriginsRemoved != 1 {
		t.Errorf("StorageOriginsRemoved = %d, want 1", d.StorageOriginsRemoved)
	}
	if d.HeadersChanged != 2 {
		t.Errorf("HeadersChanged = %d, want 2", d.HeadersChanged)
	}
	if d.ExtensionsAdded != 1 {
		t.Errorf("ExtensionsAdded = %d, want 1", d.ExtensionsAdded)
	}
	if d.ExtensionsRemoved != 1 {
		t.Errorf("ExtensionsRemoved = %d, want 1", d.ExtensionsRemoved)
	}
}

func TestProfileValidate(t *testing.T) {
	p := &UserProfile{
		Version: 1,
		Name:    "valid",
		Cookies: []Cookie{{Name: "c", Domain: ".example.com"}},
		Storage: map[string]ProfileOriginStorage{
			"https://example.com": {},
		},
	}

	if err := p.Validate(); err != nil {
		t.Errorf("valid profile should pass: %v", err)
	}
}

func TestProfileValidate_Empty(t *testing.T) {
	p := &UserProfile{}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for empty profile")
	}

	// Should fail on version and name.
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention version: %v", err)
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention name: %v", err)
	}
}

func TestProfileValidate_BadCookieDomain(t *testing.T) {
	p := &UserProfile{Version: 1, Name: "test", Cookies: []Cookie{{Name: "c", Domain: ""}}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for cookie without domain")
	}
}

func TestProfileValidate_BadStorageOrigin(t *testing.T) {
	p := &UserProfile{
		Version: 1,
		Name:    "test",
		Storage: map[string]ProfileOriginStorage{"not-a-url": {}},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for invalid storage origin")
	}
}
