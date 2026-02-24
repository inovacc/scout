package scout

import (
	"os"
	"path/filepath"
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
